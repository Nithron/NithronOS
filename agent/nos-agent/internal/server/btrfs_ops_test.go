package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBtrfsMount(t *testing.T) {
	f := filepath.Join(t.TempDir(), "mounts")
	content := "/dev/sda1 / ext4 rw 0 0\n/dev/sdb /mnt/pool btrfs rw,compress=zstd 0 0\n/dev/sdc /mnt/my\\040pool btrfs rw 0 0\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := mountsFile
	mountsFile = f
	t.Cleanup(func() { mountsFile = old })

	for path, want := range map[string]bool{
		"/mnt/pool": true, "/mnt/my pool": true, "/": false, "/mnt": false,
		"/mnt/pool/": false, "/mnt/pool/../pool": false, "mnt/pool": false, "": false,
	} {
		if got := isBtrfsMount(path); got != want {
			t.Errorf("%q: %v", path, got)
		}
	}

	// Operations refuse anything that isn't a Btrfs mount point.
	mux := buildMux()
	for _, p := range []string{"/v1/btrfs/scrub/start", "/v1/btrfs/scrub/cancel", "/v1/btrfs/balance/start", "/v1/btrfs/balance/cancel"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, p, strings.NewReader(`{"mount":"/"}`)))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s on /: %d", p, rr.Code)
		}
	}
}

func TestParseSubvolumeList(t *testing.T) {
	out := "ID 256 gen 10 top level 5 path data\nID 257 gen 11 top level 5 path snaps/my snap\nbogus\n"
	got := parseSubvolumeList(out)
	if len(got) != 2 || got[0].ID != "256" || got[0].Path != "data" || got[1].Path != "snaps/my snap" || got[1].TopLevel != "5" {
		t.Fatalf("%+v", got)
	}
}
