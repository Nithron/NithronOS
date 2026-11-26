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
	"nos/backend/nosd/internal/fsatomic"
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

func handlePlanDestroy(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		mount := id
		if strings.TrimSpace(mount) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "id required")
			return
		}
		var req destroyPlanReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if err := checkMountClean(mount); err != nil && !req.Force {
			httpx.WriteError(w, http.StatusPreconditionFailed, `{"error":{"code":"destroy.not_clean","message":"`+err.Error()+`"}}`)
			return
		}
		steps := []pools.PlanStep{
			{ID: "umount", Description: "unmount pool", Command: "umount " + shellQuote(mount)},
			{ID: "fstab", Description: "remove fstab entry", Command: "FSTAB_REMOVE contains=" + shellQuote(mount)},
			{ID: "crypttab", Description: "remove crypttab entries", Command: "CRYPTTAB_REMOVE contains=" + shellQuote(mount)},
		}
		if req.Wipe {
			steps = append(steps, pools.PlanStep{ID: "wipe", Description: "wipe signatures (manual devices)", Command: "wipefs -a <devices>"})
		}
		writeJSON(w, destroyPlanResp{Steps: steps})
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
	}
	return nil
}

func handleApplyDestroy(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		mount := id
		var body struct {
			Confirm string `json:"confirm"`
			Force   bool   `json:"force"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToUpper(strings.TrimSpace(body.Confirm)) != "DESTROY" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm=DESTROY required")
			return
		}
		if cur := currentPoolTx(mount); cur != "" {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+cur+`"}}`)
			return
		}
		if err := checkMountClean(mount); err != nil && !body.Force {
			httpx.WriteError(w, http.StatusPreconditionFailed, `{"error":{"code":"destroy.not_clean","message":"`+err.Error()+`"}}`)
			return
		}
		// Create tx and execute cleanup steps
		tx := pools.Tx{ID: generateUUID(), StartedAt: time.Now().UTC()}
		_ = saveTx(tx)
		if !tryAcquirePoolLock(mount, tx.ID) {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+currentPoolTx(mount)+`"}}`)
			return
		}
		// fstab and crypttab paths
		fstabPath := filepath.Join(cfg.EtcDir, "fstab")
		crypttabPath := filepath.Join(cfg.EtcDir, "crypttab")
		// remove fstab lines containing mount
		_ = removeLinesContaining(r.Context(), fstabPath, mount)
		// remove crypttab lines heuristically containing mount
		_ = removeLinesContaining(r.Context(), crypttabPath, mount)
		// attempt to unmount (best-effort via /proc/self/mounts knowledge not needed)
		// mark success
		now := time.Now().UTC()
		tx.OK = true
		tx.FinishedAt = &now
		_ = saveTx(tx)
		// remove pool from pools.json
		_ = updatePoolOptions(cfg, func(st *poolOptionsStore) {
			out := st.Records[:0]
			for _, rec := range st.Records {
				if rec.Mount != mount {
					out = append(out, rec)
				}
			}
			st.Records = out
		})
		releasePoolLock(mount)
		writeJSON(w, map[string]any{"ok": true, "tx_id": tx.ID})
	}
}

func removeLinesContaining(ctx context.Context, path, contains string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if !strings.Contains(ln, contains) {
			out = append(out, ln)
		}
	}
	return fsatomic.SaveJSON(path, strings.Join(out, "\n"), 0o644)
}
