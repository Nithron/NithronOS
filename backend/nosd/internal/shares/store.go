package shares

import (
	"context"
	"io/fs"
	"sync"

	"nithronos/backend/nosd/internal/fsatomic"
)

type Share struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"` // smb|nfs
	Path  string   `json:"path"`
	Name  string   `json:"name"`
	RO    bool     `json:"ro"`
	Users []string `json:"users"`
	
	// Sync-related fields for NithronSync
	SyncEnabled     bool     `json:"sync_enabled"`
	SyncMaxSize     int64    `json:"sync_max_size,omitempty"`     // Max sync size in bytes (0 = unlimited)
	SyncExclude     []string `json:"sync_exclude,omitempty"`      // Glob patterns to exclude from sync
	SyncAllowedUsers []string `json:"sync_allowed_users,omitempty"` // Users allowed to sync (empty = all share users)
}

type Store struct {
	path string
	mu   sync.RWMutex
	list []Share
}

func NewStore(path string) *Store {
	s := &Store{path: path}
	_ = s.load()
	return s
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []Share
	ok, err := fsatomic.LoadJSON(s.path, &list)
	if err != nil {
		return err
	}
	if !ok {
		list = nil
	}
	s.list = list
	return nil
}

func (s *Store) snapshot() []Share {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Share, len(s.list))
	copy(out, s.list)
	return out
}

func (s *Store) List() []Share {
	return s.snapshot()
}

func (s *Store) Add(sh Share) error {
	s.mu.Lock()
	s.list = append(s.list, sh)
	data := make([]Share, len(s.list))
	copy(data, s.list)
	s.mu.Unlock()
	return fsatomic.WithLock(s.path, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.path, data, fs.FileMode(0o600))
	})
}

func (s *Store) GetByID(id string) (Share, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sh := range s.list {
		if sh.ID == id {
			return sh, true
		}
	}
	return Share{}, false
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	out := s.list[:0]
	for _, sh := range s.list {
		if sh.ID != id {
			out = append(out, sh)
		}
	}
	s.list = out
	data := make([]Share, len(s.list))
	copy(data, s.list)
	s.mu.Unlock()
	return fsatomic.WithLock(s.path, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.path, data, fs.FileMode(0o600))
	})
}
