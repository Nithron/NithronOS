package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

type fakeAppHost struct {
	ran   []string
	dirs  map[string]string // path -> "mode uid:gid"
	files map[string]string
	gone  []string
}

func newFakeAppHost() *fakeAppHost {
	return &fakeAppHost{dirs: map[string]string{}, files: map[string]string{caddyfile: "{\n  admin off\n  auto_https off\n}\n\n:443 {\n  tls x y\n}\n"}}
}
func (f *fakeAppHost) Run(name string, args ...string) ([]byte, error) {
	f.ran = append(f.ran, name+" "+strings.Join(args, " "))
	return nil, nil
}
func (f *fakeAppHost) MkdirOwned(p string, mode os.FileMode, uid, gid int) error {
	f.dirs[p] = mode.String() + " " + itoa(uid) + ":" + itoa(gid)
	return nil
}
func (f *fakeAppHost) WriteFile(p string, d []byte) error { f.files[p] = string(d); return nil }
func (f *fakeAppHost) Remove(p string) error              { f.gone = append(f.gone, p); return nil }
func (f *fakeAppHost) ReadFile(p string) ([]byte, error) {
	if v, ok := f.files[p]; ok {
		return []byte(v), nil
	}
	return nil, os.ErrNotExist
}
func (f *fakeAppHost) ListDir(string) []string {
	return []string{"/srv/apps/.snapshots/nai/20261126-120000-pre-upgrade"}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestAppOps(t *testing.T) {
	old := nosIDs
	nosIDs = func() (int, int, error) { return 990, 990, nil }
	t.Cleanup(func() { nosIDs = old })

	h := newFakeAppHost()
	if _, _, err := runAppOp(h, AppOpRequest{Op: "prepare", AppID: "nai", UID: 1000, GID: 1000}); err != nil {
		t.Fatal(err)
	}
	if h.dirs["/srv/apps/nai/config"] != "-rwxr-x--- 990:990" {
		t.Errorf("config folder: %v", h.dirs)
	}
	if !slices.Contains(h.ran, "/usr/lib/nos/apps/nos-app-snapshot.sh ensure-subvolume nai") || !slices.Contains(h.ran, "chown 1000:1000 /srv/apps/nai/data") {
		t.Errorf("prepare: %v", h.ran)
	}
	if _, code, _ := runAppOp(h, AppOpRequest{Op: "prepare", AppID: "nai", UID: 0, GID: 0}); code != http.StatusBadRequest {
		t.Error("data owned by root must be refused")
	}

	h.ran = nil
	if _, _, err := runAppOp(h, AppOpRequest{Op: "start", AppID: "nai"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(h.ran, ";") != "systemctl enable nos-app@nai.service;systemctl start nos-app@nai.service" {
		t.Errorf("start: %v", h.ran)
	}

	// Proxy: the snippet points at the host port and the Caddyfile loads it.
	h.ran = nil
	if _, _, err := runAppOp(h, AppOpRequest{Op: "proxy", AppID: "nai", Port: 8012}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.files["/etc/caddy/Caddyfile.d/app-nai.caddy"], "handle_path /apps/nai/* {\n\treverse_proxy 127.0.0.1:8012") {
		t.Errorf("snippet: %s", h.files["/etc/caddy/Caddyfile.d/app-nai.caddy"])
	}
	conf := h.files[caddyfile]
	if !strings.Contains(conf, ":443 {\n  # Apps installed by nOS\n  "+caddyAppsImport) || !strings.Contains(conf, caddyAdmin) || strings.Contains(conf, "admin off") {
		t.Errorf("Caddyfile: %s", conf)
	}
	if ensureCaddyAppsImport(conf) != conf {
		t.Error("ensureCaddyAppsImport must be idempotent")
	}
	if !slices.Contains(h.ran, "systemctl reload-or-restart caddy") {
		t.Errorf("caddy not reloaded: %v", h.ran)
	}

	// Removing without keeping data deletes the subvolumes first.
	h.ran = nil
	if _, _, err := runAppOp(h, AppOpRequest{Op: "remove", AppID: "nai"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(h.ran, "btrfs subvolume delete /srv/apps/nai/data") || !slices.Contains(h.gone, "/srv/apps/nai") {
		t.Errorf("remove: %v %v", h.ran, h.gone)
	}

	for _, bad := range []AppOpRequest{
		{Op: "snapshot", AppID: "nai", Name: "../x"},
		{Op: "rollback", AppID: "nai", Snapshot: "../../etc"},
		{Op: "proxy", AppID: "nai", Port: 0},
		{Op: "exec", AppID: "nai"},
	} {
		if _, code, err := runAppOp(h, bad); err == nil || code != http.StatusBadRequest {
			t.Errorf("%+v accepted", bad)
		}
	}

	// App IDs are checked before anything runs.
	mux := buildMux()
	b, _ := json.Marshal(AppOpRequest{Op: "start", AppID: "../../etc"})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/apps/op", bytes.NewReader(b)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad app id: %d", rr.Code)
	}
}
