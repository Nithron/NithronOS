package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func changeTypes(set *ChangeSet) map[string]FileChangeType {
	out := map[string]FileChangeType{}
	for _, c := range set.Changes {
		out[c.Path] = c.Type
	}
	return out
}

func TestChangesSinceSnapshot(t *testing.T) {
	share := t.TempDir()
	ct := NewChangeTracker(zerolog.Nop(), DefaultChangeTrackerConfig())
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(share, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a")
	write("b.txt", "b")
	write(".nos-delta-123", "half a file") // a delta upload being rebuilt

	first, err := ct.Changes(share, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := changeTypes(first); len(got) != 2 || got["a.txt"] != ChangeTypeCreate || got["b.txt"] != ChangeTypeCreate {
		t.Fatalf("first listing: %v", got)
	}

	// A file whose size and time are unchanged isn't read again: a cached
	// hash is reused (a forged cache entry proves it).
	full := filepath.Join(share, "a.txt")
	info, _ := os.Stat(full)
	ct.hashes[full] = cachedHash{size: info.Size(), mtime: info.ModTime().UnixNano(), hash: "cached"}
	again, _ := ct.Changes(share, first.Next, 0)
	if got := changeTypes(again); got["a.txt"] != ChangeTypeModify || again.Changes[0].Hash != "cached" {
		t.Fatalf("cache not used: %+v", again.Changes)
	}
	delete(ct.hashes, full)

	// Edit, delete, create.
	write("a.txt", "a2")
	later := time.Now().Add(time.Second)
	_ = os.Chtimes(full, later, later)
	_ = os.Remove(filepath.Join(share, "b.txt"))
	write("c.txt", "c")
	next, _ := ct.Changes(share, first.Next, 0)
	got := changeTypes(next)
	if got["a.txt"] != ChangeTypeModify || got["b.txt"] != ChangeTypeDelete || got["c.txt"] != ChangeTypeCreate || len(got) != 3 {
		t.Fatalf("after edits: %v", got)
	}
}

// A list cut at the limit continues next time from where it stopped.
func TestChangesPastTheLimit(t *testing.T) {
	share := t.TempDir()
	ct := NewChangeTracker(zerolog.Nop(), DefaultChangeTrackerConfig())
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		if err := os.WriteFile(filepath.Join(share, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	var since map[string]string
	for round := 0; round < 5; round++ {
		set, err := ct.Changes(share, since, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range set.Changes {
			seen[c.Path] = true
		}
		since = set.Next
		if !set.HasMore {
			break
		}
	}
	if len(seen) != 5 {
		t.Fatalf("saw %v, want all 5 files", seen)
	}
}

func TestCursorStore(t *testing.T) {
	cs, err := NewCursorStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"a.txt": "h1"}
	id, err := cs.Save("dev1", "s1", files)
	if err != nil || len(id) != 32 {
		t.Fatalf("save: %q %v", id, err)
	}
	if got, ok := cs.Load("dev1", "s1", id); !ok || got["a.txt"] != "h1" {
		t.Fatalf("load: %v %v", got, ok)
	}
	// Another device's, an older and an empty cursor aren't this one.
	if _, ok := cs.Load("dev2", "s1", id); ok {
		t.Fatal("another device's snapshot")
	}
	id2, _ := cs.Save("dev1", "s1", map[string]string{})
	if _, ok := cs.Load("dev1", "s1", id); ok {
		t.Fatal("an older cursor still loads")
	}
	if got, ok := cs.Load("dev1", "s1", id2); !ok || len(got) != 0 {
		t.Fatalf("latest: %v %v", got, ok)
	}
	if _, ok := cs.Load("dev1", "s1", ""); ok {
		t.Fatal("empty cursor")
	}
}
