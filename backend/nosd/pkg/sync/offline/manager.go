package offline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SyncMode represents the sync mode
type SyncMode string

const (
	ModeOnline   SyncMode = "online"
	ModeOffline  SyncMode = "offline"
	ModeSyncing  SyncMode = "syncing"
)

// NetworkStatus represents the network connectivity status
type NetworkStatus string

const (
	NetworkConnected    NetworkStatus = "connected"
	NetworkDisconnected NetworkStatus = "disconnected"
	NetworkMetered      NetworkStatus = "metered"
)

// OfflineManager manages offline-first sync functionality
type OfflineManager struct {
	dataDir       string
	queue         *OperationQueue
	mode          SyncMode
	networkStatus NetworkStatus
	lastSync      time.Time
	syncInterval  time.Duration
	
	// Local state tracking
	localState    map[string]*LocalFileState
	localStateMu  sync.RWMutex
	
	// Callbacks
	onModeChange    func(mode SyncMode)
	onNetworkChange func(status NetworkStatus)
	onSyncComplete  func(stats SyncStats)
	
	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	
	logger zerolog.Logger
	mu     sync.RWMutex
}

// LocalFileState represents the local state of a file
type LocalFileState struct {
	ShareID       string            `json:"share_id"`
	Path          string            `json:"path"`
	LocalHash     string            `json:"local_hash"`
	RemoteHash    string            `json:"remote_hash,omitempty"`
	LocalVersion  int64             `json:"local_version"`
	RemoteVersion int64             `json:"remote_version"`
	Size          int64             `json:"size"`
	ModifiedAt    time.Time         `json:"modified_at"`
	SyncedAt      *time.Time        `json:"synced_at,omitempty"`
	IsAvailable   bool              `json:"is_available"`
	IsPinned      bool              `json:"is_pinned"`
	IsPlaceholder bool              `json:"is_placeholder"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// SyncStats contains sync statistics
type SyncStats struct {
	FilesUploaded   int   `json:"files_uploaded"`
	FilesDownloaded int   `json:"files_downloaded"`
	BytesUploaded   int64 `json:"bytes_uploaded"`
	BytesDownloaded int64 `json:"bytes_downloaded"`
	Conflicts       int   `json:"conflicts"`
	Errors          int   `json:"errors"`
	Duration        time.Duration `json:"duration"`
}

// NewOfflineManager creates a new offline manager
func NewOfflineManager(dataDir string, logger zerolog.Logger) (*OfflineManager, error) {
	queue, err := NewOperationQueue(dataDir)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &OfflineManager{
		dataDir:       dataDir,
		queue:         queue,
		mode:          ModeOnline,
		networkStatus: NetworkConnected,
		syncInterval:  5 * time.Minute,
		localState:    make(map[string]*LocalFileState),
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger.With().Str("component", "offline-manager").Logger(),
	}

	// Load local state
	if err := m.loadLocalState(); err != nil {
		logger.Warn().Err(err).Msg("Failed to load local state")
	}

	return m, nil
}

// Start starts the offline manager
func (m *OfflineManager) Start() {
	go m.syncLoop()
	go m.networkMonitor()
}

// Stop stops the offline manager
func (m *OfflineManager) Stop() {
	m.cancel()
}

// GetMode returns the current sync mode
func (m *OfflineManager) GetMode() SyncMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// SetMode sets the sync mode
func (m *OfflineManager) SetMode(mode SyncMode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode == mode {
		return
	}

	m.mode = mode
	if m.onModeChange != nil {
		m.onModeChange(mode)
	}

	m.logger.Info().Str("mode", string(mode)).Msg("Sync mode changed")
}

// GetNetworkStatus returns the current network status
func (m *OfflineManager) GetNetworkStatus() NetworkStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.networkStatus
}

// SetNetworkStatus sets the network status
func (m *OfflineManager) SetNetworkStatus(status NetworkStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.networkStatus == status {
		return
	}

	oldStatus := m.networkStatus
	m.networkStatus = status

	if m.onNetworkChange != nil {
		m.onNetworkChange(status)
	}

	m.logger.Info().
		Str("old_status", string(oldStatus)).
		Str("new_status", string(status)).
		Msg("Network status changed")

	// Trigger sync when coming online
	if status == NetworkConnected && oldStatus == NetworkDisconnected {
		go m.triggerSync()
	}
}

// GetQueue returns the operation queue
func (m *OfflineManager) GetQueue() *OperationQueue {
	return m.queue
}

// TrackFileChange tracks a local file change
func (m *OfflineManager) TrackFileChange(shareID, path string, opType OperationType, size int64, hash string) error {
	// Update local state
	m.localStateMu.Lock()
	key := shareID + ":" + path
	state, exists := m.localState[key]
	if !exists {
		state = &LocalFileState{
			ShareID:     shareID,
			Path:        path,
			IsAvailable: true,
		}
		m.localState[key] = state
	}
	state.LocalHash = hash
	state.Size = size
	state.ModifiedAt = time.Now()
	state.LocalVersion++
	m.localStateMu.Unlock()

	// Queue the operation
	op := &QueuedOperation{
		ShareID:      shareID,
		Path:         path,
		Type:         opType,
		Size:         size,
		Hash:         hash,
		LocalVersion: state.LocalVersion,
	}

	// Set priority based on operation type
	switch opType {
	case OpDelete:
		op.Priority = 100 // High priority
	case OpCreate:
		op.Priority = 80
	case OpModify:
		op.Priority = 60
	case OpRename, OpMove:
		op.Priority = 90
	}

	return m.queue.Enqueue(op)
}

// GetLocalState returns the local state for a file
func (m *OfflineManager) GetLocalState(shareID, path string) *LocalFileState {
	m.localStateMu.RLock()
	defer m.localStateMu.RUnlock()
	return m.localState[shareID+":"+path]
}

// SetLocalState sets the local state for a file
func (m *OfflineManager) SetLocalState(state *LocalFileState) {
	m.localStateMu.Lock()
	defer m.localStateMu.Unlock()
	m.localState[state.ShareID+":"+state.Path] = state
	m.saveLocalState()
}

// MarkAsSynced marks a file as synced
func (m *OfflineManager) MarkAsSynced(shareID, path string, remoteVersion int64, remoteHash string) {
	m.localStateMu.Lock()
	defer m.localStateMu.Unlock()

	key := shareID + ":" + path
	if state, ok := m.localState[key]; ok {
		now := time.Now()
		state.SyncedAt = &now
		state.RemoteVersion = remoteVersion
		state.RemoteHash = remoteHash
	}
	m.saveLocalState()
}

// GetPendingChanges returns files with pending local changes
func (m *OfflineManager) GetPendingChanges() []*LocalFileState {
	m.localStateMu.RLock()
	defer m.localStateMu.RUnlock()

	var result []*LocalFileState
	for _, state := range m.localState {
		if state.LocalHash != state.RemoteHash || state.LocalVersion > state.RemoteVersion {
			result = append(result, state)
		}
	}
	return result
}

// GetAvailableOffline returns files available offline
func (m *OfflineManager) GetAvailableOffline(shareID string) []*LocalFileState {
	m.localStateMu.RLock()
	defer m.localStateMu.RUnlock()

	var result []*LocalFileState
	for _, state := range m.localState {
		if state.ShareID == shareID && state.IsAvailable {
			result = append(result, state)
		}
	}
	return result
}

// PinFile pins a file for offline availability
func (m *OfflineManager) PinFile(shareID, path string) error {
	m.localStateMu.Lock()
	defer m.localStateMu.Unlock()

	key := shareID + ":" + path
	if state, ok := m.localState[key]; ok {
		state.IsPinned = true
	} else {
		m.localState[key] = &LocalFileState{
			ShareID:  shareID,
			Path:     path,
			IsPinned: true,
		}
	}

	return m.saveLocalState()
}

// UnpinFile unpins a file
func (m *OfflineManager) UnpinFile(shareID, path string) error {
	m.localStateMu.Lock()
	defer m.localStateMu.Unlock()

	key := shareID + ":" + path
	if state, ok := m.localState[key]; ok {
		state.IsPinned = false
	}

	return m.saveLocalState()
}

// GetPinnedFiles returns all pinned files
func (m *OfflineManager) GetPinnedFiles(shareID string) []*LocalFileState {
	m.localStateMu.RLock()
	defer m.localStateMu.RUnlock()

	var result []*LocalFileState
	for _, state := range m.localState {
		if (shareID == "" || state.ShareID == shareID) && state.IsPinned {
			result = append(result, state)
		}
	}
	return result
}

// DetectConflict checks if there's a conflict between local and remote versions
func (m *OfflineManager) DetectConflict(shareID, path string, remoteVersion FileVersion) *SyncConflict {
	m.localStateMu.RLock()
	defer m.localStateMu.RUnlock()

	key := shareID + ":" + path
	state, ok := m.localState[key]
	if !ok {
		return nil
	}

	// No conflict if hashes match
	if state.LocalHash == remoteVersion.Hash {
		return nil
	}

	// No conflict if local hasn't changed since last sync
	if state.SyncedAt != nil && state.LocalHash == state.RemoteHash {
		return nil
	}

	// Determine conflict type
	var conflictType ConflictType
	if state.LocalVersion > 0 && remoteVersion.Version > state.RemoteVersion {
		conflictType = ConflictTypeModifyModify
	}

	return &SyncConflict{
		ShareID: shareID,
		Path:    path,
		LocalVersion: FileVersion{
			Version:    state.LocalVersion,
			Hash:       state.LocalHash,
			Size:       state.Size,
			ModifiedAt: state.ModifiedAt,
		},
		RemoteVersion: remoteVersion,
		ConflictType:  conflictType,
		CreatedAt:     time.Now(),
	}
}

// TriggerSync triggers an immediate sync
func (m *OfflineManager) TriggerSync() {
	go m.triggerSync()
}

func (m *OfflineManager) triggerSync() {
	m.mu.Lock()
	if m.mode == ModeSyncing {
		m.mu.Unlock()
		return
	}
	m.mode = ModeSyncing
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.mode = ModeOnline
		m.mu.Unlock()
	}()

	m.logger.Info().Msg("Starting sync")
	startTime := time.Now()
	stats := SyncStats{}

	// Process upload queue
	for {
		op := m.queue.Dequeue()
		if op == nil {
			break
		}

		// In a real implementation, this would upload/sync the file
		// For now, we just mark it as completed
		m.logger.Debug().
			Str("operation", string(op.Type)).
			Str("path", op.Path).
			Msg("Processing operation")

		// Simulate processing
		time.Sleep(100 * time.Millisecond)

		if err := m.queue.UpdateStatus(op.ID, StatusCompleted, ""); err != nil {
			m.logger.Error().Err(err).Str("op_id", op.ID).Msg("Failed to update status")
			stats.Errors++
		} else {
			stats.FilesUploaded++
			stats.BytesUploaded += op.Size
		}
	}

	stats.Duration = time.Since(startTime)
	m.lastSync = time.Now()

	if m.onSyncComplete != nil {
		m.onSyncComplete(stats)
	}

	m.logger.Info().
		Int("files_uploaded", stats.FilesUploaded).
		Int64("bytes_uploaded", stats.BytesUploaded).
		Dur("duration", stats.Duration).
		Msg("Sync completed")
}

func (m *OfflineManager) syncLoop() {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if m.networkStatus == NetworkConnected && m.mode == ModeOnline {
				m.triggerSync()
			}
		}
	}
}

func (m *OfflineManager) networkMonitor() {
	// In a real implementation, this would monitor network connectivity
	// For now, it's a placeholder
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Check network connectivity
			// This is a placeholder - real implementation would check actual network status
		}
	}
}

func (m *OfflineManager) loadLocalState() error {
	statePath := filepath.Join(m.dataDir, "local_state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var states []*LocalFileState
	if err := json.Unmarshal(data, &states); err != nil {
		return err
	}

	m.localStateMu.Lock()
	defer m.localStateMu.Unlock()
	for _, state := range states {
		m.localState[state.ShareID+":"+state.Path] = state
	}

	return nil
}

func (m *OfflineManager) saveLocalState() error {
	states := make([]*LocalFileState, 0, len(m.localState))
	for _, state := range m.localState {
		states = append(states, state)
	}

	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(m.dataDir, "local_state.json")
	return os.WriteFile(statePath, data, 0640)
}

// SetOnModeChange sets the callback for mode changes
func (m *OfflineManager) SetOnModeChange(callback func(mode SyncMode)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onModeChange = callback
}

// SetOnNetworkChange sets the callback for network changes
func (m *OfflineManager) SetOnNetworkChange(callback func(status NetworkStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNetworkChange = callback
}

// SetOnSyncComplete sets the callback for sync completion
func (m *OfflineManager) SetOnSyncComplete(callback func(stats SyncStats)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSyncComplete = callback
}

// GetLastSync returns the last sync time
func (m *OfflineManager) GetLastSync() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSync
}

// SetSyncInterval sets the sync interval
func (m *OfflineManager) SetSyncInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncInterval = interval
}

// GetStatus returns the overall offline manager status
func (m *OfflineManager) GetStatus() OfflineStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return OfflineStatus{
		Mode:          m.mode,
		NetworkStatus: m.networkStatus,
		LastSync:      m.lastSync,
		QueueStats:    m.queue.GetStats(),
		ConflictCount: len(m.queue.GetConflicts()),
	}
}

// OfflineStatus represents the current status of the offline manager
type OfflineStatus struct {
	Mode          SyncMode      `json:"mode"`
	NetworkStatus NetworkStatus `json:"network_status"`
	LastSync      time.Time     `json:"last_sync"`
	QueueStats    QueueStats    `json:"queue_stats"`
	ConflictCount int           `json:"conflict_count"`
}

