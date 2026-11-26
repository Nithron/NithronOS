package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nos/backend/nosd/internal/fsatomic"
)

type User struct {
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	PasswordHash   string   `json:"password_hash"`
	Roles          []string `json:"roles"`
	TOTPEnc        string   `json:"totp_enc"`
	RecoveryHashes []string `json:"recovery_hashes"`
	// Pending two-factor enrollment: becomes TOTPEnc/RecoveryHashes once a
	// valid code confirms it.
	TOTPPendingEnc        string   `json:"totp_pending_enc,omitempty"`
	RecoveryPendingHashes []string `json:"recovery_pending_hashes,omitempty"`
	CreatedAt             string   `json:"created_at"`
	UpdatedAt             string   `json:"updated_at"`
	LastLoginAt           string   `json:"last_login_at"`
	FailedAttempts        int      `json:"failed_attempts"`
	LockedUntil           string   `json:"locked_until"`
}

type dbFile struct {
	Version int    `json:"version"`
	Users   []User `json:"users"`
}

var (
	ErrUserNotFound = errors.New("user not found")
)

// Store is the user database (users.json). Several instances, and tools
// outside nosd, may use the same file: every read reloads the file when it
// changed on disk, and every write is reload, change and save under the
// file lock, so no instance works from a stale copy or undoes another's
// change. A file that can't be read is never overwritten.
type Store struct {
	path    string
	mu      sync.Mutex
	users   map[string]User // by username
	seen    os.FileInfo     // the file as last loaded; nil if missing
	loadErr error           // why the file on disk could not be read
	ioMu    sync.Mutex      // serialize writers within this process
}

// ErrUnreadable means users.json exists but can't be read; writes are refused
// so the accounts in it are not lost.
var ErrUnreadable = errors.New("users file is unreadable; not overwriting it")

// New opens the store at path. A missing file is an empty store; an
// unreadable one is reported by every write (see ErrUnreadable).
func New(path string) (*Store, error) {
	s := &Store{path: path, users: map[string]User{}}
	s.mu.Lock()
	s.refreshLocked()
	s.mu.Unlock()
	return s, nil
}

// refreshLocked reloads the file if it changed since it was last loaded.
// Writes replace the file (rename), so any write changes its identity.
func (s *Store) refreshLocked() {
	fi, err := os.Stat(s.path)
	if err != nil {
		fi = nil
	}
	if sameFile(fi, s.seen) {
		return
	}
	users, err := readUsers(s.path)
	s.seen = fi
	if err != nil {
		s.loadErr = err // keep the last good copy for reads
		return
	}
	s.users, s.loadErr = users, nil
}

func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.ModTime().Equal(b.ModTime()) && a.Size() == b.Size()
}

// readUsers loads path. A missing or empty file, or one without users and
// without a version ("{}"), is an empty store.
func readUsers(path string) (map[string]User, error) {
	users := map[string]User{}
	if raw, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(raw)) == 0 {
		return users, nil
	}
	var f dbFile
	ok, err := fsatomic.LoadJSON(path, &f)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !ok || (f.Version == 0 && len(f.Users) == 0) {
		return users, nil
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported users db version %d", ErrUnreadable, f.Version)
	}
	for _, u := range f.Users {
		users[u.Username] = u
	}
	return users, nil
}

// read runs fn on the current users.
func (s *Store) read(fn func(map[string]User)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshLocked()
	fn(s.users)
}

// update applies change to a copy of the current users and saves it.
func (s *Store) update(change func(map[string]User) error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	return fsatomic.WithLock(s.path, func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.refreshLocked()
		if s.loadErr != nil {
			return s.loadErr
		}
		next := make(map[string]User, len(s.users)+1)
		for k, v := range s.users {
			next[k] = v
		}
		if err := change(next); err != nil {
			return err
		}
		if err := writeUsers(s.path, next); err != nil {
			return err
		}
		s.users = next
		if fi, err := os.Stat(s.path); err == nil {
			s.seen = fi
		}
		return nil
	})
}

func writeUsers(path string, users map[string]User) error {
	// Test hook: simulate write failure for transactional tests
	if os.Getenv("NOS_TEST_SIMULATE_WRITE_FAIL") == "1" {
		return &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission}
	}
	list := make([]User, 0, len(users))
	for _, u := range users {
		list = append(list, u)
	}
	return fsatomic.SaveJSON(path, dbFile{Version: 1, Users: list}, fs.FileMode(0o600))
}

func (s *Store) HasAdmin() bool {
	has := false
	s.read(func(users map[string]User) {
		for _, u := range users {
			if isAdminUser(u) {
				has = true
				return
			}
		}
	})
	return has
}

func (s *Store) FindByUsername(username string) (User, error) {
	var (
		u  User
		ok bool
	)
	s.read(func(users map[string]User) { u, ok = users[username] })
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (s *Store) FindByID(id string) (User, error) {
	var (
		found User
		ok    bool
	)
	s.read(func(users map[string]User) {
		for _, u := range users {
			if u.ID == id {
				found, ok = u, true
				return
			}
		}
	})
	if !ok {
		return User{}, ErrUserNotFound
	}
	return found, nil
}

func (s *Store) List() ([]User, error) {
	var list []User
	s.read(func(users map[string]User) {
		list = make([]User, 0, len(users))
		for _, u := range users {
			list = append(list, u)
		}
	})
	return list, nil
}

// UpsertUser creates or replaces the user with u's username.
func (s *Store) UpsertUser(u User) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if u.CreatedAt == "" {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	return s.update(func(users map[string]User) error {
		users[u.Username] = u
		return nil
	})
}

// DeleteUser removes the user with the given ID. It returns ErrUserNotFound
// when there is no such user, and refuses (ErrLastAdmin) to delete the only
// remaining administrator.
func (s *Store) DeleteUser(id string) error {
	return s.update(func(users map[string]User) error {
		var victim User
		found, admins := false, 0
		for _, u := range users {
			if isAdminUser(u) {
				admins++
			}
			if u.ID == id {
				victim, found = u, true
			}
		}
		if !found {
			return ErrUserNotFound
		}
		if isAdminUser(victim) && admins <= 1 {
			return ErrLastAdmin
		}
		delete(users, victim.Username)
		return nil
	})
}

// ErrLastAdmin is returned when an operation would leave no administrator.
var ErrLastAdmin = errors.New("cannot remove the last administrator")

func isAdminUser(u User) bool {
	for _, r := range u.Roles {
		if r == "admin" {
			return true
		}
	}
	return false
}
