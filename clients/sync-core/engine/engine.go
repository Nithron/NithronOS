// Package engine provides the main sync engine.
package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"nithronos/clients/sync-core/api"
	"nithronos/clients/sync-core/config"
	"nithronos/clients/sync-core/db"
	"nithronos/clients/sync-core/hash"
	"nithronos/clients/sync-core/watcher"
)

// State represents the current sync state.
type State int

const (
	StateStopped State = iota
	StateStarting
	StateIdle
	StateSyncing
	StatePaused
	StateError
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateIdle:
		return "idle"
	case StateSyncing:
		return "syncing"
	case StatePaused:
		return "paused"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Engine is the main sync engine.
type Engine struct {
	cfg       *config.Config
	apiClient *api.Client
	webdav    *api.WebDAVClient
	database  *db.Database
	watchers  map[string]*watcher.Watcher // shareID -> watcher
	logger    zerolog.Logger

	state      atomic.Int32
	lastError  atomic.Value
	syncMu     sync.Mutex
	shares     []api.SyncShare
	sharesMu   sync.RWMutex

	// Progress tracking
	currentFile    atomic.Value
	uploadedBytes  atomic.Int64
	downloadedBytes atomic.Int64
	pendingUploads  atomic.Int64
	pendingDownloads atomic.Int64

	// Control
	ctx        context.Context
	cancel     context.CancelFunc
	pauseChan  chan struct{}
	resumeChan chan struct{}

	// Event callbacks
	onStateChange   func(State)
	onProgress      func(Progress)
	onError         func(error)
	onConflict      func(Conflict)
}

// Progress represents sync progress.
type Progress struct {
	State            State
	CurrentFile      string
	UploadedBytes    int64
	DownloadedBytes  int64
	PendingUploads   int64
	PendingDownloads int64
	TotalFiles       int64
	SyncedFiles      int64
}

// Conflict represents a sync conflict.
type Conflict struct {
	ShareID       string
	Path          string
	LocalModTime  time.Time
	RemoteModTime time.Time
	LocalSize     int64
	RemoteSize    int64
}

// New creates a new sync engine.
func New(cfg *config.Config, logger zerolog.Logger) (*Engine, error) {
	database, err := db.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:        cfg,
		apiClient:  api.NewClient(cfg),
		webdav:     api.NewWebDAVClient(cfg),
		database:   database,
		watchers:   make(map[string]*watcher.Watcher),
		logger:     logger.With().Str("component", "sync-engine").Logger(),
		ctx:        ctx,
		cancel:     cancel,
		pauseChan:  make(chan struct{}),
		resumeChan: make(chan struct{}),
	}

	e.state.Store(int32(StateStopped))
	e.currentFile.Store("")

	// Set up token refresh callback
	e.apiClient.SetTokenRefreshCallback(func(access, refresh string) {
		e.logger.Info().Msg("Token refreshed successfully")
	})

	return e, nil
}

// SetCallbacks sets event callbacks.
func (e *Engine) SetCallbacks(onState func(State), onProgress func(Progress), onError func(error), onConflict func(Conflict)) {
	e.onStateChange = onState
	e.onProgress = onProgress
	e.onError = onError
	e.onConflict = onConflict
}

// Start starts the sync engine.
func (e *Engine) Start() error {
	e.setState(StateStarting)

	// Verify configuration
	if !e.cfg.IsConfigured() {
		return fmt.Errorf("sync client not configured")
	}

	// Check server connectivity
	if err := e.apiClient.HealthCheck(e.ctx); err != nil {
		e.setState(StateError)
		e.setError(err)
		return fmt.Errorf("server not reachable: %w", err)
	}

	// Fetch available shares
	shares, err := e.apiClient.ListShares(e.ctx)
	if err != nil {
		e.setState(StateError)
		e.setError(err)
		return fmt.Errorf("failed to list shares: %w", err)
	}

	e.sharesMu.Lock()
	e.shares = shares
	e.sharesMu.Unlock()

	// Ensure sync folder exists
	syncFolder := e.cfg.SyncFolder
	if err := os.MkdirAll(syncFolder, 0755); err != nil {
		e.setState(StateError)
		return fmt.Errorf("failed to create sync folder: %w", err)
	}

	// Start file watchers for each share
	for _, share := range shares {
		if !e.isShareEnabled(share.ID) {
			continue
		}

		sharePath := filepath.Join(syncFolder, share.Name)
		if err := os.MkdirAll(sharePath, 0755); err != nil {
			e.logger.Error().Err(err).Str("share", share.Name).Msg("Failed to create share folder")
			continue
		}

		watcherCfg := watcher.DefaultConfig(sharePath)
		watcherCfg.ExcludePatterns = append(watcherCfg.ExcludePatterns, share.SyncExclude...)

		w, err := watcher.New(watcherCfg, e.logger)
		if err != nil {
			e.logger.Error().Err(err).Str("share", share.Name).Msg("Failed to create watcher")
			continue
		}

		if err := w.Start(); err != nil {
			e.logger.Error().Err(err).Str("share", share.Name).Msg("Failed to start watcher")
			continue
		}

		e.watchers[share.ID] = w

		// Start processing watcher events
		go e.processWatcherEvents(share.ID, share.Name, w)
	}

	// Start main sync loop
	go e.syncLoop()

	e.setState(StateIdle)
	e.logger.Info().Int("shares", len(e.watchers)).Msg("Sync engine started")

	return nil
}

// Stop stops the sync engine.
func (e *Engine) Stop() error {
	e.cancel()

	// Stop all watchers
	for shareID, w := range e.watchers {
		if err := w.Stop(); err != nil {
			e.logger.Error().Err(err).Str("shareID", shareID).Msg("Failed to stop watcher")
		}
	}

	// Close database
	if err := e.database.Close(); err != nil {
		e.logger.Error().Err(err).Msg("Failed to close database")
	}

	e.setState(StateStopped)
	e.logger.Info().Msg("Sync engine stopped")

	return nil
}

// Pause pauses syncing.
func (e *Engine) Pause() {
	e.setState(StatePaused)
	close(e.pauseChan)
	e.pauseChan = make(chan struct{})
	e.logger.Info().Msg("Sync paused")
}

// Resume resumes syncing.
func (e *Engine) Resume() {
	e.setState(StateIdle)
	close(e.resumeChan)
	e.resumeChan = make(chan struct{})
	e.logger.Info().Msg("Sync resumed")
}

// SyncNow triggers an immediate sync.
func (e *Engine) SyncNow() {
	if e.getState() == StatePaused {
		return
	}

	e.sharesMu.RLock()
	shares := e.shares
	e.sharesMu.RUnlock()

	for _, share := range shares {
		if e.isShareEnabled(share.ID) {
			go e.syncShare(share.ID, share.Name)
		}
	}
}

// GetState returns the current state.
func (e *Engine) GetState() State {
	return e.getState()
}

// GetProgress returns the current progress.
func (e *Engine) GetProgress() Progress {
	var currentFile string
	if v := e.currentFile.Load(); v != nil {
		currentFile = v.(string)
	}

	return Progress{
		State:            e.getState(),
		CurrentFile:      currentFile,
		UploadedBytes:    e.uploadedBytes.Load(),
		DownloadedBytes:  e.downloadedBytes.Load(),
		PendingUploads:   e.pendingUploads.Load(),
		PendingDownloads: e.pendingDownloads.Load(),
	}
}

// GetStats returns sync statistics.
func (e *Engine) GetStats() map[string]*db.SyncStats {
	stats := make(map[string]*db.SyncStats)

	e.sharesMu.RLock()
	shares := e.shares
	e.sharesMu.RUnlock()

	for _, share := range shares {
		if s, err := e.database.GetStats(share.ID); err == nil {
			stats[share.ID] = s
		}
	}

	return stats
}

// GetRecentActivity returns recent activity.
func (e *Engine) GetRecentActivity(limit int) ([]db.Activity, error) {
	return e.database.GetRecentActivity(limit)
}

// Internal methods

func (e *Engine) getState() State {
	return State(e.state.Load())
}

func (e *Engine) setState(state State) {
	old := State(e.state.Swap(int32(state)))
	if old != state && e.onStateChange != nil {
		e.onStateChange(state)
	}
}

func (e *Engine) setError(err error) {
	e.lastError.Store(err)
	if e.onError != nil {
		e.onError(err)
	}
}

func (e *Engine) isShareEnabled(shareID string) bool {
	if len(e.cfg.SyncShares) == 0 {
		return true // Sync all shares if none specified
	}
	for _, id := range e.cfg.SyncShares {
		if id == shareID {
			return true
		}
	}
	return false
}

func (e *Engine) processWatcherEvents(shareID, shareName string, w *watcher.Watcher) {
	for {
		select {
		case <-e.ctx.Done():
			return

		case event, ok := <-w.Events():
			if !ok {
				return
			}

			if e.getState() == StatePaused {
				continue
			}

			relPath, err := w.GetRelativePath(event.Path)
			if err != nil {
				continue
			}

			e.logger.Debug().
				Str("share", shareName).
				Str("path", relPath).
				Str("op", event.Op.String()).
				Msg("File change detected")

			// Queue the operation
			switch event.Op {
			case watcher.OpCreate, watcher.OpWrite:
				e.database.EnqueueOperation(shareID, relPath, "upload", 1)
				e.pendingUploads.Add(1)
			case watcher.OpRemove:
				e.database.EnqueueOperation(shareID, relPath, "delete_remote", 0)
			case watcher.OpRename:
				// Renames are handled as delete + create by fsnotify
				e.database.EnqueueOperation(shareID, relPath, "upload", 1)
			}

		case err, ok := <-w.Errors():
			if !ok {
				return
			}
			e.logger.Error().Err(err).Str("share", shareName).Msg("Watcher error")
		}
	}
}

func (e *Engine) syncLoop() {
	pollInterval := time.Duration(e.cfg.PollIntervalSecs) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 30 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Initial sync
	e.SyncNow()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			if e.getState() == StatePaused {
				continue
			}
			e.SyncNow()

		case <-e.pauseChan:
			// Wait for resume
			<-e.resumeChan
		}
	}
}

func (e *Engine) syncShare(shareID, shareName string) {
	e.syncMu.Lock()
	defer e.syncMu.Unlock()

	if e.getState() == StatePaused || e.getState() == StateStopped {
		return
	}

	e.setState(StateSyncing)
	defer func() {
		if e.getState() == StateSyncing {
			e.setState(StateIdle)
		}
	}()

	e.logger.Debug().Str("share", shareName).Msg("Starting share sync")

	// Get current cursor
	cursor, _ := e.database.GetCursor(shareID)

	// Fetch remote changes
	changes, err := e.apiClient.GetChanges(e.ctx, shareID, cursor, 1000)
	if err != nil {
		e.logger.Error().Err(err).Str("share", shareName).Msg("Failed to fetch changes")
		return
	}

	// Process remote changes
	for _, change := range changes.Changes {
		if err := e.processRemoteChange(shareID, shareName, change); err != nil {
			e.logger.Error().Err(err).
				Str("share", shareName).
				Str("path", change.Path).
				Msg("Failed to process remote change")
		}
	}

	// Update cursor
	if changes.Cursor != "" {
		e.database.SetCursor(shareID, changes.Cursor)
	}

	// Process pending local operations
	e.processLocalOperations(shareID, shareName)

	e.logger.Debug().Str("share", shareName).Msg("Share sync completed")
}

func (e *Engine) processRemoteChange(shareID, shareName string, change api.FileChange) error {
	sharePath := filepath.Join(e.cfg.SyncFolder, shareName)
	localPath := filepath.Join(sharePath, change.Path)

	switch change.Action {
	case "created", "modified":
		if change.Type == "dir" {
			return os.MkdirAll(localPath, 0755)
		}

		// Check for conflicts
		if info, err := os.Stat(localPath); err == nil {
			localHash, _ := hash.QuickHash(localPath)
			
			// Check if local file was modified
			entry, _ := e.database.GetFile(shareID, change.Path)
			if entry != nil && entry.LocalHash != localHash {
				// Conflict detected
				e.handleConflict(shareID, change.Path, info.ModTime(), localHash, change)
				return nil
			}
		}

		// Download the file
		e.currentFile.Store(change.Path)
		e.pendingDownloads.Add(1)
		defer e.pendingDownloads.Add(-1)

		if err := e.downloadFile(shareID, shareName, change.Path, localPath); err != nil {
			return err
		}

		// Update database
		entry := &db.FileEntry{
			ShareID:    shareID,
			Path:       change.Path,
			Type:       change.Type,
			Size:       change.Size,
			RemoteHash: change.ContentHash,
			SyncStatus: "synced",
			LastSyncAt: time.Now(),
			Version:    change.Version,
		}
		entry.LocalHash, _ = hash.QuickHash(localPath)

		return e.database.UpsertFile(entry)

	case "deleted":
		// Delete local file
		if err := os.RemoveAll(localPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return e.database.DeleteFile(shareID, change.Path)

	case "moved":
		if change.PreviousPath != "" {
			oldPath := filepath.Join(sharePath, change.PreviousPath)
			if err := os.Rename(oldPath, localPath); err != nil {
				// If rename fails, download the file
				return e.downloadFile(shareID, shareName, change.Path, localPath)
			}
			e.database.DeleteFile(shareID, change.PreviousPath)
		}
		return nil
	}

	return nil
}

func (e *Engine) downloadFile(shareID, shareName, remotePath, localPath string) error {
	// Use delta sync for large files
	if info, err := os.Stat(localPath); err == nil && info.Size() > hash.DefaultBlockSize {
		return e.deltaDownload(shareID, shareName, remotePath, localPath)
	}

	// Regular download
	if err := e.webdav.Download(e.ctx, shareID, remotePath, localPath); err != nil {
		e.database.LogActivity(shareID, remotePath, "download", "error", err.Error(), 0)
		return err
	}

	info, _ := os.Stat(localPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	e.downloadedBytes.Add(size)
	e.database.LogActivity(shareID, remotePath, "download", "success", "", size)

	return nil
}

func (e *Engine) deltaDownload(shareID, shareName, remotePath, localPath string) error {
	// Get remote block hashes
	remoteHashes, err := e.apiClient.GetBlockHashes(e.ctx, shareID, remotePath, hash.DefaultBlockSize)
	if err != nil {
		// Fall back to regular download
		return e.webdav.Download(e.ctx, shareID, remotePath, localPath)
	}

	// Compute delta plan
	localHashes, err := hash.ComputeBlockHashes(localPath, hash.DefaultBlockSize)
	if err != nil {
		return e.webdav.Download(e.ctx, shareID, remotePath, localPath)
	}

	// Find blocks that need to be downloaded
	neededBlocks := make([]int, 0)
	remoteBlockMap := make(map[string]api.BlockHash)
	for i, rb := range remoteHashes.Blocks {
		remoteBlockMap[rb.StrongHash] = rb
		found := false
		for _, lb := range localHashes.Blocks {
			if lb.StrongHash == rb.StrongHash {
				found = true
				break
			}
		}
		if !found {
			neededBlocks = append(neededBlocks, i)
		}
	}

	if len(neededBlocks) == 0 {
		// Files are identical
		return nil
	}

	// Download only needed blocks and reconstruct
	tmpPath := localPath + ".nstmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		os.Remove(tmpPath)
		return e.webdav.Download(e.ctx, shareID, remotePath, localPath)
	}
	defer localFile.Close()

	// Build the new file from local blocks and downloaded blocks
	for i, rb := range remoteHashes.Blocks {
		var blockData []byte

		// Check if we have this block locally
		found := false
		for _, lb := range localHashes.Blocks {
			if lb.StrongHash == rb.StrongHash {
				// Read from local file
				blockData = make([]byte, lb.Size)
				localFile.Seek(lb.Offset, io.SeekStart)
				localFile.Read(blockData)
				found = true
				break
			}
		}

		if !found {
			// Download this block
			blockData, err = e.webdav.DownloadRange(e.ctx, shareID, remotePath, rb.Offset, int64(rb.Size))
			if err != nil {
				os.Remove(tmpPath)
				return e.webdav.Download(e.ctx, shareID, remotePath, localPath)
			}
			e.downloadedBytes.Add(int64(len(blockData)))
		}

		if _, err := f.Write(blockData); err != nil {
			os.Remove(tmpPath)
			return err
		}

		_ = i // Used for tracking progress
	}

	f.Close()
	localFile.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

func (e *Engine) processLocalOperations(shareID, shareName string) {
	ops, err := e.database.DequeueOperations(e.cfg.MaxConcurrent)
	if err != nil {
		e.logger.Error().Err(err).Msg("Failed to dequeue operations")
		return
	}

	if len(ops) == 0 {
		return
	}

	g, ctx := errgroup.WithContext(e.ctx)

	for _, op := range ops {
		op := op // Capture for closure
		g.Go(func() error {
			return e.processOperation(ctx, shareID, shareName, op)
		})
	}

	if err := g.Wait(); err != nil {
		e.logger.Error().Err(err).Msg("Error processing operations")
	}
}

func (e *Engine) processOperation(ctx context.Context, shareID, shareName string, op db.QueueOperation) error {
	sharePath := filepath.Join(e.cfg.SyncFolder, shareName)
	localPath := filepath.Join(sharePath, op.Path)

	switch op.Operation {
	case "upload":
		return e.uploadFile(ctx, shareID, shareName, op.Path, localPath, op.RetryCount)

	case "delete_remote":
		if err := e.webdav.Delete(ctx, shareID, op.Path); err != nil {
			e.logger.Error().Err(err).Str("path", op.Path).Msg("Failed to delete remote file")
			// Requeue if not permanent failure
			if op.RetryCount < e.cfg.RetryAttempts {
				e.database.RequeueOperation(shareID, op.Path, op.Operation, err.Error(), op.RetryCount+1, time.Duration(e.cfg.RetryDelaySecs)*time.Second)
			}
			return err
		}
		e.database.DeleteFile(shareID, op.Path)
		e.database.LogActivity(shareID, op.Path, "delete_remote", "success", "", 0)

	case "mkdir":
		if err := e.webdav.MkdirAll(ctx, shareID, op.Path); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) uploadFile(ctx context.Context, shareID, shareName, remotePath, localPath string, retryCount int) error {
	info, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		// File was deleted before upload
		return nil
	}
	if err != nil {
		return err
	}

	if info.IsDir() {
		return e.webdav.MkdirAll(ctx, shareID, remotePath)
	}

	e.currentFile.Store(remotePath)
	defer e.currentFile.Store("")

	// Use delta sync for large files if remote exists
	if info.Size() > hash.DefaultBlockSize {
		if exists, _ := e.webdav.Exists(ctx, shareID, remotePath); exists {
			if err := e.deltaUpload(ctx, shareID, remotePath, localPath); err == nil {
				return nil
			}
			// Fall through to regular upload on error
		}
	}

	// Regular upload
	if err := e.webdav.Upload(ctx, shareID, localPath, remotePath); err != nil {
		e.database.LogActivity(shareID, remotePath, "upload", "error", err.Error(), 0)
		
		if retryCount < e.cfg.RetryAttempts {
			e.database.RequeueOperation(shareID, remotePath, "upload", err.Error(), retryCount+1, time.Duration(e.cfg.RetryDelaySecs)*time.Second)
		}
		return err
	}

	e.uploadedBytes.Add(info.Size())
	e.pendingUploads.Add(-1)

	// Update database
	localHash, _ := hash.QuickHash(localPath)
	entry := &db.FileEntry{
		ShareID:    shareID,
		Path:       remotePath,
		Type:       "file",
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		LocalHash:  localHash,
		SyncStatus: "synced",
		LastSyncAt: time.Now(),
	}
	e.database.UpsertFile(entry)
	e.database.LogActivity(shareID, remotePath, "upload", "success", "", info.Size())

	return nil
}

func (e *Engine) deltaUpload(ctx context.Context, shareID, remotePath, localPath string) error {
	// Get remote block hashes
	remoteHashes, err := e.apiClient.GetBlockHashes(ctx, shareID, remotePath, hash.DefaultBlockSize)
	if err != nil {
		return err
	}

	// Compute delta plan
	plan, err := hash.ComputeDeltaPlan(localPath, &hash.BlockHashes{
		BlockSize: remoteHashes.BlockSize,
		FileSize:  remoteHashes.Size,
		Blocks:    convertAPIBlockHashes(remoteHashes.Blocks),
	}, hash.DefaultBlockSize)
	if err != nil {
		return err
	}

	// If delta savings are minimal, do full upload
	if float64(plan.SavedBytes)/float64(plan.TotalBytes) < 0.2 {
		return fmt.Errorf("delta savings too small")
	}

	// For simplicity, we'll do a full upload for now
	// A full delta upload implementation would send only the changed blocks
	// and have the server reconstruct the file
	return fmt.Errorf("delta upload requires server-side support")
}

func (e *Engine) handleConflict(shareID, path string, localModTime time.Time, localHash string, remote api.FileChange) {
	e.logger.Warn().
		Str("share", shareID).
		Str("path", path).
		Msg("Conflict detected")

	shareName := e.getShareName(shareID)
	sharePath := filepath.Join(e.cfg.SyncFolder, shareName)
	localPath := filepath.Join(sharePath, path)

	switch e.cfg.ConflictPolicy {
	case "keep_local":
		// Upload local version
		e.database.EnqueueOperation(shareID, path, "upload", 2)

	case "keep_remote":
		// Download remote version
		e.downloadFile(shareID, shareName, path, localPath)

	case "keep_both":
		fallthrough
	default:
		// Rename local file and download remote
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)
		conflictPath := fmt.Sprintf("%s (Conflict %s)%s", base, time.Now().Format("2006-01-02 15-04-05"), ext)
		
		conflictLocalPath := filepath.Join(sharePath, conflictPath)
		os.Rename(localPath, conflictLocalPath)
		
		// Download remote version
		e.downloadFile(shareID, shareName, path, localPath)
		
		// Upload the conflict copy
		e.database.EnqueueOperation(shareID, conflictPath, "upload", 0)
	}

	// Record conflict
	e.database.AddConflict(&db.Conflict{
		ShareID:       shareID,
		Path:          path,
		LocalHash:     localHash,
		RemoteHash:    remote.ContentHash,
		LocalModTime:  localModTime,
	})

	if e.onConflict != nil {
		e.onConflict(Conflict{
			ShareID:       shareID,
			Path:          path,
			LocalModTime:  localModTime,
		})
	}
}

func (e *Engine) getShareName(shareID string) string {
	e.sharesMu.RLock()
	defer e.sharesMu.RUnlock()
	
	for _, share := range e.shares {
		if share.ID == shareID {
			return share.Name
		}
	}
	return shareID
}

func convertAPIBlockHashes(blocks []api.BlockHash) []hash.BlockHash {
	result := make([]hash.BlockHash, len(blocks))
	for i, b := range blocks {
		result[i] = hash.BlockHash{
			Index:      b.Index,
			Offset:     b.Offset,
			Size:       b.Size,
			StrongHash: b.StrongHash,
			WeakHash:   b.WeakHash,
		}
	}
	return result
}

