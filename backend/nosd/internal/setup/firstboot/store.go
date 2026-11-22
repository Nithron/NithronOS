// Package firstboot owns the one-time setup code (OTP) that lets the first
// administrator be created on a fresh NithronOS install.
//
// nosd is the only component that creates, rotates, displays or removes the
// code. The code lives in one JSON file (Store), is replaced automatically when
// it expires or after too many wrong guesses (Manager), and is shown on the
// local console through an agetty issue snippet (see issue.go).
package firstboot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is the persisted first-boot state.
type State struct {
	OTP       string    `json:"otp"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	// Attempts counts wrong guesses against this code.
	Attempts int `json:"attempts"`
}

// Expired reports whether the code is past its expiry at now.
func (s *State) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt) }

// Store reads and writes the state file atomically.
type Store struct {
	path string
}

// New returns a Store backed by path.
func New(path string) *Store { return &Store{path: path} }

// Path returns the state file path.
func (s *Store) Path() string { return s.path }

// Load returns the stored state, or nil if there is none. Expired state is
// returned as-is; callers decide what to do with it. Files written by older
// releases ({otp, created_at, used}) are read as a code issued at created_at
// with the default TTL; a "used" legacy file counts as no state.
func (s *Store) Load() (*State, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var raw struct {
		OTP       string    `json:"otp"`
		IssuedAt  time.Time `json:"issued_at"`
		ExpiresAt time.Time `json:"expires_at"`
		Attempts  int       `json:"attempts"`
		// legacy schema
		CreatedAt string `json:"created_at"`
		Used      bool   `json:"used"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	if raw.Used || raw.OTP == "" {
		return nil, nil
	}
	st := &State{OTP: raw.OTP, IssuedAt: raw.IssuedAt, ExpiresAt: raw.ExpiresAt, Attempts: raw.Attempts}
	if st.ExpiresAt.IsZero() {
		t, perr := time.Parse(time.RFC3339, raw.CreatedAt)
		if perr != nil {
			// No usable timestamps: treat as expired so it gets replaced.
			return &State{OTP: raw.OTP}, nil
		}
		st.IssuedAt = t
		st.ExpiresAt = t.Add(DefaultTTL)
	}
	return st, nil
}

// Save writes st atomically (temp file, fsync, rename) with mode 0600.
func (s *Store) Save(st *State) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".firstboot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// Remove deletes the state file. A missing file is not an error.
func (s *Store) Remove() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) String() string { return fmt.Sprintf("Store(%s)", s.path) }
