package ratelimit

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistenceFixedWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ratelimit.json")
	s := New(path)
	key := "test:ip:1.2.3.4"
	// limit 1 per window; the window must be long enough that a slow CI
	// machine can't let it lapse between the calls below.
	const window = 2 * time.Second
	ok, _, _ := s.Allow(key, 1, window)
	if !ok {
		t.Fatal("first allow should pass")
	}
	ok, _, _ = s.Allow(key, 1, window)
	if ok {
		t.Fatal("second allow should be limited")
	}
	// ensure persisted state before simulating restart
	_ = s.Flush()
	// simulate restart by reloading store
	s2 := New(path)
	ok2, _, reset := s2.Allow(key, 1, window)
	if ok2 {
		t.Fatal("expected persisted limit to remain after restart")
	}
	// wait for window
	time.Sleep(time.Until(reset) + 10*time.Millisecond)
	ok3, _, _ := s2.Allow(key, 2, window)
	if !ok3 {
		t.Fatal("expected allow after window reset")
	}
}
