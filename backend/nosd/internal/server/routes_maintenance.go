package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"nithronos/backend/nosd/internal/pools"
	"nithronos/backend/nosd/pkg/agentclient"

	// "nithronos/backend/nosd/pkg/firewall"
	"nithronos/backend/nosd/pkg/httpx"
	poolroots "nithronos/backend/nosd/pkg/pools"

	"nithronos/backend/nosd/pkg/snapdb"
)

// registerMaintenance serves the support bundle, snapshots, updates and rollback.
func (d *routeDeps) registerMaintenance(pr chi.Router) {
	cfg := d.cfg
	adminRequired := d.adminRequired
	// Support bundle
	pr.Get("/api/v1/support/bundle", handleSupportBundle(cfg))

	// Firewall legacy routes removed; use /api/v1/net/firewall/*

	// Snapshots
	pr.Get("/api/v1/pools/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		list, _ := pools.ListSnapshots(r.Context(), id)
		writeJSON(w, list)
	})

	// Updates: check (redundant with /api/v1/updates/* handler, but retain convenience)
	pr.Get("/api/v1/updates/check", func(w http.ResponseWriter, r *http.Request) {
		client := agentclient.New("/run/nos-agent.sock")
		var planResp map[string]any
		_ = client.PostJSON(r.Context(), "/v1/updates/plan", map[string]any{}, &planResp)
		// attach snapshot targets (best-effort)
		roots, _ := poolroots.AllowedRoots()
		writeJSON(w, map[string]any{"plan": planResp, "snapshot_roots": roots})
	})

	// Updates: apply
	pr.With(adminRequired).Post("/api/v1/updates/apply", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Packages []string `json:"packages"`
			Snapshot bool     `json:"snapshot"`
			Confirm  string   `json:"confirm"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToLower(body.Confirm) != "yes" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm\u003dyes required")
			return
		}
		client := agentclient.New("/run/nos-agent.sock")
		// create tx and persist initial state
		txID := generateUUID()
		tx := snapdb.UpdateTx{TxID: txID, StartedAt: time.Now().UTC(), Packages: body.Packages, Reason: "pre-update"}
		_ = snapdb.Append(tx)
		// load snapshot targets via pools roots (simplified: allowed roots)
		roots, _ := poolroots.AllowedRoots()
		if body.Snapshot {
			for _, p := range roots {
				var sresp struct {
					OK                 bool `json:"ok"`
					ID, Type, Location string
				}
				if err := client.PostJSON(r.Context(), "/v1/snapshot/create", map[string]any{"path": p, "mode": "auto", "reason": "pre-update"}, &sresp); err != nil {
					mark := false
					now := time.Now().UTC()
					tx.FinishedAt = &now
					tx.Success = &mark
					tx.Notes = "snapshot failed: " + errString(err)
					_ = snapdb.Append(tx)
					httpx.WriteError(w, http.StatusInternalServerError, "snapshot failed")
					return
				}
				// append target on success
				tx.Targets = append(tx.Targets, snapdb.SnapshotTarget{ID: sresp.ID, Path: p, Type: sresp.Type, Location: sresp.Location, CreatedAt: time.Now().UTC()})
			}
		}
		// perform updates apply on agent
		var applyResp map[string]any
		if err := client.PostJSON(r.Context(), "/v1/updates/apply", map[string]any{"packages": body.Packages}, &applyResp); err != nil {
			mark := false
			now := time.Now().UTC()
			tx.FinishedAt = &now
			tx.Success = &mark
			tx.Notes = "apply failed: " + errString(err)
			_ = snapdb.Append(tx)
			httpx.WriteError(w, http.StatusInternalServerError, "updates apply failed")
			return
		}
		// success
		mark := true
		now := time.Now().UTC()
		tx.FinishedAt = &now
		tx.Success = &mark
		_ = snapdb.Append(tx)
		writeJSON(w, map[string]any{"ok": true, "tx_id": txID, "snapshots_count": len(tx.Targets), "updates_count": len(applyResp)})
	})

	// Snapshots: prune
	pr.With(adminRequired).Post("/api/v1/snapshots/prune", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			KeepPerTarget int `json:"keep_per_target"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.KeepPerTarget <= 0 {
			body.KeepPerTarget = 5
		}
		client := agentclient.New("/run/nos-agent.sock")
		var resp map[string]any
		if err := client.PostJSON(r.Context(), "/v1/snapshot/prune", map[string]any{"keep_per_target": body.KeepPerTarget}, &resp); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, resp)
	})

	// Updates: rollback
	pr.With(adminRequired).Post("/api/v1/updates/rollback", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			TxID    string `json:"tx_id"`
			Confirm string `json:"confirm"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToLower(body.Confirm) != "yes" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm\u003dyes required")
			return
		}
		orig, err := snapdb.FindByTx(body.TxID)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "tx not found")
			return
		}
		client := agentclient.New("/run/nos-agent.sock")
		// start rollback tx record
		roll := snapdb.UpdateTx{TxID: generateUUID(), StartedAt: time.Now().UTC(), Packages: orig.Packages, Reason: "rollback"}
		for _, t := range orig.Targets {
			var resp map[string]any
			if err := client.PostJSON(r.Context(), "/v1/snapshot/rollback", map[string]any{
				"path": t.Path, "snapshot_id": t.ID, "type": t.Type,
			}, &resp); err != nil {
				mark := false
				now := time.Now().UTC()
				roll.FinishedAt = &now
				roll.Success = &mark
				roll.Notes = "rollback failed for target " + t.Path + ": " + err.Error()
				_ = snapdb.Append(roll)
				httpx.WriteError(w, http.StatusInternalServerError, "rollback failed")
				return
			}
		}
		// persist rollback record and adjust ordering by writing a small "touch"
		now := time.Now().UTC()
		roll.FinishedAt = &now
		okMark := true
		roll.Success = &okMark
		roll.Notes = "rollback of " + orig.TxID
		_ = snapdb.Append(roll)
		writeJSON(w, map[string]any{"ok": true})
	})

	// Snapshots DB: recent
	pr.Get("/api/v1/snapshots/recent", func(w http.ResponseWriter, r *http.Request) {
		list, err := snapdb.ListRecent(20)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// project limited fields
		out := make([]map[string]any, 0, len(list))
		for _, tx := range list {
			miniTargets := make([]map[string]any, 0, len(tx.Targets))
			for _, t := range tx.Targets {
				miniTargets = append(miniTargets, map[string]any{
					"id": t.ID, "type": t.Type, "location": t.Location,
				})
			}
			ok := false
			if tx.Success != nil {
				ok = *tx.Success
			}
			out = append(out, map[string]any{
				"tx_id":    tx.TxID,
				"time":     tx.StartedAt,
				"packages": tx.Packages,
				"targets":  miniTargets,
				"success":  ok,
			})
		}
		writeJSON(w, out)
	})

	// Back-compat: verify-totp path expected by FE
	pr.Post("/api/v1/auth/verify-totp", func(w http.ResponseWriter, r *http.Request) {
		// Delegate to /api/v1/auth/totp/verify handler
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/api/v1/auth/totp/verify"
		pr.ServeHTTP(w, r2)
	})

	// Snapshots DB: by tx id
	pr.Get("/api/v1/snapshots/{tx_id}", func(w http.ResponseWriter, r *http.Request) {
		txID := chi.URLParam(r, "tx_id")
		tx, err := snapdb.FindByTx(txID)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "tx not found")
			return
		}
		writeJSON(w, tx)
	})

	pr.With(adminRequired).Post("/api/v1/pools/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			Subvol string
			Name   string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		client := agentclient.New("/run/nos-agent.sock")
		var resp map[string]any
		err := client.PostJSON(r.Context(), "/v1/btrfs/snapshot", map[string]any{"path": body.Subvol, "name": body.Name}, &resp)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = id // unused for now
		writeJSON(w, resp)
	})
}
