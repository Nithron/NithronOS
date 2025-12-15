package server

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/webdav"

	"nithronos/backend/nosd/internal/shares"
	nosync "nithronos/backend/nosd/pkg/sync"
)

// WebDAVHandler provides WebDAV access to sync-enabled shares
type WebDAVHandler struct {
	shareStore *shares.Store
	deviceMgr  *nosync.DeviceManager
	logger     zerolog.Logger
	handlers   map[string]*webdav.Handler // shareID -> handler
}

// NewWebDAVHandler creates a new WebDAV handler
func NewWebDAVHandler(shareStore *shares.Store, deviceMgr *nosync.DeviceManager, logger zerolog.Logger) *WebDAVHandler {
	return &WebDAVHandler{
		shareStore: shareStore,
		deviceMgr:  deviceMgr,
		logger:     logger.With().Str("component", "webdav").Logger(),
		handlers:   make(map[string]*webdav.Handler),
	}
}

// ServeHTTP implements http.Handler
func (h *WebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract share ID from path: /dav/{share_id}/...
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/dav/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Share ID required", http.StatusBadRequest)
		return
	}
	shareID := pathParts[0]

	// Authenticate using device token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="NithronOS Sync"`)
		http.Error(w, "Authorization required", http.StatusUnauthorized)
		return
	}

	var device *nosync.DeviceToken
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if strings.HasPrefix(token, "nos_dt_") {
			ip := r.RemoteAddr
			if i := strings.LastIndex(ip, ":"); i >= 0 {
				ip = ip[:i]
			}
			var err error
			device, err = h.deviceMgr.ValidateDeviceToken(token, ip, r.Header.Get("User-Agent"))
			if err != nil {
				h.logger.Warn().Err(err).Str("share_id", shareID).Msg("WebDAV auth failed")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
		}
	}

	// Also support Basic auth for legacy clients
	if device == nil && strings.HasPrefix(authHeader, "Basic ") {
		// For now, only device tokens are supported
		http.Error(w, "Device token required", http.StatusUnauthorized)
		return
	}

	if device == nil {
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Get share
	share, ok := h.shareStore.GetByID(shareID)
	if !ok {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	// Check user access
	hasAccess := false
	if len(share.Users) == 0 {
		hasAccess = true // No restrictions
	} else {
		for _, u := range share.Users {
			if u == device.UserID {
				hasAccess = true
				break
			}
		}
	}
	if !hasAccess {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Get or create handler for this share
	handler, ok := h.handlers[shareID]
	if !ok {
		handler = &webdav.Handler{
			Prefix:     "/dav/" + shareID,
			FileSystem: &shareFileSystem{basePath: share.Path},
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					h.logger.Warn().
						Err(err).
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("WebDAV request error")
				} else {
					h.logger.Debug().
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("WebDAV request")
				}
			},
		}
		h.handlers[shareID] = handler
	}

	// Serve WebDAV request
	handler.ServeHTTP(w, r)
}

// shareFileSystem implements webdav.FileSystem for a share
type shareFileSystem struct {
	basePath string
}

func (sfs *shareFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fullPath := filepath.Join(sfs.basePath, name)
	if !sfs.isValidPath(fullPath) {
		return os.ErrPermission
	}
	return os.Mkdir(fullPath, perm)
}

func (sfs *shareFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	fullPath := filepath.Join(sfs.basePath, name)
	if !sfs.isValidPath(fullPath) {
		return nil, os.ErrPermission
	}
	return os.OpenFile(fullPath, flag, perm)
}

func (sfs *shareFileSystem) RemoveAll(ctx context.Context, name string) error {
	fullPath := filepath.Join(sfs.basePath, name)
	if !sfs.isValidPath(fullPath) {
		return os.ErrPermission
	}
	// Don't allow removing the root
	if fullPath == sfs.basePath || fullPath == sfs.basePath+"/" {
		return os.ErrPermission
	}
	return os.RemoveAll(fullPath)
}

func (sfs *shareFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	oldPath := filepath.Join(sfs.basePath, oldName)
	newPath := filepath.Join(sfs.basePath, newName)
	if !sfs.isValidPath(oldPath) || !sfs.isValidPath(newPath) {
		return os.ErrPermission
	}
	return os.Rename(oldPath, newPath)
}

func (sfs *shareFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fullPath := filepath.Join(sfs.basePath, name)
	if !sfs.isValidPath(fullPath) {
		return nil, os.ErrPermission
	}
	return os.Stat(fullPath)
}

func (sfs *shareFileSystem) isValidPath(path string) bool {
	// Ensure the path doesn't escape the share directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(sfs.basePath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absBase)
}

// WebDAVInfo provides information about the WebDAV endpoint
type WebDAVInfo struct {
	Endpoint    string   `json:"endpoint"`
	Shares      []string `json:"shares"`
	AuthMethods []string `json:"auth_methods"`
}

// GetInfo returns WebDAV endpoint information
func (h *WebDAVHandler) GetInfo() WebDAVInfo {
	shares := h.shareStore.List()
	shareIDs := make([]string, len(shares))
	for i, s := range shares {
		shareIDs[i] = s.ID
	}
	return WebDAVInfo{
		Endpoint:    "/dav/{share_id}",
		Shares:      shareIDs,
		AuthMethods: []string{"Bearer (device token)"},
	}
}

// WrappedFile wraps os.File to implement webdav.File
type WrappedFile struct {
	*os.File
	path string
}

// ContentType returns the content type of the file
func (f *WrappedFile) ContentType(ctx context.Context) (string, error) {
	return "", webdav.ErrNotImplemented
}

// ETag returns the ETag of the file
func (f *WrappedFile) ETag(ctx context.Context) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	return `"` + info.ModTime().Format(time.RFC3339Nano) + `"`, nil
}

// Readdir reads the directory
func (f *WrappedFile) Readdir(count int) ([]fs.FileInfo, error) {
	return f.File.Readdir(count)
}

