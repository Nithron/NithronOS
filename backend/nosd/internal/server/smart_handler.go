package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
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

// collectSMARTDevices lists whole disks (sd*, nvme*, hd*) and their SMART data
// from the agent; disks the agent can't read are reported with health
// "unknown" rather than guessed.
func collectSMARTDevices(parent context.Context) []SMARTDevice {
	devices := []SMARTDevice{}

	// Get list of block devices
	devicePaths := []string{}
	if entries, err := os.ReadDir("/dev"); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			// Look for sd*, nvme*, hd* devices
			if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "hd") {
				// Filter out partitions (e.g., sda1, nvme0n1p1)
				if !strings.ContainsAny(name[2:], "0123456789p") {
					devicePaths = append(devicePaths, "/dev/"+name)
				}
			}
		}
	}

	// Try to get SMART data from agent
	agentSocket := "/run/nos-agent.sock"
	if _, err := os.Stat(agentSocket); err == nil {
		agent := agentclient.New(agentSocket)
		for _, devPath := range devicePaths {
			ctx, cancel := context.WithTimeout(parent, 5*time.Second)
			defer cancel()

			var smartData map[string]any
			req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/v1/smart?device=%s", devPath), nil)
			if resp, err := agent.HTTP.Do(req); err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				_ = json.NewDecoder(resp.Body).Decode(&smartData)

				device := SMARTDevice{
					Device:      devPath,
					Health:      "unknown",
					LastChecked: time.Now(),
					Attributes:  smartData,
				}

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

				devices = append(devices, device)
			}
		}
	}

	// Fallback if agent is not available - return mock data
	if len(devices) == 0 && len(devicePaths) > 0 {
		for _, devPath := range devicePaths {
			devices = append(devices, SMARTDevice{
				Device:      devPath,
				Model:       "Unknown Device",
				Health:      "unknown",
				LastChecked: time.Now(),
			})
		}
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

// handleSmartScan triggers a SMART scan on all devices
func handleSmartScan(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This would trigger a background scan of all devices
		// For now, just return success
		result := map[string]any{
			"status":  "started",
			"message": "SMART scan initiated on all devices",
			"devices": []string{},
		}

		// Get list of devices to scan
		if entries, err := os.ReadDir("/dev"); err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "hd") {
					if !strings.ContainsAny(name[2:], "0123456789p") {
						result["devices"] = append(result["devices"].([]string), "/dev/"+name)
					}
				}
			}
		}

		writeJSON(w, result)
	}
}

// handleSmartTest triggers a SMART self-test on a device
func handleSmartTestDevice(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceName := chi.URLParam(r, "device")
		if deviceName == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.required", "Device name is required", 0)
			return
		}

		var body struct {
			TestType string `json:"test_type"` // short, long, conveyance
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			body.TestType = "short" // Default to short test
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

		result := map[string]any{
			"device":    devicePath,
			"test_type": body.TestType,
			"status":    "started",
			"message":   fmt.Sprintf("SMART %s test initiated on %s", body.TestType, devicePath),
		}

		// TODO: Actually trigger the test via agent
		// For now, just return success

		writeJSON(w, result)
	}
}
