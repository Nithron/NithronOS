// Package sync provides file synchronization functionality for NithronSync clients.
// It enables cross-platform file sync between NithronOS and desktop/mobile devices.
package sync

import (
	"time"
)

// DeviceType represents the type of sync device
type DeviceType string

const (
	DeviceTypeWindows DeviceType = "windows"
	DeviceTypeLinux   DeviceType = "linux"
	DeviceTypeMacOS   DeviceType = "macos"
	DeviceTypeAndroid DeviceType = "android"
	DeviceTypeIOS     DeviceType = "ios"
	DeviceTypeUnknown DeviceType = "unknown"
)

// DeviceToken represents a registered sync device with its authentication token
type DeviceToken struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	DeviceName    string     `json:"device_name"`
	DeviceType    DeviceType `json:"device_type"`
	OSVersion     string     `json:"os_version,omitempty"`
	ClientVersion string     `json:"client_version,omitempty"`
	
	// Token hashes (never exposed)
	TokenHash   string `json:"-"`
	RefreshHash string `json:"-"`
	
	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	
	// Connection info
	LastIP     string `json:"last_ip,omitempty"`
	LastUserAgent string `json:"last_user_agent,omitempty"`
	
	// Scopes granted to this device
	Scopes []string `json:"scopes"`
	
	// Statistics
	SyncCount   int64 `json:"sync_count"`
	BytesSynced int64 `json:"bytes_synced"`
}

// DeviceTokenPublic is the public view of a device token (for API responses)
type DeviceTokenPublic struct {
	ID            string     `json:"id"`
	DeviceName    string     `json:"device_name"`
	DeviceType    DeviceType `json:"device_type"`
	OSVersion     string     `json:"os_version,omitempty"`
	ClientVersion string     `json:"client_version,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	LastIP        string     `json:"last_ip,omitempty"`
	SyncCount     int64      `json:"sync_count"`
	BytesSynced   int64      `json:"bytes_synced"`
	IsRevoked     bool       `json:"is_revoked"`
}

// ToPublic converts a DeviceToken to its public representation
func (d *DeviceToken) ToPublic() DeviceTokenPublic {
	return DeviceTokenPublic{
		ID:            d.ID,
		DeviceName:    d.DeviceName,
		DeviceType:    d.DeviceType,
		OSVersion:     d.OSVersion,
		ClientVersion: d.ClientVersion,
		CreatedAt:     d.CreatedAt,
		LastSyncAt:    d.LastSyncAt,
		LastSeenAt:    d.LastSeenAt,
		LastIP:        d.LastIP,
		SyncCount:     d.SyncCount,
		BytesSynced:   d.BytesSynced,
		IsRevoked:     d.RevokedAt != nil,
	}
}

// SyncShare represents a share that is enabled for sync
type SyncShare struct {
	ShareID         string   `json:"share_id"`
	ShareName       string   `json:"share_name"`
	SharePath       string   `json:"share_path"`
	SyncEnabled     bool     `json:"sync_enabled"`
	TotalSize       int64    `json:"total_size"`
	FileCount       int64    `json:"file_count"`
	MaxSyncSize     int64    `json:"max_sync_size,omitempty"`     // 0 = unlimited
	ExcludePatterns []string `json:"exclude_patterns,omitempty"` // e.g., ["*.tmp", ".git"]
	AllowedUsers    []string `json:"allowed_users,omitempty"`    // Empty = all share users
}

// SyncConfig represents device-specific sync configuration
type SyncConfig struct {
	DeviceID          string   `json:"device_id"`
	SyncShares        []string `json:"sync_shares"`         // Share IDs to sync
	SelectivePaths    []string `json:"selective_paths"`     // Specific paths within shares
	BandwidthLimitKBps int     `json:"bandwidth_limit_kbps"` // 0 = unlimited
	PauseSync         bool     `json:"pause_sync"`
	SyncOnMobileData  bool     `json:"sync_on_mobile_data"`
}

// FileChangeType represents the type of file change
type FileChangeType string

const (
	ChangeTypeCreate FileChangeType = "create"
	ChangeTypeModify FileChangeType = "modify"
	ChangeTypeDelete FileChangeType = "delete"
	ChangeTypeRename FileChangeType = "rename"
)

// FileChange represents a change to a file
type FileChange struct {
	Path     string         `json:"path"`
	Type     FileChangeType `json:"type"`
	Size     int64          `json:"size"`
	MTime    time.Time      `json:"mtime"`
	Hash     string         `json:"hash,omitempty"` // SHA-256
	OldPath  string         `json:"old_path,omitempty"` // For renames
	IsDir    bool           `json:"is_dir"`
}

// ChangesResponse is the response for the changes endpoint
type ChangesResponse struct {
	Changes []FileChange `json:"changes"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"has_more"`
}

// FileMetadata represents metadata about a file
type FileMetadata struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	MTime    time.Time `json:"mtime"`
	Hash     string    `json:"hash"` // SHA-256
	IsDir    bool      `json:"is_dir"`
	Mode     uint32    `json:"mode,omitempty"`
}

// BlockHash represents a hash of a file block for delta sync
type BlockHash struct {
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	Hash   string `json:"hash"` // SHA-256 of block
	Weak   uint32 `json:"weak"` // Rolling checksum (Adler-32)
}

// BlockHashRequest is the request for computing block hashes
type BlockHashRequest struct {
	Path      string `json:"path"`
	BlockSize int64  `json:"block_size"` // Default: 4MB
}

// BlockHashResponse is the response containing block hashes
type BlockHashResponse struct {
	Path      string      `json:"path"`
	Size      int64       `json:"size"`
	Hash      string      `json:"hash"` // Full file hash
	BlockSize int64       `json:"block_size"`
	Blocks    []BlockHash `json:"blocks"`
}

// SyncState represents the sync state for a device and share
type SyncState struct {
	DeviceID   string               `json:"device_id"`
	ShareID    string               `json:"share_id"`
	Cursor     string               `json:"cursor"`
	LastSync   time.Time            `json:"last_sync"`
	Files      map[string]FileState `json:"files"`
	TotalFiles int64                `json:"total_files"`
	TotalBytes int64                `json:"total_bytes"`
}

// FileState represents the sync state of a single file
type FileState struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	MTime    time.Time `json:"mtime"`
	Hash     string    `json:"hash"`
	SyncedAt time.Time `json:"synced_at"`
	Version  int64     `json:"version"`
}

// ConflictResolution defines how conflicts should be resolved
type ConflictResolution string

const (
	ConflictKeepBoth   ConflictResolution = "keep_both"
	ConflictKeepServer ConflictResolution = "keep_server"
	ConflictKeepClient ConflictResolution = "keep_client"
	ConflictNewerWins  ConflictResolution = "newer_wins"
)

// SyncConflict represents a sync conflict
type SyncConflict struct {
	Path           string             `json:"path"`
	ServerVersion  FileMetadata       `json:"server_version"`
	ClientVersion  FileMetadata       `json:"client_version"`
	DetectedAt     time.Time          `json:"detected_at"`
	Resolution     ConflictResolution `json:"resolution,omitempty"`
	ResolvedAt     *time.Time         `json:"resolved_at,omitempty"`
}

// DeviceRegisterRequest is the request to register a new device
type DeviceRegisterRequest struct {
	DeviceName    string     `json:"device_name" validate:"required,min=1,max=64"`
	DeviceType    DeviceType `json:"device_type" validate:"required,oneof=windows linux macos android ios"`
	OSVersion     string     `json:"os_version,omitempty"`
	ClientVersion string     `json:"client_version,omitempty"`
}

// DeviceRegisterResponse is the response after registering a device
type DeviceRegisterResponse struct {
	DeviceID     string    `json:"device_id"`
	DeviceToken  string    `json:"device_token"`   // nos_dt_...
	RefreshToken string    `json:"refresh_token"`  // nos_rt_...
	ExpiresAt    time.Time `json:"expires_at"`
}

// DeviceRefreshRequest is the request to refresh device tokens
type DeviceRefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// DeviceRefreshResponse is the response after refreshing tokens
type DeviceRefreshResponse struct {
	DeviceToken  string    `json:"device_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SyncScope defines the available scopes for sync operations
type SyncScope string

const (
	ScopeSyncRead    SyncScope = "sync.read"
	ScopeSyncWrite   SyncScope = "sync.write"
	ScopeSyncDevices SyncScope = "sync.devices"
	ScopeSyncAdmin   SyncScope = "sync.admin"
)

// DefaultDeviceScopes returns the default scopes for a new device
func DefaultDeviceScopes() []string {
	return []string{
		string(ScopeSyncRead),
		string(ScopeSyncWrite),
	}
}

// DeviceTokenTTL is the default TTL for device tokens (90 days)
const DeviceTokenTTL = 90 * 24 * time.Hour

// RefreshTokenTTL is the default TTL for refresh tokens (1 year)
const RefreshTokenTTL = 365 * 24 * time.Hour

// DefaultBlockSize is the default block size for delta sync (4MB)
const DefaultBlockSize = 4 * 1024 * 1024

// MaxBlockSize is the maximum allowed block size (64MB)
const MaxBlockSize = 64 * 1024 * 1024

// MinBlockSize is the minimum allowed block size (64KB)
const MinBlockSize = 64 * 1024

// MaxFileSize is the maximum file size for sync (50GB)
const MaxFileSize = 50 * 1024 * 1024 * 1024

// MaxChangesPerRequest is the maximum number of changes per request
const MaxChangesPerRequest = 1000

