package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// File managers send the device token as a Basic password; a request
// without one is challenged for Basic credentials. (PROPFIND reaching the
// handler at all needs webdav_methods.go.)
func TestWebDAVBasicAuth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOS_SHARES_PATH", filepath.Join(dir, "shares.json"))
	t.Setenv("NOS_ETC_DIR", dir)
	e := newLoginEnv(t, false)
	admin, csrf := e.signIn(t)
	res := e.call(http.MethodPost, "/api/v1/shares", `{"name":"docs","owners":["group:users"]}`, admin, csrf)
	var share struct{ ID, Path string }
	if err := json.Unmarshal(res.Body.Bytes(), &share); err != nil || share.ID == "" {
		t.Fatalf("share: %d %s", res.Code, res.Body.String())
	}
	res = e.call(http.MethodPost, "/api/v1/sync/devices/register", `{"device_name":"pc","device_type":"windows"}`, admin, csrf)
	var reg struct {
		DeviceToken string `json:"device_token"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &reg)

	propfind := func(set func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PROPFIND", "/dav/"+share.ID+"/", nil)
		req.Header.Set("Depth", "0")
		set(req)
		rr := httptest.NewRecorder()
		e.r.ServeHTTP(rr, req)
		return rr
	}
	// Past authentication (the share's folder under /srv/shares doesn't
	// exist here, so what follows isn't a listing).
	if rr := propfind(func(r *http.Request) { r.SetBasicAuth("me", reg.DeviceToken) }); rr.Code == http.StatusUnauthorized || rr.Code == http.StatusMethodNotAllowed {
		t.Fatalf("basic with the token: %d %s", rr.Code, rr.Body.String())
	}
	if rr := propfind(func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+reg.DeviceToken) }); rr.Code == http.StatusUnauthorized || rr.Code == http.StatusMethodNotAllowed {
		t.Fatalf("bearer: %d", rr.Code)
	}
	for name, set := range map[string]func(*http.Request){
		"none":       func(*http.Request) {},
		"wrong":      func(r *http.Request) { r.SetBasicAuth("me", "nos_dt_wrong") },
		"a password": func(r *http.Request) { r.SetBasicAuth("admin", "Admin-Pass-1!") },
	} {
		rr := propfind(set)
		if rr.Code != http.StatusUnauthorized || !strings.HasPrefix(rr.Header().Get("WWW-Authenticate"), "Basic ") {
			t.Errorf("%s: %d, challenge %q", name, rr.Code, rr.Header().Values("WWW-Authenticate"))
		}
	}
}
