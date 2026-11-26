package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"errors"
	pwhash "nithronos/backend/nosd/internal/auth/hash"
	"nithronos/backend/nosd/internal/auth/session"
	userstore "nithronos/backend/nosd/internal/auth/store"

	// "nithronos/backend/nosd/pkg/firewall"
	"nithronos/backend/nosd/pkg/httpx"

	"strconv"
)

// registerAuth serves sign-in, refresh, sign-out and session management.
func (d *routeDeps) registerAuth(r chi.Router) {
	cfg := d.cfg
	users := d.users
	rlStore := d.rlStore
	mgr := d.mgr
	// (Removed legacy unversioned first-admin handler; canonical handler is under /api/v1/setup in the block above.)

	// Auth (legacy + new store integration)

	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			Code       string `json:"code"`
			RememberMe bool   `json:"rememberMe"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		uname := strings.TrimSpace(body.Username)
		pass := body.Password

		// Apply rate limiting first (before any other checks)
		ip := clientIP(r, cfg)
		loginWin := time.Duration(cfg.RateLoginWindowSec) * time.Second
		if loginWin <= 0 {
			loginWin = 15 * time.Minute
		}
		okIP, _, resetIP := rlStore.Allow("login:ip:"+ip, cfg.RateLoginPer15m, loginWin)
		okUser, _, resetUser := rlStore.Allow("login:user:"+strings.ToLower(uname), cfg.RateLoginPer15m, loginWin)
		if !okIP || !okUser {
			retry := resetIP
			if time.Until(resetUser) > 0 && resetUser.After(retry) {
				retry = resetUser
			}
			Logger(cfg).Warn().Str("event", "rate.limited").Str("key", "login").Str("ip", ip).Int("limit", cfg.RateLoginPer15m).Time("resetAt", retry).Msg("")
			w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(retry).Seconds())))
			httpx.WriteError(w, http.StatusTooManyRequests, `{"error":{"code":"rate.limited","retryAfterSec":`+strconv.Itoa(int(time.Until(retry).Seconds()))+`}}`)
			return
		}

		// During setup, allow login if admin exists (needed for steps 4-7)
		// Only block login if no admin exists yet
		us, _ := userstore.New(cfg.UsersPath)
		if us != nil && !us.HasAdmin() {
			// No admin yet, cannot login
			httpx.WriteTypedError(w, http.StatusForbidden, "setup.required", "System setup required. Please create an admin account first.", 0)
			return
		}
		u, err := users.FindByUsername(uname)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Check account lock
		if u.LockedUntil != "" {
			if t, err := time.Parse(time.RFC3339, u.LockedUntil); err == nil && time.Now().Before(t) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		// Only real password hashes are accepted (no "plain:"/"dev:" shortcuts).
		recordFailure := func() {
			// lock after 10 failed attempts (password or second factor)
			u.FailedAttempts++
			if u.FailedAttempts >= 10 {
				u.FailedAttempts = 0
				u.LockedUntil = time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339)
			}
			_ = users.UpsertUser(u)
		}
		if !pwhash.VerifyPassword(u.PasswordHash, pass) {
			recordFailure()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second factor: required whenever the user has enabled 2FA.
		if userHasTOTP(u) {
			if strings.TrimSpace(body.Code) == "" {
				httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.totp.required", "Two-factor code required", 0)
				return
			}
			if !verifySecondFactor(cfg, &u, body.Code) {
				recordFailure()
				httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.totp.invalid", "Invalid two-factor code", 0)
				return
			}
		}
		// success: reset counters
		u.FailedAttempts = 0
		u.LockedUntil = ""
		_ = users.UpsertUser(u)
		// Start the server-side session and set its cookies.
		rec, err := mgr.Create(u.ID, r.Header.Get("User-Agent"), r.RemoteAddr, sessionTTL, body.RememberMe)
		if err == nil {
			err = issueAuthCookies(w, cfg, rec)
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "session error")
			return
		}
		issueCSRFCookie(w)
		writeJSON(w, map[string]any{"ok": true})
	})

	// Record refresh events in sessions store (best-effort)
	r.Post("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		// The refresh token names its session and that session's current
		// refresh id. A revoked or expired session can't be refreshed; an
		// old refresh id means the token was copied, and every session of
		// the user ends.
		uid, sid, rid, ok := decodeRefresh(r, cfg)
		if !ok {
			clearAuthCookies(w)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rec, err := mgr.Refresh(sid, uid, rid, sessionTTL)
		if err != nil {
			if errors.Is(err, session.ErrRefreshReuse) {
				Logger(cfg).Warn().Str("event", "auth.refresh.reuse").Str("uid", uid).Msg("refresh token reused; revoking all sessions")
			}
			clearAuthCookies(w)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := issueAuthCookies(w, cfg, rec); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "session error")
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// Logout ends this session only (it used to sign the user out on every
	// device). The session id comes from the access cookie or, once that has
	// expired, from the refresh cookie.
	r.Post("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if _, sid, ok := decodeSessionParts(r, cfg); ok {
			_ = mgr.RevokeSID(sid)
		} else if _, sid, _, ok := decodeRefresh(r, cfg); ok {
			_ = mgr.RevokeSID(sid)
		}
		clearAuthCookies(w)
		w.WriteHeader(http.StatusNoContent)
	})

	// Session info (single) for compatibility with nos-client
	r.Get("/api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		uid, ok := signedInUID(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if u, err := users.FindByID(uid); err == nil {
			// Minimal shape expected by FE
			writeJSON(w, map[string]any{
				"user": map[string]any{
					"id":       u.ID,
					"username": u.Username,
					"roles":    u.Roles,
					"isAdmin":  hasRole(u.Roles, "admin"),
				},
				"expiresAt": time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Protected API group (auth required)
	r.Group(func(pr chi.Router) {
		pr.Use(requireAuth)
		// Session endpoints (self scope)
		pr.Get("/api/v1/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := signedInUID(r)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			list := mgr.ListByUser(uid)
			// mark current
			curSID := r.Header.Get("X-SID")
			out := make([]map[string]any, 0, len(list))
			for _, s := range list {
				out = append(out, map[string]any{
					"sid":        s.SID,
					"createdAt":  s.CreatedAt,
					"lastSeenAt": s.LastSeen,
					"expiresAt":  s.ExpiresAt,
					"ipPrefix":   s.IPPrefix,
					"userAgent":  s.UserAgent,
					"current":    s.SID == curSID,
				})
			}
			writeJSON(w, out)
		})
		pr.Post("/api/v1/auth/sessions/revoke", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := signedInUID(r)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body struct{ Scope, SID string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Scope == "" {
				body.Scope = "current"
			}
			ip := r.RemoteAddr
			if h := r.Header.Get("X-Forwarded-For"); h != "" {
				ip = strings.Split(h, ",")[0]
			}
			switch body.Scope {
			case "current":
				cur := r.Header.Get("X-SID")
				if cur != "" {
					_ = mgr.RevokeSID(cur)
				}
				clearAuthCookies(w)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "current").Str("sid", cur).Str("ip", ip).Msg("")
			case "all":
				_ = mgr.RevokeAll(uid)
				clearAuthCookies(w)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "all").Str("ip", ip).Msg("")
			case "sid":
				if body.SID == "" {
					httpx.WriteError(w, http.StatusBadRequest, "sid required")
					return
				}
				// validate ownership
				owned := false
				for _, s := range mgr.ListByUser(uid) {
					if s.SID == body.SID {
						owned = true
						break
					}
				}
				if !owned {
					httpx.WriteError(w, http.StatusForbidden, "not your session")
					return
				}
				_ = mgr.RevokeSID(body.SID)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "sid").Str("sid", body.SID).Str("ip", ip).Msg("")
			default:
				httpx.WriteError(w, http.StatusBadRequest, "invalid scope")
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		})
	})

	r.Get("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := signedInUID(r); ok {
			if u, err := users.FindByID(uid); err == nil {
				writeJSON(w, map[string]any{"user": map[string]any{"id": u.ID, "username": u.Username, "roles": u.Roles}})
				return
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	// (POST /api/v1/auth/totp/setup and /totp/confirm were removed: they were
	// reachable without signing in and never checked the password. TOTP is
	// enrolled through the authenticated /api/v1/auth/totp/enroll and /verify.)

}
