package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient handles communication with the NithronOS API
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// newAPIClient creates a new API client
func newAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request
func (c *APIClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			if errResp.Error != "" {
				return nil, fmt.Errorf("API error: %s", errResp.Error)
			}
			if errResp.Message != "" {
				return nil, fmt.Errorf("API error: %s", errResp.Message)
			}
		}
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return respBody, nil
}

// System API

func (c *APIClient) testConnection() error {
	_, err := c.doRequest("GET", "/api/v1/health", nil)
	return err
}

func (c *APIClient) getSystemStatus() (*SystemStatus, error) {
	data, err := c.doRequest("GET", "/api/v1/system/status", nil)
	if err != nil {
		return nil, err
	}

	var status SystemStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (c *APIClient) getSystemInfo() (*SystemInfo, error) {
	data, err := c.doRequest("GET", "/api/v1/system/info", nil)
	if err != nil {
		return nil, err
	}

	var info SystemInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Storage API

func (c *APIClient) listSnapshots() ([]Snapshot, error) {
	data, err := c.doRequest("GET", "/api/v1/backup/snapshots", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []Snapshot `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

func (c *APIClient) createSnapshot(subvols []string, tag string) (*Job, error) {
	req := map[string]interface{}{
		"subvols": subvols,
		"tag":     tag,
	}

	data, err := c.doRequest("POST", "/api/v1/backup/snapshots/create", req)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

func (c *APIClient) deleteSnapshot(id string) error {
	_, err := c.doRequest("DELETE", "/api/v1/backup/snapshots/"+id, nil)
	return err
}

// Apps API

func (c *APIClient) listApps() ([]App, error) {
	data, err := c.doRequest("GET", "/api/v1/apps/installed", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []App `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

func (c *APIClient) installApp(id string, params map[string]interface{}) error {
	req := map[string]interface{}{
		"id":     id,
		"params": params,
	}

	_, err := c.doRequest("POST", "/api/v1/apps/install", req)
	return err
}

func (c *APIClient) uninstallApp(id string, keepData bool) error {
	req := map[string]interface{}{
		"keep_data": keepData,
	}

	_, err := c.doRequest("DELETE", "/api/v1/apps/"+id, req)
	return err
}

func (c *APIClient) restartApp(id string) error {
	_, err := c.doRequest("POST", "/api/v1/apps/"+id+"/restart", nil)
	return err
}

// Backup API

func (c *APIClient) runBackup(scheduleID string) (*Job, error) {
	req := map[string]interface{}{
		"schedule_id": scheduleID,
	}

	data, err := c.doRequest("POST", "/api/v1/backup/run", req)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

func (c *APIClient) restore(sourceType, sourceID, restoreType, targetPath string) (*Job, error) {
	req := map[string]interface{}{
		"source_type":  sourceType,
		"source_id":    sourceID,
		"restore_type": restoreType,
		"target_path":  targetPath,
	}

	data, err := c.doRequest("POST", "/api/v1/backup/restore/apply", req)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

func (c *APIClient) getBackupJob(id string) (*Job, error) {
	data, err := c.doRequest("GET", "/api/v1/backup/jobs/"+id, nil)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// Alerts API

func (c *APIClient) listAlertRules() ([]AlertRule, error) {
	data, err := c.doRequest("GET", "/api/v1/monitor/alerts/rules", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Rules []AlertRule `json:"rules"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Rules, nil
}

func (c *APIClient) createAlertRule(name, metric, operator string, threshold float64, duration int, severity string) (*AlertRule, error) {
	req := map[string]interface{}{
		"name":      name,
		"metric":    metric,
		"operator":  operator,
		"threshold": threshold,
		"duration":  duration,
		"severity":  severity,
		"enabled":   true,
	}

	data, err := c.doRequest("POST", "/api/v1/monitor/alerts/rules", req)
	if err != nil {
		return nil, err
	}

	var rule AlertRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, err
	}

	return &rule, nil
}

// Tokens API

func (c *APIClient) listTokens() ([]Token, error) {
	data, err := c.doRequest("GET", "/api/v1/tokens", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tokens []Token `json:"tokens"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Tokens, nil
}

func (c *APIClient) createToken(name string, scopes []string, expires string) (*Token, string, error) {
	req := map[string]interface{}{
		"type":    "personal",
		"name":    name,
		"scopes":  scopes,
		"expires": expires,
	}

	data, err := c.doRequest("POST", "/api/v1/tokens", req)
	if err != nil {
		return nil, "", err
	}

	var result struct {
		Token Token  `json:"token"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, "", err
	}

	return &result.Token, result.Value, nil
}

func (c *APIClient) deleteToken(id string) error {
	_, err := c.doRequest("DELETE", "/api/v1/tokens/"+id, nil)
	return err
}

// OpenAPI

func (c *APIClient) getOpenAPISpec() ([]byte, error) {
	return c.doRequest("GET", "/api/v1/openapi.json", nil)
}

// Types

type SystemStatus struct {
	Version        string    `json:"version"`
	Uptime         string    `json:"uptime"`
	Load1          float64   `json:"load_1"`
	Load5          float64   `json:"load_5"`
	Load15         float64   `json:"load_15"`
	CPUUsage       float64   `json:"cpu_usage"`
	MemoryUsed     int64     `json:"memory_used"`
	MemoryTotal    int64     `json:"memory_total"`
	MemoryPercent  float64   `json:"memory_percent"`
	StorageUsed    int64     `json:"storage_used"`
	StorageTotal   int64     `json:"storage_total"`
	StoragePercent float64   `json:"storage_percent"`
	Services       []Service `json:"services"`
}

type Service struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Active bool   `json:"active"`
}

type SystemInfo struct {
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Kernel     string `json:"kernel"`
	Arch       string `json:"arch"`
	CPUs       int    `json:"cpus"`
	Memory     int64  `json:"memory"`
	NOSVersion string `json:"nos_version"`
}

type Snapshot struct {
	ID        string `json:"id"`
	Subvolume string `json:"subvolume"`
	CreatedAt string `json:"created_at"`
	Size      int64  `json:"size"`
}

type App struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Health  string `json:"health"`
}

type Job struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	State    string `json:"state"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

type AlertRule struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Metric       string    `json:"metric"`
	Operator     string    `json:"operator"`
	Threshold    float64   `json:"threshold"`
	Duration     int       `json:"duration"`
	Severity     string    `json:"severity"`
	Enabled      bool      `json:"enabled"`
	CurrentState RuleState `json:"current_state"`
}

type RuleState struct {
	Firing bool `json:"firing"`
}

type Token struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
}
