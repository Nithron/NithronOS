package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/httpx"
)

const (
	// SetupCookieName is the cookie used to carry the first-boot setup token
	SetupCookieName = "nos_setup"
	// SetupCookiePath scopes the cookie to setup endpoints only under v1
	SetupCookiePath = "/api/v1/setup"
)

// ctxSetupClaims is the context key for validated setup claims
const ctxSetupClaims ctxKey = "setupClaims"

func writeSetupCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SetupCookieName,
		Value:    token,
		Path:     SetupCookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl / time.Second),
	})
}

func clearSetupCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SetupCookieName,
		Value:    "",
		Path:     SetupCookiePath,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// isSecureRequest attempts to determine if the incoming request is using HTTPS.
func isSecureRequest(r *http.Request, cfg config.Config) bool {
	if r.TLS != nil {
		return true
	}
	if cfg.TrustProxy || RuntimeTrustProxy() {
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			return true
		}
	}
	return false
}

// extractSetupToken fetches a setup token from Cookie, X-Setup-Token or Authorization: Bearer
func extractSetupToken(r *http.Request) string {
	// Header takes precedence for CLI/testing
	tok := strings.TrimSpace(r.Header.Get("X-Setup-Token"))
	if tok != "" {
		return tok
	}
	if ck, err := r.Cookie(SetupCookieName); err == nil && strings.TrimSpace(ck.Value) != "" {
		return ck.Value
	}
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(authz, prefix) {
		return strings.TrimSpace(authz[len(prefix):])
	}
	return ""
}

// requireSetupAuth validates the setup token and places claims into context.
func requireSetupAuth(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := extractSetupToken(r)
			if tok == "" {
				httpx.WriteTypedError(w, http.StatusUnauthorized, "setup.session.invalid", "Missing setup token", 0)
				return
			}
			claims, err := setupDecodeToken(cfg, tok)
			if err != nil {
				httpx.WriteTypedError(w, http.StatusUnauthorized, "setup.session.invalid", "Invalid setup token", 0)
				return
			}
			if claims["purpose"] != "setup" {
				httpx.WriteTypedError(w, http.StatusUnauthorized, "setup.session.invalid", "Invalid setup token", 0)
				return
			}
			if expStr, ok := claims["exp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, expStr); err != nil || time.Now().After(t) {
					httpx.WriteTypedError(w, http.StatusBadRequest, "setup.otp.expired", "Setup token expired", 0)
					return
				}
			} else {
				httpx.WriteTypedError(w, http.StatusUnauthorized, "setup.session.invalid", "Invalid setup token", 0)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ctxSetupClaims, claims))
			next.ServeHTTP(w, r)
		})
	}
}
