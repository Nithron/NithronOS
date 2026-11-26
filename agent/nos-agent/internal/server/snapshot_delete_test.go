package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotDelete(t *testing.T) {
	base := t.TempDir()
	t.Setenv("NOS_SNAPSHOT_BASE", base)
	archive := filepath.Join(snapshotsTarDirForPath("/etc/nos"), "20261126-120000-pre-update.tar.gz")
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mux := buildMux()
	post := func(body string) int {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/snapshot/delete", strings.NewReader(body)))
		return rr.Code
	}
	for _, bad := range []string{
		`{"path":"/etc/nos","snapshot_id":"../../etc/passwd","type":"tar"}`,
		`{"path":"/etc/nos","snapshot_id":"20261126-120000-x/../../y","type":"tar"}`,
		`{"path":"etc","snapshot_id":"20261126-120000-pre-update","type":"tar"}`,
		`{"path":"/etc/nos","snapshot_id":"20261126-120000-pre-update","type":"zip"}`,
	} {
		if code := post(bad); code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, code)
		}
	}
	if code := post(`{"path":"/etc/nos","snapshot_id":"20261126-120000-pre-update","type":"tar"}`); code != http.StatusOK {
		t.Fatalf("delete: %d", code)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("archive still there")
	}
}
