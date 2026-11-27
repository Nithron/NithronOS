// Package api provides the HTTP client for communicating with nOS server.
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

	"nos/clients/sync-core/config"
)

// Client is the API client for nOS sync endpoints.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	baseURL    string
	mu         sync.RWMutex

	// Token refresh callback
	onTokenRefresh func(deviceToken, refreshToken string)
}

// NewClient creates a new API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
				DisableKeepAlives:  false,
				MaxConnsPerHost:    10,
				TLSClientConfig:    tlsConfig(cfg.GetServerKeySHA256()),
			},
		},
		baseURL: cfg.GetServerURL(),
	}
}

// SetTokenRefreshCallback sets the callback for token refresh events.
func (c *Client) SetTokenRefreshCallback(fn func(deviceToken, refreshToken string)) {
	c.mu.Lock()
	c.onTokenRefresh = fn
	c.mu.Unlock()
}

// request makes an authenticated HTTP request.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, method, path, body, result, true)
}

// doRequest refreshes the token and retries once on a 401. (It retried
// for as long as the server said 401, so an endpoint that refused the
// token made the client refresh it several times a second, forever.)
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}, mayRefresh bool) error {
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

	// The device token from pairing. (The client sent an "access token"
	// that nosd never issues, so no request was ever authenticated.)
	if tok := c.cfg.GetDeviceToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - try to refresh token
	if resp.StatusCode == http.StatusUnauthorized && mayRefresh {
		if err := c.refreshToken(ctx); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		return c.doRequest(ctx, method, path, body, result, false)
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode >= 400 {
		var body struct {
			Error APIError `json:"error"`
		}
		if err := json.Unmarshal(respBody, &body); err != nil || body.Error.Code == "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		body.Error.Status = resp.StatusCode
		return &body.Error
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
	if resp.DeviceToken == "" {
		return fmt.Errorf("token refresh returned no device token")
	}
	if err := c.cfg.SetTokens(resp.DeviceToken, resp.RefreshToken); err != nil {
		return err
	}

	// Notify callback
	c.mu.RLock()
	callback := c.onTokenRefresh
	c.mu.RUnlock()
	if callback != nil {
		callback(resp.DeviceToken, resp.RefreshToken)
	}

	return nil
}

// APIError is nosd's typed error ({"error": {"code", "message"}}).
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Request/Response types

// TokenRefreshRequest is the request body for token refresh.
type TokenRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

// TokenRefreshResponse is nosd's answer to a refresh (pkg/sync
// DeviceRefreshResponse).
type TokenRefreshResponse struct {
	DeviceToken  string `json:"device_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// The types below are nosd's wire format (backend/nosd/pkg/sync/types.go).
// (They were written against another API: share "id" for "share_id",
// "mod_time" for "mtime", "content_hash" for "hash", a bare list of shares
// for {"shares": [...]}, change "action"s nosd doesn't send, so nothing the
// server returned decoded.)

// SyncShare is a share this device may sync.
type SyncShare struct {
	ID          string `json:"share_id"`
	Name        string `json:"share_name"`
	Path        string `json:"share_path"`
	SyncEnabled bool   `json:"sync_enabled"`
	TotalSize   int64  `json:"total_size"`
	FileCount   int64  `json:"file_count"`
}

// Change types nosd reports.
const (
	ChangeCreate = "create"
	ChangeModify = "modify"
	ChangeDelete = "delete"
	ChangeRename = "rename"
)

// FileChange is one change on the server.
type FileChange struct {
	Path    string    `json:"path"`
	Type    string    `json:"type"` // ChangeCreate, ...
	Size    int64     `json:"size"`
	MTime   time.Time `json:"mtime"`
	Hash    string    `json:"hash,omitempty"`     // SHA-256, when nosd has it
	OldPath string    `json:"old_path,omitempty"` // for renames
	IsDir   bool      `json:"is_dir"`
}

// ChangesResponse is a page of changes after a cursor.
type ChangesResponse struct {
	Changes []FileChange `json:"changes"`
	Cursor  string       `json:"cursor"`
	HasMore bool         `json:"has_more"`
}

// BlockHash is one block of the server's copy of a file.
type BlockHash struct {
	Offset     int64  `json:"offset"`
	Size       int64  `json:"size"`
	StrongHash string `json:"hash"` // SHA-256
	WeakHash   uint32 `json:"weak"` // Adler-32
}

// BlockHashResponse is the server's blocks of a file. BlockSize is the
// server's choice (it doesn't take the client's).
type BlockHashResponse struct {
	Path      string      `json:"path"`
	Size      int64       `json:"size"`
	Hash      string      `json:"hash"`
	BlockSize int64       `json:"block_size"`
	Blocks    []BlockHash `json:"blocks"`
}

// API Methods

// ListShares returns the shares this device may sync.
func (c *Client) ListShares(ctx context.Context) ([]SyncShare, error) {
	var resp struct {
		Shares []SyncShare `json:"shares"`
	}
	err := c.request(ctx, "GET", "/api/v1/sync/shares", nil, &resp)
	return resp.Shares, err
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

// DeltaOp is one step of a delta upload: copy Size bytes at Offset of the
// server's current file, or new Data.
type DeltaOp struct {
	Offset int64  `json:"offset,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Data   []byte `json:"data,omitempty"`
}

// UploadDelta rebuilds path on the server from its current content and ops;
// sha256Hex is what the result must hash to (the server refuses otherwise).
func (c *Client) UploadDelta(ctx context.Context, shareID, path, sha256Hex string, ops []DeltaOp) error {
	reqPath := fmt.Sprintf("/api/v1/sync/files/%s/delta", url.QueryEscape(shareID))
	body := map[string]any{"path": path, "sha256": sha256Hex, "ops": ops}
	return c.request(ctx, "POST", reqPath, body, nil)
}

// HealthCheck checks if the server is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.request(ctx, "GET", "/api/v1/health", nil, nil)
}
