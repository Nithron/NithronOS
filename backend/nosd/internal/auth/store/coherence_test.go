package store

import (
	"os"
	"path/filepath"
	"testing"
)

// Several Store instances (and tools outside nosd) use the same users.json.
// Each must see the others' writes, and a write must never undo them.
func TestInstancesSeeEachOthersWrites(t *testing.T) {
	p := filepath.Join(t.TempDir(), "users.json")
	a, _ := New(p)
	b, _ := New(p)

	if err := a.UpsertUser(User{ID: "1", Username: "alice", Roles: []string{"admin"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.FindByID("1"); err != nil {
		t.Fatalf("b does not see a's new user: %v", err)
	}
	if !b.HasAdmin() {
		t.Fatal("b does not see a's admin")
	}

	// b changes alice (e.g. a password reset through the recovery API) ...
	u, _ := b.FindByUsername("alice")
	u.PasswordHash = "new"
	if err := b.UpsertUser(u); err != nil {
		t.Fatal(err)
	}
	// ... and a's next write, for another user, must not revert it.
	if err := a.UpsertUser(User{ID: "2", Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := New(p)
	if got, _ := fresh.FindByUsername("alice"); got.PasswordHash != "new" {
		t.Fatalf("a's write reverted b's change: %q", got.PasswordHash)
	}
	if _, err := fresh.FindByUsername("bob"); err != nil {
		t.Fatal("bob lost")
	}

	if err := b.DeleteUser("2"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FindByID("2"); err == nil {
		t.Fatal("a still sees the user b deleted")
	}
}

// A users.json that can't be read must never be replaced: before, the store
// started empty and its next write wiped every account.
func TestUnreadableFileIsNotOverwritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "users.json")
	garbage := []byte(`{"version": 1, "users": [ {"id": "1", "username": `)
	if err := os.WriteFile(p, garbage, 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := New(p)
	if err := s.UpsertUser(User{ID: "9", Username: "mallory", Roles: []string{"admin"}}); err == nil {
		t.Fatal("write over an unreadable users file must fail")
	}
	if got, _ := os.ReadFile(p); string(got) != string(garbage) {
		t.Fatalf("unreadable users file was overwritten: %s", got)
	}
	// Once the file is repaired, the store picks it up.
	if err := os.WriteFile(p, []byte(`{"version":1,"users":[{"id":"1","username":"alice","roles":["admin"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FindByID("1"); err != nil {
		t.Fatalf("repaired file not loaded: %v", err)
	}
	if err := s.UpsertUser(User{ID: "2", Username: "bob"}); err != nil {
		t.Fatalf("write after repair: %v", err)
	}
}

func TestEmptyFilesAreAnEmptyStore(t *testing.T) {
	for _, content := range []string{"", "  \n", "{}"} {
		p := filepath.Join(t.TempDir(), "users.json")
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		s, _ := New(p)
		if err := s.UpsertUser(User{ID: "1", Username: "alice"}); err != nil {
			t.Fatalf("%q: %v", content, err)
		}
	}
}
