package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/fsatomic"
	"nithronos/backend/nosd/internal/pools"
	"nithronos/backend/nosd/pkg/agentclient"
	"nithronos/backend/nosd/pkg/httpx"
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
	_ = client.PostJSON(context.TODO(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": cmd, "args": args}}}, &resp)
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
				_ = client.PostJSON(context.TODO(), "/v1/fstab/remove", map[string]any{"contains": ln}, nil)
			}
			// best-effort close any opened luks mappings implied by plan so far
			for _, step := range tx.Steps {
				if strings.HasPrefix(step.ID, "luks-open-") {
					p := strings.Fields(step.Cmd)
					if len(p) >= 6 && p[0] == "cryptsetup" && p[1] == "open" {
						name := p[len(p)-1]
						_ = client.PostJSON(context.TODO(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": "cryptsetup", "args": []string{"close", name}}}}, nil)
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
		_ = client.PostJSON(context.TODO(), "/v1/fstab/ensure", map[string]any{"line": ln}, nil)
	}
	// mark success
	now := time.Now().UTC()
	tx.OK = true
	tx.FinishedAt = &now
	_ = saveTx(tx)
	// Persist pool record (best-effort minimal)
	type PoolRecord struct {
		Name, Mount  string
		Devices      []string
		MountOptions string
		CreatedAt    time.Time
	}
	// derive mount options from fstab
	mountOpts := ""
	for _, ln := range req.Fstab {
		if strings.HasPrefix(ln, "[crypttab]") {
			continue
		}
		parts := strings.Fields(ln)
		if len(parts) >= 4 && parts[2] == "btrfs" {
			mountOpts = parts[3]
			break
		}
	}
	rec := PoolRecord{Name: "pool", Mount: "", Devices: nil, MountOptions: mountOpts, CreatedAt: time.Now().UTC()}
	_ = fsatomic.WithLock(filepath.Join(cfg.EtcDir, "nos", "pools.json"), func() error {
		var list []PoolRecord
		_, _ = fsatomic.LoadJSON(filepath.Join(cfg.EtcDir, "nos", "pools.json"), &list)
		list = append(list, rec)
		return fsatomic.SaveJSON(context.TODO(), filepath.Join(cfg.EtcDir, "nos", "pools.json"), list, 0o600)
	})
}
