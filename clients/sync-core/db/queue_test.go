package db

import (
	"path/filepath"
	"testing"
)

// A share's dequeue returns only that share's operations.
func TestDequeueIsPerShare(t *testing.T) {
	d, err := OpenPath(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, op := range []struct{ share, path string }{{"docs", "a.txt"}, {"media", "b.mkv"}, {"docs", "c.txt"}} {
		if err := d.EnqueueOperation(op.share, op.path, "upload", 0); err != nil {
			t.Fatal(err)
		}
	}
	ops, err := d.DequeueOperations("docs", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || ops[0].ShareID != "docs" || ops[1].ShareID != "docs" {
		t.Fatalf("docs: %+v", ops)
	}
	if ops, _ := d.DequeueOperations("docs", 10); len(ops) != 0 {
		t.Fatalf("docs again: %+v", ops)
	}
	if ops, _ := d.DequeueOperations("media", 10); len(ops) != 1 || ops[0].Path != "b.mkv" {
		t.Fatalf("media: %+v", ops)
	}
}
