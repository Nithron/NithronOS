package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nithronos/backend/nosd/pkg/validate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
	"nithronos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ShareConfig represents a network share configuration
type ShareConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Protocol    string            `json:"protocol"` // smb, nfs
	Enabled     bool              `json:"enabled"`
	ReadOnly    bool              `json:"readOnly"`
	GuestAccess bool              `json:"guestAccess,omitempty"`
	Users       []string          `json:"users,omitempty"`
	Groups      []string          `json:"groups,omitempty"`
	Hosts       []string          `json:"hosts,omitempty"` // For NFS
	Options     map[string]string `json:"options,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// SharesStore manages share configurations
type SharesStore struct {
	path   string
	shares map[string]*ShareConfig
	mu     sync.RWMutex
}

// NewSharesStore creates a new shares store
func NewSharesStore(path string) (*SharesStore, error) {
	s := &SharesStore{
		path:   path,
		shares: make(map[string]*ShareConfig),
	}

	// Load existing shares
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *SharesStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var shares []*ShareConfig
	ok, err := fsatomic.LoadJSON(s.path, &shares)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	s.shares = make(map[string]*ShareConfig)
	for _, share := range shares {
		s.shares[share.ID] = share
	}

	return nil
}

func (s *SharesStore) save() error {
	shares := make([]*ShareConfig, 0, len(s.shares))
	for _, share := range s.shares {
		shares = append(shares, share)
	}

	return fsatomic.SaveJSON(s.path, shares, 0600)
}

func (s *SharesStore) List() []*ShareConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shares := make([]*ShareConfig, 0, len(s.shares))
	for _, share := range s.shares {
		shares = append(shares, share)
	}
	return shares
}

func (s *SharesStore) Get(id string) (*ShareConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	share, ok := s.shares[id]
	return share, ok
}

func (s *SharesStore) Create(share *ShareConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if share.ID == "" {
		share.ID = uuid.New().String()
	}

	share.CreatedAt = time.Now()
	share.UpdatedAt = time.Now()

	s.shares[share.ID] = share
	return s.save()
}

func (s *SharesStore) Update(id string, updates *ShareConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	share, ok := s.shares[id]
	if !ok {
		return fmt.Errorf("share not found")
	}

	// Update fields
	if updates.Name != "" {
		share.Name = updates.Name
	}
	if updates.Path != "" {
		share.Path = updates.Path
	}
	if updates.Protocol != "" {
		share.Protocol = updates.Protocol
	}
	share.Enabled = updates.Enabled
	share.ReadOnly = updates.ReadOnly
	share.GuestAccess = updates.GuestAccess
	if updates.Users != nil {
		share.Users = updates.Users
	}
	if updates.Groups != nil {
		share.Groups = updates.Groups
	}
	if updates.Hosts != nil {
		share.Hosts = updates.Hosts
	}
	if updates.Options != nil {
		share.Options = updates.Options
	}
	if updates.Description != "" {
		share.Description = updates.Description
	}
	share.UpdatedAt = time.Now()

	return s.save()
}

func (s *SharesStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.shares[id]; !ok {
		return fmt.Errorf("share not found")
	}

	delete(s.shares, id)
	return s.save()
}

// SambaManager manages Samba/SMB shares
type SambaManager struct {
	configPath string
}

func NewSambaManager() *SambaManager {
	return &SambaManager{
		configPath: "/etc/samba/smb.conf",
	}
}

func (m *SambaManager) ApplyShare(share *ShareConfig) error {
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

func (m *SambaManager) TestShare(share *ShareConfig) error {
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

func (m *NFSManager) ApplyShare(share *ShareConfig) error {
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

func (m *NFSManager) TestShare(share *ShareConfig) error {
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
	store *SharesStore
	samba *SambaManager
	nfs   *NFSManager
	agent AgentClient
}

// NewSharesHandlerV2 creates a new shares handler
func NewSharesHandlerV2(storePath string, agent AgentClient) (*SharesHandlerV2, error) {
	store, err := NewSharesStore(storePath)
	if err != nil {
		return nil, err
	}

	return &SharesHandlerV2{
		store: store,
		samba: NewSambaManager(),
		nfs:   NewNFSManager(),
		agent: agent,
	}, nil
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
	shares := h.store.List()
	writeJSON(w, shares)
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
	var share ShareConfig
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

	// The ID names the share's file under /etc/samba/shares.d, so it is
	// always assigned here and never taken from the request.
	share.ID = ""

	// Check if path exists
	if _, err := os.Stat(share.Path); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Share path does not exist")
		return
	}

	// Create share in store
	if err := h.store.Create(&share); err != nil {
		log.Error().Err(err).Msg("Failed to create share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to create share")
		return
	}

	// Apply to system
	if share.Enabled {
		if err := h.applyShare(&share); err != nil {
			log.Error().Err(err).Str("id", share.ID).Msg("Failed to apply share")
			// Don't fail the request, share is saved
		}
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, share)
}

// UpdateShare updates an existing share
func (h *SharesHandlerV2) UpdateShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var updates ShareConfig
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

	// Get existing share
	existing, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	// Update in store
	if err := h.store.Update(id, &updates); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to update share")
		return
	}

	// Re-apply to system if enabled
	updated, _ := h.store.Get(id)
	if updated.Enabled {
		// Remove old config
		_ = h.removeShare(existing)
		// Apply new config
		if err := h.applyShare(updated); err != nil {
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
	if err := h.removeShare(share); err != nil {
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
		TestShare(*ShareConfig) error
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

	if err := manager.TestShare(share); err != nil {
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

	share, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	share.Enabled = true
	if err := h.store.Update(id, share); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to enable share")
		return
	}

	// Apply to system
	if err := h.applyShare(share); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to apply share")
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to apply share configuration")
		return
	}

	writeJSON(w, share)
}

// DisableShare disables a share
func (h *SharesHandlerV2) DisableShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	share, ok := h.store.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Share not found")
		return
	}

	share.Enabled = false
	if err := h.store.Update(id, share); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to disable share")
		return
	}

	// Remove from system
	if err := h.removeShare(share); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to remove share from system")
	}

	writeJSON(w, share)
}

func (h *SharesHandlerV2) applyShare(share *ShareConfig) error {
	switch share.Protocol {
	case "smb":
		return h.samba.ApplyShare(share)
	case "nfs":
		return h.nfs.ApplyShare(share)
	default:
		return fmt.Errorf("unknown protocol: %s", share.Protocol)
	}
}

func (h *SharesHandlerV2) removeShare(share *ShareConfig) error {
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
func validateShareFields(s *ShareConfig) error {
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
