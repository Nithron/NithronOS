package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	pwhash "nos/backend/nosd/internal/auth/hash"
	userstore "nos/backend/nosd/internal/auth/store"

	// "nos/backend/nosd/pkg/firewall"
	"nos/backend/nosd/pkg/httpx"

	firstboot "nos/backend/nosd/internal/setup/firstboot"
)

// registerSetup serves the first-boot setup wizard; it answers 410 once setup is done.
func (d *routeDeps) registerSetup(r chi.Router) {
	cfg := d.cfg
	users := d.users
	rlStore := d.rlStore
	fb := d.fb
	// Setup routes are always registered under /api/v1, but gated with 410 when setup is complete
	r.Route("/api/v1/setup", func(sr chi.Router) {
		sr.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Allow /complete endpoint to bypass the check
				if strings.HasSuffix(r.URL.Path, "/complete") {
					next.ServeHTTP(w, r)
					return
				}
				// Evaluate setup completion from disk on every request (robust against file changes)
				us, _ := userstore.New(cfg.UsersPath)
				if us != nil && us.HasAdmin() {
					// "Is this first boot?" has an answer, not an error: the
					// UI asks on every page (a 410 here was logged as an
					// error in the browser each time). Setup actions are gone.
					if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/state") {
						writeJSON(w, map[string]any{"firstBoot": false, "otpRequired": false, "complete": true})
						return
					}
					httpx.WriteTypedError(w, http.StatusGone, "setup.complete", "Setup already completed", 0)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		sr.Get("/state", func(w http.ResponseWriter, r *http.Request) {
			// While no administrator exists a valid code always exists (a new one
			// is issued here if the last one expired), so the UI never waits for one.
			st, err := fb.Current()
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.otp.store").Err(err).Msg("")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.otp.store", "Cannot create the setup code", 0)
				return
			}
			resp := map[string]any{"firstBoot": st != nil, "otpRequired": st != nil}
			if st != nil {
				resp["expiresAt"] = st.ExpiresAt
			}
			writeJSON(w, resp)
		})

		// Rate limiter (persisted): per-IP cfg.RateOTPPerMin per minute for setup endpoints
		sr.Post("/otp/verify", func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, cfg)
			otpWin := time.Duration(cfg.RateOTPWindowSec) * time.Second
			if otpWin <= 0 {
				otpWin = time.Minute
			}
			ok1, rem1, reset1 := rlStore.Allow("otp:ip:"+ip, cfg.RateOTPPerMin, otpWin)
			if !ok1 {
				retry := int(time.Until(reset1).Seconds())
				Logger(cfg).Warn().Str("event", "rate.limited").Str("route", "/api/v1/setup/otp/verify").Str("key", "otp:ip:"+ip).Int("remaining", rem1).Int("retryAfterSec", retry).Msg("")
				httpx.WriteTypedError(w, http.StatusTooManyRequests, "rate.limited", "Too many attempts. Try later.", retry)
				return
			}
			var body struct{ OTP string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			code := strings.ReplaceAll(strings.TrimSpace(body.OTP), " ", "")
			if len(code) != firstboot.CodeLength {
				httpx.WriteTypedError(w, http.StatusBadRequest, "setup.otp.invalid", "Enter the 6-digit code", 0)
				return
			}
			res, err := fb.Verify(code)
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.otp.store").Err(err).Msg("")
				if os.IsPermission(err) {
					httpx.WriteTypedError(w, http.StatusInternalServerError, "storage_error", "setup storage not writable", 0)
					return
				}
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.otp.store", "Cannot read the setup code", 0)
				return
			}
			switch res {
			case firstboot.Valid:
			case firstboot.Expired:
				httpx.WriteTypedError(w, http.StatusGone, "setup.otp.expired", "That code expired. A new code is shown on the server console.", 0)
				return
			case firstboot.NotRequired:
				httpx.WriteTypedError(w, http.StatusGone, "setup.complete", "Setup already completed", 0)
				return
			default:
				httpx.WriteTypedError(w, http.StatusBadRequest, "setup.otp.invalid", "Invalid one-time code", 0)
				return
			}
			payload := map[string]any{"purpose": "setup", "exp": time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)}
			val, err := setupEncodeToken(cfg, payload)
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.token.error").Err(err).Msg("")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "secret_unreadable", "secret.key not readable", 0)
				return
			}
			// Set setup session cookie under /api/v1/setup
			secure := isSecureRequest(r, cfg)
			writeSetupCookie(w, val, 10*time.Minute, secure)
			writeJSON(w, map[string]any{"ok": true, "token": val})
		})

		// First admin creation (consumes setup token)
		sr.With(requireSetupAuth(cfg)).Post("/first-admin", func(w http.ResponseWriter, r *http.Request) {
			if users == nil {
				httpx.WriteTypedError(w, http.StatusInternalServerError, "store.lock", "User store unavailable", 0)
				return
			}
			var body struct {
				Username   string `json:"username"`
				Password   string `json:"password"`
				EnableTOTP bool   `json:"enable_totp"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			uname := strings.TrimSpace(body.Username)
			if !validAccountName(uname) {
				httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid username", 0)
				return
			}
			if !validPassword(body.Password) || passwordProblem(body.Password) != "" {
				httpx.WriteTypedError(w, http.StatusBadRequest, "input.weak_password", "Choose a stronger password", 0)
				return
			}
			if _, err := users.FindByUsername(uname); err == nil {
				httpx.WriteTypedError(w, http.StatusConflict, "input.username_taken", "Username is taken", 0)
				return
			}
			phc, err := pwhash.HashPassword(body.Password)
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.hash.error").Err(err).Msg("")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "store.atomic_fail", "Internal error", 0)
				return
			}
			now := time.Now().UTC().Format(time.RFC3339)
			u := userstore.User{ID: generateUUID(), Username: uname, PasswordHash: phc, Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now}
			if body.EnableTOTP {
				u.TOTPEnc = "pending"
			}
			if err := users.UpsertUser(u); err != nil {
				code := "store.atomic_fail"
				if os.IsPermission(err) || os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
					code = "setup.write_failed"
					// Emit clear hint for permission issues
					info := dirPermInfo(filepath.Dir(cfg.UsersPath))
					Logger(cfg).Error().Str("event", "setup.persist.error").Str("code", code).Str("hint", info).Err(err).Msg("")
				} else {
					Logger(cfg).Error().Str("event", "setup.persist.error").Str("code", code).Err(err).Msg("")
				}
				httpx.WriteErrorWithDetails(w, http.StatusInternalServerError, code, "Service cannot write /etc/nos/users.json", map[string]any{"path": cfg.UsersPath})
				return
			}
			// The administrator's file-sharing (SMB) account, with the same
			// password. Setup doesn't depend on it: a later password reset
			// creates it too.
			if d.agentClient != nil {
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				if err := (agentAccounts{agent: d.agentClient}).SetPassword(ctx, uname, body.Password); err != nil {
					Logger(cfg).Warn().Str("event", "setup.smb_account").Err(err).Msg("the administrator's SMB account was not created")
				}
				cancel()
			}
			// The administrator exists: the setup code and its console banner go away.
			if err := fb.Consume(); err != nil {
				Logger(cfg).Warn().Str("event", "setup.otp.consume").Err(err).Msg("")
			}
			// success; return 200 to advance UI reliably
			w.WriteHeader(http.StatusOK)
		})

		// Mark setup as complete - called after all setup steps are done
		sr.With(requireSetupAuth(cfg)).Post("/complete", func(w http.ResponseWriter, r *http.Request) {
			// Check if already complete
			setupCompleteFile := filepath.Join(cfg.EtcDir, "nos", "setup-complete")
			if _, err := os.Stat(setupCompleteFile); err == nil {
				// Already marked complete, just return success
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Create setup-complete marker file
			dir := filepath.Dir(setupCompleteFile)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				Logger(cfg).Error().Err(err).Str("dir", dir).Msg("Failed to create setup-complete directory")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.write_failed", "Failed to mark setup as complete", 0)
				return
			}
			if err := os.WriteFile(setupCompleteFile, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
				Logger(cfg).Error().Err(err).Str("file", setupCompleteFile).Msg("Failed to write setup-complete file")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.write_failed", "Failed to mark setup as complete", 0)
				return
			}

			if err := fb.Consume(); err != nil {
				Logger(cfg).Warn().Str("event", "setup.otp.consume").Err(err).Msg("")
			}
			// Clear setup cookie now that setup is complete
			clearSetupCookie(w)
			w.WriteHeader(http.StatusNoContent)
		})
	})

}
