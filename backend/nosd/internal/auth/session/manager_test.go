package session

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newManager(t *testing.T) (*Manager, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sessions.json")
	return New(p), p
}

func TestCreateAndActive(t *testing.T) {
	m, _ := newManager(t)
	rec, err := m.Create("u1", "Firefox", "192.168.1.20:5555", time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if rec.SID == "" || rec.RefreshID == "" {
		t.Fatalf("missing ids: %+v", rec)
	}
	if !m.Active(rec.SID, "u1") {
		t.Fatal("new session must be active")
	}
	if m.Active(rec.SID, "u2") {
		t.Fatal("session must not be active for another user")
	}
	if m.Active("nope", "u1") {
		t.Fatal("unknown session must not be active")
	}
	if rec.IPPrefix != "192.168.1.0" {
		t.Fatalf("ip prefix: %q", rec.IPPrefix)
	}
}

func TestIDsAreUnique(t *testing.T) {
	m, _ := newManager(t)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		rec, _ := m.Create("u1", "", "", time.Hour, false)
		if seen[rec.SID] || seen[rec.RefreshID] {
			t.Fatal("duplicate id")
		}
		seen[rec.SID], seen[rec.RefreshID] = true, true
	}
}

func TestExpiredSessionIsInactiveAndPruned(t *testing.T) {
	m, p := newManager(t)
	rec, _ := m.Create("u1", "", "", -time.Second, false)
	if m.Active(rec.SID, "u1") {
		t.Fatal("expired session must not be active")
	}
	if len(m.ListByUser("u1")) != 0 {
		t.Fatal("expired session listed")
	}
	_, _ = m.Create("u1", "", "", time.Hour, false) // any write prunes
	if got := len(New(p).ListByUser("u1")); got != 1 {
		t.Fatalf("after reload: %d sessions, want 1", got)
	}
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	m, _ := newManager(t)
	rec, _ := m.Create("u1", "", "", time.Hour, false)
	other, _ := m.Create("u1", "", "", time.Hour, false)

	next, err := m.Refresh(rec.SID, "u1", rec.RefreshID, time.Hour)
	if err != nil || next.RefreshID == rec.RefreshID {
		t.Fatalf("rotate: %v %+v", err, next)
	}
	// Within the grace window the previous id still works (two tabs
	// refreshing at once) and yields the current one.
	again, err := m.Refresh(rec.SID, "u1", rec.RefreshID, time.Hour)
	if err != nil || again.RefreshID != next.RefreshID {
		t.Fatalf("grace: %v %+v", err, again)
	}
	// Outside the window, an old id means the token was copied: every
	// session of the user ends.
	m.mu.Lock()
	r := m.sessions[rec.SID]
	r.RotatedAt = time.Now().Add(-time.Hour)
	m.sessions[rec.SID] = r
	m.mu.Unlock()
	if _, err := m.Refresh(rec.SID, "u1", rec.RefreshID, time.Hour); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("reuse: got %v", err)
	}
	if m.Active(rec.SID, "u1") || m.Active(other.SID, "u1") {
		t.Fatal("reuse must revoke all of the user's sessions")
	}
}

func TestRefreshRejectsRevokedOrForeignSession(t *testing.T) {
	m, _ := newManager(t)
	rec, _ := m.Create("u1", "", "", time.Hour, false)
	if _, err := m.Refresh(rec.SID, "u2", rec.RefreshID, time.Hour); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("foreign: got %v", err)
	}
	_ = m.RevokeSID(rec.SID)
	if _, err := m.Refresh(rec.SID, "u1", rec.RefreshID, time.Hour); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("revoked: got %v", err)
	}
}

func TestRevokeAllAndPersistence(t *testing.T) {
	m, p := newManager(t)
	a, _ := m.Create("u1", "", "", time.Hour, false)
	b, _ := m.Create("u2", "", "", time.Hour, false)
	if err := m.RevokeAll("u1"); err != nil {
		t.Fatal(err)
	}
	reloaded := New(p)
	if reloaded.Active(a.SID, "u1") {
		t.Fatal("revoked session came back after reload")
	}
	if !reloaded.Active(b.SID, "u2") {
		t.Fatal("other user's session lost")
	}
}
