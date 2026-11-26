package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every CSRF token used to be "" (encodeBase64 always failed), so an empty
// cookie and no header passed the check.
func TestCSRF(t *testing.T) {
	if v := encodeBase64(make([]byte, 32)); len(v) < 40 {
		t.Fatalf("token too short: %q", v)
	}
	ok := requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	try := func(cookie *string, header string) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if cookie != nil {
			req.AddCookie(&http.Cookie{Name: cookieCSRF, Value: *cookie})
		}
		if header != "" {
			req.Header.Set("X-CSRF-Token", header)
		}
		rr := httptest.NewRecorder()
		ok.ServeHTTP(rr, req)
		return rr.Code
	}
	empty, good := "", encodeBase64([]byte("0123456789abcdef0123456789abcdef"))
	for name, tc := range map[string]struct {
		cookie *string
		header string
		want   int
	}{
		"no cookie":      {nil, good, 401},
		"empty cookie":   {&empty, "", 401},
		"missing header": {&good, "", 401},
		"wrong header":   {&good, good + "x", 401},
		"matching":       {&good, good, 204},
	} {
		if got := try(tc.cookie, tc.header); got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
}
