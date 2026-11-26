package shares

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadsLegacyObjectFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shares.json")
	// What earlier versions wrote at every start (empty), and what the old
	// sync-side Share type looked like.
	if err := os.WriteFile(p, []byte(`{"version":1,"items":[{"id":"a","name":"media","path":"/srv/media","type":"nfs","ro":true,"users":["bob"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	sh, ok := s.Get("a")
	if !ok || sh.Protocol != "nfs" || !sh.ReadOnly || sh.Users[0] != "bob" {
		t.Fatalf("legacy share: %+v", sh)
	}

	if err := os.WriteFile(p, []byte(`{"version":1,"items":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(p); err != nil {
		t.Fatalf("empty legacy file must load: %v", err)
	}
}

func TestCRUDPersistsAndReturnsCopies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shares.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(Share{ID: "../../evil", Name: "media", Path: "/srv/media", Protocol: "smb", Users: []string{"alice"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "../../evil" || created.ID == "" {
		t.Fatalf("id must be assigned by the store: %q", created.ID)
	}

	// Changing a returned share must not change the store.
	got, _ := s.Get(created.ID)
	got.Users[0] = "mallory"
	got.Name = "changed"
	if again, _ := s.Get(created.ID); again.Users[0] != "alice" || again.Name != "media" {
		t.Fatalf("store changed through a returned copy: %+v", again)
	}

	before, _ := s.Get(created.ID)
	updated, err := s.Update(created.ID, Share{Protocol: "nfs", Enabled: true})
	if err != nil || updated.Protocol != "nfs" || !updated.Enabled || updated.Name != "media" {
		t.Fatalf("update: %v %+v", err, updated)
	}
	if before.Protocol != "smb" {
		t.Fatal("the share read before the update must keep the old protocol")
	}

	if _, err := s.SetEnabled(created.ID, false); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if sh, ok := reloaded.Get(created.ID); !ok || sh.Enabled || sh.Protocol != "nfs" {
		t.Fatalf("after reload: %+v", sh)
	}

	if err := s.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.Update("nope", Share{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}
}
