package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nithronos/backend/nosd/internal/config"
)

// Regression: the metrics routes were registered before the session
// middleware, and chi panics on middleware after routes, so enabling metrics
// crashed nosd at startup.
func TestRouterStartsWithMetricsEnabled(t *testing.T) {
	t.Setenv("NOS_METRICS", "1")
	var r http.Handler
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("NewRouter panicked with metrics enabled: %v", p)
			}
		}()
		r = NewRouter(config.FromEnv())
	}()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("/metrics: %d", res.Code)
	}
}
