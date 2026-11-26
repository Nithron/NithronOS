package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/fsatomic"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"

	"github.com/rs/zerolog/log"
)

type applyCreateRequest struct {
	Plan    pools.CreatePlan `json:"plan"`
	Fstab   []string         `json:"fstab"`
	Confirm string           `json:"confirm"`
}

func handleApplyCreate(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req applyCreateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.ToUpper(strings.TrimSpace(req.Confirm)) != "CREATE" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm=CREATE required")
			return
		}
		if len(req.Plan.Steps) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "empty plan")
			return
		}
		// Busy check: use a stable create key
		poolID := "create"
		if cur := currentPoolTx(poolID); cur != "" {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+cur+`"}}`)
			return
		}
		// Create transaction and save initial state
		tx := pools.Tx{ID: generateUUID(), StartedAt: time.Now().UTC()}
		for _, st := range req.Plan.Steps {
			tx.Steps = append(tx.Steps, pools.TxStep{ID: st.ID, Name: st.Description, Cmd: st.Command, Destructive: st.Destructive, Status: "pending"})
		}
		_ = saveTx(tx)
		if !tryAcquirePoolLock(poolID, tx.ID) {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+currentPoolTx(poolID)+`"}}`)
			return
		}
		// Execute asynchronously
		txWorkers.Add(1)
		go func() {
			defer txWorkers.Done()
			executePlan(tx.ID, req, cfg)
		}()
		writeJSON(w, map[string]any{"ok": true, "tx_id": tx.ID})
	}
}

// agentStepRunner can be overridden in tests to avoid calling the real agent.
var agentStepRunner = func(cmd string, args []string) (code int, stdout string) {
	client := agentclient.New("/run/nos-agent.sock")
	var resp struct {
		Results []struct {
			Code           int
			Stdout, Stderr string
		}
	}
	ctx, cancel := poolCallCtx(poolLongStepTimeout)
	_ = client.PostJSON(ctx, "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": cmd, "args": args}}}, &resp)
	cancel()
	if len(resp.Results) == 0 {
		return -1, ""
	}
	return resp.Results[0].Code, resp.Results[0].Stdout
}

func executePlan(txID string, req applyCreateRequest, cfg config.Config) {
	// load current tx
	var tx pools.Tx
	_, _ = fsatomic.LoadJSON(txPath(txID), &tx)
	for i, st := range tx.Steps {
		now := time.Now().UTC()
		tx.Steps[i].Status = "running"
		tx.Steps[i].StartedAt = &now
		_ = saveTx(tx)
		appendTxLog(tx.ID, "info", st.ID, "starting")
		parts := strings.Fields(st.Cmd)
		code, out := agentStepRunner(parts[0], parts[1:])
		if code != 0 {
			tx.OK = false
			tx.Error = fmt.Sprintf("step %s failed", st.ID)
			done := time.Now().UTC()
			tx.Steps[i].Status = "error"
			tx.Steps[i].FinishedAt = &done
			_ = saveTx(tx)
			appendTxLog(tx.ID, "error", st.ID, tx.Error)
			// rollback fstab edits if any
			client := agentclient.New("/run/nos-agent.sock")
			for _, ln := range req.Fstab {
				ctx, cancel := poolCallCtx(poolQuickCallTimeout)
				_ = client.PostJSON(ctx, "/v1/fstab/remove", map[string]any{"contains": ln}, nil)
				cancel()
			}
			// best-effort close any opened luks mappings implied by plan so far
			for _, step := range tx.Steps {
				if strings.HasPrefix(step.ID, "luks-open-") {
					p := strings.Fields(step.Cmd)
					if len(p) >= 6 && p[0] == "cryptsetup" && p[1] == "open" {
						name := p[len(p)-1]
						ctx, cancel := poolCallCtx(poolQuickCallTimeout)
						_ = client.PostJSON(ctx, "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": "cryptsetup", "args": []string{"close", name}}}}, nil)
						cancel()
					}
				}
			}
			return
		}
		done := time.Now().UTC()
		tx.Steps[i].Status = "ok"
		tx.Steps[i].FinishedAt = &done
		_ = saveTx(tx)
		appendTxLog(tx.ID, "info", st.ID, strings.TrimSpace(out))
	}
	// Ensure fstab lines
	client := agentclient.New("/run/nos-agent.sock")
	for _, ln := range req.Fstab {
		ctx, cancel := poolCallCtx(poolQuickCallTimeout)
		_ = client.PostJSON(ctx, "/v1/fstab/ensure", map[string]any{"line": ln}, nil)
		cancel()
	}
	// mark success
	now := time.Now().UTC()
	tx.OK = true
	tx.FinishedAt = &now
	_ = saveTx(tx)
	// Remember the pool: its mount point, UUID and options come from the
	// fstab line the plan wrote.
	for _, ln := range req.Fstab {
		if strings.HasPrefix(ln, "[crypttab]") {
			continue
		}
		parts := strings.Fields(ln)
		if len(parts) < 4 || parts[2] != "btrfs" {
			continue
		}
		rec := poolOptionsRecord{Mount: parts[1], MountOptions: parts[3], Name: filepath.Base(parts[1])}
		if uuid, ok := strings.CutPrefix(parts[0], "UUID="); ok {
			rec.UUID = uuid
		}
		if err := recordPool(cfg, rec); err != nil {
			log.Error().Err(err).Str("mount", rec.Mount).Msg("pools: could not record the new pool")
		}
		break
	}
}
