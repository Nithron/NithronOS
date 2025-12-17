// Package smartsync provides on-demand file synchronization (smart sync/files on demand).
package smartsync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// PlaceholderState represents the state of a placeholder file
type PlaceholderState string

const (
	// StateCloud indicates the file is only in the cloud (placeholder)
	StateCloud PlaceholderState = "cloud"
	// StateHydrating indicates the file is being downloaded
	StateHydrating PlaceholderState = "hydrating"
	// StateLocal indicates the file is available locally
	StateLocal PlaceholderState = "local"
	// StatePinned indicates the file is pinned (always keep locally)
	StatePinned PlaceholderState = "pinned"
)

// HydrationPriority represents the priority for hydrating files
type HydrationPriority int

const (
	PriorityLow      HydrationPriority = 0
	PriorityNormal   HydrationPriority = 50
	PriorityHigh     HydrationPriority = 100
	PriorityUrgent   HydrationPriority = 200
)

// PlaceholderFile represents a placeholder (cloud-only) file
type PlaceholderFile struct {
	ShareID       string            `json:"share_id"`
	Path          string            `json:"path"`
	Name          string            `json:"name"`
	Size          int64             `json:"size"`
	Hash          string            `json:"hash"`
	ModifiedAt    time.Time         `json:"modified_at"`
	State         PlaceholderState  `json:"state"`
	IsPinned      bool              `json:"is_pinned"`
	LastAccessed  *time.Time        `json:"last_accessed,omitempty"`
	HydrationProgress float64       `json:"hydration_progress,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HydrationRequest represents a request to download a file
type HydrationRequest struct {
	ShareID      string            `json:"share_id"`
	Path         string            `json:"path"`
	Priority     HydrationPriority `json:"priority"`
	RequestedAt  time.Time         `json:"requested_at"`
	RequestedBy  string            `json:"requested_by,omitempty"`
	Callback     func(error)       `json:"-"`
}

// DehydrationPolicy defines when files should be dehydrated (converted to placeholders)
type DehydrationPolicy struct {
	Enabled            bool          `json:"enabled"`
	MaxLocalSize       int64         `json:"max_local_size"`        // Max total size of local files
	MaxFileAge         time.Duration `json:"max_file_age"`          // Max age before dehydration
	MinFreeSpace       int64         `json:"min_free_space"`        // Min free disk space to maintain
	ExcludePatterns    []string      `json:"exclude_patterns"`      // Patterns to never dehydrate
	PinnedAlwaysLocal  bool          `json:"pinned_always_local"`   // Keep pinned files local
}

// SmartSyncManager manages on-demand file synchronization
type SmartSyncManager struct {
	dataDir           string
	placeholders      map[string]*PlaceholderFile
	hydrationQueue    []*HydrationRequest
	policy            DehydrationPolicy
	totalLocalSize    int64
	
	// Callbacks
	onHydrationStart    func(file *PlaceholderFile)
	onHydrationProgress func(file *PlaceholderFile, progress float64)
	onHydrationComplete func(file *PlaceholderFile, err error)
	onDehydrate         func(file *PlaceholderFile)
	
	// Download function (to be provided by caller)
	downloadFunc func(shareID, path string, dest string) error
	
	ctx    context.Context
	cancel context.CancelFunc
	
	logger zerolog.Logger
	mu     sync.RWMutex
}

// NewSmartSyncManager creates a new smart sync manager
func NewSmartSyncManager(dataDir string, logger zerolog.Logger) (*SmartSyncManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &SmartSyncManager{
		dataDir:      dataDir,
		placeholders: make(map[string]*PlaceholderFile),
		policy: DehydrationPolicy{
			Enabled:           true,
			MaxLocalSize:      10 * 1024 * 1024 * 1024, // 10GB default
			MaxFileAge:        30 * 24 * time.Hour,     // 30 days
			MinFreeSpace:      1024 * 1024 * 1024,      // 1GB
			PinnedAlwaysLocal: true,
		},
		ctx:    ctx,
		cancel: cancel,
		logger: logger.With().Str("component", "smart-sync").Logger(),
	}

	// Create smart sync directory
	smartDir := filepath.Join(dataDir, "smartsync")
	if err := os.MkdirAll(smartDir, 0750); err != nil {
		cancel()
		return nil, err
	}

	// Load existing placeholders
	if err := m.load(); err != nil {
		logger.Warn().Err(err).Msg("Failed to load smart sync state")
	}

	return m, nil
}

// Start starts the smart sync manager
func (m *SmartSyncManager) Start() {
	go m.processHydrationQueue()
	go m.dehydrationLoop()
}

// Stop stops the smart sync manager
func (m *SmartSyncManager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// SetDownloadFunc sets the function used to download files
func (m *SmartSyncManager) SetDownloadFunc(fn func(shareID, path string, dest string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadFunc = fn
}

// SetPolicy sets the dehydration policy
func (m *SmartSyncManager) SetPolicy(policy DehydrationPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
}

// GetPolicy returns the current dehydration policy
func (m *SmartSyncManager) GetPolicy() DehydrationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// RegisterPlaceholder registers a new placeholder file
func (m *SmartSyncManager) RegisterPlaceholder(file *PlaceholderFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := file.ShareID + ":" + file.Path
	m.placeholders[key] = file

	return m.save()
}

// GetPlaceholder returns a placeholder file by path
func (m *SmartSyncManager) GetPlaceholder(shareID, path string) *PlaceholderFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.placeholders[shareID+":"+path]
}

// ListPlaceholders returns all placeholders for a share
func (m *SmartSyncManager) ListPlaceholders(shareID string) []*PlaceholderFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PlaceholderFile
	for _, p := range m.placeholders {
		if p.ShareID == shareID {
			result = append(result, p)
		}
	}
	return result
}

// GetCloudOnlyFiles returns files that are only in the cloud
func (m *SmartSyncManager) GetCloudOnlyFiles(shareID string) []*PlaceholderFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PlaceholderFile
	for _, p := range m.placeholders {
		if p.ShareID == shareID && p.State == StateCloud {
			result = append(result, p)
		}
	}
	return result
}

// GetLocalFiles returns files available locally
func (m *SmartSyncManager) GetLocalFiles(shareID string) []*PlaceholderFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PlaceholderFile
	for _, p := range m.placeholders {
		if p.ShareID == shareID && (p.State == StateLocal || p.State == StatePinned) {
			result = append(result, p)
		}
	}
	return result
}

// RequestHydration requests a file to be downloaded
func (m *SmartSyncManager) RequestHydration(shareID, path string, priority HydrationPriority, callback func(error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	placeholder, ok := m.placeholders[key]
	if !ok {
		return errors.New("file not found")
	}

	if placeholder.State == StateLocal || placeholder.State == StatePinned {
		// Already local
		if callback != nil {
			go callback(nil)
		}
		return nil
	}

	if placeholder.State == StateHydrating {
		// Already hydrating, add callback
		return nil
	}

	// Add to queue
	request := &HydrationRequest{
		ShareID:     shareID,
		Path:        path,
		Priority:    priority,
		RequestedAt: time.Now(),
		Callback:    callback,
	}

	// Insert in priority order
	inserted := false
	for i, r := range m.hydrationQueue {
		if priority > r.Priority {
			m.hydrationQueue = append(m.hydrationQueue[:i], append([]*HydrationRequest{request}, m.hydrationQueue[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.hydrationQueue = append(m.hydrationQueue, request)
	}

	placeholder.State = StateHydrating
	m.logger.Info().
		Str("share_id", shareID).
		Str("path", path).
		Int("priority", int(priority)).
		Msg("File queued for hydration")

	return nil
}

// CancelHydration cancels a pending hydration request
func (m *SmartSyncManager) CancelHydration(shareID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove from queue
	for i, r := range m.hydrationQueue {
		if r.ShareID == shareID && r.Path == path {
			m.hydrationQueue = append(m.hydrationQueue[:i], m.hydrationQueue[i+1:]...)
			break
		}
	}

	// Reset state
	key := shareID + ":" + path
	if placeholder, ok := m.placeholders[key]; ok {
		placeholder.State = StateCloud
		placeholder.HydrationProgress = 0
	}

	return nil
}

// PinFile pins a file to always keep it local
func (m *SmartSyncManager) PinFile(shareID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	placeholder, ok := m.placeholders[key]
	if !ok {
		return errors.New("file not found")
	}

	placeholder.IsPinned = true
	if placeholder.State == StateLocal {
		placeholder.State = StatePinned
	}

	// If not local, trigger hydration
	if placeholder.State == StateCloud {
		m.mu.Unlock()
		err := m.RequestHydration(shareID, path, PriorityHigh, nil)
		m.mu.Lock()
		if err != nil {
			return err
		}
	}

	return m.save()
}

// UnpinFile unpins a file
func (m *SmartSyncManager) UnpinFile(shareID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	placeholder, ok := m.placeholders[key]
	if !ok {
		return errors.New("file not found")
	}

	placeholder.IsPinned = false
	if placeholder.State == StatePinned {
		placeholder.State = StateLocal
	}

	return m.save()
}

// Dehydrate converts a local file to a placeholder
func (m *SmartSyncManager) Dehydrate(shareID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	placeholder, ok := m.placeholders[key]
	if !ok {
		return errors.New("file not found")
	}

	if placeholder.IsPinned {
		return errors.New("cannot dehydrate pinned file")
	}

	if placeholder.State != StateLocal {
		return nil // Already a placeholder
	}

	placeholder.State = StateCloud
	m.totalLocalSize -= placeholder.Size

	if m.onDehydrate != nil {
		m.onDehydrate(placeholder)
	}

	m.logger.Info().
		Str("share_id", shareID).
		Str("path", path).
		Int64("size", placeholder.Size).
		Msg("File dehydrated")

	return m.save()
}

// UpdateHydrationProgress updates the progress of a hydration
func (m *SmartSyncManager) UpdateHydrationProgress(shareID, path string, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	if placeholder, ok := m.placeholders[key]; ok {
		placeholder.HydrationProgress = progress
		if m.onHydrationProgress != nil {
			m.onHydrationProgress(placeholder, progress)
		}
	}
}

// CompleteHydration marks a hydration as complete
func (m *SmartSyncManager) CompleteHydration(shareID, path string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	placeholder, ok := m.placeholders[key]
	if !ok {
		return
	}

	if err != nil {
		placeholder.State = StateCloud
		placeholder.HydrationProgress = 0
	} else {
		if placeholder.IsPinned {
			placeholder.State = StatePinned
		} else {
			placeholder.State = StateLocal
		}
		placeholder.HydrationProgress = 100
		m.totalLocalSize += placeholder.Size
		now := time.Now()
		placeholder.LastAccessed = &now
	}

	if m.onHydrationComplete != nil {
		m.onHydrationComplete(placeholder, err)
	}

	m.save()
}

// RecordAccess records file access time
func (m *SmartSyncManager) RecordAccess(shareID, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := shareID + ":" + path
	if placeholder, ok := m.placeholders[key]; ok {
		now := time.Now()
		placeholder.LastAccessed = &now
	}
}

// GetStats returns smart sync statistics
func (m *SmartSyncManager) GetStats() SmartSyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := SmartSyncStats{}
	for _, p := range m.placeholders {
		stats.TotalFiles++
		stats.TotalSize += p.Size

		switch p.State {
		case StateCloud:
			stats.CloudOnlyFiles++
			stats.CloudOnlySize += p.Size
		case StateLocal:
			stats.LocalFiles++
			stats.LocalSize += p.Size
		case StatePinned:
			stats.PinnedFiles++
			stats.PinnedSize += p.Size
		case StateHydrating:
			stats.HydratingFiles++
		}
	}

	stats.QueueLength = len(m.hydrationQueue)
	return stats
}

// SmartSyncStats contains smart sync statistics
type SmartSyncStats struct {
	TotalFiles     int   `json:"total_files"`
	TotalSize      int64 `json:"total_size"`
	CloudOnlyFiles int   `json:"cloud_only_files"`
	CloudOnlySize  int64 `json:"cloud_only_size"`
	LocalFiles     int   `json:"local_files"`
	LocalSize      int64 `json:"local_size"`
	PinnedFiles    int   `json:"pinned_files"`
	PinnedSize     int64 `json:"pinned_size"`
	HydratingFiles int   `json:"hydrating_files"`
	QueueLength    int   `json:"queue_length"`
}

func (m *SmartSyncManager) processHydrationQueue() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.processNextHydration()
	}
}

func (m *SmartSyncManager) processNextHydration() {
	m.mu.Lock()
	if len(m.hydrationQueue) == 0 {
		m.mu.Unlock()
		return
	}

	if m.downloadFunc == nil {
		m.mu.Unlock()
		return
	}

	// Get next request
	request := m.hydrationQueue[0]
	m.hydrationQueue = m.hydrationQueue[1:]
	m.mu.Unlock()

	// Start hydration
	key := request.ShareID + ":" + request.Path
	m.mu.RLock()
	placeholder, ok := m.placeholders[key]
	m.mu.RUnlock()

	if !ok {
		if request.Callback != nil {
			request.Callback(errors.New("file not found"))
		}
		return
	}

	if m.onHydrationStart != nil {
		m.onHydrationStart(placeholder)
	}

	m.logger.Info().
		Str("share_id", request.ShareID).
		Str("path", request.Path).
		Msg("Starting file hydration")

	// Download file
	dest := filepath.Join(m.dataDir, "cache", request.ShareID, request.Path)
	if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
		m.CompleteHydration(request.ShareID, request.Path, err)
		if request.Callback != nil {
			request.Callback(err)
		}
		return
	}

	err := m.downloadFunc(request.ShareID, request.Path, dest)
	m.CompleteHydration(request.ShareID, request.Path, err)

	if request.Callback != nil {
		request.Callback(err)
	}
}

func (m *SmartSyncManager) dehydrationLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.performDehydration()
	}
}

func (m *SmartSyncManager) performDehydration() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.policy.Enabled {
		return
	}

	// Check if we need to free space
	needToFree := int64(0)
	if m.totalLocalSize > m.policy.MaxLocalSize {
		needToFree = m.totalLocalSize - m.policy.MaxLocalSize
	}

	// Collect candidates for dehydration
	type candidate struct {
		key  string
		file *PlaceholderFile
		age  time.Duration
	}

	var candidates []candidate
	now := time.Now()

	for key, p := range m.placeholders {
		if p.State != StateLocal {
			continue
		}
		if p.IsPinned && m.policy.PinnedAlwaysLocal {
			continue
		}

		age := time.Duration(0)
		if p.LastAccessed != nil {
			age = now.Sub(*p.LastAccessed)
		} else {
			age = now.Sub(p.ModifiedAt)
		}

		candidates = append(candidates, candidate{key, p, age})
	}

	// Sort by age (oldest first)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].age < candidates[j].age {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Dehydrate files
	freedSpace := int64(0)
	for _, c := range candidates {
		// Check age policy
		if c.age > m.policy.MaxFileAge || freedSpace < needToFree {
			c.file.State = StateCloud
			m.totalLocalSize -= c.file.Size
			freedSpace += c.file.Size

			if m.onDehydrate != nil {
				m.onDehydrate(c.file)
			}

			m.logger.Debug().
				Str("path", c.file.Path).
				Dur("age", c.age).
				Int64("size", c.file.Size).
				Msg("Dehydrated file")
		}

		if freedSpace >= needToFree && c.age <= m.policy.MaxFileAge {
			break
		}
	}

	if freedSpace > 0 {
		m.save()
		m.logger.Info().
			Int64("freed_bytes", freedSpace).
			Int64("total_local", m.totalLocalSize).
			Msg("Dehydration complete")
	}
}

func (m *SmartSyncManager) load() error {
	statePath := filepath.Join(m.dataDir, "smartsync", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var files []*PlaceholderFile
	if err := json.Unmarshal(data, &files); err != nil {
		return err
	}

	for _, f := range files {
		m.placeholders[f.ShareID+":"+f.Path] = f
		if f.State == StateLocal || f.State == StatePinned {
			m.totalLocalSize += f.Size
		}
	}

	return nil
}

func (m *SmartSyncManager) save() error {
	files := make([]*PlaceholderFile, 0, len(m.placeholders))
	for _, f := range m.placeholders {
		files = append(files, f)
	}

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(m.dataDir, "smartsync", "state.json")
	return os.WriteFile(statePath, data, 0640)
}

// Callbacks
func (m *SmartSyncManager) SetOnHydrationStart(fn func(file *PlaceholderFile)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHydrationStart = fn
}

func (m *SmartSyncManager) SetOnHydrationProgress(fn func(file *PlaceholderFile, progress float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHydrationProgress = fn
}

func (m *SmartSyncManager) SetOnHydrationComplete(fn func(file *PlaceholderFile, err error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHydrationComplete = fn
}

func (m *SmartSyncManager) SetOnDehydrate(fn func(file *PlaceholderFile)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDehydrate = fn
}

