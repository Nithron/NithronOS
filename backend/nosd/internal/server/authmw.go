package server

import (
	"net/http"

	"nithronos/backend/nosd/pkg/httpx"
)

type ctxKey string

const (
	ctxUserID ctxKey = "uid"
)

// requireAuth answers 401 unless the request has an active session.
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := signedInUID(r); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		ck, err := r.Cookie(cookieCSRF)
		if err != nil {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.csrf.missing", "Missing CSRF token", 0)
			return
		}
		if r.Header.Get("X-CSRF-Token") != ck.Value {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.csrf.invalid", "Invalid CSRF token", 0)
			return
		}
		next.ServeHTTP(w, r)
	})
}
