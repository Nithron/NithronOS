package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// A share with sync turned off isn't offered to sync clients and its sync
// endpoints answer "not found"; other shares are unaffected.
func TestShareSyncSwitch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOS_SHARES_PATH", filepath.Join(dir, "shares.json"))
	t.Setenv("NOS_ETC_DIR", dir)
	e := newLoginEnv(t, false)
	admin, csrf := e.signIn(t)
	for _, body := range []string{
		`{"name":"docs","owners":["group:users"]}`,
		`{"name":"media","owners":["group:users"],"sync":{"enabled":false}}`,
	} {
		if res := e.call(http.MethodPost, "/api/v1/shares", body, admin, csrf); res.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", body, res.Code, res.Body.String())
		}
	}
	res := e.call(http.MethodPost, "/api/v1/sync/devices/register", `{"device_name":"laptop","device_type":"linux"}`, admin, csrf)
	var reg struct {
		DeviceToken string `json:"device_token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &reg); err != nil || reg.DeviceToken == "" {
		t.Fatalf("register: %d %s", res.Code, res.Body.String())
	}
	device := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+reg.DeviceToken)
		rr := httptest.NewRecorder()
		e.r.ServeHTTP(rr, req)
		return rr
	}
	list := device("/api/v1/sync/shares").Body.String()
	if !strings.Contains(list, `"docs"`) || strings.Contains(list, `"media"`) {
		t.Fatalf("sync shares: %s", list)
	}
	var shares []struct {
		ID, Name string
		Sync     struct{ Enabled bool }
	}
	if err := json.Unmarshal(e.call(http.MethodGet, "/api/v1/shares", "", admin, "").Body.Bytes(), &shares); err != nil {
		t.Fatal(err)
	}
	for _, sh := range shares {
		if sh.Sync.Enabled != (sh.Name == "docs") {
			t.Errorf("%s: sync.enabled=%v", sh.Name, sh.Sync.Enabled)
		}
		code := device("/api/v1/sync/state/" + sh.ID).Code
		if sh.Name == "media" && code != http.StatusNotFound {
			t.Errorf("media's sync state: %d", code)
		}
		if sh.Name == "docs" && code != http.StatusOK {
			t.Errorf("docs's sync state: %d", code)
		}
	}
}
