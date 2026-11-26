package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"
)

// Regression (S4): middleware.RealIP copied a client-supplied X-Forwarded-For
// into RemoteAddr, so remote clients passed the localhost-only checks on the
// recovery endpoints (password reset, 2FA disable, deleting users).
func TestIsLocalRequest(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		header map[string]string
		want   bool
	}{
		{"ipv4 loopback", "127.0.0.1:5000", nil, true},
		{"ipv6 loopback", "[::1]:5000", nil, true},
		{"lan peer", "192.168.1.20:5000", nil, false},
		{"lan peer claiming loopback", "192.168.1.20:5000", map[string]string{"X-Forwarded-For": "127.0.0.1"}, false},
		{"through caddy", "127.0.0.1:5000", map[string]string{"X-Forwarded-For": "192.168.1.20"}, false},
		{"x-real-ip", "127.0.0.1:5000", map[string]string{"X-Real-IP": "127.0.0.1"}, false},
		{"garbage", "not-an-address", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			if got := isLocalRequest(req); got != tc.want {
				t.Fatalf("isLocalRequest = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClientIPIgnoresForwardedForUnlessTrusted(t *testing.T) {
	cfg := config.FromEnv()
	cfg.TrustProxy = false
	SetRuntimeTrustProxy(false)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8::7]:443"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	if got := clientIP(req, cfg); got != "2001:db8::7" {
		t.Fatalf("got %q", got)
	}
}

func TestRecoverEndpointRejectsForgedLocalhost(t *testing.T) {
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	r := NewRouter(config.FromEnv())
	req := httptestRequest(http.MethodPost, "/api/v1/setup/recover", `{"confirm":"yes","delete_users":true}`)
	req.RemoteAddr = "192.168.1.20:40000"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.Header.Set("X-Real-IP", "127.0.0.1")
	res := serve(r, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("forged localhost reached the recovery endpoint: %d", res.Code)
	}
}

func httptestRequest(method, path, body string) *http.Request {
	return httptest.NewRequest(method, path, strings.NewReader(body))
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

// Regression (S5): a client-supplied X-UID header was passed through to
// handlers that read the signed-in user from it.
func TestForgedUserHeaderIsIgnored(t *testing.T) {
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	r := NewRouter(config.FromEnv())
	req := httptestRequest(http.MethodPost, "/api/v1/sync/devices/register", `{"device_name":"x","device_type":"linux"}`)
	req.Header.Set("X-UID", "u-admin")
	req.Header.Set("Content-Type", "application/json")
	res := serve(r, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("forged X-UID registered a device: %d %s", res.Code, res.Body.String())
	}
}
