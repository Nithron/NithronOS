package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// A file written right after its folders are made is seen (the folders are
// watched only once they exist, so it would be missed otherwise).
func TestNewFolderContentsAreSeen(t *testing.T) {
	root := t.TempDir()
	w, err := New(DefaultConfig(root), zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	file := filepath.Join(root, "notes", "2026", "todo.txt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == file {
				return
			}
		case <-deadline:
			t.Fatal("no event for a file written right after its folders were made")
		}
	}
}
