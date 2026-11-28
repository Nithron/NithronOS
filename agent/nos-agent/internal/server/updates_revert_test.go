package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDebFileNameEscapesTheEpoch(t *testing.T) {
	if got := debFileName("libc6", "1:2.36-9+deb12u4", "amd64"); got != "libc6_1%3a2.36-9+deb12u4_amd64.deb" {
		t.Fatalf("got %s", got)
	}
}

func TestValidKeptRefusesOptionsAndPaths(t *testing.T) {
	id := "0b7c3c55-8f0e-4e0b-9a39-2d0b6f6c1e11"
	cases := []struct {
		name string
		id   string
		pkg  KeptPackage
		ok   bool
	}{
		{"plain", id, KeptPackage{Name: "nosd", Version: "0.9.6~pre-alpha-1"}, true},
		{"epoch", id, KeptPackage{Name: "libc6", Version: "1:2.36-9"}, true},
		{"option as name", id, KeptPackage{Name: "-oAPT::x", Version: "1"}, false},
		{"path as version", id, KeptPackage{Name: "nosd", Version: "1/../../x"}, false},
		{"empty version", id, KeptPackage{Name: "nosd", Version: ""}, false},
		{"path as id", "../etc", KeptPackage{Name: "nosd", Version: "1"}, false},
	}
	for _, c := range cases {
		err := validKept(c.id, []KeptPackage{c.pkg})
		if (err == nil) != c.ok {
			t.Errorf("%s: err %v", c.name, err)
		}
	}
}

func TestKeepDebOrder(t *testing.T) {
	dir, archive := t.TempDir(), t.TempDir()
	oldArchive, oldArch, oldCmd := aptArchive, installedArch, keepCommand
	t.Cleanup(func() { aptArchive, installedArch, keepCommand = oldArchive, oldArch, oldCmd })
	aptArchive = archive
	installedArch = func(name string) string {
		if name == "gone" {
			return ""
		}
		return "amd64"
	}
	var ran []string
	keepCommand = func(_ string, _ time.Duration, name string, args ...string) error {
		ran = append(ran, name+" "+strings.Join(args, " "))
		if name == "apt-get" {
			return errors.New("not in the repository")
		}
		return nil
	}

	// In apt's cache: copied, nothing run.
	cached := debFileName("nosd", "1", "amd64")
	if err := os.WriteFile(filepath.Join(archive, cached), []byte("deb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := keepDeb(dir, KeptPackage{Name: "nosd", Version: "1"}); err != nil || !fileExists(filepath.Join(dir, cached)) || len(ran) != 0 {
		t.Fatalf("cache: %v %v", err, ran)
	}

	// Not cached and not downloadable: rebuilt with dpkg-repack.
	if err := keepDeb(dir, KeptPackage{Name: "curl", Version: "2"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ran, []string{"apt-get download curl=2", "dpkg-repack curl"}) {
		t.Fatalf("ran %v", ran)
	}

	if err := keepDeb(dir, KeptPackage{Name: "gone", Version: "1"}); err == nil {
		t.Fatal("a package that isn't installed can't be kept")
	}
}

func TestRevertCommandPassesFilesAsArguments(t *testing.T) {
	cmd := revertCommand([]string{"/var/lib/nos/update-debs/x/nosd_1_amd64.deb"})
	i := slices.Index(cmd, "/bin/sh")
	if i < 0 || cmd[i+1] != "-c" || cmd[i+2] != revertScript || cmd[len(cmd)-1] != "/var/lib/nos/update-debs/x/nosd_1_amd64.deb" {
		t.Fatalf("command: %v", cmd)
	}
	if !strings.Contains(revertScript, "--allow-downgrades") || !strings.Contains(revertScript, "systemctl restart --no-block nosd") {
		t.Fatal("the script must downgrade and restart nosd")
	}
}

func TestUpdatesHealth(t *testing.T) {
	oldActive, oldWeb, oldIndex := unitActive, webAnswers, webIndex
	t.Cleanup(func() { unitActive, webAnswers, webIndex = oldActive, oldWeb, oldIndex })
	index := filepath.Join(t.TempDir(), "index.html")
	_ = os.WriteFile(index, []byte("<html>"), 0o644)
	webIndex = index
	webAnswers = func() error { return nil }
	unitActive = func(string) string { return "active" }
	if h := updatesHealth(); !h.OK || len(h.Problems) != 0 {
		t.Fatalf("healthy: %+v", h)
	}

	unitActive = func(u string) string {
		if u == "caddy" {
			return "failed"
		}
		return "active"
	}
	webAnswers = func() error { return errors.New("connection refused") }
	h := updatesHealth()
	if h.OK || len(h.Problems) != 2 || h.Problems[0] != "caddy is failed" {
		t.Fatalf("unhealthy: %+v", h)
	}
}

func TestKeepDeleteRefusesBadIDs(t *testing.T) {
	old := keptDebsDir
	t.Cleanup(func() { keptDebsDir = old })
	keptDebsDir = t.TempDir()
	victim := filepath.Join(keptDebsDir, "..", "victim")
	_ = os.MkdirAll(victim, 0o755)
	t.Cleanup(func() { _ = os.RemoveAll(victim) })

	body, _ := json.Marshal(map[string]string{"id": ".."})
	rr := httptest.NewRecorder()
	handleUpdatesKeepDelete(rr, httptest.NewRequest(http.MethodPost, "/v1/updates/keep/delete", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest || !fileExists(victim) {
		t.Fatalf("code %d", rr.Code)
	}

	id := "0b7c3c55-8f0e-4e0b-9a39-2d0b6f6c1e11"
	_ = os.MkdirAll(filepath.Join(keptDebsDir, id), 0o755)
	body, _ = json.Marshal(map[string]string{"id": id})
	rr = httptest.NewRecorder()
	handleUpdatesKeepDelete(rr, httptest.NewRequest(http.MethodPost, "/v1/updates/keep/delete", bytes.NewReader(body)))
	if rr.Code != http.StatusOK || fileExists(filepath.Join(keptDebsDir, id)) {
		t.Fatalf("delete: %d", rr.Code)
	}
}
