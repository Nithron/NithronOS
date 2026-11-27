package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"
)

// refuseAll stands in for the admin check.
func refuseAll(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
}

func passAll(next http.Handler) http.Handler { return next }

func appearanceRouter(t *testing.T, adminOnly func(http.Handler) http.Handler) http.Handler {
	t.Helper()
	cfg := config.FromEnv()
	cfg.EtcDir = t.TempDir()
	return NewAppearanceHandler(cfg).Routes(adminOnly)
}

func TestAppearanceReadByAnyoneChangedByAdminsOnly(t *testing.T) {
	r := appearanceRouter(t, refuseAll)

	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET: %d, want 200", res.Code)
	}
	var got AppearanceSettings
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil || got.Theme != "auto" {
		t.Fatalf("GET: theme %q (%v), want auto by default", got.Theme, err)
	}

	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"theme":"dark"}`)))
	if res.Code != http.StatusForbidden {
		t.Fatalf("PUT without the admin check passing: %d, want 403", res.Code)
	}
}

func TestAppearanceThemeIsSavedAndValidated(t *testing.T) {
	r := appearanceRouter(t, passAll)

	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"theme":"purple"}`)))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("PUT an unknown theme: %d, want 400", res.Code)
	}

	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"theme":"light"}`)))
	if res.Code != http.StatusOK {
		t.Fatalf("PUT light: %d %s", res.Code, res.Body.String())
	}
	res = httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	var got AppearanceSettings
	_ = json.Unmarshal(res.Body.Bytes(), &got)
	if got.Theme != "light" {
		t.Fatalf("theme after PUT: %q, want light", got.Theme)
	}
}
