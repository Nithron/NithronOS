package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"

	// "nos/backend/nosd/pkg/firewall"
	"nos/backend/nosd/pkg/httpx"
)

// registerMaintenance serves the support bundle and pool snapshots.
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

	// Back-compat: verify-totp path expected by FE
	pr.Post("/api/v1/auth/verify-totp", func(w http.ResponseWriter, r *http.Request) {
		// Delegate to /api/v1/auth/totp/verify handler
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/api/v1/auth/totp/verify"
		pr.ServeHTTP(w, r2)
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
