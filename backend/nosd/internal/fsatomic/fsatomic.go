package fsatomic

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// SaveJSON atomically writes v as pretty JSON to path with durability guarantees.
// It writes to a new temp file next to path, fsyncs, fsyncs the parent directory, renames into place,
// then fsyncs the parent directory again. On any error, it removes the temp file.
// If perm is 0, 0600 is used.
func SaveJSON(path string, v any, perm fs.FileMode) error {
	if perm == 0 {
		perm = 0o600
	}
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o770); err != nil {
		return err
	}
	// Marshal with trailing newline
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	// A temp file of its own (path.tmp-XXXX): concurrent writers can't
	// clobber each other's, and readers never touch it.
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = f.Close() }()
	if err := f.Chmod(perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// fsync parent before and after rename (no-op on Windows)
	if err := fsyncDir(filepath.Dir(path)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Rename, with Windows-friendly fallback and small retry for EBUSY
	renamed := false
	for i := 0; i < 5; i++ {
		if err := os.Rename(tmp, path); err == nil {
			renamed = true
			break
		} else if runtime.GOOS == "windows" {
			// On Windows, destination existing or transient file-in-use can cause failure
			_ = os.Remove(path)
			time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
			continue
		} else {
			_ = os.Remove(tmp)
			return err
		}
	}
	if !renamed {
		_ = os.Remove(tmp)
		return errors.New("rename failed after retries")
	}
	if err := fsyncDir(filepath.Dir(path)); err != nil {
		return err
	}
	return nil
}

// LoadJSON loads JSON from path into v. Returns exists=false if file is missing.
//
// It never touches temp files: it used to delete path+".tmp", which could be
// the file a concurrent SaveJSON was about to rename into place, so that save
// failed and was lost.
func LoadJSON(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if len(data) == 0 {
		return true, nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, err
	}
	return true, nil
}

// WriteJSONAtomic writes JSON atomically with default permissions (0600).
// It ensures the parent directory exists with 0770 permissions.
func WriteJSONAtomic(ctx context.Context, path string, v any) error {
	return SaveJSON(path, v, 0)
}

// ReadJSON is LoadJSON.
func ReadJSON(path string, v any) (bool, error) { return LoadJSON(path, v) }

// WithLock acquires an exclusive advisory lock (path+".lock") for the duration of fn.
// Lock is released after fn returns. Nested calls in the same process are safe as the
// OS-level lock is re-entrant per file descriptor semantics; do not hold locks longer than needed.
func WithLock(path string, fn func() error) error {
	// Ensure lock parent exists
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	unlock, err := flockExclusive(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// fsyncDir calls Sync on a directory to persist metadata; no-op on Windows.
func fsyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// FsyncDir is an exported helper for callers needing to sync a directory.
func FsyncDir(dir string) error { return fsyncDir(dir) }
