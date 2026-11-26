package ratelimit

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

// State represents a simple rate-limit state persisted to disk
type State struct {
	Version int               `json:"version"`
	Buckets map[string]Bucket `json:"buckets"`
}

type Bucket struct {
	Hits   int    `json:"hits"`
	Window string `json:"window"`
}

type Store struct {
	path        string
	mu          sync.RWMutex
	st          State
	lastPersist time.Time
	ops         int
}

func New(path string) *Store {
	s := &Store{path: path, st: State{Version: 1, Buckets: map[string]Bucket{}}}
	_ = s.load()
	return s
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st State
	ok, err := fsatomic.LoadJSON(s.path, &st)
	if err != nil || !ok {
		return err
	}
	s.st = st
	if s.st.Buckets == nil {
		s.st.Buckets = map[string]Bucket{}
	}
	s.lastPersist = time.Now()
	return nil
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st
}

func (s *Store) Put(key string, b Bucket) error {
	s.mu.Lock()
	s.st.Buckets[key] = b
	st := s.st
	s.mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	return fsatomic.WithLock(s.path, func() error {
		return fsatomic.SaveJSON(s.path, st, fs.FileMode(0o600))
	})
}

// Allow applies a fixed-window limit (max within window) and persists the bucket when needed.
// Returns ok, remaining, and resetAt time.
func (s *Store) Allow(key string, limit int, window time.Duration) (bool, int, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	b := s.st.Buckets[key]
	start := parseWindow(b.Window)
	if start.IsZero() || now.Sub(start) >= window {
		start = now
		b.Window = start.Format(time.RFC3339Nano)
		b.Hits = 0
	}
	resetAt := start.Add(window)
	if b.Hits >= limit {
		s.maybePersistLocked()
		return false, 0, resetAt
	}
	b.Hits++
	s.st.Buckets[key] = b
	s.maybePersistLocked()
	remaining := limit - b.Hits
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining, resetAt
}

// Flush forces a persist to disk.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked()
}

func (s *Store) persistLocked() error {
	st := s.st
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	if err := fsatomic.WithLock(s.path, func() error {
		return fsatomic.SaveJSON(s.path, st, fs.FileMode(0o600))
	}); err != nil {
		return err
	}
	s.lastPersist = time.Now()
	s.ops = 0
	return nil
}

// maybePersistLocked persists every ~2s or every 10 ops to reduce IO.
func (s *Store) maybePersistLocked() {
	s.ops++
	if s.ops%10 == 0 || time.Since(s.lastPersist) >= 2*time.Second {
		_ = s.persistLocked()
	}
}

func parseWindow(val string) time.Time {
	if val == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t
	}
	return time.Time{}
}

func NowUTC() string { return time.Now().UTC().Format(time.RFC3339) }
