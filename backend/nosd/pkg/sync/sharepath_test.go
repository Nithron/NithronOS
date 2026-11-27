package sync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInShare(t *testing.T) {
	dir := t.TempDir()
	share := filepath.Join(dir, "media")
	if err := os.MkdirAll(filepath.Join(share, "films"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "secret")
	if err := os.MkdirAll(secret, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(share, "out")); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"films/a.mkv":              filepath.Join(share, "films/a.mkv"),
		"/films/a.mkv":             filepath.Join(share, "films/a.mkv"),
		"../../../etc/passwd":      filepath.Join(share, "etc/passwd"),
		"films/../../secret/x":     filepath.Join(share, "secret/x"),
		"new/folder/not/there.txt": filepath.Join(share, "new/folder/not/there.txt"),
	} {
		got, err := ResolveInShare(share, rel)
		if err != nil || got != want {
			t.Errorf("%q: %q %v, want %q", rel, got, err, want)
		}
	}
	for _, rel := range []string{"out/x", "out", "films/\x00"} {
		if _, err := ResolveInShare(share, rel); !errors.Is(err, ErrOutsideShare) {
			t.Errorf("%q: %v, want ErrOutsideShare", rel, err)
		}
	}
}
