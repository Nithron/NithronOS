package sync

import (
	"crypto/sha256"
	"encoding/hex"
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

	// hashes caches file hashes by full path: a file is hashed again only
	// when its size or modification time changes. (Every poll by every
	// device read every file in the share.)
	hashMu sync.Mutex
	hashes map[string]cachedHash
}

type cachedHash struct {
	size  int64
	mtime int64 // UnixNano
	hash  string
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
			".nos",
			".nos-delta-*", // delta uploads being rebuilt (sync_delta.go)
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
		hashes:          make(map[string]cachedHash),
	}
}

// ChangeSet is the changes since a snapshot and the snapshot to continue
// from (path -> hash; "" for directories).
type ChangeSet struct {
	Changes []FileChange
	Next    map[string]string
	HasMore bool
}

// Changes compares the share with since (nil: nothing known, so every file
// is new) and returns at most limit changes, oldest first. When there are
// more, Next includes only the changes returned, so the rest come next
// time. (The next cursor covered every file even when the list was cut,
// so changes past the limit were never reported.)
func (ct *ChangeTracker) Changes(sharePath string, since map[string]string, limit int) (*ChangeSet, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	if limit <= 0 || limit > MaxChangesPerRequest {
		limit = MaxChangesPerRequest
	}
	info, err := os.Stat(sharePath)
	if err != nil {
		return nil, fmt.Errorf("share path not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("share path is not a directory")
	}

	currentFiles := make(map[string]FileMetadata)
	if err := ct.scanDirectory(sharePath, "", currentFiles); err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}
	ct.pruneHashes(sharePath, currentFiles)

	changes := ct.computeChanges(since, currentFiles)
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].MTime.Before(changes[j].MTime)
	})

	set := &ChangeSet{Changes: changes}
	if len(changes) > limit {
		set.Changes, set.HasMore = changes[:limit], true
		set.Next = make(map[string]string, len(since)+limit)
		for p, h := range since {
			set.Next[p] = h
		}
		for _, c := range set.Changes {
			if c.Type == ChangeTypeDelete {
				delete(set.Next, c.Path)
			} else {
				set.Next[c.Path] = c.Hash
			}
		}
		return set, nil
	}
	set.Next = make(map[string]string, len(currentFiles))
	for p, meta := range currentFiles {
		set.Next[p] = meta.Hash
	}
	return set, nil
}

// pruneHashes forgets cached hashes of files under sharePath that are gone.
func (ct *ChangeTracker) pruneHashes(sharePath string, current map[string]FileMetadata) {
	prefix := filepath.Clean(sharePath) + string(filepath.Separator)
	ct.hashMu.Lock()
	defer ct.hashMu.Unlock()
	for full := range ct.hashes {
		if rel, ok := strings.CutPrefix(full, prefix); ok {
			if _, still := current[rel]; !still {
				delete(ct.hashes, full)
			}
		}
	}
}

// fileHash is the file's SHA-256, from the cache while its size and
// modification time are unchanged.
func (ct *ChangeTracker) fileHash(fullPath string, info os.FileInfo) (string, error) {
	key := cachedHash{size: info.Size(), mtime: info.ModTime().UnixNano()}
	ct.hashMu.Lock()
	c, ok := ct.hashes[fullPath]
	ct.hashMu.Unlock()
	if ok && c.size == key.size && c.mtime == key.mtime {
		return c.hash, nil
	}
	h, err := ct.computeFileHash(fullPath)
	if err != nil {
		return "", err
	}
	key.hash = h
	ct.hashMu.Lock()
	ct.hashes[fullPath] = key
	ct.hashMu.Unlock()
	return h, nil
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

			hash, err := ct.fileHash(fullPath, info)
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
	changes := []FileChange{} // [] in JSON, not null

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
	// (A prefix check without a separator let share "media" reach a sibling
	// share "media2", and didn't follow symlinks.)
	fullPath, err := ResolveInShare(sharePath, filePath)
	if err != nil {
		return nil, err
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
	files := []FileMetadata{} // [] in JSON, not null

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
