package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

func TestSaveAtomicConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	// Seed with one user
	_ = s.UpsertUser(User{ID: "u0", Username: "admin", PasswordHash: "plain:x", Roles: []string{"admin"}})

	// Start concurrent writers
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u := User{ID: "u" + string(rune('a'+i)), Username: "user" + string(rune('a'+i)), PasswordHash: "plain:x"}
			_ = s.UpsertUser(u)
		}(i)
	}
	wg.Wait()

	// Ensure file exists and is valid JSON
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read users: %v", err)
	}
	var f dbFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}

func TestCrashRecoveryFromTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	s, _ := New(path)

	// Write a good file first
	good := dbFile{Version: 1, Users: []User{{ID: "x", Username: "x"}}}
	if err := fsatomic.SaveJSON(context.TODO(), path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	// Create a broken crash artifact
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Next load should ignore tmp and keep good file
	if err := s.load(); err != nil {
		t.Fatalf("load after tmp: %v", err)
	}
	// users.json should still exist
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("users.json missing: %v", err)
	}
}

// Regression (S11): deleting a user re-saved the other users and never
// removed the deleted one, and every admin was undeletable.
func TestDeleteUser(t *testing.T) {
	p := filepath.Join(t.TempDir(), "users.json")
	s, _ := New(p)
	for _, u := range []User{
		{ID: "a1", Username: "admin1", PasswordHash: "x", Roles: []string{"admin"}},
		{ID: "a2", Username: "admin2", PasswordHash: "x", Roles: []string{"admin"}},
		{ID: "u1", Username: "bob", PasswordHash: "x", Roles: []string{"user"}},
	} {
		if err := s.UpsertUser(u); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteUser("u1"); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := New(p)
	if _, err := reloaded.FindByID("u1"); err != ErrUserNotFound {
		t.Fatalf("bob should be gone after reload: %v", err)
	}
	if err := s.DeleteUser("nope"); err != ErrUserNotFound {
		t.Fatalf("missing user: %v", err)
	}
	if err := s.DeleteUser("a1"); err != nil {
		t.Fatalf("one of two admins: %v", err)
	}
	if err := s.DeleteUser("a2"); err != ErrLastAdmin {
		t.Fatalf("last admin must be kept: %v", err)
	}
	if !s.HasAdmin() {
		t.Fatal("an admin must remain")
	}
}
