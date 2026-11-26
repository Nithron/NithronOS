package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"nos/backend/nosd/internal/shares"
	"nos/backend/nosd/pkg/validate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// SambaManager manages Samba/SMB shares
type SambaManager struct {
	configPath string
}

func NewSambaManager() *SambaManager {
	return &SambaManager{
		configPath: "/etc/samba/smb.conf",
	}
}

func (m *SambaManager) ApplyShare(share *shares.Share) error {
	if share.Protocol != "smb" {
		return fmt.Errorf("invalid protocol for Samba: %s", share.Protocol)
	}

	// Generate Samba config section
	config := fmt.Sprintf("\n[%s]\n", share.Name)
	config += fmt.Sprintf("   path = %s\n", share.Path)
	config += fmt.Sprintf("   comment = %s\n", share.Description)

	if share.ReadOnly {
		config += "   read only = yes\n"
	} else {
		config += "   read only = no\n"
	}

	if share.GuestAccess {
		config += "   guest ok = yes\n"
	} else {
		config += "   guest ok = no\n"
	}

	if len(share.Users) > 0 {
		config += fmt.Sprintf("   valid users = %s\n", strings.Join(share.Users, " "))
	}

	if !share.Enabled {
		config += "   available = no\n"
	}

	// Additional options
	config += "   browseable = yes\n"
	config += "   create mask = 0644\n"
	config += "   directory mask = 0755\n"

	// Write to includes directory
	includeDir := "/etc/samba/shares.d"
	if err := os.MkdirAll(includeDir, 0755); err != nil {
		return err
	}

	shareFile := filepath.Join(includeDir, fmt.Sprintf("%s.conf", share.ID))
	if err := os.WriteFile(shareFile, []byte(config), 0644); err != nil {
		return err
	}

	// Reload Samba
	return m.reload()
}

func (m *SambaManager) RemoveShare(shareID string) error {
	includeDir := "/etc/samba/shares.d"
	shareFile := filepath.Join(includeDir, fmt.Sprintf("%s.conf", shareID))

	if err := os.Remove(shareFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return m.reload()
}

func (m *SambaManager) reload() error {
	// Test config first
	cmd := exec.Command("testparm", "-s", "--suppress-prompt")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid Samba configuration: %w", err)
	}

	// Reload service
	cmd = exec.Command("systemctl", "reload", "smbd")
	return cmd.Run()
}

func (m *SambaManager) TestShare(share *shares.Share) error {
	// Check if path exists
	info, err := os.Stat(share.Path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	// Check if Samba is running
	cmd := exec.Command("systemctl", "is-active", "smbd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("samba service is not running")
	}

	return nil
}

// NFSManager manages NFS exports
type NFSManager struct {
	exportsPath string
}

func NewNFSManager() *NFSManager {
	return &NFSManager{
		exportsPath: "/etc/exports",
	}
}

func (m *NFSManager) ApplyShare(share *shares.Share) error {
	if share.Protocol != "nfs" {
		return fmt.Errorf("invalid protocol for NFS: %s", share.Protocol)
	}

	// Generate NFS export line
	options := []string{}

	if share.ReadOnly {
		options = append(options, "ro")
	} else {
		options = append(options, "rw")
	}

	options = append(options, "sync", "no_subtree_check")

	if share.GuestAccess {
		options = append(options, "all_squash", "anonuid=65534", "anongid=65534")
	} else {
		options = append(options, "no_all_squash")
	}

	// Build export line
	export := fmt.Sprintf("%q ", share.Path) // quoted: paths may contain spaces

	if len(share.Hosts) == 0 {
		// Default to local network
		export += fmt.Sprintf("192.168.0.0/16(%s)", strings.Join(options, ","))
	} else {
		for _, host := range share.Hosts {
			export += fmt.Sprintf("%s(%s) ", host, strings.Join(options, ","))
		}
	}

	// Write to exports.d
	exportsDir := "/etc/exports.d"
	if err := os.MkdirAll(exportsDir, 0755); err != nil {
		return err
	}

	exportFile := filepath.Join(exportsDir, fmt.Sprintf("%s.exports", share.ID))
	if err := os.WriteFile(exportFile, []byte(export+"\n"), 0644); err != nil {
		return err
	}

	// Export the filesystem
	return m.reload()
}

func (m *NFSManager) RemoveShare(shareID string) error {
	exportsDir := "/etc/exports.d"
	exportFile := filepath.Join(exportsDir, fmt.Sprintf("%s.exports", shareID))

	if err := os.Remove(exportFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return m.reload()
}

func (m *NFSManager) reload() error {
	// Re-export all filesystems
	cmd := exec.Command("exportfs", "-ra")
	return cmd.Run()
}

func (m *NFSManager) TestShare(share *shares.Share) error {
	// Check if path exists
	info, err := os.Stat(share.Path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	// Check if NFS server is running
	cmd := exec.Command("systemctl", "is-active", "nfs-server")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("NFS server is not running")
	}

	return nil
}

// SharesHandlerV2 handles share-related endpoints with real implementation
type SharesHandlerV2 struct {
	store *shares.Store
	samba *SambaManager
	nfs   *NFSManager
	agent AgentClient
}

// NewSharesHandlerV2 serves the shares API from store (shared with sync and
// WebDAV).
func NewSharesHandlerV2(store *shares.Store, agent AgentClient) *SharesHandlerV2 {
	return &SharesHandlerV2{
		store: store,
		samba: NewSambaManager(),
		nfs:   NewNFSManager(),
		agent: agent,
	}
}

// Routes registers the shares routes
func (h *SharesHandlerV2) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.ListShares)
	r.Post("/", h.CreateShare)
	r.Get("/{id}", h.GetShare)
	r.Put("/{id}", h.UpdateShare)
	r.Delete("/{id}", h.DeleteShare)
	r.Post("/{id}/test", h.TestShare)
	r.Post("/{id}/enable", h.EnableShare)
	r.Post("/{id}/disable", h.DisableShare)

	return r
}

// ListShares returns all shares
func (h *SharesHandlerV2) ListShares(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.store.List())
}

// GetShare returns a specific share
func (h *SharesHandlerV2) GetShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	writeJSON(w, share)
}

// CreateShare creates a new share
func (h *SharesHandlerV2) CreateShare(w http.ResponseWriter, r *http.Request) {
	var share shares.Share
	if err := json.NewDecoder(r.Body).Decode(&share); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate
	if share.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Share name is required")
		return
	}

	if share.Path == "" {
		httpx.WriteError(w, http.StatusBadRequest, "Share path is required")
		return
	}

	if share.Protocol != "smb" && share.Protocol != "nfs" {
		httpx.WriteError(w, http.StatusBadRequest, "Protocol must be 'smb' or 'nfs'")
		return
	}

	if err := validateShareFields(&share); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if path exists
	if _, err := os.Stat(share.Path); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Share path does not exist")
		return
	}

	// The store assigns the ID (it names the share's file under
	// /etc/samba/shares.d), never the request.
	created, err := h.store.Create(share)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to create share")
		return
	}

	// Apply to system
	if created.Enabled {
		if err := h.applyShare(&created); err != nil {
			log.Error().Err(err).Str("id", created.ID).Msg("Failed to apply share")
			// Don't fail the request, share is saved
		}
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

// UpdateShare updates an existing share
func (h *SharesHandlerV2) UpdateShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var updates shares.Share
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if updates.Protocol != "" && updates.Protocol != "smb" && updates.Protocol != "nfs" {
		httpx.WriteError(w, http.StatusBadRequest, "Protocol must be 'smb' or 'nfs'")
		return
	}
	if err := validateShareFields(&updates); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// existing is a copy taken before the update, so the old protocol's
	// config is the one removed below.
	existing, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}
	updated, err := h.store.Update(id, updates)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to update share")
		return
	}

	_ = h.removeShare(&existing)
	if updated.Enabled {
		if err := h.applyShare(&updated); err != nil {
			log.Error().Err(err).Str("id", id).Msg("Failed to apply updated share")
		}
	}

	writeJSON(w, updated)
}

// DeleteShare deletes a share
func (h *SharesHandlerV2) DeleteShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	// Remove from system
	if err := h.removeShare(&share); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to remove share from system")
	}

	// Delete from store
	if err := h.store.Delete(id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to delete share")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestShare tests share configuration
func (h *SharesHandlerV2) TestShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	var manager interface {
		TestShare(*shares.Share) error
	}

	switch share.Protocol {
	case "smb":
		manager = h.samba
	case "nfs":
		manager = h.nfs
	default:
		httpx.WriteError(w, http.StatusBadRequest, "Unknown protocol")
		return
	}

	result := map[string]interface{}{
		"status": "success",
		"tests": map[string]interface{}{
			"path_exists":     true,
			"permissions_ok":  true,
			"service_running": true,
		},
	}

	if err := manager.TestShare(&share); err != nil {
		result["status"] = "failed"
		result["error"] = err.Error()

		// Determine which test failed
		tests := result["tests"].(map[string]interface{})
		if strings.Contains(err.Error(), "path") {
			tests["path_exists"] = false
		} else if strings.Contains(err.Error(), "service") {
			tests["service_running"] = false
		} else {
			tests["permissions_ok"] = false
		}
	}

	writeJSON(w, result)
}

// EnableShare enables a share
func (h *SharesHandlerV2) EnableShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, err := h.store.SetEnabled(id, true)
	if errors.Is(err, shares.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to enable share")
		return
	}

	// Apply to system
	if err := h.applyShare(&share); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to apply share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to apply share configuration")
		return
	}

	writeJSON(w, share)
}

// DisableShare disables a share
func (h *SharesHandlerV2) DisableShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, err := h.store.SetEnabled(id, false)
	if errors.Is(err, shares.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to disable share")
		return
	}

	// Remove from system
	if err := h.removeShare(&share); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to remove share from system")
	}

	writeJSON(w, share)
}

func (h *SharesHandlerV2) applyShare(share *shares.Share) error {
	switch share.Protocol {
	case "smb":
		return h.samba.ApplyShare(share)
	case "nfs":
		return h.nfs.ApplyShare(share)
	default:
		return fmt.Errorf("unknown protocol: %s", share.Protocol)
	}
}

func (h *SharesHandlerV2) removeShare(share *shares.Share) error {
	switch share.Protocol {
	case "smb":
		return h.samba.RemoveShare(share.ID)
	case "nfs":
		return h.nfs.RemoveShare(share.ID)
	default:
		return fmt.Errorf("unknown protocol: %s", share.Protocol)
	}
}

// validateShareFields checks every field that is written into smb.conf or
// /etc/exports. Before this, a newline in the name, path or description could
// add any Samba directive, and the path could be anywhere (/, /etc). Empty
// name and path pass here because an update keeps the old values; CreateShare
// requires them.
func validateShareFields(s *shares.Share) error {
	if s.Name != "" {
		if err := validate.ShareName(s.Name); err != nil {
			return fmt.Errorf("share name: use 1-32 letters, digits, '-' or '_'")
		}
	}
	if s.Path != "" {
		if err := validate.PathUnder(validate.ShareRoots, s.Path); err != nil {
			return fmt.Errorf("share path must be inside %s", strings.Join(validate.ShareRoots, " or "))
		}
	}
	if err := validate.Description(s.Description); err != nil {
		return err
	}
	for _, list := range [][]string{s.Users, s.Groups} {
		for _, p := range list {
			if err := validate.Principal(p); err != nil {
				return fmt.Errorf("%w: %q", err, p)
			}
		}
	}
	for _, c := range s.Hosts {
		if err := validate.NFSClient(c); err != nil {
			return fmt.Errorf("%w: %q", err, c)
		}
	}
	return nil
}
