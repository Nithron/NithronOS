package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// WebDAV can't follow a link out of the share.
func TestShareFileSystemStaysInShare(t *testing.T) {
	dir := t.TempDir()
	share := filepath.Join(dir, "share")
	secret := filepath.Join(dir, "etc-nos")
	for _, d := range []string{share, secret} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(secret, "users.json"), []byte("hashes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(share, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(share, "out")); err != nil {
		t.Fatal(err)
	}
	fs, ctx := &shareFileSystem{basePath: share}, context.Background()

	if f, err := fs.OpenFile(ctx, "/ok.txt", os.O_RDONLY, 0); err != nil {
		t.Fatalf("a file in the share: %v", err)
	} else {
		f.Close()
	}
	if _, err := fs.OpenFile(ctx, "/out/users.json", os.O_RDONLY, 0); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("through the link: %v", err)
	}
	if _, err := fs.Stat(ctx, "/out"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("stat the link: %v", err)
	}
	if err := fs.Rename(ctx, "/ok.txt", "/out/stolen.txt"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("move out through the link: %v", err)
	}
	if err := fs.RemoveAll(ctx, "/"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("remove the share: %v", err)
	}
	// ".." stays in the share: this is the share's ok.txt.
	if _, err := fs.Stat(ctx, "/../../ok.txt"); err != nil {
		t.Fatalf("dot-dot: %v", err)
	}
}
