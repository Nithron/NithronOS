// Package groups stores nOS user groups (/etc/nos/groups.json).
//
// A group is a named list of nOS usernames. Shares grant access to groups
// ("group:<name>" in a share's owners or readers); nos-agent mirrors each
// group as the Unix group nos-<name>, so Samba, NFS and file ACLs see the
// same membership.
package groups

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"sort"
	"sync"
	"time"

	"nos/backend/nosd/internal/fsatomic"
)

var (
	ErrNotFound = errors.New("group not found")
	ErrExists   = errors.New("a group with this name already exists")
	ErrBadName  = errors.New("group name: 1-28 lowercase letters, digits, '-' or '_', starting with a letter or '_'; \"users\" is reserved")
)

// UnixPrefix is prepended to a group's name for its Unix group.
const UnixPrefix = "nos-"

// AllUsers is the Unix group nos-agent keeps with every nOS user in it; a
// share without owners and readers is open to it. No nOS group may use the
// name.
const AllUsers = "users"

// 28 characters, so nos-<name> fits Unix's 32.
var reName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,27}$`)

// ValidName reports whether name can name a group.
func ValidName(name string) bool { return reName.MatchString(name) && name != AllUsers }

// Group is a named set of users.
type Group struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Members     []string  `json:"members"` // usernames
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (g Group) clone() Group {
	g.Members = slices.Clone(g.Members)
	if g.Members == nil {
		g.Members = []string{}
	}
	return g
}

// Store is safe for concurrent use. All methods return copies.
type Store struct {
	path   string
	mu     sync.RWMutex
	groups map[string]Group
}

type file struct {
	Version int     `json:"version"`
	Groups  []Group `json:"groups"`
}

// NewStore loads path (a missing or empty file is an empty store).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, groups: map[string]Group{}}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) > 0 {
		var f file
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, g := range f.Groups {
			s.groups[g.Name] = g
		}
	}
	return s, nil
}

// List returns all groups sorted by name.
func (s *Store) List() []Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g.clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns one group.
func (s *Store) Get(name string) (Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[name]
	return g.clone(), ok
}

// Of returns the names of the groups username is a member of.
func (s *Store) Of(username string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for _, g := range s.groups {
		if slices.Contains(g.Members, username) {
			out = append(out, g.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Create stores a new group.
func (s *Store) Create(g Group) (Group, error) {
	if !ValidName(g.Name) {
		return Group{}, ErrBadName
	}
	now := time.Now().UTC()
	g = g.clone()
	g.Members = normalize(g.Members)
	g.CreatedAt, g.UpdatedAt = now, now
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.groups[g.Name]; taken {
		return Group{}, ErrExists
	}
	s.groups[g.Name] = g
	if err := s.saveLocked(); err != nil {
		delete(s.groups, g.Name)
		return Group{}, err
	}
	return g.clone(), nil
}

// Update replaces a group's description and members.
func (s *Store) Update(name, description string, members []string) (Group, error) {
	return s.modify(name, func(g *Group) {
		g.Description = description
		g.Members = normalize(members)
	})
}

// RemoveMember takes username out of every group (when the user is deleted).
func (s *Store) RemoveMember(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := make(map[string]Group, len(s.groups))
	changed := false
	for name, g := range s.groups {
		if i := slices.Index(g.Members, username); i >= 0 {
			old[name] = g
			g = g.clone()
			g.Members = slices.Delete(g.Members, i, i+1)
			g.UpdatedAt = time.Now().UTC()
			s.groups[name] = g
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := s.saveLocked(); err != nil {
		maps.Copy(s.groups, old)
		return err
	}
	return nil
}

// Delete removes a group.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.groups[name]
	if !ok {
		return ErrNotFound
	}
	delete(s.groups, name)
	if err := s.saveLocked(); err != nil {
		s.groups[name] = old
		return err
	}
	return nil
}

func (s *Store) modify(name string, change func(*Group)) (Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.groups[name]
	if !ok {
		return Group{}, ErrNotFound
	}
	g := old.clone()
	change(&g)
	g.UpdatedAt = time.Now().UTC()
	s.groups[name] = g
	if err := s.saveLocked(); err != nil {
		s.groups[name] = old
		return Group{}, err
	}
	return g.clone(), nil
}

func (s *Store) saveLocked() error {
	list := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return fsatomic.SaveJSON(s.path, file{Version: 1, Groups: list}, 0o600)
}

// normalize sorts members and drops duplicates.
func normalize(members []string) []string {
	out := slices.Clone(members)
	sort.Strings(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}
