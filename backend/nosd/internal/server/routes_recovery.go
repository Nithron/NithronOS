package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	pwhash "nithronos/backend/nosd/internal/auth/hash"
	userstore "nithronos/backend/nosd/internal/auth/store"

	// "nithronos/backend/nosd/pkg/firewall"
	"nithronos/backend/nosd/pkg/httpx"
)

// registerRecovery serves the local-only recovery API (recovery mode only).
func (d *routeDeps) registerRecovery(r chi.Router) {
	cfg := d.cfg
	fb := d.fb
	// Recovery routes (localhost only)
	if cfg.RecoveryMode {
		r.Route("/api/v1/recovery", func(rr chi.Router) {
			rr.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !isLocalRequest(r) {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			rr.Post("/reset-password", func(w http.ResponseWriter, r *http.Request) {
				var body struct{ Username, Password string }
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.Password) == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				users, err := userstore.New(cfg.UsersPath)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u, err := users.FindByUsername(strings.ToLower(body.Username))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				h, herr := pwhash.HashPassword(body.Password)
				if herr != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u.PasswordHash = h
				u.LockedUntil = ""
				u.FailedAttempts = 0
				_ = users.UpsertUser(u)
				writeJSON(w, map[string]any{"ok": true})
			})
			rr.Post("/disable-2fa", func(w http.ResponseWriter, r *http.Request) {
				var body struct{ Username string }
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.TrimSpace(body.Username) == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				users, err := userstore.New(cfg.UsersPath)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u, err := users.FindByUsername(strings.ToLower(body.Username))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				u.TOTPEnc = ""
				u.RecoveryHashes = nil
				_ = users.UpsertUser(u)
				writeJSON(w, map[string]any{"ok": true})
			})
			rr.Post("/generate-otp", func(w http.ResponseWriter, r *http.Request) {
				// Issue a fresh setup code (only while no administrator exists).
				st, err := fb.Rotate()
				if err != nil {
					httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.otp.store", "Cannot write the setup code", 0)
					return
				}
				if st == nil {
					httpx.WriteTypedError(w, http.StatusConflict, "setup.complete", "Setup already completed", 0)
					return
				}
				writeJSON(w, map[string]any{"otp": st.OTP, "expiresAt": st.ExpiresAt})
			})
		})
	}

}

// registerFirstBootReset serves the local-only reset of the first-boot state.
func (d *routeDeps) registerFirstBootReset(r chi.Router) {
	cfg := d.cfg
	fb := d.fb
	// Recovery: local-only endpoint to clear first-boot state and optionally users
	r.Post("/api/v1/setup/recover", func(w http.ResponseWriter, r *http.Request) {
		// Guard: localhost only
		if !isLocalRequest(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var body struct {
			Confirm     string `json:"confirm"`
			DeleteUsers bool   `json:"delete_users"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToLower(strings.TrimSpace(body.Confirm)) != "yes" {
			httpx.WriteTypedError(w, http.StatusPreconditionRequired, "confirm.required", "confirm=yes required", 0)
			return
		}
		if body.DeleteUsers {
			_ = os.Remove(cfg.UsersPath)
		}
		// Drop the old code; with the users gone a new one is issued and shown
		// on the console right away.
		_ = fb.Consume()
		if body.DeleteUsers {
			_, _ = fb.Current()
		}
		writeJSON(w, map[string]any{"ok": true})
	})

}
