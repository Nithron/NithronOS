package server

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDelta(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(target, []byte("hello brave world"), 0o640); err != nil {
		t.Fatal(err)
	}
	want := []byte("hello new world!")
	sum := sha256.Sum256(want)
	ops := []DeltaOp{{Offset: 0, Size: 6}, {Data: []byte("new")}, {Offset: 11, Size: 6}, {Data: []byte("!")}}
	n, err := applyDelta(target, ops, sum[:])
	if err != nil || n != int64(len(want)) {
		t.Fatalf("apply: %d %v", n, err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(want) {
		t.Fatalf("got %q", got)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}

	// A result that doesn't hash right changes nothing.
	wrong := sha256.Sum256([]byte("something else"))
	if _, err := applyDelta(target, []DeltaOp{{Data: []byte("x")}}, wrong[:]); !errors.Is(err, errDeltaMismatch) {
		t.Fatalf("mismatch: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != string(want) {
		t.Fatalf("file changed on a mismatch: %q", got)
	}
	// Copies outside the current file are refused.
	for _, bad := range [][]DeltaOp{{{Offset: 10, Size: 100}}, {{Offset: -1, Size: 2}}, {{Offset: 0, Size: 0}}} {
		if _, err := applyDelta(target, bad, sum[:]); err == nil || errors.Is(err, errDeltaMismatch) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	// No temporary files are left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
	if _, err := applyDelta(filepath.Join(dir, "missing"), ops, sum[:]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing base: %v", err)
	}
}
