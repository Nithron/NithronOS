package server

import (
	"net/http/httptest"
	"testing"

	"nithronos/backend/nosd/internal/config"
)

// Fuzz token/cookie parsers to ensure no panics and safe failures.
func FuzzDecodeSessionParts(f *testing.F) {
	f.Add("", "")
	f.Add("uid=abc; nos_session=garbled", "Mozilla/5.0")
	f.Fuzz(func(t *testing.T, cookie string, ua string) {
		r := httptest.NewRequest("GET", "/", nil)
		if cookie != "" {
			r.Header.Set("Cookie", cookie)
		}
		if ua != "" {
			r.Header.Set("User-Agent", ua)
		}
		cfg := config.Defaults()
		_, _, _ = decodeSessionParts(r, cfg)
		_, _, _, _ = decodeRefresh(r, cfg)
	})
}
