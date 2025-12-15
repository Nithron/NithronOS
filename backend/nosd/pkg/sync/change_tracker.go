package sync

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ChangeTracker tracks file changes in sync-enabled shares
type ChangeTracker struct {
	logger zerolog.Logger
	mu     sync.RWMutex
	
	// Configuration
	excludePatterns []string
	maxFileSize     int64
}

// ChangeTrackerConfig holds configuration for the change tracker
type ChangeTrackerConfig struct {
	ExcludePatterns []string
	MaxFileSize     int64
}

// DefaultChangeTrackerConfig returns the default configuration
func DefaultChangeTrackerConfig() ChangeTrackerConfig {
	return ChangeTrackerConfig{
		ExcludePatterns: []string{
			"*.tmp",
			"*.temp",
			"~*",
			".DS_Store",
			"Thumbs.db",
			".git",
			".svn",
			"*.swp",
			"*.bak",
			".sync",
			".nithronos",
		},
		MaxFileSize: MaxFileSize,
	}
}

// NewChangeTracker creates a new change tracker
func NewChangeTracker(logger zerolog.Logger, config ChangeTrackerConfig) *ChangeTracker {
	return &ChangeTracker{
		logger:          logger.With().Str("component", "change-tracker").Logger(),
		excludePatterns: config.ExcludePatterns,
		maxFileSize:     config.MaxFileSize,
	}
}

// Cursor represents a sync cursor containing the last known state
type Cursor struct {
	Timestamp   time.Time `json:"timestamp"`
	FileHashes  map[string]string `json:"file_hashes"` // path -> hash
	LastScanned time.Time `json:"last_scanned"`
}

// EncodeCursor encodes a cursor to a string
func EncodeCursor(c *Cursor) string {
	if c == nil {
		return ""
	}
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeCursor decodes a cursor from a string
func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	return &c, nil
}

// GetChanges returns file changes since the given cursor
func (ct *ChangeTracker) GetChanges(sharePath, cursorStr string, limit int) (*ChangesResponse, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	if limit <= 0 || limit > MaxChangesPerRequest {
		limit = MaxChangesPerRequest
	}
	
	// Validate share path exists
	info, err := os.Stat(sharePath)
	if err != nil {
		return nil, fmt.Errorf("share path not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("share path is not a directory")
	}
	
	// Decode cursor
	cursor, err := DecodeCursor(cursorStr)
	if err != nil {
		return nil, err
	}
	
	// If no cursor, return all files
	if cursor == nil {
		cursor = &Cursor{
			Timestamp:  time.Time{},
			FileHashes: make(map[string]string),
		}
	}
	
	// Scan directory for changes
	currentFiles := make(map[string]FileMetadata)
	err = ct.scanDirectory(sharePath, "", currentFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}
	
	// Compute changes
	changes := ct.computeChanges(cursor.FileHashes, currentFiles)
	
	// Sort changes by modification time
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].MTime.Before(changes[j].MTime)
	})
	
	// Limit results
	hasMore := len(changes) > limit
	if hasMore {
		changes = changes[:limit]
	}
	
	// Create new cursor with current state
	newFileHashes := make(map[string]string, len(currentFiles))
	for path, meta := range currentFiles {
		newFileHashes[path] = meta.Hash
	}
	newCursor := &Cursor{
		Timestamp:   time.Now(),
		FileHashes:  newFileHashes,
		LastScanned: time.Now(),
	}
	
	return &ChangesResponse{
		Changes: changes,
		Cursor:  EncodeCursor(newCursor),
		HasMore: hasMore,
	}, nil
}

// scanDirectory recursively scans a directory for files
func (ct *ChangeTracker) scanDirectory(basePath, relativePath string, files map[string]FileMetadata) error {
	currentPath := basePath
	if relativePath != "" {
		currentPath = filepath.Join(basePath, relativePath)
	}
	
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		name := entry.Name()
		relPath := name
		if relativePath != "" {
			relPath = filepath.Join(relativePath, name)
		}
		
		// Check exclusions
		if ct.shouldExclude(name, relPath) {
			continue
		}
		
		fullPath := filepath.Join(basePath, relPath)
		info, err := entry.Info()
		if err != nil {
			ct.logger.Warn().Err(err).Str("path", relPath).Msg("Failed to get file info")
			continue
		}
		
		if entry.IsDir() {
			// Add directory entry
			files[relPath] = FileMetadata{
				Path:  relPath,
				IsDir: true,
				MTime: info.ModTime(),
				Mode:  uint32(info.Mode()),
			}
			// Recurse into directory
			if err := ct.scanDirectory(basePath, relPath, files); err != nil {
				ct.logger.Warn().Err(err).Str("path", relPath).Msg("Failed to scan subdirectory")
			}
		} else if info.Mode().IsRegular() {
			// Skip files that are too large
			if info.Size() > ct.maxFileSize {
				ct.logger.Debug().
					Str("path", relPath).
					Int64("size", info.Size()).
					Int64("max", ct.maxFileSize).
					Msg("Skipping file (too large)")
				continue
			}
			
			// Compute file hash
			hash, err := ct.computeFileHash(fullPath)
			if err != nil {
				ct.logger.Warn().Err(err).Str("path", relPath).Msg("Failed to compute file hash")
				continue
			}
			
			files[relPath] = FileMetadata{
				Path:  relPath,
				Size:  info.Size(),
				MTime: info.ModTime(),
				Hash:  hash,
				IsDir: false,
				Mode:  uint32(info.Mode()),
			}
		}
	}
	
	return nil
}

// computeChanges computes the differences between old and new file states
func (ct *ChangeTracker) computeChanges(oldHashes map[string]string, currentFiles map[string]FileMetadata) []FileChange {
	var changes []FileChange
	
	// Check for new and modified files
	for path, meta := range currentFiles {
		oldHash, existed := oldHashes[path]
		if !existed {
			// New file
			changes = append(changes, FileChange{
				Path:  path,
				Type:  ChangeTypeCreate,
				Size:  meta.Size,
				MTime: meta.MTime,
				Hash:  meta.Hash,
				IsDir: meta.IsDir,
			})
		} else if oldHash != meta.Hash {
			// Modified file
			changes = append(changes, FileChange{
				Path:  path,
				Type:  ChangeTypeModify,
				Size:  meta.Size,
				MTime: meta.MTime,
				Hash:  meta.Hash,
				IsDir: meta.IsDir,
			})
		}
	}
	
	// Check for deleted files
	for path := range oldHashes {
		if _, exists := currentFiles[path]; !exists {
			changes = append(changes, FileChange{
				Path:  path,
				Type:  ChangeTypeDelete,
				MTime: time.Now(),
			})
		}
	}
	
	return changes
}

// computeFileHash computes the SHA-256 hash of a file
func (ct *ChangeTracker) computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(h.Sum(nil)), nil
}

// shouldExclude checks if a path should be excluded from sync
func (ct *ChangeTracker) shouldExclude(name, relPath string) bool {
	// Check each exclude pattern
	for _, pattern := range ct.excludePatterns {
		// Handle directory patterns
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath+"/", pattern) {
				return true
			}
			continue
		}
		
		// Handle glob patterns
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
		
		// Also check the full relative path
		matched, _ = filepath.Match(pattern, relPath)
		if matched {
			return true
		}
	}
	
	// Check for hidden files (Unix convention)
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		// Allow some hidden files
		allowedHidden := map[string]bool{
			".htaccess": true,
			".env":      false, // Exclude .env files
		}
		if exclude, found := allowedHidden[name]; found {
			return exclude
		}
	}
	
	return false
}

// GetFileMetadata returns metadata for a specific file
func (ct *ChangeTracker) GetFileMetadata(sharePath, filePath string) (*FileMetadata, error) {
	fullPath := filepath.Join(sharePath, filePath)
	
	// Ensure the path is within the share
	cleanPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	cleanSharePath, _ := filepath.Abs(sharePath)
	if !strings.HasPrefix(cleanPath, cleanSharePath) {
		return nil, fmt.Errorf("path escapes share directory")
	}
	
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}
	
	meta := &FileMetadata{
		Path:  filePath,
		Size:  info.Size(),
		MTime: info.ModTime(),
		IsDir: info.IsDir(),
		Mode:  uint32(info.Mode()),
	}
	
	if !info.IsDir() && info.Size() <= ct.maxFileSize {
		hash, err := ct.computeFileHash(fullPath)
		if err == nil {
			meta.Hash = hash
		}
	}
	
	return meta, nil
}

// GetFilesMetadata returns metadata for multiple files
func (ct *ChangeTracker) GetFilesMetadata(sharePath string, filePaths []string) ([]FileMetadata, error) {
	results := make([]FileMetadata, 0, len(filePaths))
	
	for _, filePath := range filePaths {
		meta, err := ct.GetFileMetadata(sharePath, filePath)
		if err != nil {
			// Skip files that don't exist or have errors
			ct.logger.Debug().Err(err).Str("path", filePath).Msg("Failed to get file metadata")
			continue
		}
		results = append(results, *meta)
	}
	
	return results, nil
}

// WalkShare walks a share directory and returns all files
func (ct *ChangeTracker) WalkShare(sharePath string) ([]FileMetadata, error) {
	var files []FileMetadata
	
	err := filepath.WalkDir(sharePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		relPath, err := filepath.Rel(sharePath, path)
		if err != nil || relPath == "." {
			return nil
		}
		
		if ct.shouldExclude(d.Name(), relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		info, err := d.Info()
		if err != nil {
			return nil
		}
		
		meta := FileMetadata{
			Path:  relPath,
			IsDir: d.IsDir(),
			MTime: info.ModTime(),
			Mode:  uint32(info.Mode()),
		}
		
		if !d.IsDir() {
			meta.Size = info.Size()
			if info.Size() <= ct.maxFileSize {
				hash, err := ct.computeFileHash(path)
				if err == nil {
					meta.Hash = hash
				}
			}
		}
		
		files = append(files, meta)
		return nil
	})
	
	return files, err
}

// ShareStats returns statistics about a share
func (ct *ChangeTracker) ShareStats(sharePath string) (totalFiles int64, totalSize int64, err error) {
	err = filepath.WalkDir(sharePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		
		if d.IsDir() {
			return nil
		}
		
		relPath, err := filepath.Rel(sharePath, path)
		if err != nil {
			return nil
		}
		
		if ct.shouldExclude(d.Name(), relPath) {
			return nil
		}
		
		info, err := d.Info()
		if err != nil {
			return nil
		}
		
		totalFiles++
		totalSize += info.Size()
		return nil
	})
	
	return totalFiles, totalSize, err
}

// SetExcludePatterns updates the exclude patterns
func (ct *ChangeTracker) SetExcludePatterns(patterns []string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.excludePatterns = patterns
}

// GetExcludePatterns returns the current exclude patterns
func (ct *ChangeTracker) GetExcludePatterns() []string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	result := make([]string, len(ct.excludePatterns))
	copy(result, ct.excludePatterns)
	return result
}

