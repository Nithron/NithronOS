package server

import (
	"net/http"
	"testing"

	"nos/backend/nosd/internal/config"
)

func TestClientIPExtractor(t *testing.T) {
	cases := []struct {
		name   string
		trust  bool
		remote string
		xff    string
		want   string
	}{
		{"no-proxy", false, "1.1.1.1:123", "", "1.1.1.1"},
		{"proxy-single", true, "10.0.0.1:443", "203.0.113.5", "203.0.113.5"},
		{"proxy-multi", true, "10.0.0.1:443", "10.0.0.2, 203.0.113.9 ", "203.0.113.9"},
		{"malformed", true, "198.51.100.7:555", ",,  203.0.113.10,,,", "203.0.113.10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.FromEnv()
			cfg.TrustProxy = tc.trust
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remote
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(req, cfg)
			if got != tc.want {
				t.Fatalf("want %s got %s", tc.want, got)
			}
		})
	}
}
