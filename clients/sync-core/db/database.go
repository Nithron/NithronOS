// Package db provides the local SQLite database for sync state.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nithronos/clients/sync-core/config"
)

// Database represents the local sync state database.
type Database struct {
	db *sql.DB
}

// FileEntry represents a file in the local database.
type FileEntry struct {
	ID           int64
	ShareID      string
	Path         string
	Type         string // "file" or "dir"
	Size         int64
	ModTime      time.Time
	LocalHash    string
	RemoteHash   string
	SyncStatus   string // "synced", "pending_upload", "pending_download", "conflict", "error"
	LastSyncAt   time.Time
	Version      int
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SyncCursor represents a sync cursor for a share.
type SyncCursor struct {
	ShareID   string
	Cursor    string
	UpdatedAt time.Time
}

// Open opens or creates the database.
func Open() (*Database, error) {
	dataDir, err := config.GetDataDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "sync.db")
	return OpenPath(dbPath)
}

// OpenPath opens a database at the specified path.
func OpenPath(path string) (*Database, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return database, nil
}

// Close closes the database.
func (d *Database) Close() error {
	return d.db.Close()
}

// migrate creates or updates the database schema.
func (d *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		share_id TEXT NOT NULL,
		path TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'file',
		size INTEGER NOT NULL DEFAULT 0,
		mod_time DATETIME,
		local_hash TEXT,
		remote_hash TEXT,
		sync_status TEXT NOT NULL DEFAULT 'pending',
		last_sync_at DATETIME,
		version INTEGER NOT NULL DEFAULT 0,
		error_message TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(share_id, path)
	);

	CREATE INDEX IF NOT EXISTS idx_files_share_path ON files(share_id, path);
	CREATE INDEX IF NOT EXISTS idx_files_sync_status ON files(sync_status);
	CREATE INDEX IF NOT EXISTS idx_files_share_status ON files(share_id, sync_status);

	CREATE TABLE IF NOT EXISTS cursors (
		share_id TEXT PRIMARY KEY,
		cursor TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sync_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		share_id TEXT NOT NULL,
		path TEXT NOT NULL,
		operation TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 0,
		retry_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		scheduled_at DATETIME,
		UNIQUE(share_id, path, operation)
	);

	CREATE INDEX IF NOT EXISTS idx_queue_priority ON sync_queue(priority DESC, created_at ASC);
	CREATE INDEX IF NOT EXISTS idx_queue_scheduled ON sync_queue(scheduled_at);

	CREATE TABLE IF NOT EXISTS conflicts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		share_id TEXT NOT NULL,
		path TEXT NOT NULL,
		local_hash TEXT,
		remote_hash TEXT,
		local_mod_time DATETIME,
		remote_mod_time DATETIME,
		resolution TEXT,
		resolved_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS activity_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		share_id TEXT,
		path TEXT,
		action TEXT NOT NULL,
		status TEXT NOT NULL,
		message TEXT,
		bytes_transferred INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_activity_created ON activity_log(created_at DESC);
	`

	_, err := d.db.Exec(schema)
	return err
}

// GetFile retrieves a file entry by share and path.
func (d *Database) GetFile(shareID, path string) (*FileEntry, error) {
	query := `SELECT id, share_id, path, type, size, mod_time, local_hash, remote_hash, 
			  sync_status, last_sync_at, version, error_message, created_at, updated_at
			  FROM files WHERE share_id = ? AND path = ?`

	var f FileEntry
	var modTime, lastSyncAt, createdAt, updatedAt sql.NullTime
	var localHash, remoteHash, errorMsg sql.NullString

	err := d.db.QueryRow(query, shareID, path).Scan(
		&f.ID, &f.ShareID, &f.Path, &f.Type, &f.Size, &modTime,
		&localHash, &remoteHash, &f.SyncStatus, &lastSyncAt,
		&f.Version, &errorMsg, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if modTime.Valid {
		f.ModTime = modTime.Time
	}
	if lastSyncAt.Valid {
		f.LastSyncAt = lastSyncAt.Time
	}
	if createdAt.Valid {
		f.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		f.UpdatedAt = updatedAt.Time
	}
	f.LocalHash = localHash.String
	f.RemoteHash = remoteHash.String
	f.ErrorMessage = errorMsg.String

	return &f, nil
}

// UpsertFile creates or updates a file entry.
func (d *Database) UpsertFile(f *FileEntry) error {
	query := `INSERT INTO files (share_id, path, type, size, mod_time, local_hash, remote_hash, 
			  sync_status, last_sync_at, version, error_message, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(share_id, path) DO UPDATE SET
			  type = excluded.type,
			  size = excluded.size,
			  mod_time = excluded.mod_time,
			  local_hash = excluded.local_hash,
			  remote_hash = excluded.remote_hash,
			  sync_status = excluded.sync_status,
			  last_sync_at = excluded.last_sync_at,
			  version = excluded.version,
			  error_message = excluded.error_message,
			  updated_at = CURRENT_TIMESTAMP`

	var lastSync interface{}
	if !f.LastSyncAt.IsZero() {
		lastSync = f.LastSyncAt
	}

	_, err := d.db.Exec(query, f.ShareID, f.Path, f.Type, f.Size, f.ModTime,
		f.LocalHash, f.RemoteHash, f.SyncStatus, lastSync, f.Version, f.ErrorMessage)
	return err
}

// DeleteFile removes a file entry.
func (d *Database) DeleteFile(shareID, path string) error {
	_, err := d.db.Exec("DELETE FROM files WHERE share_id = ? AND path = ?", shareID, path)
	return err
}

// DeleteFilesByShare removes all file entries for a share.
func (d *Database) DeleteFilesByShare(shareID string) error {
	_, err := d.db.Exec("DELETE FROM files WHERE share_id = ?", shareID)
	return err
}

// ListFiles lists all files for a share.
func (d *Database) ListFiles(shareID string) ([]FileEntry, error) {
	query := `SELECT id, share_id, path, type, size, mod_time, local_hash, remote_hash, 
			  sync_status, last_sync_at, version, error_message, created_at, updated_at
			  FROM files WHERE share_id = ? ORDER BY path`

	rows, err := d.db.Query(query, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileEntry
	for rows.Next() {
		var f FileEntry
		var modTime, lastSyncAt, createdAt, updatedAt sql.NullTime
		var localHash, remoteHash, errorMsg sql.NullString

		err := rows.Scan(&f.ID, &f.ShareID, &f.Path, &f.Type, &f.Size, &modTime,
			&localHash, &remoteHash, &f.SyncStatus, &lastSyncAt,
			&f.Version, &errorMsg, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		if modTime.Valid {
			f.ModTime = modTime.Time
		}
		if lastSyncAt.Valid {
			f.LastSyncAt = lastSyncAt.Time
		}
		if createdAt.Valid {
			f.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			f.UpdatedAt = updatedAt.Time
		}
		f.LocalHash = localHash.String
		f.RemoteHash = remoteHash.String
		f.ErrorMessage = errorMsg.String

		files = append(files, f)
	}

	return files, rows.Err()
}

// ListPendingFiles lists files with pending sync status.
func (d *Database) ListPendingFiles(shareID string, limit int) ([]FileEntry, error) {
	query := `SELECT id, share_id, path, type, size, mod_time, local_hash, remote_hash, 
			  sync_status, last_sync_at, version, error_message, created_at, updated_at
			  FROM files 
			  WHERE share_id = ? AND sync_status IN ('pending_upload', 'pending_download', 'pending')
			  ORDER BY updated_at ASC
			  LIMIT ?`

	rows, err := d.db.Query(query, shareID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileEntry
	for rows.Next() {
		var f FileEntry
		var modTime, lastSyncAt, createdAt, updatedAt sql.NullTime
		var localHash, remoteHash, errorMsg sql.NullString

		err := rows.Scan(&f.ID, &f.ShareID, &f.Path, &f.Type, &f.Size, &modTime,
			&localHash, &remoteHash, &f.SyncStatus, &lastSyncAt,
			&f.Version, &errorMsg, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		if modTime.Valid {
			f.ModTime = modTime.Time
		}
		if lastSyncAt.Valid {
			f.LastSyncAt = lastSyncAt.Time
		}
		f.LocalHash = localHash.String
		f.RemoteHash = remoteHash.String
		f.ErrorMessage = errorMsg.String

		files = append(files, f)
	}

	return files, rows.Err()
}

// GetCursor retrieves the sync cursor for a share.
func (d *Database) GetCursor(shareID string) (string, error) {
	var cursor string
	err := d.db.QueryRow("SELECT cursor FROM cursors WHERE share_id = ?", shareID).Scan(&cursor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cursor, err
}

// SetCursor updates the sync cursor for a share.
func (d *Database) SetCursor(shareID, cursor string) error {
	query := `INSERT INTO cursors (share_id, cursor, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(share_id) DO UPDATE SET cursor = excluded.cursor, updated_at = CURRENT_TIMESTAMP`
	_, err := d.db.Exec(query, shareID, cursor)
	return err
}

// QueueOperation represents a queued sync operation.
type QueueOperation struct {
	ID          int64
	ShareID     string
	Path        string
	Operation   string // "upload", "download", "delete", "mkdir"
	Priority    int
	RetryCount  int
	LastError   string
	CreatedAt   time.Time
	ScheduledAt time.Time
}

// EnqueueOperation adds an operation to the sync queue.
func (d *Database) EnqueueOperation(shareID, path, operation string, priority int) error {
	query := `INSERT INTO sync_queue (share_id, path, operation, priority, scheduled_at)
			  VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(share_id, path, operation) DO UPDATE SET
			  priority = MAX(excluded.priority, sync_queue.priority),
			  scheduled_at = CURRENT_TIMESTAMP`
	_, err := d.db.Exec(query, shareID, path, operation, priority)
	return err
}

// DequeueOperations retrieves and removes operations from the queue.
func (d *Database) DequeueOperations(limit int) ([]QueueOperation, error) {
	query := `SELECT id, share_id, path, operation, priority, retry_count, last_error, created_at, scheduled_at
			  FROM sync_queue
			  WHERE scheduled_at <= CURRENT_TIMESTAMP
			  ORDER BY priority DESC, created_at ASC
			  LIMIT ?`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []QueueOperation
	for rows.Next() {
		var op QueueOperation
		var lastError sql.NullString
		var scheduledAt sql.NullTime

		err := rows.Scan(&op.ID, &op.ShareID, &op.Path, &op.Operation,
			&op.Priority, &op.RetryCount, &lastError, &op.CreatedAt, &scheduledAt)
		if err != nil {
			return nil, err
		}

		op.LastError = lastError.String
		if scheduledAt.Valid {
			op.ScheduledAt = scheduledAt.Time
		}

		ops = append(ops, op)
	}

	// Remove dequeued operations
	for _, op := range ops {
		d.db.Exec("DELETE FROM sync_queue WHERE id = ?", op.ID)
	}

	return ops, rows.Err()
}

// RequeueOperation puts an operation back in the queue with a delay.
func (d *Database) RequeueOperation(shareID, path, operation, lastError string, retryCount int, delay time.Duration) error {
	query := `INSERT INTO sync_queue (share_id, path, operation, priority, retry_count, last_error, scheduled_at)
			  VALUES (?, ?, ?, 0, ?, ?, datetime('now', '+' || ? || ' seconds'))
			  ON CONFLICT(share_id, path, operation) DO UPDATE SET
			  retry_count = excluded.retry_count,
			  last_error = excluded.last_error,
			  scheduled_at = excluded.scheduled_at`
	_, err := d.db.Exec(query, shareID, path, operation, retryCount, lastError, int(delay.Seconds()))
	return err
}

// Conflict represents a sync conflict.
type Conflict struct {
	ID            int64
	ShareID       string
	Path          string
	LocalHash     string
	RemoteHash    string
	LocalModTime  time.Time
	RemoteModTime time.Time
	Resolution    string
	ResolvedAt    time.Time
	CreatedAt     time.Time
}

// AddConflict records a sync conflict.
func (d *Database) AddConflict(c *Conflict) error {
	query := `INSERT INTO conflicts (share_id, path, local_hash, remote_hash, local_mod_time, remote_mod_time)
			  VALUES (?, ?, ?, ?, ?, ?)`
	_, err := d.db.Exec(query, c.ShareID, c.Path, c.LocalHash, c.RemoteHash, c.LocalModTime, c.RemoteModTime)
	return err
}

// ListConflicts lists unresolved conflicts.
func (d *Database) ListConflicts(shareID string) ([]Conflict, error) {
	query := `SELECT id, share_id, path, local_hash, remote_hash, local_mod_time, remote_mod_time, 
			  resolution, resolved_at, created_at
			  FROM conflicts WHERE share_id = ? AND resolution IS NULL ORDER BY created_at DESC`

	rows, err := d.db.Query(query, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conflicts []Conflict
	for rows.Next() {
		var c Conflict
		var resolution sql.NullString
		var resolvedAt sql.NullTime

		err := rows.Scan(&c.ID, &c.ShareID, &c.Path, &c.LocalHash, &c.RemoteHash,
			&c.LocalModTime, &c.RemoteModTime, &resolution, &resolvedAt, &c.CreatedAt)
		if err != nil {
			return nil, err
		}

		c.Resolution = resolution.String
		if resolvedAt.Valid {
			c.ResolvedAt = resolvedAt.Time
		}

		conflicts = append(conflicts, c)
	}

	return conflicts, rows.Err()
}

// ResolveConflict marks a conflict as resolved.
func (d *Database) ResolveConflict(id int64, resolution string) error {
	query := `UPDATE conflicts SET resolution = ?, resolved_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := d.db.Exec(query, resolution, id)
	return err
}

// LogActivity logs a sync activity.
func (d *Database) LogActivity(shareID, path, action, status, message string, bytesTransferred int64) error {
	query := `INSERT INTO activity_log (share_id, path, action, status, message, bytes_transferred)
			  VALUES (?, ?, ?, ?, ?, ?)`
	_, err := d.db.Exec(query, shareID, path, action, status, message, bytesTransferred)
	return err
}

// Activity represents an activity log entry.
type Activity struct {
	ID               int64
	ShareID          string
	Path             string
	Action           string
	Status           string
	Message          string
	BytesTransferred int64
	CreatedAt        time.Time
}

// GetRecentActivity retrieves recent activity.
func (d *Database) GetRecentActivity(limit int) ([]Activity, error) {
	query := `SELECT id, share_id, path, action, status, message, bytes_transferred, created_at
			  FROM activity_log ORDER BY created_at DESC LIMIT ?`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		var shareID, path, message sql.NullString
		var bytesTransferred sql.NullInt64

		err := rows.Scan(&a.ID, &shareID, &path, &a.Action, &a.Status, &message, &bytesTransferred, &a.CreatedAt)
		if err != nil {
			return nil, err
		}

		a.ShareID = shareID.String
		a.Path = path.String
		a.Message = message.String
		a.BytesTransferred = bytesTransferred.Int64

		activities = append(activities, a)
	}

	return activities, rows.Err()
}

// GetStats returns sync statistics.
func (d *Database) GetStats(shareID string) (*SyncStats, error) {
	stats := &SyncStats{}

	// Count files by status
	query := `SELECT sync_status, COUNT(*), COALESCE(SUM(size), 0) FROM files WHERE share_id = ? GROUP BY sync_status`
	rows, err := d.db.Query(query, shareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		var size int64
		if err := rows.Scan(&status, &count, &size); err != nil {
			return nil, err
		}

		switch status {
		case "synced":
			stats.SyncedFiles = count
			stats.SyncedBytes = size
		case "pending_upload":
			stats.PendingUploadFiles = count
			stats.PendingUploadBytes = size
		case "pending_download":
			stats.PendingDownloadFiles = count
			stats.PendingDownloadBytes = size
		case "error":
			stats.ErrorFiles = count
		case "conflict":
			stats.ConflictFiles = count
		}
	}

	stats.TotalFiles = stats.SyncedFiles + stats.PendingUploadFiles + stats.PendingDownloadFiles + stats.ErrorFiles + stats.ConflictFiles

	return stats, rows.Err()
}

// SyncStats contains sync statistics.
type SyncStats struct {
	TotalFiles           int64
	SyncedFiles          int64
	SyncedBytes          int64
	PendingUploadFiles   int64
	PendingUploadBytes   int64
	PendingDownloadFiles int64
	PendingDownloadBytes int64
	ErrorFiles           int64
	ConflictFiles        int64
}

// PruneActivityLog removes old activity log entries.
func (d *Database) PruneActivityLog(olderThan time.Duration) error {
	query := `DELETE FROM activity_log WHERE created_at < datetime('now', '-' || ? || ' seconds')`
	_, err := d.db.Exec(query, int(olderThan.Seconds()))
	return err
}

