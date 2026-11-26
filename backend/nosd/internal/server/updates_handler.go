package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nos/backend/nosd/internal/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// UpdateInfo represents information about an available update
type UpdateInfo struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseNotes    string    `json:"release_notes"`
	ReleaseDate     time.Time `json:"release_date"`
	DownloadURL     string    `json:"download_url"`
	DownloadSize    int64     `json:"download_size"`
	Checksum        string    `json:"checksum"`
	Channel         string    `json:"channel"` // stable, beta, nightly
	AutoUpdate      bool      `json:"auto_update"`
}

// UpdateSettings represents update configuration
type UpdateSettings struct {
	AutoUpdate     bool      `json:"auto_update"`
	Channel        string    `json:"channel"`
	CheckInterval  int       `json:"check_interval_hours"`
	LastCheck      time.Time `json:"last_check"`
	NotifyOnUpdate bool      `json:"notify_on_update"`
}

// UpdatesHandler handles system update endpoints
type UpdatesHandler struct {
	config       config.Config
	settingsPath string
}

// NewUpdatesHandler creates a new updates handler
func NewUpdatesHandler(cfg config.Config) *UpdatesHandler {
	return &UpdatesHandler{
		config:       cfg,
		settingsPath: filepath.Join(cfg.EtcDir, "nos", "update-settings.json"),
	}
}

// Routes returns the routes for the updates handler
func (h *UpdatesHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/check", h.CheckForUpdates)
	r.Post("/apply", h.ApplyUpdate)
	return r
}

// CheckForUpdates checks for available system updates
func (h *UpdatesHandler) CheckForUpdates(w http.ResponseWriter, r *http.Request) {
	// Get current version
	currentVersion := h.getCurrentVersion()

	// Check for updates based on OS
	updateInfo := UpdateInfo{
		CurrentVersion: currentVersion,
		Channel:        h.getUpdateChannel(),
		AutoUpdate:     h.getAutoUpdateEnabled(),
	}

	if runtime.GOOS == "linux" {
		// Check APT for updates
		if err := h.checkAPTUpdates(&updateInfo); err != nil {
			// Fallback to GitHub releases
			_ = h.checkGitHubReleases(&updateInfo)
		}
	} else {
		// Check GitHub releases for other platforms
		_ = h.checkGitHubReleases(&updateInfo)
	}

	// Save last check time
	h.saveLastCheckTime()

	writeJSON(w, updateInfo)
}

// GetUpdateSettings returns current update settings
func (h *UpdatesHandler) GetUpdateSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.loadSettings()
	writeJSON(w, settings)
}

// UpdateSettings updates the update configuration
func (h *UpdatesHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings UpdateSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "updates.invalid_request", "Invalid request body", 0)
		return
	}

	// Validate channel
	if settings.Channel != "stable" && settings.Channel != "beta" && settings.Channel != "nightly" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "updates.invalid_channel", "Invalid update channel", 0)
		return
	}

	// Save settings
	if err := h.saveSettings(settings); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "updates.save_failed", "Failed to save settings", 0)
		return
	}

	// If auto-update is enabled, schedule update checks
	if settings.AutoUpdate {
		go h.scheduleUpdateChecks(settings.CheckInterval)
	}

	writeJSON(w, settings)
}

// ApplyUpdate applies a pending update
func (h *UpdatesHandler) ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	// Check if an update is available
	updateInfo := h.getLatestUpdateInfo()
	if !updateInfo.UpdateAvailable {
		httpx.WriteTypedError(w, http.StatusNotFound, "updates.none_available", "No update available", 0)
		return
	}

	// Start update process in background
	go h.performUpdate(updateInfo)

	writeJSON(w, map[string]any{
		"message": "Update process started",
		"version": updateInfo.LatestVersion,
	})
}

// GetUpdateHistory returns update history
func (h *UpdatesHandler) GetUpdateHistory(w http.ResponseWriter, r *http.Request) {
	history := h.loadUpdateHistory()
	writeJSON(w, history)
}

// Helper methods

func (h *UpdatesHandler) getCurrentVersion() string {
	// Try to get version from file
	versionFile := filepath.Join(h.config.EtcDir, "nos", "version")
	if data, err := os.ReadFile(versionFile); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Fall back to the version compiled into this build
	return version.Version
}

func (h *UpdatesHandler) getUpdateChannel() string {
	settings := h.loadSettings()
	if settings.Channel == "" {
		return "stable"
	}
	return settings.Channel
}

func (h *UpdatesHandler) getAutoUpdateEnabled() bool {
	settings := h.loadSettings()
	return settings.AutoUpdate
}

func (h *UpdatesHandler) checkAPTUpdates(info *UpdateInfo) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("APT updates only available on Linux")
	}

	// Update package list
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "apt-get", "update")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Check for nos package updates
	cmd = exec.CommandContext(ctx, "apt-cache", "policy", "nos")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Parse output to find available version
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Candidate:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				candidateVersion := parts[1]
				if candidateVersion != info.CurrentVersion {
					info.LatestVersion = candidateVersion
					info.UpdateAvailable = true
					info.ReleaseDate = time.Now() // APT doesn't provide release date
					info.ReleaseNotes = "Update available via APT"
				}
			}
		}
	}

	return nil
}

func (h *UpdatesHandler) checkGitHubReleases(info *UpdateInfo) error {
	// Check GitHub API for latest release
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/Nithron/nOS/releases/latest", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name        string `json:"name"`
			Size        int64  `json:"size"`
			DownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	// Compare versions
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion != info.CurrentVersion {
		info.LatestVersion = latestVersion
		info.UpdateAvailable = true
		info.ReleaseNotes = release.Body
		info.ReleaseDate = release.PublishedAt

		// Find appropriate asset
		osArch := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
		for _, asset := range release.Assets {
			if strings.Contains(asset.Name, osArch) {
				info.DownloadURL = asset.DownloadURL
				info.DownloadSize = asset.Size
				break
			}
		}
	}

	return nil
}

func (h *UpdatesHandler) loadSettings() UpdateSettings {
	settings := UpdateSettings{
		AutoUpdate:     false,
		Channel:        "stable",
		CheckInterval:  24,
		NotifyOnUpdate: true,
	}

	if data, err := os.ReadFile(h.settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	return settings
}

func (h *UpdatesHandler) saveSettings(settings UpdateSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(h.settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(h.settingsPath, data, 0644)
}

func (h *UpdatesHandler) saveLastCheckTime() {
	settings := h.loadSettings()
	settings.LastCheck = time.Now()
	_ = h.saveSettings(settings)
}

func (h *UpdatesHandler) getLatestUpdateInfo() UpdateInfo {
	info := UpdateInfo{
		CurrentVersion: h.getCurrentVersion(),
		Channel:        h.getUpdateChannel(),
		AutoUpdate:     h.getAutoUpdateEnabled(),
	}

	if runtime.GOOS == "linux" {
		_ = h.checkAPTUpdates(&info)
	} else {
		_ = h.checkGitHubReleases(&info)
	}

	return info
}

func (h *UpdatesHandler) performUpdate(info UpdateInfo) {
	// Log update start
	h.logUpdate("Starting update to version " + info.LatestVersion)

	if runtime.GOOS == "linux" {
		// Use APT for updates
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", "nos")
		if output, err := cmd.CombinedOutput(); err != nil {
			h.logUpdate(fmt.Sprintf("Update failed: %v\nOutput: %s", err, output))
			return
		}

		h.logUpdate("Update completed successfully")

		// Restart service
		_ = exec.Command("systemctl", "restart", "nosd").Run()
	} else if info.DownloadURL != "" {
		// Download and apply update for non-Linux systems
		if err := h.downloadAndApplyUpdate(info); err != nil {
			h.logUpdate(fmt.Sprintf("Update failed: %v", err))
			return
		}
		h.logUpdate("Update completed successfully")
	}
}

func (h *UpdatesHandler) downloadAndApplyUpdate(info UpdateInfo) error {
	// Download update
	resp, err := http.Get(info.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), "nos-update")
	out, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	// Apply update (platform-specific)
	// This is a simplified version - real implementation would need proper update mechanism
	return fmt.Errorf("automatic updates not yet implemented for this platform")
}

func (h *UpdatesHandler) scheduleUpdateChecks(intervalHours int) {
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		info := h.getLatestUpdateInfo()
		if info.UpdateAvailable && h.getAutoUpdateEnabled() {
			h.performUpdate(info)
		}
	}
}

func (h *UpdatesHandler) loadUpdateHistory() []map[string]any {
	historyFile := filepath.Join(h.config.EtcDir, "nos", "update-history.json")
	var history []map[string]any

	if data, err := os.ReadFile(historyFile); err == nil {
		_ = json.Unmarshal(data, &history)
	}

	if history == nil {
		history = []map[string]any{}
	}

	return history
}

func (h *UpdatesHandler) logUpdate(message string) {
	entry := map[string]any{
		"timestamp": time.Now(),
		"message":   message,
	}

	history := h.loadUpdateHistory()
	history = append(history, entry)

	// Keep only last 100 entries
	if len(history) > 100 {
		history = history[len(history)-100:]
	}

	historyFile := filepath.Join(h.config.EtcDir, "nos", "update-history.json")
	if data, err := json.MarshalIndent(history, "", "  "); err == nil {
		_ = os.WriteFile(historyFile, data, 0644)
	}
}
