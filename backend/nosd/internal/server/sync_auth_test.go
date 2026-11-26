package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// NithronSync clients authenticate with a device token only; the web UI uses
// the browser session. Both must reach the sync API (the device token used
// to be refused by the session check in front of it, and the session by the
// device check).
func TestSyncAPIAcceptsDeviceTokenAndSession(t *testing.T) {
	t.Setenv("NOS_TEST_SKIP_AUTH", "0") // real auth here
	e := newLoginEnv(t, false)
	b, _ := json.Marshal(map[string]string{"username": "alice", "password": "Correct-Horse-9"})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	if res.Code != http.StatusOK {
		t.Fatalf("login: %d %s", res.Code, res.Body.String())
	}
	var cookies []*http.Cookie
	csrf := ""
	for _, c := range res.Result().Cookies() {
		if c.MaxAge < 0 {
			continue // deleted
		}
		cookies = append(cookies, c)
		if c.Name == cookieCSRF {
			csrf = c.Value
		}
	}
	session := func(method, path, body string, withCSRF bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		if withCSRF {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		rr := httptest.NewRecorder()
		e.r.ServeHTTP(rr, req)
		return rr
	}

	// Registering a device needs the session and the CSRF token.
	if rr := session(http.MethodPost, "/api/v1/sync/devices/register", `{"device_name":"laptop","device_type":"linux"}`, false); rr.Code != http.StatusUnauthorized {
		t.Fatalf("register without CSRF: %d", rr.Code)
	}
	rr := session(http.MethodPost, "/api/v1/sync/devices/register", `{"device_name":"laptop","device_type":"linux"}`, true)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rr.Code, rr.Body.String())
	}
	if csrf == "" {
		t.Fatal("login set an empty CSRF token")
	}
	var reg struct {
		DeviceToken string `json:"device_token"`
		DeviceID    string `json:"device_id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &reg)
	if !strings.HasPrefix(reg.DeviceToken, "nos_dt_") {
		t.Fatalf("no device token: %s", rr.Body.String())
	}

	// The device: token only, no cookies, no CSRF.
	device := func(method, path, token string, extra map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		rr := httptest.NewRecorder()
		e.r.ServeHTTP(rr, req)
		return rr
	}
	if rr := device(http.MethodGet, "/api/v1/sync/shares", reg.DeviceToken, nil); rr.Code != http.StatusOK {
		t.Fatalf("device token on /sync/shares: %d %s", rr.Code, rr.Body.String())
	}
	if rr := device(http.MethodGet, "/api/v1/sync/devices", "nos_dt_bogus", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", rr.Code)
	}
	// A client can't claim another user by header.
	rr = device(http.MethodGet, "/api/v1/sync/devices", reg.DeviceToken, map[string]string{"X-Device-User-ID": "someone-else"})
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), reg.DeviceID) {
		t.Fatalf("device list: %d %s", rr.Code, rr.Body.String())
	}

	// The web UI: session, CSRF for changes.
	if rr := session(http.MethodGet, "/api/v1/sync/devices", "", false); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), reg.DeviceID) {
		t.Fatalf("session device list: %d %s", rr.Code, rr.Body.String())
	}
	if rr := session(http.MethodDelete, "/api/v1/sync/devices/"+reg.DeviceID, "", false); rr.Code != http.StatusUnauthorized {
		t.Fatalf("session change without CSRF: %d", rr.Code)
	}
	// Neither: 401.
	anon := httptest.NewRecorder()
	e.r.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/api/v1/sync/shares", nil))
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", anon.Code)
	}
}
