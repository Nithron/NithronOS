package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

// registerSMBUsers serves the SMB accounts: a nOS user needs one (a Unix
// account with a Samba password) to open SMB shares and to be granted access
// in the folder ACLs.
func (d *routeDeps) registerSMBUsers(pr chi.Router) {
	pr.With(d.adminRequired).Get("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
		var out struct {
			Users []string `json:"users"`
		}
		if err := d.agentClient.GetJSON(r.Context(), "/v1/smb/users", &out); err != nil {
			httpx.WriteTypedError(w, http.StatusServiceUnavailable, "agent.unavailable", "Could not list SMB users: "+err.Error(), 0)
			return
		}
		writeJSON(w, out.Users)
	})

	// Gives a nOS user an SMB password. The account takes the nOS username,
	// so share permissions (user:<name>) apply to it.
	pr.With(d.adminRequired).Post("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "smb.invalid_body", "Invalid request body", 0)
			return
		}
		if _, err := d.users.FindByUsername(body.Username); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "smb.unknown_user", "Not a nOS user: "+body.Username, 0)
			return
		}
		var resp map[string]any
		err := d.agentClient.PostJSON(r.Context(), "/v1/smb/user-create", map[string]any{"username": body.Username, "password": body.Password}, &resp)
		var he *agentclient.HTTPError
		switch {
		case errors.As(err, &he) && he.Status == http.StatusBadRequest:
			httpx.WriteTypedError(w, http.StatusBadRequest, "smb.invalid", he.Body, 0)
			return
		case err != nil:
			httpx.WriteTypedError(w, http.StatusBadGateway, "agent.failed", err.Error(), 0)
			return
		}
		// The new account can now be granted in ACLs and groups.
		if d.sharesApplier != nil {
			d.sharesApplier.Apply(r.Context())
		}
		writeJSON(w, map[string]any{"ok": true})
	})
}
