// Package sync provides file synchronization functionality for NithronOS.
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConflictResolution represents how a conflict was resolved
type ConflictResolution string

const (
	ResolutionKeepLocal  ConflictResolution = "keep_local"
	ResolutionKeepRemote ConflictResolution = "keep_remote"
	ResolutionKeepBoth   ConflictResolution = "keep_both"
	ResolutionMerge      ConflictResolution = "merge"
)

// FileVersion represents a version of a file in conflict
type FileVersion struct {
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	Modified   time.Time `json:"modified"`
	ModifiedBy string    `json:"modified_by,omitempty"`
}

// SyncConflict represents a file synchronization conflict
type SyncConflict struct {
	ID            string              `json:"id"`
	ShareID       string              `json:"share_id"`
	DeviceID      string              `json:"device_id"`
	Path          string              `json:"path"`
	LocalVersion  FileVersion         `json:"local_version"`
	RemoteVersion FileVersion         `json:"remote_version"`
	DetectedAt    time.Time           `json:"detected_at"`
	Resolved      bool                `json:"resolved"`
	Resolution    ConflictResolution  `json:"resolution,omitempty"`
	ResolvedAt    *time.Time          `json:"resolved_at,omitempty"`
	ResolvedBy    string              `json:"resolved_by,omitempty"`
}

// ConflictStore manages sync conflicts
type ConflictStore struct {
	dataDir   string
	conflicts map[string]*SyncConflict
	mu        sync.RWMutex
}

// NewConflictStore creates a new conflict store
func NewConflictStore(dataDir string) (*ConflictStore, error) {
	store := &ConflictStore{
		dataDir:   dataDir,
		conflicts: make(map[string]*SyncConflict),
	}

	// Create data directory if needed
	conflictsDir := filepath.Join(dataDir, "conflicts")
	if err := os.MkdirAll(conflictsDir, 0750); err != nil {
		return nil, err
	}

	// Load existing conflicts
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// CreateConflict records a new sync conflict
func (s *ConflictStore) CreateConflict(shareID, deviceID, path string, local, remote FileVersion) (*SyncConflict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conflict := &SyncConflict{
		ID:            uuid.New().String(),
		ShareID:       shareID,
		DeviceID:      deviceID,
		Path:          path,
		LocalVersion:  local,
		RemoteVersion: remote,
		DetectedAt:    time.Now(),
		Resolved:      false,
	}

	s.conflicts[conflict.ID] = conflict

	if err := s.save(); err != nil {
		delete(s.conflicts, conflict.ID)
		return nil, err
	}

	return conflict, nil
}

// GetConflict retrieves a conflict by ID
func (s *ConflictStore) GetConflict(id string) (*SyncConflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflict, ok := s.conflicts[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return conflict, nil
}

// ListConflicts returns all conflicts, optionally filtered
func (s *ConflictStore) ListConflicts(shareID, deviceID string, unresolvedOnly bool) []*SyncConflict {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SyncConflict
	for _, c := range s.conflicts {
		if shareID != "" && c.ShareID != shareID {
			continue
		}
		if deviceID != "" && c.DeviceID != deviceID {
			continue
		}
		if unresolvedOnly && c.Resolved {
			continue
		}
		result = append(result, c)
	}
	return result
}

// ResolveConflict marks a conflict as resolved
func (s *ConflictStore) ResolveConflict(id string, resolution ConflictResolution, resolvedBy string) (*SyncConflict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conflict, ok := s.conflicts[id]
	if !ok {
		return nil, os.ErrNotExist
	}

	now := time.Now()
	conflict.Resolved = true
	conflict.Resolution = resolution
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = resolvedBy

	if err := s.save(); err != nil {
		return nil, err
	}

	return conflict, nil
}

// DeleteConflict removes a conflict
func (s *ConflictStore) DeleteConflict(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conflicts[id]; !ok {
		return os.ErrNotExist
	}

	delete(s.conflicts, id)
	return s.save()
}

// CountUnresolved returns the number of unresolved conflicts
func (s *ConflictStore) CountUnresolved(shareID, deviceID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, c := range s.conflicts {
		if c.Resolved {
			continue
		}
		if shareID != "" && c.ShareID != shareID {
			continue
		}
		if deviceID != "" && c.DeviceID != deviceID {
			continue
		}
		count++
	}
	return count
}

// load reads conflicts from disk
func (s *ConflictStore) load() error {
	filePath := filepath.Join(s.dataDir, "conflicts", "conflicts.json")
	
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var conflicts []*SyncConflict
	if err := json.Unmarshal(data, &conflicts); err != nil {
		return err
	}

	for _, c := range conflicts {
		s.conflicts[c.ID] = c
	}

	return nil
}

// save writes conflicts to disk
func (s *ConflictStore) save() error {
	conflicts := make([]*SyncConflict, 0, len(s.conflicts))
	for _, c := range s.conflicts {
		conflicts = append(conflicts, c)
	}

	data, err := json.MarshalIndent(conflicts, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.dataDir, "conflicts", "conflicts.json")
	return os.WriteFile(filePath, data, 0640)
}

