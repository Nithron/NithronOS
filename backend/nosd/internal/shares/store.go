// Package shares stores the share definitions (/etc/nos/shares.json).
//
// A share is a folder (by default /srv/shares/<name>) that can be exported
// over SMB and NFS at the same time. Owners may read and write it, readers
// only read it; principals are "user:<name>" or "group:<name>". nosd's shares
// API, NithronSync and WebDAV all use one Store; nos-agent turns the list
// into Samba and NFS configuration and file ACLs.
//
// Earlier versions stored one share per protocol ({protocol, readOnly,
// guestAccess, users, hosts}), first inside a {version, items} object and then
// as a bare array. Both still load and are converted.
package shares

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nos/backend/nosd/internal/fsatomic"
)

// Root is where new shares are created: /srv/shares/<name>.
const Root = "/srv/shares"

// DefaultRecycleDir is the recycle bin folder inside an SMB share.
const DefaultRecycleDir = ".recycle"

var (
	// ErrNotFound is returned for an unknown share.
	ErrNotFound = errors.New("share not found")
	// ErrExists is returned when another share already has the name.
	ErrExists = errors.New("a share with this name already exists")
)

// SMB is a share's SMB export.
type SMB struct {
	Enabled     bool     `json:"enabled"`
	Guest       bool     `json:"guest,omitempty"`
	TimeMachine bool     `json:"time_machine,omitempty"`
	Recycle     *Recycle `json:"recycle,omitempty"`
}

// Recycle keeps files deleted over SMB in a folder of the share.
type Recycle struct {
	Enabled   bool   `json:"enabled"`
	Directory string `json:"directory,omitempty"`
}

// NFS is a share's NFS export.
type NFS struct {
	Enabled  bool     `json:"enabled"`
	Networks []string `json:"networks,omitempty"` // clients: hosts, addresses, networks
	ReadOnly bool     `json:"read_only,omitempty"`
}

// Share is one shared folder.
type Share struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	SMB         *SMB     `json:"smb,omitempty"`
	NFS         *NFS     `json:"nfs,omitempty"`
	Owners      []string `json:"owners,omitempty"`  // read and write
	Readers     []string `json:"readers,omitempty"` // read only
	// Apps lists installed apps (by id) that may read the share, e.g. an
	// indexer. They read through their own group nos-app-<id>, apart from
	// the people in Owners and Readers.
	Apps      []string  `json:"apps,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// NithronSync settings.
	SyncEnabled      bool     `json:"sync_enabled,omitempty"`
	SyncMaxSize      int64    `json:"sync_max_size,omitempty"` // bytes, 0 = unlimited
	SyncExclude      []string `json:"sync_exclude,omitempty"`  // glob patterns
	SyncAllowedUsers []string `json:"sync_allowed_users,omitempty"`
}

// SMBEnabled reports whether the share is exported over SMB.
func (s Share) SMBEnabled() bool { return s.SMB != nil && s.SMB.Enabled }

// NFSEnabled reports whether the share is exported over NFS.
func (s Share) NFSEnabled() bool { return s.NFS != nil && s.NFS.Enabled }

// Clone returns a deep copy, so callers never share slices or pointers with
// the store.
func (s Share) Clone() Share {
	s.Owners = slices.Clone(s.Owners)
	s.Readers = slices.Clone(s.Readers)
	s.Apps = slices.Clone(s.Apps)
	s.SyncExclude = slices.Clone(s.SyncExclude)
	s.SyncAllowedUsers = slices.Clone(s.SyncAllowedUsers)
	if s.SMB != nil {
		smb := *s.SMB
		if smb.Recycle != nil {
			rc := *smb.Recycle
			smb.Recycle = &rc
		}
		s.SMB = &smb
	}
	if s.NFS != nil {
		nfs := *s.NFS
		nfs.Networks = slices.Clone(nfs.Networks)
		s.NFS = &nfs
	}
	return s
}

// Store is safe for concurrent use. All methods return copies.
type Store struct {
	path   string
	mu     sync.RWMutex
	shares map[string]Share // by ID
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

// file is the on-disk format.
type file struct {
	Version int     `json:"version"`
	Shares  []Share `json:"shares"`
}

const fileVersion = 2

// legacyShare is a share as earlier versions stored it: one protocol per
// share. ("type" and "ro" come from the oldest {version, items} format.)
type legacyShare struct {
	Share
	Protocol    string    `json:"protocol"`
	Type        string    `json:"type"`
	Enabled     bool      `json:"enabled"`
	ReadOnly    bool      `json:"readOnly"`
	RO          bool      `json:"ro"`
	GuestAccess bool      `json:"guestAccess"`
	Users       []string  `json:"users"`
	Groups      []string  `json:"groups"`
	Hosts       []string  `json:"hosts"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (l legacyShare) convert() Share {
	sh := l.Share
	proto := l.Protocol
	if proto == "" {
		proto = l.Type
	}
	readOnly := l.ReadOnly || l.RO
	switch proto {
	case "smb":
		sh.SMB = &SMB{Enabled: l.Enabled, Guest: l.GuestAccess}
	case "nfs":
		sh.NFS = &NFS{Enabled: l.Enabled, Networks: l.Hosts, ReadOnly: readOnly}
	}
	var principals []string
	for _, u := range l.Users {
		principals = append(principals, "user:"+u)
	}
	for _, g := range l.Groups {
		principals = append(principals, "group:"+g)
	}
	if len(principals) == 0 {
		// No user list meant no restriction.
		principals = []string{Everyone}
	}
	if readOnly {
		sh.Readers = principals
	} else {
		sh.Owners = principals
	}
	if sh.CreatedAt.IsZero() {
		sh.CreatedAt = l.CreatedAt
	}
	if sh.UpdatedAt.IsZero() {
		sh.UpdatedAt = l.UpdatedAt
	}
	if sh.ID == "" {
		sh.ID = uuid.NewString()
	}
	return sh
}

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
	if data[0] == '{' {
		var f struct {
			Version int               `json:"version"`
			Shares  []Share           `json:"shares"`
			Items   []json.RawMessage `json:"items"` // the oldest format
		}
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if f.Version >= fileVersion {
			return f.Shares, nil
		}
		return convertLegacy(path, f.Items)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return convertLegacy(path, items)
}

func convertLegacy(path string, items []json.RawMessage) ([]Share, error) {
	list := make([]Share, 0, len(items))
	for _, raw := range items {
		var l legacyShare
		if err := json.Unmarshal(raw, &l); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		list = append(list, l.convert())
	}
	return list, nil
}

// List returns all shares sorted by name.
func (s *Store) List() []Share {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Share, 0, len(s.shares))
	for _, sh := range s.shares {
		out = append(out, sh.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns the share with the given id.
func (s *Store) Get(id string) (Share, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sh, ok := s.shares[id]
	return sh.Clone(), ok
}

// Lookup finds a share by name, or else by id.
func (s *Store) Lookup(key string) (Share, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sh, ok := s.byNameLocked(key); ok {
		return sh.Clone(), true
	}
	sh, ok := s.shares[key]
	return sh.Clone(), ok
}

// byNameLocked matches names case-insensitively: Samba treats share names
// that way, so "Media" and "media" can't both exist.
func (s *Store) byNameLocked(name string) (Share, bool) {
	for _, sh := range s.shares {
		if strings.EqualFold(sh.Name, name) {
			return sh, true
		}
	}
	return Share{}, false
}

// Create stores a new share under a new id (any id in sh is ignored). Names
// are unique.
func (s *Store) Create(sh Share) (Share, error) {
	now := time.Now().UTC()
	sh = sh.Clone()
	sh.ID, sh.CreatedAt, sh.UpdatedAt = uuid.NewString(), now, now
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.byNameLocked(sh.Name); taken {
		return Share{}, ErrExists
	}
	s.shares[sh.ID] = sh
	if err := s.saveLocked(); err != nil {
		delete(s.shares, sh.ID)
		return Share{}, err
	}
	return sh.Clone(), nil
}

// Modify changes a copy of share id with change and stores it, unless change
// returns an error or saving fails. The id and creation time can't change.
func (s *Store) Modify(id string, change func(*Share) error) (Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.shares[id]
	if !ok {
		return Share{}, ErrNotFound
	}
	sh := old.Clone()
	if err := change(&sh); err != nil {
		return Share{}, err
	}
	sh.ID, sh.CreatedAt, sh.UpdatedAt = old.ID, old.CreatedAt, time.Now().UTC()
	if other, taken := s.byNameLocked(sh.Name); taken && other.ID != id {
		return Share{}, ErrExists
	}
	s.shares[id] = sh
	if err := s.saveLocked(); err != nil {
		s.shares[id] = old
		return Share{}, err
	}
	return sh.Clone(), nil
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

func (s *Store) saveLocked() error {
	list := make([]Share, 0, len(s.shares))
	for _, sh := range s.shares {
		list = append(list, sh)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return fsatomic.SaveJSON(s.path, file{Version: fileVersion, Shares: list}, 0o600)
}
