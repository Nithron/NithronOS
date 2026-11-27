package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// /api/v1/health answers without signing in (sync clients and monitoring
// check it), even with a device token in Authorization.
func TestHealthIsPublic(t *testing.T) {
	e := newLoginEnv(t, false)
	for _, authz := range []string{"", "Bearer nos_dt_not-a-session"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rr := httptest.NewRecorder()
		e.r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("authorization %q: %d %s", authz, rr.Code, rr.Body.String())
		}
	}
}
