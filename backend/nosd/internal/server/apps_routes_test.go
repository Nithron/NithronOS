package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"
)

// Regression: a legacy POST /api/v1/apps/install, registered after the app
// manager's, replaced it, so catalog installs ran a hard-coded template list.
func TestAppsInstallReachesAppManager(t *testing.T) {
	t.Setenv("NOS_APPS_STATE", t.TempDir()+"/apps.json")
	r := NewRouter(config.FromEnv())
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/apps/install", bytes.NewBufferString(`{}`)))
	if res.Code == http.StatusServiceUnavailable {
		t.Skip("app manager unavailable in this environment")
	}
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "App ID is required") {
		t.Fatalf("install was not handled by the app manager: %d %s", res.Code, res.Body.String())
	}
}
