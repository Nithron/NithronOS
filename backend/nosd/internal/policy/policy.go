// Package policy stores system-wide rules that admins set
// (/etc/nos/policy.json).
package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"nos/backend/nosd/internal/fsatomic"
)

// Policy is the whole policy document.
type Policy struct {
	Shares Shares `json:"shares"`
}

// Shares are the rules for shares.
type Shares struct {
	// GuestAccessForbidden refuses shares that SMB guests can open.
	GuestAccessForbidden bool `json:"guest_access_forbidden"`
}

// Store keeps the policy in memory and on disk.
type Store struct {
	path string
	mu   sync.RWMutex
	cur  Policy
}

// NewStore loads path (a missing or empty file is the default policy).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &s.cur); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	return s, nil
}

// Get returns the policy.
func (s *Store) Get() Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Set replaces the policy.
func (s *Store) Set(p Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fsatomic.SaveJSON(s.path, p, 0o600); err != nil {
		return err
	}
	s.cur = p
	return nil
}
