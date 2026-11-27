package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nos/backend/nosd/internal/config"
)

// The old NOS_TEST_SKIP_AUTH=1 turned off sign-in, admin and CSRF checks in
// the shipped binary. No environment variable may do that any more.
func TestEnvironmentCannotDisableAuth(t *testing.T) {
	withRealAuth(t)
	t.Setenv("NOS_TEST_SKIP_AUTH", "1")
	r := NewRouter(config.FromEnv())

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/shares"},
		{http.MethodGet, "/api/v1/sync/devices"},
		{http.MethodPost, "/api/v1/sync/devices/register"},
	} {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a session: %d, want 401", tc.method, tc.path, res.Code)
		}
	}
}
