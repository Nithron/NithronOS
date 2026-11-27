package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/httpx"

	"github.com/rs/zerolog/log"
)

type applyCreateRequest struct {
	planCreateRequest
	Confirm string `json:"confirm"`
}

func handleApplyCreate(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req applyCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if strings.ToUpper(strings.TrimSpace(req.Confirm)) != "CREATE" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm=CREATE required")
			return
		}
		body, spec, opts, err := req.agentCreateBody(r.Context(), false)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		// One create at a time.
		poolID := "create"
		tx := pools.Tx{ID: generateUUID(), StartedAt: time.Now().UTC(),
			Steps: []pools.TxStep{{ID: "create", Name: "create pool " + spec.Name, Destructive: true, Status: "pending"}}}
		if !tryAcquirePoolLock(poolID, tx.ID) {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+currentPoolTx(poolID)+`"}}`)
			return
		}
		_ = saveTx(tx)
		txWorkers.Add(1)
		go func() {
			defer txWorkers.Done()
			defer releasePoolLock(poolID)
			runPoolCreate(cfg, tx, spec, opts, body)
		}()
		writeJSON(w, map[string]any{"ok": true, "tx_id": tx.ID})
	}
}

// runPoolCreate has the agent create the pool, records its steps in the
// transaction and, once it worked, remembers the pool.
func runPoolCreate(cfg config.Config, tx pools.Tx, spec pools.PoolSpec, opts string, body map[string]any) {
	started := time.Now().UTC()
	tx.Steps[0].Status = "running"
	tx.Steps[0].StartedAt = &started
	_ = saveTx(tx)
	appendTxLog(tx.ID, "info", "create", "creating pool "+spec.Name+" at "+spec.Mountpoint)

	ctx, cancel := context.WithTimeout(context.Background(), poolCreateTimeout)
	defer cancel()
	var res agentPoolResult
	err := agentPoolOp(ctx, "/v1/pools/create", body, &res)
	if err == nil && !res.OK {
		err = errors.New(res.Error)
	}
	steps := make([]pools.TxStep, 0, len(res.Steps))
	for _, st := range res.Steps {
		status := st.Status
		if status == "planned" {
			status = "pending"
		}
		steps = append(steps, pools.TxStep{ID: st.ID, Name: describePoolStep(st.ID), Cmd: st.Command, Destructive: st.Destructive, Status: status, Err: firstLine(st.Output, status == "error")})
		level := "info"
		if status == "error" {
			level = "error"
		}
		appendTxLog(tx.ID, level, st.ID, strings.TrimSpace(st.Command+"\n"+st.Output))
	}
	if len(steps) > 0 {
		tx.Steps = steps
	}
	done := time.Now().UTC()
	tx.FinishedAt = &done
	if err != nil {
		tx.OK = false
		tx.Error = err.Error()
		if len(res.Steps) == 0 {
			tx.Steps[0].Status = "error"
			tx.Steps[0].Err = err.Error()
		}
		_ = saveTx(tx)
		appendTxLog(tx.ID, "error", "create", tx.Error)
		log.Error().Err(err).Str("pool", spec.Name).Msg("pools: create failed")
		return
	}
	tx.OK = true
	_ = saveTx(tx)
	rec := poolOptionsRecord{Name: spec.Name, Mount: spec.Mountpoint, UUID: res.UUID, MountOptions: opts, Devices: spec.Devices, CreatedAt: done}
	if err := recordPool(cfg, rec); err != nil {
		log.Error().Err(err).Str("mount", rec.Mount).Msg("pools: could not record the new pool")
	}
	appendTxLog(tx.ID, "info", "create", "pool "+spec.Name+" created ("+res.UUID+")")
}

func firstLine(s string, keep bool) string {
	if !keep {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
