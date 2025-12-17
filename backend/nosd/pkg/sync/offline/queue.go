// Package offline provides offline-first sync capabilities for NithronSync.
package offline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperationType represents the type of sync operation
type OperationType string

const (
	OpCreate OperationType = "create"
	OpModify OperationType = "modify"
	OpDelete OperationType = "delete"
	OpRename OperationType = "rename"
	OpMove   OperationType = "move"
)

// OperationStatus represents the status of an operation
type OperationStatus string

const (
	StatusPending   OperationStatus = "pending"
	StatusInProgress OperationStatus = "in_progress"
	StatusCompleted OperationStatus = "completed"
	StatusFailed    OperationStatus = "failed"
	StatusConflict  OperationStatus = "conflict"
	StatusCancelled OperationStatus = "cancelled"
)

// QueuedOperation represents an operation waiting to be synced
type QueuedOperation struct {
	ID            string            `json:"id"`
	ShareID       string            `json:"share_id"`
	Path          string            `json:"path"`
	OldPath       string            `json:"old_path,omitempty"`
	Type          OperationType     `json:"type"`
	Status        OperationStatus   `json:"status"`
	Priority      int               `json:"priority"`
	Size          int64             `json:"size"`
	Hash          string            `json:"hash,omitempty"`
	LocalVersion  int64             `json:"local_version"`
	RemoteVersion int64             `json:"remote_version,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ModifiedAt    time.Time         `json:"modified_at"`
	AttemptCount  int               `json:"attempt_count"`
	LastAttempt   *time.Time        `json:"last_attempt,omitempty"`
	Error         string            `json:"error,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ConflictResolution represents how a conflict was resolved
type ConflictResolution string

const (
	ResolutionKeepLocal   ConflictResolution = "keep_local"
	ResolutionKeepRemote  ConflictResolution = "keep_remote"
	ResolutionKeepBoth    ConflictResolution = "keep_both"
	ResolutionMerge       ConflictResolution = "merge"
	ResolutionManual      ConflictResolution = "manual"
)

// SyncConflict represents a conflict between local and remote versions
type SyncConflict struct {
	ID               string                 `json:"id"`
	ShareID          string                 `json:"share_id"`
	Path             string                 `json:"path"`
	LocalVersion     FileVersion            `json:"local_version"`
	RemoteVersion    FileVersion            `json:"remote_version"`
	BaseVersion      *FileVersion           `json:"base_version,omitempty"`
	ConflictType     ConflictType           `json:"conflict_type"`
	Resolution       *ConflictResolution    `json:"resolution,omitempty"`
	ResolvedAt       *time.Time             `json:"resolved_at,omitempty"`
	ResolvedBy       string                 `json:"resolved_by,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ConflictType represents the type of conflict
type ConflictType string

const (
	ConflictTypeModifyModify ConflictType = "modify_modify"
	ConflictTypeModifyDelete ConflictType = "modify_delete"
	ConflictTypeDeleteModify ConflictType = "delete_modify"
	ConflictTypeCreateCreate ConflictType = "create_create"
	ConflictTypeMoveMove     ConflictType = "move_move"
)

// FileVersion represents a version of a file
type FileVersion struct {
	Version    int64     `json:"version"`
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	ModifiedBy string    `json:"modified_by,omitempty"`
	DeviceID   string    `json:"device_id,omitempty"`
}

// OperationQueue manages the queue of pending sync operations
type OperationQueue struct {
	dataDir    string
	operations map[string]*QueuedOperation
	conflicts  map[string]*SyncConflict
	mu         sync.RWMutex
	
	// Callbacks
	onStatusChange func(op *QueuedOperation)
	onConflict     func(conflict *SyncConflict)
}

// NewOperationQueue creates a new operation queue
func NewOperationQueue(dataDir string) (*OperationQueue, error) {
	q := &OperationQueue{
		dataDir:    dataDir,
		operations: make(map[string]*QueuedOperation),
		conflicts:  make(map[string]*SyncConflict),
	}

	// Create queue directory
	queueDir := filepath.Join(dataDir, "queue")
	if err := os.MkdirAll(queueDir, 0750); err != nil {
		return nil, err
	}

	// Load existing operations
	if err := q.load(); err != nil {
		return nil, err
	}

	return q, nil
}

// Enqueue adds a new operation to the queue
func (q *OperationQueue) Enqueue(op *QueuedOperation) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if op.ID == "" {
		op.ID = uuid.New().String()
	}
	if op.Status == "" {
		op.Status = StatusPending
	}
	if op.CreatedAt.IsZero() {
		op.CreatedAt = time.Now()
	}
	op.ModifiedAt = time.Now()

	// Check for duplicate operations on same path
	for _, existing := range q.operations {
		if existing.ShareID == op.ShareID && existing.Path == op.Path && existing.Status == StatusPending {
			// Merge or replace based on operation types
			if existing.Type == op.Type {
				// Update existing operation
				existing.Size = op.Size
				existing.Hash = op.Hash
				existing.ModifiedAt = time.Now()
				return q.save()
			}
		}
	}

	q.operations[op.ID] = op
	return q.save()
}

// Dequeue removes and returns the next operation to process
func (q *OperationQueue) Dequeue() *QueuedOperation {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Get all pending operations sorted by priority and creation time
	var pending []*QueuedOperation
	for _, op := range q.operations {
		if op.Status == StatusPending {
			pending = append(pending, op)
		}
	}

	if len(pending) == 0 {
		return nil
	}

	// Sort by priority (higher first) then by creation time
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Priority != pending[j].Priority {
			return pending[i].Priority > pending[j].Priority
		}
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})

	op := pending[0]
	op.Status = StatusInProgress
	op.ModifiedAt = time.Now()
	now := time.Now()
	op.LastAttempt = &now
	op.AttemptCount++

	_ = q.save()
	return op
}

// UpdateStatus updates the status of an operation
func (q *OperationQueue) UpdateStatus(id string, status OperationStatus, err string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, ok := q.operations[id]
	if !ok {
		return errors.New("operation not found")
	}

	op.Status = status
	op.Error = err
	op.ModifiedAt = time.Now()

	if q.onStatusChange != nil {
		q.onStatusChange(op)
	}

	// Remove completed/cancelled operations after some time
	if status == StatusCompleted || status == StatusCancelled {
		// Keep for 24 hours for history
		go func() {
			time.Sleep(24 * time.Hour)
			q.mu.Lock()
			defer q.mu.Unlock()
			delete(q.operations, id)
			_ = q.save()
		}()
	}

	return q.save()
}

// Retry marks a failed operation for retry
func (q *OperationQueue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, ok := q.operations[id]
	if !ok {
		return errors.New("operation not found")
	}

	if op.Status != StatusFailed {
		return errors.New("operation is not failed")
	}

	op.Status = StatusPending
	op.Error = ""
	op.ModifiedAt = time.Now()

	return q.save()
}

// Cancel cancels a pending operation
func (q *OperationQueue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	op, ok := q.operations[id]
	if !ok {
		return errors.New("operation not found")
	}

	if op.Status != StatusPending && op.Status != StatusFailed {
		return errors.New("cannot cancel operation in current status")
	}

	op.Status = StatusCancelled
	op.ModifiedAt = time.Now()

	return q.save()
}

// GetPending returns all pending operations
func (q *OperationQueue) GetPending() []*QueuedOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*QueuedOperation
	for _, op := range q.operations {
		if op.Status == StatusPending {
			result = append(result, op)
		}
	}
	return result
}

// GetAll returns all operations
func (q *OperationQueue) GetAll() []*QueuedOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*QueuedOperation, 0, len(q.operations))
	for _, op := range q.operations {
		result = append(result, op)
	}
	return result
}

// GetByShare returns operations for a specific share
func (q *OperationQueue) GetByShare(shareID string) []*QueuedOperation {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*QueuedOperation
	for _, op := range q.operations {
		if op.ShareID == shareID {
			result = append(result, op)
		}
	}
	return result
}

// GetStats returns queue statistics
func (q *OperationQueue) GetStats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := QueueStats{}
	for _, op := range q.operations {
		stats.Total++
		switch op.Status {
		case StatusPending:
			stats.Pending++
			stats.PendingBytes += op.Size
		case StatusInProgress:
			stats.InProgress++
		case StatusCompleted:
			stats.Completed++
		case StatusFailed:
			stats.Failed++
		case StatusConflict:
			stats.Conflicts++
		}
	}
	return stats
}

// QueueStats contains queue statistics
type QueueStats struct {
	Total        int   `json:"total"`
	Pending      int   `json:"pending"`
	PendingBytes int64 `json:"pending_bytes"`
	InProgress   int   `json:"in_progress"`
	Completed    int   `json:"completed"`
	Failed       int   `json:"failed"`
	Conflicts    int   `json:"conflicts"`
}

// AddConflict adds a new conflict
func (q *OperationQueue) AddConflict(conflict *SyncConflict) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if conflict.ID == "" {
		conflict.ID = uuid.New().String()
	}
	if conflict.CreatedAt.IsZero() {
		conflict.CreatedAt = time.Now()
	}

	q.conflicts[conflict.ID] = conflict

	if q.onConflict != nil {
		q.onConflict(conflict)
	}

	return q.save()
}

// GetConflicts returns all unresolved conflicts
func (q *OperationQueue) GetConflicts() []*SyncConflict {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*SyncConflict
	for _, c := range q.conflicts {
		if c.Resolution == nil {
			result = append(result, c)
		}
	}
	return result
}

// ResolveConflict resolves a conflict
func (q *OperationQueue) ResolveConflict(id string, resolution ConflictResolution, resolvedBy string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	conflict, ok := q.conflicts[id]
	if !ok {
		return errors.New("conflict not found")
	}

	conflict.Resolution = &resolution
	now := time.Now()
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = resolvedBy

	return q.save()
}

// SetOnStatusChange sets the callback for operation status changes
func (q *OperationQueue) SetOnStatusChange(callback func(op *QueuedOperation)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onStatusChange = callback
}

// SetOnConflict sets the callback for new conflicts
func (q *OperationQueue) SetOnConflict(callback func(conflict *SyncConflict)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onConflict = callback
}

func (q *OperationQueue) load() error {
	// Load operations
	opsPath := filepath.Join(q.dataDir, "queue", "operations.json")
	if data, err := os.ReadFile(opsPath); err == nil {
		var ops []*QueuedOperation
		if err := json.Unmarshal(data, &ops); err == nil {
			for _, op := range ops {
				q.operations[op.ID] = op
			}
		}
	}

	// Load conflicts
	conflictsPath := filepath.Join(q.dataDir, "queue", "conflicts.json")
	if data, err := os.ReadFile(conflictsPath); err == nil {
		var conflicts []*SyncConflict
		if err := json.Unmarshal(data, &conflicts); err == nil {
			for _, c := range conflicts {
				q.conflicts[c.ID] = c
			}
		}
	}

	return nil
}

func (q *OperationQueue) save() error {
	// Save operations
	ops := make([]*QueuedOperation, 0, len(q.operations))
	for _, op := range q.operations {
		ops = append(ops, op)
	}
	opsData, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return err
	}
	opsPath := filepath.Join(q.dataDir, "queue", "operations.json")
	if err := os.WriteFile(opsPath, opsData, 0640); err != nil {
		return err
	}

	// Save conflicts
	conflicts := make([]*SyncConflict, 0, len(q.conflicts))
	for _, c := range q.conflicts {
		conflicts = append(conflicts, c)
	}
	conflictsData, err := json.MarshalIndent(conflicts, "", "  ")
	if err != nil {
		return err
	}
	conflictsPath := filepath.Join(q.dataDir, "queue", "conflicts.json")
	return os.WriteFile(conflictsPath, conflictsData, 0640)
}

// Clear removes all completed and cancelled operations
func (q *OperationQueue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for id, op := range q.operations {
		if op.Status == StatusCompleted || op.Status == StatusCancelled {
			delete(q.operations, id)
		}
	}

	return q.save()
}

// RetryAll retries all failed operations
func (q *OperationQueue) RetryAll() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, op := range q.operations {
		if op.Status == StatusFailed {
			op.Status = StatusPending
			op.Error = ""
			op.ModifiedAt = time.Now()
		}
	}

	return q.save()
}

