package sync

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	gosync "sync"
)

// CursorStore keeps, per device and share, the snapshot (path -> hash) the
// device's last change cursor stands for; the cursor itself is a short id.
// (The cursor used to be the whole snapshot, base64 in the URL: a share
// with tens of thousands of files made a request too large to send.)
// Only the latest snapshot per device and share is kept: a device always
// continues from its last cursor. An unknown cursor (an older one, or
// nosd's state lost) lists the whole share again, which clients handle by
// comparing hashes.
type CursorStore struct {
	dir string
	mu  gosync.Mutex
}

type cursorFile struct {
	ID    string            `json:"id"`
	Files map[string]string `json:"files"`
}

// NewCursorStore keeps its files in dir.
func NewCursorStore(dir string) (*CursorStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &CursorStore{dir: dir}, nil
}

func (s *CursorStore) path(owner, shareID string) string {
	sum := sha256.Sum256([]byte(owner + "\x00" + shareID))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:16])+".json")
}

// Load returns the snapshot cursor stands for, if it is owner's latest.
func (s *CursorStore) Load(owner, shareID, cursor string) (map[string]string, bool) {
	if cursor == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(owner, shareID))
	if err != nil {
		return nil, false
	}
	var f cursorFile
	if json.Unmarshal(data, &f) != nil || f.ID != cursor {
		return nil, false
	}
	if f.Files == nil {
		f.Files = map[string]string{}
	}
	return f.Files, true
}

// Save stores files as owner's latest snapshot and returns its cursor.
func (s *CursorStore) Save(owner, shareID string, files map[string]string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	data, err := json.Marshal(cursorFile{ID: id, Files: files})
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.path(owner, shareID)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", fmt.Errorf("save sync cursor: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", fmt.Errorf("save sync cursor: %w", err)
	}
	return id, nil
}
