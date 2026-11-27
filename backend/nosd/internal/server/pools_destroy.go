package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

type destroyPlanReq struct {
	Force bool `json:"force"`
	Wipe  bool `json:"wipe"`
}

type destroyPlanResp struct {
	Steps    []pools.PlanStep `json:"steps"`
	Warnings []string         `json:"warnings"`
}

// resolvePoolMount finds a mounted pool by id, UUID, label or mountpoint.
func resolvePoolMount(ctx context.Context, id string) (string, bool) {
	list, _ := pools.ListPools(ctx)
	for _, p := range list {
		if p.Mount != "" && (id == p.ID || id == p.UUID || id == p.Label || id == p.Mount) {
			return p.Mount, true
		}
	}
	return "", false
}

func handlePlanDestroy(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mount, ok := resolvePoolMount(r.Context(), chi.URLParam(r, "id"))
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "no such pool")
			return
		}
		var req destroyPlanReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		warnings := []string{}
		if err := checkMountClean(mount); err != nil {
			if !req.Force {
				httpx.WriteError(w, http.StatusPreconditionFailed, `{"error":{"code":"destroy.not_clean","message":"`+err.Error()+`"}}`)
				return
			}
			warnings = append(warnings, err.Error())
		}
		ctx, cancel := context.WithTimeout(r.Context(), poolQuickCallTimeout)
		defer cancel()
		var res agentPoolResult
		if err := agentPoolOp(ctx, "/v1/pools/destroy", map[string]any{"mountpoint": mount, "wipe": req.Wipe, "dry_run": true}, &res); err != nil {
			writeAgentPoolError(w, err)
			return
		}
		steps := make([]pools.PlanStep, 0, len(res.Steps))
		for _, st := range res.Steps {
			steps = append(steps, pools.PlanStep{ID: st.ID, Description: describePoolStep(st.ID), Command: st.Command, Destructive: st.Destructive})
		}
		writeJSON(w, destroyPlanResp{Steps: steps, Warnings: warnings})
	}
}

func checkMountClean(mount string) error {
	fi, err := os.Stat(mount)
	if err != nil || !fi.IsDir() {
		return errors.New("mount not accessible")
	}
	ents, err := os.ReadDir(mount)
	if err != nil {
		return errors.New("cannot read mount")
	}
	allowed := map[string]struct{}{"data": {}, "snaps": {}, "apps": {}}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") { // ignore dotfiles
			continue
		}
		if _, ok := allowed[name]; !ok {
			return errors.New("unexpected entry: " + name)
		}
		// data and apps hold shares and app data
		if name != "snaps" {
			if sub, err := os.ReadDir(filepath.Join(mount, name)); err != nil || len(sub) > 0 {
				return errors.New(name + " is not empty")
			}
		}
	}
	return nil
}

func handleApplyDestroy(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Confirm string `json:"confirm"`
			Force   bool   `json:"force"`
			Wipe    bool   `json:"wipe"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToUpper(strings.TrimSpace(body.Confirm)) != "DESTROY" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm=DESTROY required")
			return
		}
		mount, ok := resolvePoolMount(r.Context(), chi.URLParam(r, "id"))
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "no such pool")
			return
		}
		if err := checkMountClean(mount); err != nil && !body.Force {
			httpx.WriteError(w, http.StatusPreconditionFailed, `{"error":{"code":"destroy.not_clean","message":"`+err.Error()+`"}}`)
			return
		}
		tx := pools.Tx{ID: generateUUID(), StartedAt: time.Now().UTC()}
		if !tryAcquirePoolLock(mount, tx.ID) {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+currentPoolTx(mount)+`"}}`)
			return
		}
		defer releasePoolLock(mount)
		ctx, cancel := context.WithTimeout(r.Context(), poolCreateTimeout)
		defer cancel()
		var res agentPoolResult
		err := agentPoolOp(ctx, "/v1/pools/destroy", map[string]any{"mountpoint": mount, "wipe": body.Wipe}, &res)
		for _, st := range res.Steps {
			tx.Steps = append(tx.Steps, pools.TxStep{ID: st.ID, Name: describePoolStep(st.ID), Cmd: st.Command, Destructive: st.Destructive, Status: st.Status, Err: firstLine(st.Output, st.Status == "error")})
		}
		now := time.Now().UTC()
		tx.FinishedAt = &now
		if err == nil && !res.OK {
			err = errors.New(res.Error)
		}
		if err != nil {
			tx.Error = err.Error()
			_ = saveTx(tx)
			writeAgentPoolError(w, err)
			return
		}
		tx.OK = true
		_ = saveTx(tx)
		// forget the pool's options
		_ = updatePoolOptions(cfg, func(st *poolOptionsStore) {
			out := st.Records[:0]
			for _, rec := range st.Records {
				if rec.Mount != mount {
					out = append(out, rec)
				}
			}
			st.Records = out
		})
		writeJSON(w, map[string]any{"ok": true, "tx_id": tx.ID, "devices": res.Devices})
	}
}
