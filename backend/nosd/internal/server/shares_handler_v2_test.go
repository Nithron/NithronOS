package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestSharesHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := NewSharesHandlerV2(filepath.Join(t.TempDir(), "shares.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return h.Routes()
}

func postShare(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

// Fields written into smb.conf or /etc/exports must not be able to add
// directives, and shares must stay inside /srv or /mnt.
func TestCreateShareRejectsUnsafeInput(t *testing.T) {
	h := newTestSharesHandler(t)
	base := map[string]any{"name": "media", "path": "/mnt", "protocol": "smb"}
	cases := map[string]map[string]any{
		"newline in name":        {"name": "x]\n[global"},
		"path outside roots":     {"path": "/etc"},
		"path with dotdot":       {"path": "/srv/../etc"},
		"newline in path":        {"path": "/srv\n   guest ok = yes"},
		"newline in description": {"description": "x\n   guest ok = yes"},
		"bad user":               {"users": []string{"alice\nbob"}},
		"nfs client options":     {"protocol": "nfs", "hosts": []string{"*(rw,no_root_squash)"}},
	}
	for name, override := range cases {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		for k, v := range override {
			body[k] = v
		}
		if res := postShare(t, h, http.MethodPost, "/", body); res.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400 (%s)", name, res.Code, res.Body.String())
		}
	}
}

// The share ID names a file under /etc/samba/shares.d, so a client-supplied
// ID (e.g. "../../cron.d/x") must be ignored.
func TestCreateShareAssignsID(t *testing.T) {
	if _, err := os.Stat("/mnt"); err != nil {
		t.Skip("/mnt not present")
	}
	h := newTestSharesHandler(t)
	res := postShare(t, h, http.MethodPost, "/", map[string]any{"id": "../../cron.d/x", "name": "media", "path": "/mnt", "protocol": "smb"})
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var got ShareConfig
	_ = json.Unmarshal(res.Body.Bytes(), &got)
	if got.ID == "" || strings.ContainsAny(got.ID, "./") {
		t.Fatalf("client ID kept: %q", got.ID)
	}
	if res := postShare(t, h, http.MethodPut, "/"+got.ID, map[string]any{"path": "/"}); res.Code != http.StatusBadRequest {
		t.Fatalf("update to /: got %d, want 400", res.Code)
	}
}
