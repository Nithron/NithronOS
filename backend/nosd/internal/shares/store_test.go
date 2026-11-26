package shares

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadsLegacyFormats(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shares.json")
	// The oldest format: a {version, items} object with "type" and "ro".
	if err := os.WriteFile(p, []byte(`{"version":1,"items":[{"id":"a","name":"media","path":"/srv/media","type":"nfs","enabled":true,"ro":true,"users":["bob"],"hosts":["10.0.0.0/8"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	sh, ok := s.Get("a")
	if !ok || !sh.NFSEnabled() || !sh.NFS.ReadOnly || sh.NFS.Networks[0] != "10.0.0.0/8" || sh.SMB != nil {
		t.Fatalf("legacy nfs share: %+v", sh)
	}
	if len(sh.Readers) != 1 || sh.Readers[0] != "user:bob" || len(sh.Owners) != 0 {
		t.Fatalf("legacy read-only users must become readers: %+v", sh)
	}

	// The array format of the one-protocol API.
	if err := os.WriteFile(p, []byte(`[{"id":"b","name":"docs","path":"/srv/shares/docs","protocol":"smb","enabled":true,"guestAccess":true,"users":["alice"],"groups":["staff"],"createdAt":"2026-01-02T03:04:05Z"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	sh, _ = s.Get("b")
	if !sh.SMBEnabled() || !sh.SMB.Guest || sh.NFS != nil || sh.CreatedAt.Year() != 2026 {
		t.Fatalf("legacy smb share: %+v", sh)
	}
	if strings.Join(sh.Owners, ",") != "user:alice,group:staff" {
		t.Fatalf("legacy users/groups must become owners: %v", sh.Owners)
	}

	// No users meant no restriction: it becomes everyone.
	if err := os.WriteFile(p, []byte(`[{"id":"c","name":"open","path":"/srv/shares/open","protocol":"smb","enabled":true}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ = NewStore(p)
	if sh, _ := s.Get("c"); len(sh.Owners) != 1 || sh.Owners[0] != Everyone {
		t.Fatalf("legacy share without users: %+v", sh.Owners)
	}

	for _, empty := range []string{`{"version":1,"items":[]}`, `[]`, ``} {
		if err := os.WriteFile(p, []byte(empty), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewStore(p); err != nil {
			t.Fatalf("%q must load: %v", empty, err)
		}
	}
}

func TestCRUDPersistsAndReturnsCopies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shares.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(Share{ID: "../../evil", Name: "media", Path: "/srv/shares/media", SMB: &SMB{Enabled: true}, Owners: []string{"user:alice"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "../../evil" || created.ID == "" {
		t.Fatalf("id must be assigned by the store: %q", created.ID)
	}
	if _, err := s.Create(Share{Name: "Media"}); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate name (Samba ignores case): %v", err)
	}

	// Changing a returned share must not change the store.
	got, _ := s.Lookup("media")
	got.Owners[0] = "user:mallory"
	got.SMB.Enabled = false
	if again, _ := s.Get(created.ID); again.Owners[0] != "user:alice" || !again.SMBEnabled() {
		t.Fatalf("store changed through a returned copy: %+v", again)
	}

	updated, err := s.Modify(created.ID, func(sh *Share) error {
		sh.ID = "other"
		sh.NFS = &NFS{Enabled: true, Networks: []string{"10.0.0.0/8"}}
		return nil
	})
	if err != nil || updated.ID != created.ID || !updated.NFSEnabled() || !updated.SMBEnabled() {
		t.Fatalf("modify: %v %+v", err, updated)
	}
	if _, err := s.Modify(created.ID, func(*Share) error { return errors.New("no") }); err == nil {
		t.Fatal("a failing change must not be stored")
	}

	reloaded, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if sh, ok := reloaded.Lookup(created.ID); !ok || !sh.NFSEnabled() || sh.Name != "media" {
		t.Fatalf("after reload: %+v", sh)
	}

	other, _ := s.Create(Share{Name: "other"})
	if _, err := s.Modify(other.ID, func(sh *Share) error { sh.Name = "media"; return nil }); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto a taken name: %v", err)
	}

	if err := s.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.Modify("nope", func(*Share) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("modify unknown: %v", err)
	}
}

func TestAccess(t *testing.T) {
	sh := Share{Owners: []string{"user:alice", "group:editors"}, Readers: []string{"group:family", "bogus"}}
	cases := []struct {
		user        string
		groups      []string
		read, write bool
	}{
		{"alice", nil, true, true},
		{"bob", []string{"editors"}, true, true},
		{"carol", []string{"family"}, true, false},
		{"dave", []string{"other"}, false, false},
		{"bogus", nil, false, false},
	}
	for _, c := range cases {
		r, w := sh.Access(c.user, c.groups)
		if r != c.read || w != c.write {
			t.Errorf("%s %v: got read=%v write=%v", c.user, c.groups, r, w)
		}
	}
	if r, w := (Share{}).Access("anyone", nil); r || w {
		t.Error("a share without owners and readers is closed")
	}
	if r, w := (Share{Readers: []string{Everyone}}).Access("anyone", nil); !r || w {
		t.Error("group:users is every user")
	}
}
