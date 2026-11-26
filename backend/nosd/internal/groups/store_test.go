package groups

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroups(t *testing.T) {
	p := filepath.Join(t.TempDir(), "groups.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "Users", "users", "sudo;rm", strings.Repeat("a", 29), "-x"} {
		if _, err := s.Create(Group{Name: bad}); !errors.Is(err, ErrBadName) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	g, err := s.Create(Group{Name: "media", Members: []string{"bob", "alice", "bob"}})
	if err != nil || strings.Join(g.Members, ",") != "alice,bob" {
		t.Fatalf("create: %v %+v", err, g)
	}
	if _, err := s.Create(Group{Name: "media"}); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := s.Create(Group{Name: "family", Members: []string{"bob"}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Of("bob"), ","); got != "family,media" {
		t.Fatalf("groups of bob: %s", got)
	}
	if err := s.RemoveMember("bob"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Of("bob")) != 0 {
		t.Fatal("bob still in a group after RemoveMember")
	}
	if g, _ := reloaded.Get("media"); strings.Join(g.Members, ",") != "alice" {
		t.Fatalf("media after reload: %+v", g)
	}
	if _, err := s.Update("media", "Films", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.Get("media"); g.Members == nil || len(g.Members) != 0 || g.Description != "Films" {
		t.Fatalf("update: %+v", g)
	}
	if err := s.Delete("media"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("media"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}
