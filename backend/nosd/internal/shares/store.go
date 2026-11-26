// Package shares stores the SMB/NFS share definitions (/etc/nos/shares.json).
//
// It is the only reader and writer of that file: the shares API, NithronSync
// and WebDAV share one Store. (There used to be three formats for the same
// file: a {version, items} object written at every start, an array written by
// the API, and a third Share type for sync that dropped the API's fields when
// it saved. On a fresh install the API could not load the file at all.)
package shares

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"nos/backend/nosd/internal/fsatomic"
)

// ErrNotFound is returned for an unknown share id.
var ErrNotFound = errors.New("share not found")

// Share is one SMB or NFS share.
type Share struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Protocol    string            `json:"protocol"` // smb, nfs
	Enabled     bool              `json:"enabled"`
	ReadOnly    bool              `json:"readOnly"`
	GuestAccess bool              `json:"guestAccess,omitempty"`
	Users       []string          `json:"users,omitempty"`
	Groups      []string          `json:"groups,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"` // NFS clients
	Options     map[string]string `json:"options,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`

	// NithronSync settings.
	SyncEnabled      bool     `json:"sync_enabled,omitempty"`
	SyncMaxSize      int64    `json:"sync_max_size,omitempty"` // bytes, 0 = unlimited
	SyncExclude      []string `json:"sync_exclude,omitempty"`  // glob patterns
	SyncAllowedUsers []string `json:"sync_allowed_users,omitempty"`
}

// clone returns a deep copy, so callers never share slices or maps with the
// store.
func (s Share) clone() Share {
	s.Users = slices.Clone(s.Users)
	s.Groups = slices.Clone(s.Groups)
	s.Hosts = slices.Clone(s.Hosts)
	s.SyncExclude = slices.Clone(s.SyncExclude)
	s.SyncAllowedUsers = slices.Clone(s.SyncAllowedUsers)
	if s.Options != nil {
		o := make(map[string]string, len(s.Options))
		for k, v := range s.Options {
			o[k] = v
		}
		s.Options = o
	}
	return s
}

// Store is safe for concurrent use. All methods return copies.
type Store struct {
	path   string
	mu     sync.RWMutex
	shares map[string]Share
}

// NewStore loads path (a missing file is an empty store).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, shares: map[string]Share{}}
	list, err := load(path)
	if err != nil {
		return nil, err
	}
	for _, sh := range list {
		s.shares[sh.ID] = sh
	}
	return s, nil
}

// load reads the current format (an array) and the {version, items} object
// that earlier versions created at startup, whose items used "type" and "ro"
// for the protocol and read-only flag.
func load(path string) ([]Share, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}
	if data[0] == '[' {
		var list []Share
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		return list, nil
	}
	var legacy struct {
		Items []struct {
			Share
			Type string `json:"type"`
			RO   bool   `json:"ro"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	list := make([]Share, 0, len(legacy.Items))
	for _, it := range legacy.Items {
		sh := it.Share
		if sh.Protocol == "" {
			sh.Protocol = it.Type
		}
		sh.ReadOnly = sh.ReadOnly || it.RO
		list = append(list, sh)
	}
	return list, nil
}

// List returns all shares sorted by name.
func (s *Store) List() []Share {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Share, 0, len(s.shares))
	for _, sh := range s.shares {
		out = append(out, sh.clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns one share.
func (s *Store) Get(id string) (Share, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sh, ok := s.shares[id]
	return sh.clone(), ok
}

// Create stores a new share under a new id (any id in sh is ignored).
func (s *Store) Create(sh Share) (Share, error) {
	now := time.Now().UTC()
	sh = sh.clone()
	sh.ID, sh.CreatedAt, sh.UpdatedAt = uuid.NewString(), now, now
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shares[sh.ID] = sh
	if err := s.saveLocked(); err != nil {
		delete(s.shares, sh.ID)
		return Share{}, err
	}
	return sh.clone(), nil
}

// Update applies u to share id: name, path, protocol and description when
// set, lists and options when non-nil, and the three flags always.
func (s *Store) Update(id string, u Share) (Share, error) {
	return s.modify(id, func(sh *Share) {
		if u.Name != "" {
			sh.Name = u.Name
		}
		if u.Path != "" {
			sh.Path = u.Path
		}
		if u.Protocol != "" {
			sh.Protocol = u.Protocol
		}
		if u.Description != "" {
			sh.Description = u.Description
		}
		sh.Enabled, sh.ReadOnly, sh.GuestAccess = u.Enabled, u.ReadOnly, u.GuestAccess
		if u.Users != nil {
			sh.Users = slices.Clone(u.Users)
		}
		if u.Groups != nil {
			sh.Groups = slices.Clone(u.Groups)
		}
		if u.Hosts != nil {
			sh.Hosts = slices.Clone(u.Hosts)
		}
		if u.Options != nil {
			sh.Options = u.clone().Options
		}
	})
}

// SetEnabled turns a share on or off.
func (s *Store) SetEnabled(id string, enabled bool) (Share, error) {
	return s.modify(id, func(sh *Share) { sh.Enabled = enabled })
}

// Delete removes a share.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.shares[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.shares, id)
	if err := s.saveLocked(); err != nil {
		s.shares[id] = old
		return err
	}
	return nil
}

// modify changes a copy of share id and stores it only if saving works.
func (s *Store) modify(id string, change func(*Share)) (Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.shares[id]
	if !ok {
		return Share{}, ErrNotFound
	}
	sh := old.clone()
	change(&sh)
	sh.UpdatedAt = time.Now().UTC()
	s.shares[id] = sh
	if err := s.saveLocked(); err != nil {
		s.shares[id] = old
		return Share{}, err
	}
	return sh.clone(), nil
}

func (s *Store) saveLocked() error {
	list := make([]Share, 0, len(s.shares))
	for _, sh := range s.shares {
		list = append(list, sh)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return fsatomic.SaveJSON(s.path, list, 0o600)
}
