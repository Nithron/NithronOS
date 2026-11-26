package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/pkg/agentclient"

	// "nos/backend/nosd/pkg/firewall"
	"nos/backend/nosd/pkg/httpx"
)

// registerSMBUsers serves the SMB user list and the legacy share delete.
func (d *routeDeps) registerSMBUsers(pr chi.Router) {
	adminRequired := d.adminRequired
	// Shares endpoints are handled by SharesHandler below
	// SMB users proxy
	pr.Get("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
		client := agentclient.New("/run/nos-agent.sock")
		var out struct {
			Users []string `json:"users"`
		}
		if err := client.GetJSON(r.Context(), "/v1/smb/users", &out); err != nil {
			// Graceful fallback
			writeJSON(w, []string{})
			return
		}
		writeJSON(w, out.Users)
	})
	// Shares endpoints (replaced by v1 API)
	// pr.With(adminRequired).Get("/api/shares", sharesHandler.ListShares)
	// pr.With(adminRequired).Post("/api/shares", sharesHandler.CreateShare)
	// pr.With(adminRequired).Get("/api/shares/{name}", sharesHandler.GetShare)
	// pr.With(adminRequired).Patch("/api/shares/{name}", sharesHandler.UpdateShare)
	// pr.With(adminRequired).Delete("/api/shares/{name}", sharesHandler.DeleteShare)
	// pr.With(adminRequired).Post("/api/shares/{name}/test", sharesHandler.TestShare)

	pr.With(adminRequired).Post("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		client := agentclient.New("/run/nos-agent.sock")
		var resp map[string]any
		if err := client.PostJSON(r.Context(), "/v1/smb/user-create", map[string]any{"username": body.Username, "password": body.Password}, &resp); err != nil {
			// If agent returned HTTPError 400, propagate 400
			if he, ok := err.(*agentclient.HTTPError); ok && he.Status == http.StatusBadRequest {
				httpx.WriteError(w, http.StatusBadRequest, he.Body)
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	/* Duplicate delete handler - removed
	pr.With(adminRequired).Delete("/api/shares/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		st := struct{
			GetByID func(string) (interface{}, bool)
			Delete func(string) error
			RemoveByID func(string)
		}{
			GetByID: func(id string) (interface{}, bool) { return nil, false },
			Delete: func(id string) error { return nil },
			RemoveByID: func(id string) {},
		}
		sh, ok := st.GetByID(id)
		if ok {
			// Best-effort: in dev/test on Windows or when the agent socket isn't present, skip agent calls
			var client *agentclient.Client
			if runtime.GOOS != "windows" {
				if _, err := os.Stat("/run/nos-agent.sock"); err == nil {
					client = agentclient.New("/run/nos-agent.sock")
				}
			}
			if sh.Type == "smb" {
				path := filepath.Join(cfg.EtcDir, "samba", "smb.conf.d", "nos-"+sh.Name+".conf")
				if client != nil {
					_ = client.PostJSON(r.Context(), "/v1/fs/write", map[string]any{"path": path, "content": "", "mode": "0644", "owner": "root", "group": "root"}, nil)
				}
			}
			if sh.Type == "nfs" {
				path := filepath.Join(cfg.EtcDir, "exports.d", "nos-"+sh.Name+".exports")
				if client != nil {
					_ = client.PostJSON(r.Context(), "/v1/fs/write", map[string]any{"path": path, "content": ""}, nil)
				}
			}
			_ = st.Delete(id)
			if client != nil {
				_ = client.PostJSON(r.Context(), "/v1/service/reload", map[string]any{"name": "smb"}, nil)
				_ = client.PostJSON(r.Context(), "/v1/service/reload", map[string]any{"name": "nfs"}, nil)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}) */

}
