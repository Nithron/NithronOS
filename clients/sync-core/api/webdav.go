// Package api provides WebDAV client functionality.
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nithronos/clients/sync-core/config"
)

// WebDAVClient provides WebDAV file operations.
type WebDAVClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewWebDAVClient creates a new WebDAV client.
func NewWebDAVClient(cfg *config.Config) *WebDAVClient {
	return &WebDAVClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Longer timeout for file transfers
			Transport: &http.Transport{
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				DisableCompression:    true, // Don't compress, we handle our own
				ResponseHeaderTimeout: 30 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

// buildURL constructs the WebDAV URL for a share and path.
func (w *WebDAVClient) buildURL(shareID, path string) string {
	baseURL := w.cfg.GetServerURL()
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/dav/%s%s", baseURL, shareID, path)
}

// doRequest performs an authenticated WebDAV request.
func (w *WebDAVClient) doRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Set auth header
	accessToken := w.cfg.GetAccessToken()
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	// Set additional headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	req.Header.Set("User-Agent", "NithronSync/1.0.0")

	return w.httpClient.Do(req)
}

// Download downloads a file from the server.
func (w *WebDAVClient) Download(ctx context.Context, shareID, remotePath, localPath string) error {
	url := w.buildURL(shareID, remotePath)

	resp, err := w.doRequest(ctx, "GET", url, nil, nil)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temp file for atomic write
	tmpPath := localPath + ".nstmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()

	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write file: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close file: %w", closeErr)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Set modification time if available
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := http.ParseTime(lastMod); err == nil {
			os.Chtimes(localPath, t, t)
		}
	}

	return nil
}

// DownloadRange downloads a range of bytes from a file.
func (w *WebDAVClient) DownloadRange(ctx context.Context, shareID, remotePath string, offset, length int64) ([]byte, error) {
	url := w.buildURL(shareID, remotePath)

	headers := map[string]string{
		"Range": fmt.Sprintf("bytes=%d-%d", offset, offset+length-1),
	}

	resp, err := w.doRequest(ctx, "GET", url, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("download range request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download range failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Upload uploads a file to the server.
func (w *WebDAVClient) Upload(ctx context.Context, shareID, localPath, remotePath string) error {
	url := w.buildURL(shareID, remotePath)

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	headers := map[string]string{
		"Content-Type":   "application/octet-stream",
		"Content-Length": strconv.FormatInt(stat.Size(), 10),
	}

	resp, err := w.doRequest(ctx, "PUT", url, f, headers)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UploadReader uploads data from a reader to the server.
func (w *WebDAVClient) UploadReader(ctx context.Context, shareID, remotePath string, reader io.Reader, size int64) error {
	url := w.buildURL(shareID, remotePath)

	headers := map[string]string{
		"Content-Type":   "application/octet-stream",
		"Content-Length": strconv.FormatInt(size, 10),
	}

	resp, err := w.doRequest(ctx, "PUT", url, reader, headers)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}

	return nil
}

// Delete deletes a file or directory from the server.
func (w *WebDAVClient) Delete(ctx context.Context, shareID, remotePath string) error {
	url := w.buildURL(shareID, remotePath)

	resp, err := w.doRequest(ctx, "DELETE", url, nil, nil)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

// Mkdir creates a directory on the server.
func (w *WebDAVClient) Mkdir(ctx context.Context, shareID, remotePath string) error {
	url := w.buildURL(shareID, remotePath)

	resp, err := w.doRequest(ctx, "MKCOL", url, nil, nil)
	if err != nil {
		return fmt.Errorf("mkdir request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("mkdir failed with status %d", resp.StatusCode)
	}

	return nil
}

// MkdirAll creates a directory and all parent directories.
func (w *WebDAVClient) MkdirAll(ctx context.Context, shareID, remotePath string) error {
	parts := strings.Split(strings.Trim(remotePath, "/"), "/")
	current := ""

	for _, part := range parts {
		current = current + "/" + part
		if err := w.Mkdir(ctx, shareID, current); err != nil {
			// Ignore errors for existing directories
			continue
		}
	}

	return nil
}

// Move moves/renames a file or directory.
func (w *WebDAVClient) Move(ctx context.Context, shareID, srcPath, dstPath string) error {
	srcURL := w.buildURL(shareID, srcPath)
	dstURL := w.buildURL(shareID, dstPath)

	headers := map[string]string{
		"Destination": dstURL,
		"Overwrite":   "T",
	}

	resp, err := w.doRequest(ctx, "MOVE", srcURL, nil, headers)
	if err != nil {
		return fmt.Errorf("move request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("move failed with status %d", resp.StatusCode)
	}

	return nil
}

// Copy copies a file or directory.
func (w *WebDAVClient) Copy(ctx context.Context, shareID, srcPath, dstPath string) error {
	srcURL := w.buildURL(shareID, srcPath)
	dstURL := w.buildURL(shareID, dstPath)

	headers := map[string]string{
		"Destination": dstURL,
		"Overwrite":   "T",
	}

	resp, err := w.doRequest(ctx, "COPY", srcURL, nil, headers)
	if err != nil {
		return fmt.Errorf("copy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("copy failed with status %d", resp.StatusCode)
	}

	return nil
}

// Exists checks if a file or directory exists.
func (w *WebDAVClient) Exists(ctx context.Context, shareID, remotePath string) (bool, error) {
	url := w.buildURL(shareID, remotePath)

	resp, err := w.doRequest(ctx, "HEAD", url, nil, nil)
	if err != nil {
		return false, fmt.Errorf("exists check failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// Stat returns file information.
func (w *WebDAVClient) Stat(ctx context.Context, shareID, remotePath string) (*RemoteFileInfo, error) {
	davURL := w.buildURL(shareID, remotePath)

	propfindBody := `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:getlastmodified/>
    <D:getcontentlength/>
    <D:resourcetype/>
    <D:getetag/>
  </D:prop>
</D:propfind>`

	headers := map[string]string{
		"Content-Type": "application/xml",
		"Depth":        "0",
	}

	resp, err := w.doRequest(ctx, "PROPFIND", davURL, strings.NewReader(propfindBody), headers)
	if err != nil {
		return nil, fmt.Errorf("propfind request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("propfind failed with status %d", resp.StatusCode)
	}

	// Parse the response (simplified - in production would use proper XML parsing)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info := &RemoteFileInfo{
		Path: remotePath,
	}

	// Simple parsing for common properties
	bodyStr := string(body)
	info.IsDir = strings.Contains(bodyStr, "<D:collection") || strings.Contains(bodyStr, "<d:collection")

	// Extract size
	if sizeStart := strings.Index(bodyStr, "getcontentlength>"); sizeStart != -1 {
		sizeEnd := strings.Index(bodyStr[sizeStart:], "</")
		if sizeEnd != -1 {
			sizeStr := bodyStr[sizeStart+17 : sizeStart+sizeEnd]
			info.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
		}
	}

	// Extract ETag
	if etagStart := strings.Index(bodyStr, "getetag>"); etagStart != -1 {
		etagEnd := strings.Index(bodyStr[etagStart:], "</")
		if etagEnd != -1 {
			info.ETag = strings.Trim(bodyStr[etagStart+8:etagStart+etagEnd], "\"")
		}
	}

	return info, nil
}

// RemoteFileInfo contains information about a remote file.
type RemoteFileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	ETag    string
}

// List lists directory contents.
func (w *WebDAVClient) List(ctx context.Context, shareID, remotePath string) ([]RemoteFileInfo, error) {
	davURL := w.buildURL(shareID, remotePath)
	if !strings.HasSuffix(davURL, "/") {
		davURL += "/"
	}

	propfindBody := `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:getlastmodified/>
    <D:getcontentlength/>
    <D:resourcetype/>
    <D:getetag/>
  </D:prop>
</D:propfind>`

	headers := map[string]string{
		"Content-Type": "application/xml",
		"Depth":        "1",
	}

	resp, err := w.doRequest(ctx, "PROPFIND", davURL, strings.NewReader(propfindBody), headers)
	if err != nil {
		return nil, fmt.Errorf("propfind request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("propfind failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse responses (simplified)
	var items []RemoteFileInfo
	bodyStr := string(body)
	
	// Split by response elements
	responses := strings.Split(bodyStr, "<D:response>")
	if len(responses) == 1 {
		responses = strings.Split(bodyStr, "<d:response>")
	}

	baseURL := w.buildURL(shareID, remotePath)
	
	for i, respStr := range responses {
		if i == 0 {
			continue // Skip header
		}

		// Extract href
		var href string
		if hrefStart := strings.Index(respStr, "href>"); hrefStart != -1 {
			hrefEnd := strings.Index(respStr[hrefStart:], "</")
			if hrefEnd != -1 {
				href = respStr[hrefStart+5 : hrefStart+hrefEnd]
			}
		}

		// Skip the parent directory itself
		decodedHref, _ := url.QueryUnescape(href)
		if strings.TrimSuffix(decodedHref, "/") == strings.TrimSuffix(baseURL, "/") {
			continue
		}

		info := RemoteFileInfo{
			Path:  filepath.Base(strings.TrimSuffix(decodedHref, "/")),
			IsDir: strings.Contains(respStr, "<D:collection") || strings.Contains(respStr, "<d:collection"),
		}

		// Extract size
		if sizeStart := strings.Index(respStr, "getcontentlength>"); sizeStart != -1 {
			sizeEnd := strings.Index(respStr[sizeStart:], "</")
			if sizeEnd != -1 {
				sizeStr := respStr[sizeStart+17 : sizeStart+sizeEnd]
				info.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
			}
		}

		items = append(items, info)
	}

	return items, nil
}

