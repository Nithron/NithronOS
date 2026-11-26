package server

import (
	"net/http"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"

	"github.com/go-chi/chi/v5"
)

func Test_AllRoutesUnderV1OrAllowlist(t *testing.T) {
	cfg := config.Defaults()
	r := NewRouter(cfg).(*chi.Mux)

	allow := map[string]bool{
		"/metrics":     true,
		"/metrics/all": true,
		"/healthz":     true,
	}

	var offenders []string
	_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if route == "/" { // root handler for SPA or similar; ignore
			return nil
		}
		if allow[route] {
			return nil
		}
		if len(route) >= 8 && route[:8] == "/api/v1/" {
			return nil
		}
		// Permit local-only debug/pprof tree
		if len(route) >= 12 && route[:12] == "/debug/pprof" {
			return nil
		}
		offenders = append(offenders, method+" "+route)
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("non-v1 routes found:\n%s", string([]byte(strings.Join(offenders, "\n"))))
	}
}
