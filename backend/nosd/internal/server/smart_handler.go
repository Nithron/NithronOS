package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/disks"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// SMARTDevice represents SMART data for a storage device
type SMARTDevice struct {
	Device       string         `json:"device"`
	Model        string         `json:"model"`
	SerialNumber string         `json:"serial_number"`
	Capacity     int64          `json:"capacity_bytes"`
	Temperature  int            `json:"temperature_celsius,omitempty"`
	PowerOnHours int            `json:"power_on_hours,omitempty"`
	Health       string         `json:"health"` // good, warning, critical, unknown
	LastChecked  time.Time      `json:"last_checked"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// SMARTSummary represents overall SMART health
type SMARTSummary struct {
	HealthyDevices  int       `json:"healthy_devices"`
	WarningDevices  int       `json:"warning_devices"`
	CriticalDevices int       `json:"critical_devices"`
	TotalDevices    int       `json:"total_devices"`
	LastScan        time.Time `json:"last_scan"`
	NextScan        time.Time `json:"next_scan"`
}

// handleSmartDevices returns SMART data for all devices
func handleSmartDevices(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, collectSMARTDevices(r.Context()))
	}
}

// smartAgentSocket is where collectSMARTDevices finds nos-agent.
var smartAgentSocket = "/run/nos-agent.sock"

// listWholeDisks returns the paths of the whole disks (lsblk TYPE=disk), so
// SATA, NVMe and virtio disks are all found. (The old /dev scan skipped
// every NVMe disk, whose names contain digits, and virtio disks.)
var listWholeDisks = func(ctx context.Context) []string {
	list, err := disks.Collect(ctx)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range list {
		if d.Type == "disk" && d.Path != "" && !strings.HasPrefix(d.Name, "zram") {
			out = append(out, d.Path)
		}
	}
	return out
}

// collectSMARTDevices lists the whole disks and their SMART data from the
// agent; disks the agent can't read are reported with health "unknown"
// rather than guessed.
func collectSMARTDevices(parent context.Context) []SMARTDevice {
	devices := []SMARTDevice{}
	devicePaths := listWholeDisks(parent)
	agent := agentclient.New(smartAgentSocket)
	for _, devPath := range devicePaths {
		device := SMARTDevice{Device: devPath, Health: "unknown", LastChecked: time.Now()}
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		var smartData map[string]any
		err := agent.GetJSON(ctx, "/v1/smart?device="+url.QueryEscape(devPath), &smartData)
		cancel()
		if err == nil {
			device.Attributes = smartData
			// Parse SMART response
			if passed, ok := smartData["passed"].(bool); ok {
				if passed {
					device.Health = "good"
				} else {
					device.Health = "critical"
				}
			}

			if temp, ok := smartData["temperature_c"].(float64); ok {
				device.Temperature = int(temp)
				if device.Temperature > 50 {
					device.Health = "warning"
				}
				if device.Temperature > 60 {
					device.Health = "critical"
				}
			}

			if hours, ok := smartData["power_on_hours"].(float64); ok {
				device.PowerOnHours = int(hours)
			}

		}
		devices = append(devices, device)
	}

	return devices
}

// handleSmartSummary returns overall SMART health summary
func handleSmartSummary(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary := SMARTSummary{
			LastScan: time.Now(),
			NextScan: time.Now().Add(6 * time.Hour),
		}

		// Get device health from the devices endpoint logic
		devices := []SMARTDevice{}
		devicePaths := []string{}
		if entries, err := os.ReadDir("/dev"); err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "hd") {
					if !strings.ContainsAny(name[2:], "0123456789p") {
						devicePaths = append(devicePaths, "/dev/"+name)
					}
				}
			}
		}

		agentSocket := "/run/nos-agent.sock"
		if _, err := os.Stat(agentSocket); err == nil {
			agent := agentclient.New(agentSocket)
			for _, devPath := range devicePaths {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				var smartData map[string]any
				req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/v1/smart?device=%s", devPath), nil)
				if resp, err := agent.HTTP.Do(req); err == nil && resp.StatusCode == 200 {
					defer resp.Body.Close()
					_ = json.NewDecoder(resp.Body).Decode(&smartData)

					health := "unknown"
					if passed, ok := smartData["passed"].(bool); ok {
						if passed {
							health = "good"
						} else {
							health = "critical"
						}
					}

					if temp, ok := smartData["temperature_c"].(float64); ok && temp > 50 {
						health = "warning"
						if temp > 60 {
							health = "critical"
						}
					}

					devices = append(devices, SMARTDevice{Health: health})
				}
			}
		}

		// Count devices by health status
		for _, device := range devices {
			summary.TotalDevices++
			switch device.Health {
			case "good":
				summary.HealthyDevices++
			case "warning":
				summary.WarningDevices++
			case "critical":
				summary.CriticalDevices++
			}
		}

		// If no devices found, return some defaults
		if summary.TotalDevices == 0 && len(devicePaths) > 0 {
			summary.TotalDevices = len(devicePaths)
			summary.HealthyDevices = len(devicePaths) // Assume healthy if can't check
		}

		writeJSON(w, summary)
	}
}

// handleSmartDevice returns SMART data for a specific device
func handleSmartDevice(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceName := chi.URLParam(r, "device")
		if deviceName == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.required", "Device name is required", 0)
			return
		}

		// Sanitize device name
		deviceName = strings.TrimSpace(deviceName)
		if strings.ContainsAny(deviceName, "/\\") {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.invalid", "Invalid device name", 0)
			return
		}

		devicePath := "/dev/" + deviceName

		// Check if device exists
		if _, err := os.Stat(devicePath); err != nil {
			httpx.WriteTypedError(w, http.StatusNotFound, "device.not_found", "Device not found", 0)
			return
		}

		device := SMARTDevice{
			Device:      devicePath,
			Health:      "unknown",
			LastChecked: time.Now(),
		}

		// Try to get SMART data from agent
		agentSocket := "/run/nos-agent.sock"
		if _, err := os.Stat(agentSocket); err == nil {
			agent := agentclient.New(agentSocket)
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			var smartData map[string]any
			req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/v1/smart?device=%s", devicePath), nil)
			if resp, err := agent.HTTP.Do(req); err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				_ = json.NewDecoder(resp.Body).Decode(&smartData)

				device.Attributes = smartData

				// Parse SMART response
				if passed, ok := smartData["passed"].(bool); ok {
					if passed {
						device.Health = "good"
					} else {
						device.Health = "critical"
					}
				}

				if temp, ok := smartData["temperature_c"].(float64); ok {
					device.Temperature = int(temp)
					if device.Temperature > 50 && device.Health == "good" {
						device.Health = "warning"
					}
					if device.Temperature > 60 {
						device.Health = "critical"
					}
				}

				if hours, ok := smartData["power_on_hours"].(float64); ok {
					device.PowerOnHours = int(hours)
				}
			}
		}

		writeJSON(w, device)
	}
}

// smartDevicePath turns the {device} URL parameter (sda, nvme0n1) into a
// path under /dev; anything with a slash is refused.
func smartDevicePath(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", false
	}
	return "/dev/" + name, true
}

// handleSmartScan starts a short SMART self-test on every disk. Results
// show in the disks' SMART data when the tests finish (a few minutes).
func handleSmartScan(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent := agentclient.New(cfg.AgentSocket())
		started := []string{}
		failed := map[string]string{}
		for _, dev := range listWholeDisks(r.Context()) {
			if err := agent.PostJSON(r.Context(), "/v1/smart/test", map[string]string{"device": dev, "type": "short"}, nil); err != nil {
				failed[dev] = err.Error()
				continue
			}
			started = append(started, dev)
		}
		status := http.StatusOK
		if len(started) == 0 && len(failed) > 0 {
			status = http.StatusBadGateway
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"test_type": "short", "started": started, "failed": failed})
	}
}

// handleSmartTestStart starts a SMART self-test (short by default; long or
// conveyance) on one disk.
func handleSmartTestStart(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dev, ok := smartDevicePath(chi.URLParam(r, "device"))
		if !ok {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.invalid", "Invalid device name", 0)
			return
		}
		var body struct {
			TestType string `json:"test_type"` // short, long, conveyance
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body)
		if body.TestType == "" {
			body.TestType = "short"
		}
		err := agentclient.New(cfg.AgentSocket()).PostJSON(r.Context(), "/v1/smart/test", map[string]string{"device": dev, "type": body.TestType}, nil)
		var he *agentclient.HTTPError
		switch {
		case errors.As(err, &he) && (he.Status == http.StatusBadRequest || he.Status == http.StatusConflict):
			httpx.WriteTypedError(w, he.Status, "smart.test_refused", strings.TrimSpace(he.Body), 0)
			return
		case err != nil:
			httpx.WriteTypedError(w, http.StatusBadGateway, "agent.failed", err.Error(), 0)
			return
		}
		writeJSON(w, map[string]any{"device": dev, "test_type": body.TestType, "status": "started"})
	}
}

// handleSmartTestStatus reports whether a self-test is running on a disk.
func handleSmartTestStatus(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dev, ok := smartDevicePath(chi.URLParam(r, "device"))
		if !ok {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.invalid", "Invalid device name", 0)
			return
		}
		var out map[string]any
		if err := agentclient.New(cfg.AgentSocket()).GetJSON(r.Context(), "/v1/smart/test?device="+url.QueryEscape(dev), &out); err != nil {
			httpx.WriteTypedError(w, http.StatusBadGateway, "agent.failed", err.Error(), 0)
			return
		}
		writeJSON(w, out)
	}
}
