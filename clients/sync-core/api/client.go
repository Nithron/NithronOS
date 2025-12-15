// Package api provides the HTTP client for communicating with NithronOS server.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"nithronos/clients/sync-core/config"
)

// Client is the API client for NithronOS sync endpoints.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	baseURL    string
	mu         sync.RWMutex

	// Token refresh callback
	onTokenRefresh func(accessToken, refreshToken string)
}

// NewClient creates a new API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				DisableKeepAlives:   false,
				MaxConnsPerHost:     10,
			},
		},
		baseURL: cfg.GetServerURL(),
	}
}

// SetTokenRefreshCallback sets the callback for token refresh events.
func (c *Client) SetTokenRefreshCallback(fn func(accessToken, refreshToken string)) {
	c.mu.Lock()
	c.onTokenRefresh = fn
	c.mu.Unlock()
}

// request makes an authenticated HTTP request.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	c.mu.RLock()
	baseURL := c.baseURL
	c.mu.RUnlock()

	if baseURL == "" {
		baseURL = c.cfg.GetServerURL()
	}

	fullURL := baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NithronSync/1.0.0")

	// Add authorization
	accessToken := c.cfg.GetAccessToken()
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - try to refresh token
	if resp.StatusCode == http.StatusUnauthorized {
		if err := c.refreshToken(ctx); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		// Retry with new token
		return c.request(ctx, method, path, body, result)
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return &apiErr
	}

	// Parse result
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// refreshToken attempts to refresh the access token.
func (c *Client) refreshToken(ctx context.Context) error {
	refreshToken := c.cfg.RefreshToken
	deviceID := c.cfg.DeviceID

	if refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	req := TokenRefreshRequest{
		RefreshToken: refreshToken,
		DeviceID:     deviceID,
	}

	var resp TokenRefreshResponse
	// Use device token for refresh request
	fullURL := c.cfg.GetServerURL() + "/api/v1/sync/devices/refresh"
	
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "NithronSync/1.0.0")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed with status %d", httpResp.StatusCode)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return err
	}

	// Update tokens
	if err := c.cfg.SetTokens(resp.AccessToken, resp.RefreshToken); err != nil {
		return err
	}

	// Notify callback
	c.mu.RLock()
	callback := c.onTokenRefresh
	c.mu.RUnlock()
	if callback != nil {
		callback(resp.AccessToken, resp.RefreshToken)
	}

	return nil
}

// APIError represents an API error response.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Error)
}

// Request/Response types

// TokenRefreshRequest is the request body for token refresh.
type TokenRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

// TokenRefreshResponse is the response from token refresh.
type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"access_expires_at"`
}

// DeviceRegistration is the request for registering a device.
type DeviceRegistration struct {
	DeviceName    string `json:"device_name"`
	DeviceType    string `json:"device_type"`
	ClientVersion string `json:"client_version"`
	OSVersion     string `json:"os_version,omitempty"`
}

// DeviceRegistrationResponse is the response from device registration.
type DeviceRegistrationResponse struct {
	DeviceToken struct {
		ID         string `json:"id"`
		DeviceName string `json:"device_name"`
		DeviceType string `json:"device_type"`
	} `json:"device_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// SyncShare represents a sync-enabled share.
type SyncShare struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	SyncEnabled  bool     `json:"sync_enabled"`
	SyncMaxSize  int64    `json:"sync_max_size"`
	SyncExclude  []string `json:"sync_exclude"`
}

// SyncConfig represents device sync configuration.
type SyncConfig struct {
	DeviceID        string   `json:"device_id"`
	Enabled         bool     `json:"enabled"`
	SyncShares      []string `json:"sync_shares"`
	BandwidthLimit  int      `json:"bandwidth_limit_kbps"`
	SyncOnMetered   bool     `json:"sync_on_metered"`
}

// FileChange represents a file change from the server.
type FileChange struct {
	Path         string `json:"path"`
	Type         string `json:"type"` // "file" or "dir"
	Action       string `json:"action"` // "created", "modified", "deleted", "moved"
	Size         int64  `json:"size"`
	ModTime      string `json:"mod_time"`
	ContentHash  string `json:"content_hash"`
	PreviousPath string `json:"previous_path,omitempty"`
	Version      int    `json:"version"`
}

// ChangesResponse is the response from the changes endpoint.
type ChangesResponse struct {
	Changes []FileChange `json:"changes"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"has_more"`
}

// FileMetadata represents file or directory metadata.
type FileMetadata struct {
	Path        string         `json:"path"`
	Type        string         `json:"type"`
	Size        int64          `json:"size"`
	ModTime     string         `json:"mod_time"`
	ContentHash string         `json:"content_hash,omitempty"`
	Children    []FileMetadata `json:"children,omitempty"`
}

// BlockHash represents a block hash for delta sync.
type BlockHash struct {
	Index      int    `json:"index"`
	Offset     int64  `json:"offset"`
	Size       int    `json:"size"`
	StrongHash string `json:"strong_hash"`
	WeakHash   uint32 `json:"weak_hash"`
}

// BlockHashResponse is the response from block hash endpoint.
type BlockHashResponse struct {
	Path      string      `json:"path"`
	Size      int64       `json:"size"`
	BlockSize int         `json:"block_size"`
	Blocks    []BlockHash `json:"blocks"`
}

// SyncState represents sync state for a share.
type SyncState struct {
	ShareID          string `json:"share_id"`
	DeviceID         string `json:"device_id"`
	LastSyncAt       string `json:"last_sync_at"`
	Cursor           string `json:"cursor"`
	SyncStatus       string `json:"sync_status"`
	PendingUploads   int    `json:"pending_uploads"`
	PendingDownloads int    `json:"pending_downloads"`
}

// API Methods

// ListShares returns all sync-enabled shares.
func (c *Client) ListShares(ctx context.Context) ([]SyncShare, error) {
	var shares []SyncShare
	err := c.request(ctx, "GET", "/api/v1/sync/shares", nil, &shares)
	return shares, err
}

// GetSyncConfig returns the device sync configuration.
func (c *Client) GetSyncConfig(ctx context.Context) (*SyncConfig, error) {
	var cfg SyncConfig
	err := c.request(ctx, "GET", "/api/v1/sync/config", nil, &cfg)
	return &cfg, err
}

// UpdateSyncConfig updates the device sync configuration.
func (c *Client) UpdateSyncConfig(ctx context.Context, cfg *SyncConfig) error {
	return c.request(ctx, "PUT", "/api/v1/sync/config", cfg, nil)
}

// GetChanges returns file changes since the given cursor.
func (c *Client) GetChanges(ctx context.Context, shareID, cursor string, limit int) (*ChangesResponse, error) {
	path := fmt.Sprintf("/api/v1/sync/changes?share_id=%s&limit=%d", 
		url.QueryEscape(shareID), limit)
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}

	var resp ChangesResponse
	err := c.request(ctx, "GET", path, nil, &resp)
	return &resp, err
}

// GetFileMetadata returns metadata for a file or directory.
func (c *Client) GetFileMetadata(ctx context.Context, shareID, path string, includeChildren bool) (*FileMetadata, error) {
	reqPath := fmt.Sprintf("/api/v1/sync/files/%s/metadata?path=%s&include_children=%t",
		url.QueryEscape(shareID), url.QueryEscape(path), includeChildren)

	var meta FileMetadata
	err := c.request(ctx, "GET", reqPath, nil, &meta)
	return &meta, err
}

// GetBlockHashes returns block hashes for delta sync.
func (c *Client) GetBlockHashes(ctx context.Context, shareID, path string, blockSize int) (*BlockHashResponse, error) {
	reqPath := fmt.Sprintf("/api/v1/sync/files/%s/hash", url.QueryEscape(shareID))
	
	body := map[string]interface{}{
		"path":       path,
		"block_size": blockSize,
	}

	var resp BlockHashResponse
	err := c.request(ctx, "POST", reqPath, body, &resp)
	return &resp, err
}

// GetSyncState returns the sync state for a share.
func (c *Client) GetSyncState(ctx context.Context, shareID string) (*SyncState, error) {
	path := fmt.Sprintf("/api/v1/sync/state/%s", url.QueryEscape(shareID))
	
	var state SyncState
	err := c.request(ctx, "GET", path, nil, &state)
	return &state, err
}

// UpdateSyncState updates the sync state for a share.
func (c *Client) UpdateSyncState(ctx context.Context, shareID string, cursor, status string) error {
	path := fmt.Sprintf("/api/v1/sync/state/%s", url.QueryEscape(shareID))
	
	body := map[string]string{
		"cursor":      cursor,
		"sync_status": status,
	}
	
	return c.request(ctx, "PUT", path, body, nil)
}

// HealthCheck checks if the server is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.request(ctx, "GET", "/api/v1/health", nil, nil)
}

