package server

import (
	"net/http"

	"nos/backend/nosd/pkg/httpx"
)

// unavailable answers every request with 503 and a typed error. It is mounted
// in place of a subsystem that failed to start, so the UI shows a clear error
// instead of mock data.
func unavailable(code, message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteTypedError(w, http.StatusServiceUnavailable, code, message, 0)
	})
}
