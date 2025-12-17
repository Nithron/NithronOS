// Package sync provides file synchronization functionality for NithronOS.
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ActivityAction represents the type of sync activity
type ActivityAction string

const (
	ActivityUpload   ActivityAction = "upload"
	ActivityDownload ActivityAction = "download"
	ActivityDelete   ActivityAction = "delete"
	ActivityRename   ActivityAction = "rename"
	ActivityConflict ActivityAction = "conflict"
)

// ActivityStatus represents the status of a sync activity
type ActivityStatus string

const (
	StatusPending    ActivityStatus = "pending"
	StatusInProgress ActivityStatus = "in_progress"
	StatusCompleted  ActivityStatus = "completed"
	StatusFailed     ActivityStatus = "failed"
)

// SyncActivity represents a single sync activity record
type SyncActivity struct {
	ID          string         `json:"id"`
	DeviceID    string         `json:"device_id"`
	ShareID     string         `json:"share_id"`
	Action      ActivityAction `json:"action"`
	Path        string         `json:"path"`
	OldPath     string         `json:"old_path,omitempty"`
	Size        int64          `json:"size,omitempty"`
	Status      ActivityStatus `json:"status"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Progress    int            `json:"progress,omitempty"` // 0-100
}

// ActivityListResponse represents a paginated list of activities
type ActivityListResponse struct {
	Activities []*SyncActivity `json:"activities"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
}

// ActivityStore manages sync activity history
type ActivityStore struct {
	dataDir    string
	activities []*SyncActivity
	byID       map[string]*SyncActivity
	mu         sync.RWMutex
	maxEntries int
}

// NewActivityStore creates a new activity store
func NewActivityStore(dataDir string, maxEntries int) (*ActivityStore, error) {
	if maxEntries <= 0 {
		maxEntries = 10000
	}

	store := &ActivityStore{
		dataDir:    dataDir,
		activities: make([]*SyncActivity, 0),
		byID:       make(map[string]*SyncActivity),
		maxEntries: maxEntries,
	}

	// Create data directory if needed
	activityDir := filepath.Join(dataDir, "activity")
	if err := os.MkdirAll(activityDir, 0750); err != nil {
		return nil, err
	}

	// Load existing activities
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// RecordActivity creates a new activity record
func (s *ActivityStore) RecordActivity(deviceID, shareID string, action ActivityAction, path string, size int64) *SyncActivity {
	s.mu.Lock()
	defer s.mu.Unlock()

	activity := &SyncActivity{
		ID:        uuid.New().String(),
		DeviceID:  deviceID,
		ShareID:   shareID,
		Action:    action,
		Path:      path,
		Size:      size,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}

	s.activities = append([]*SyncActivity{activity}, s.activities...)
	s.byID[activity.ID] = activity

	// Trim old entries
	s.trimOldEntries()

	// Save asynchronously
	go func() { _ = s.save() }()

	return activity
}

// UpdateActivity updates an existing activity record
func (s *ActivityStore) UpdateActivity(id string, status ActivityStatus, progress int, err string) (*SyncActivity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activity, ok := s.byID[id]
	if !ok {
		return nil, os.ErrNotExist
	}

	activity.Status = status
	activity.Progress = progress
	if err != "" {
		activity.Error = err
	}
	if status == StatusCompleted || status == StatusFailed {
		now := time.Now()
		activity.CompletedAt = &now
		activity.Progress = 100
	}

	go func() { _ = s.save() }()

	return activity, nil
}

// GetActivity retrieves an activity by ID
func (s *ActivityStore) GetActivity(id string) (*SyncActivity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activity, ok := s.byID[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return activity, nil
}

// ListActivities returns paginated activities
func (s *ActivityStore) ListActivities(deviceID, shareID string, page, pageSize int) *ActivityListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter activities
	var filtered []*SyncActivity
	for _, a := range s.activities {
		if deviceID != "" && a.DeviceID != deviceID {
			continue
		}
		if shareID != "" && a.ShareID != shareID {
			continue
		}
		filtered = append(filtered, a)
	}

	total := len(filtered)

	// Paginate
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(filtered) {
		return &ActivityListResponse{
			Activities: []*SyncActivity{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
		}
	}

	if end > len(filtered) {
		end = len(filtered)
	}

	return &ActivityListResponse{
		Activities: filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	}
}

// GetRecentActivities returns the most recent activities
func (s *ActivityStore) GetRecentActivities(deviceID string, limit int) []*SyncActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var result []*SyncActivity
	for _, a := range s.activities {
		if deviceID != "" && a.DeviceID != deviceID {
			continue
		}
		result = append(result, a)
		if len(result) >= limit {
			break
		}
	}

	return result
}

// GetPendingActivities returns activities that are pending or in progress
func (s *ActivityStore) GetPendingActivities(deviceID string) []*SyncActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SyncActivity
	for _, a := range s.activities {
		if deviceID != "" && a.DeviceID != deviceID {
			continue
		}
		if a.Status == StatusPending || a.Status == StatusInProgress {
			result = append(result, a)
		}
	}
	return result
}

// GetActivityStats returns statistics about activities
func (s *ActivityStore) GetActivityStats(deviceID, shareID string) map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]int64{
		"total":       0,
		"uploads":     0,
		"downloads":   0,
		"deletes":     0,
		"conflicts":   0,
		"completed":   0,
		"failed":      0,
		"bytes_synced": 0,
	}

	for _, a := range s.activities {
		if deviceID != "" && a.DeviceID != deviceID {
			continue
		}
		if shareID != "" && a.ShareID != shareID {
			continue
		}

		stats["total"]++

		switch a.Action {
		case ActivityUpload:
			stats["uploads"]++
		case ActivityDownload:
			stats["downloads"]++
		case ActivityDelete:
			stats["deletes"]++
		case ActivityConflict:
			stats["conflicts"]++
		}

		if a.Status == StatusCompleted {
			stats["completed"]++
			stats["bytes_synced"] += a.Size
		} else if a.Status == StatusFailed {
			stats["failed"]++
		}
	}

	return stats
}

// ClearActivities removes activities older than the specified duration
func (s *ActivityStore) ClearActivities(olderThan time.Duration, deviceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	var kept []*SyncActivity
	for _, a := range s.activities {
		if deviceID != "" && a.DeviceID != deviceID {
			kept = append(kept, a)
			continue
		}
		if a.StartedAt.Before(cutoff) {
			delete(s.byID, a.ID)
			removed++
		} else {
			kept = append(kept, a)
		}
	}

	s.activities = kept
	go func() { _ = s.save() }()

	return removed
}

// trimOldEntries removes old entries to stay within maxEntries limit
func (s *ActivityStore) trimOldEntries() {
	if len(s.activities) <= s.maxEntries {
		return
	}

	// Keep only the most recent entries
	toRemove := s.activities[s.maxEntries:]
	s.activities = s.activities[:s.maxEntries]

	for _, a := range toRemove {
		delete(s.byID, a.ID)
	}
}

// load reads activities from disk
func (s *ActivityStore) load() error {
	filePath := filepath.Join(s.dataDir, "activity", "activities.json")

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var activities []*SyncActivity
	if err := json.Unmarshal(data, &activities); err != nil {
		return err
	}

	// Sort by start time (newest first)
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].StartedAt.After(activities[j].StartedAt)
	})

	s.activities = activities
	for _, a := range activities {
		s.byID[a.ID] = a
	}

	return nil
}

// save writes activities to disk
func (s *ActivityStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.activities, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.dataDir, "activity", "activities.json")
	return os.WriteFile(filePath, data, 0640)
}

