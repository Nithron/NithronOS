package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"nos/clients/sync-core/api"
	"nos/clients/sync-core/db"
	"nos/clients/sync-core/hash"
)

// What a server change to a file that exists locally leads to.
func TestNeedsDownload(t *testing.T) {
	dir := t.TempDir()
	database, err := db.OpenPath(filepath.Join(dir, "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	e := &Engine{database: database, logger: zerolog.Nop()}
	local := filepath.Join(dir, "doc.txt")
	write := func(s string) (os.FileInfo, string) {
		if err := os.WriteFile(local, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(local)
		return info, hash.BytesHash([]byte(s))
	}
	change := func(content string) api.FileChange {
		return api.FileChange{Path: "doc.txt", Type: api.ChangeModify, Hash: hash.BytesHash([]byte(content))}
	}

	// Synced at v1; then edited here (v2, an edit the quick hash can't see).
	info, v1 := write("v1")
	e.recordSynced("s", "doc.txt", info, v1)
	info, _ = write("v2")
	// The server reports v1 again (this device's own upload): keep v2.
	if dl, err := e.needsDownload("s", local, info, change("v1")); err != nil || dl {
		t.Fatalf("own version back: download=%v %v", dl, err)
	}
	// The server now has v2 too: recorded, nothing to download.
	if dl, err := e.needsDownload("s", local, info, change("v2")); err != nil || dl {
		t.Fatalf("same content: download=%v %v", dl, err)
	}
	if entry, _ := database.GetFile("s", "doc.txt"); entry.RemoteHash != hash.BytesHash([]byte("v2")) {
		t.Fatalf("not recorded: %+v", entry)
	}
	// Someone else wrote v3 and v2 is unchanged here: download.
	if dl, err := e.needsDownload("s", local, info, change("v3")); err != nil || !dl {
		t.Fatalf("remote edit: download=%v %v", dl, err)
	}
}
