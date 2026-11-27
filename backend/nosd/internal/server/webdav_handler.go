package server

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/webdav"

	"nos/backend/nosd/internal/shares"
	nosync "nos/backend/nosd/pkg/sync"
)

// WebDAVHandler provides WebDAV access to sync-enabled shares
type WebDAVHandler struct {
	shareStore *shares.Store
	deviceMgr  *nosync.DeviceManager
	access     shareAccessChecker
	logger     zerolog.Logger

	mu       sync.Mutex
	handlers map[string]*webdav.Handler // shareID -> handler (holds its locks)
}

// NewWebDAVHandler creates a new WebDAV handler
func NewWebDAVHandler(shareStore *shares.Store, deviceMgr *nosync.DeviceManager, access shareAccessChecker, logger zerolog.Logger) *WebDAVHandler {
	return &WebDAVHandler{
		shareStore: shareStore,
		access:     access,
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

	// A device token, as a Bearer token (sync clients) or as the password of
	// Basic auth (file managers: Explorer, Finder, Files; any user name).
	// (Basic was refused, so none of the file managers could connect.)
	token := deviceTokenFrom(r)
	if token == "" {
		w.Header().Add("WWW-Authenticate", `Basic realm="nOS", charset="UTF-8"`)
		w.Header().Add("WWW-Authenticate", `Bearer realm="nOS"`)
		http.Error(w, "A device token is required (as the password)", http.StatusUnauthorized)
		return
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	device, err := h.deviceMgr.ValidateDeviceToken(token, ip, r.Header.Get("User-Agent"))
	if err != nil {
		h.logger.Warn().Err(err).Str("share_id", shareID).Msg("WebDAV auth failed")
		w.Header().Add("WWW-Authenticate", `Basic realm="nOS", charset="UTF-8"`)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Get share
	share, ok := h.shareStore.Get(shareID)
	if !ok || share.SyncDisabled {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	// Same rule as Samba and the folder ACLs: readers may read, owners may
	// also change files.
	read, write := h.access.ShareAccess(share, device.UserID)
	if !read {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}
	if !write && !webdavReadOnlyMethod(r.Method) {
		http.Error(w, "Read-only access", http.StatusForbidden)
		return
	}

	// Get or create handler for this share
	h.mu.Lock()
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
	h.mu.Unlock()

	// Serve WebDAV request
	handler.ServeHTTP(w, r)
}

// webdavReadOnlyMethod reports whether method only reads.
func webdavReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND":
		return true
	}
	return false
}

// shareFileSystem implements webdav.FileSystem for a share. Every name goes
// through nosync.ResolveInShare: ".." can't climb out, and a symbolic link
// inside the share that points outside it is refused. (Only a path prefix
// was checked, so a link in a share to /etc/nos let a user read nosd's user
// database and secret key through WebDAV.)
type shareFileSystem struct {
	basePath string
}

func (sfs *shareFileSystem) resolve(name string) (string, error) {
	p, err := nosync.ResolveInShare(sfs.basePath, name)
	switch {
	case err == nil:
		return p, nil
	case errors.Is(err, os.ErrNotExist): // the share's folder is missing
		return "", os.ErrNotExist
	default:
		return "", os.ErrPermission
	}
}

func (sfs *shareFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	p, err := sfs.resolve(name)
	if err != nil {
		return err
	}
	return os.Mkdir(p, perm)
}

func (sfs *shareFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	p, err := sfs.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(p, flag, perm)
}

func (sfs *shareFileSystem) RemoveAll(ctx context.Context, name string) error {
	p, err := sfs.resolve(name)
	if err != nil {
		return err
	}
	if p == filepath.Clean(sfs.basePath) { // never the share itself
		return os.ErrPermission
	}
	return os.RemoveAll(p)
}

func (sfs *shareFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	oldPath, err := sfs.resolve(oldName)
	if err != nil {
		return err
	}
	newPath, err := sfs.resolve(newName)
	if err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

func (sfs *shareFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	p, err := sfs.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Stat(p)
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

// deviceTokenFrom returns a nos_dt_ token from Authorization (Bearer, or
// Basic's password), or "".
func deviceTokenFrom(r *http.Request) string {
	token := ""
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	} else if _, pass, ok := r.BasicAuth(); ok {
		token = pass
	}
	if !strings.HasPrefix(token, "nos_dt_") {
		return ""
	}
	return token
}
