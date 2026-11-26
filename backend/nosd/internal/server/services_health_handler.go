package server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"` // running, stopped, failed, unknown
	Enabled     bool      `json:"enabled"`
	Healthy     bool      `json:"healthy"`
	PID         int       `json:"pid,omitempty"`
	Memory      int64     `json:"memory_bytes,omitempty"`
	CPU         float64   `json:"cpu_percent,omitempty"`
	Uptime      int64     `json:"uptime_seconds,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	Logs        []string  `json:"logs,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// ServiceHealthSummary represents overall service health
type ServiceHealthSummary struct {
	TotalServices   int             `json:"total_services"`
	RunningServices int             `json:"running_services"`
	FailedServices  int             `json:"failed_services"`
	Services        []ServiceHealth `json:"services"`
	LastCheck       time.Time       `json:"last_check"`
}

// Critical nOS services to monitor
var criticalServices = []string{
	"nosd",
	"nos-agent",
	"caddy",
	"smbd",
	"nfs-server",
	"docker",
	"snapd",
}

// handleServicesHealth returns the health status of all services
func handleServicesHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary := ServiceHealthSummary{
			Services:  []ServiceHealth{},
			LastCheck: time.Now(),
		}

		for _, serviceName := range criticalServices {
			service := getServiceHealth(serviceName)
			summary.Services = append(summary.Services, service)
			summary.TotalServices++

			switch service.Status {
			case "running":
				summary.RunningServices++
			case "failed":
				summary.FailedServices++
			}
		}

		writeJSON(w, summary)
	}
}

// handleServiceHealth returns the health status of a specific service
func handleServiceHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName := chi.URLParam(r, "service")
		if serviceName == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "service.required", "Service name is required", 0)
			return
		}

		// Validate service name (security)
		isValid := false
		for _, allowed := range criticalServices {
			if serviceName == allowed {
				isValid = true
				break
			}
		}

		// Also allow checking any systemd service if it starts with "nos-"
		if !isValid && strings.HasPrefix(serviceName, "nos-") {
			isValid = true
		}

		if !isValid {
			httpx.WriteTypedError(w, http.StatusForbidden, "service.not_allowed", "Service monitoring not allowed for this service", 0)
			return
		}

		service := getServiceHealth(serviceName)
		writeJSON(w, service)
	}
}

// handleServiceLogs returns recent logs for a service
func handleServiceLogs(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName := chi.URLParam(r, "service")
		if serviceName == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "service.required", "Service name is required", 0)
			return
		}

		// Validate service name (security)
		isValid := false
		for _, allowed := range criticalServices {
			if serviceName == allowed {
				isValid = true
				break
			}
		}

		// Also allow checking any systemd service if it starts with "nos-"
		if !isValid && strings.HasPrefix(serviceName, "nos-") {
			isValid = true
		}

		if !isValid {
			httpx.WriteTypedError(w, http.StatusForbidden, "service.not_allowed", "Service logs not allowed for this service", 0)
			return
		}

		// Get lines parameter (default 100)
		lines := 100
		if l := r.URL.Query().Get("lines"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 5000 {
				lines = parsed
			}
		}

		// Get follow parameter
		follow := r.URL.Query().Get("follow") == "true"

		logs := getServiceLogs(serviceName, lines, follow)
		writeJSON(w, map[string]any{
			"service": serviceName,
			"lines":   lines,
			"logs":    logs,
		})
	}
}

// handleServiceRestart restarts a service
func handleServiceRestart(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceName := chi.URLParam(r, "service")
		if serviceName == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "service.required", "Service name is required", 0)
			return
		}

		// Validate service name (security)
		isValid := false
		for _, allowed := range criticalServices {
			if serviceName == allowed {
				isValid = true
				break
			}
		}

		// Also allow restarting any systemd service if it starts with "nos-"
		if !isValid && strings.HasPrefix(serviceName, "nos-") {
			isValid = true
		}

		if !isValid {
			httpx.WriteTypedError(w, http.StatusForbidden, "service.not_allowed", "Service restart not allowed for this service", 0)
			return
		}

		// Don't allow restarting nosd itself (would kill the API)
		if serviceName == "nosd" {
			httpx.WriteTypedError(w, http.StatusForbidden, "service.self_restart", "Cannot restart nosd through API", 0)
			return
		}

		if runtime.GOOS == "linux" {
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
			if output, err := cmd.CombinedOutput(); err != nil {
				httpx.WriteTypedError(w, http.StatusInternalServerError, "service.restart_failed",
					fmt.Sprintf("Failed to restart service: %s", string(output)), 0)
				return
			}

			// Wait a moment for service to start
			time.Sleep(2 * time.Second)

			// Check new status
			newStatus := getServiceHealth(serviceName)
			writeJSON(w, map[string]any{
				"message": fmt.Sprintf("Service %s restarted successfully", serviceName),
				"status":  newStatus,
			})
		} else {
			httpx.WriteTypedError(w, http.StatusNotImplemented, "service.restart_unsupported",
				"Service restart not supported on this platform", 0)
		}
	}
}

// getServiceHealth gets the health status of a single service
func getServiceHealth(serviceName string) ServiceHealth {
	service := ServiceHealth{
		Name:        serviceName,
		DisplayName: getServiceDisplayName(serviceName),
		Status:      "unknown",
		Enabled:     false,
		Healthy:     false,
	}

	if runtime.GOOS == "linux" {
		// Get service status from systemd
		if output, err := exec.Command("systemctl", "show", serviceName, "--no-page").Output(); err == nil {
			props := parseSystemdShow(string(output))

			// Parse status
			if state, ok := props["ActiveState"]; ok {
				switch state {
				case "active":
					service.Status = "running"
					service.Healthy = true
				case "inactive":
					service.Status = "stopped"
				case "failed":
					service.Status = "failed"
				case "activating":
					service.Status = "starting"
				case "deactivating":
					service.Status = "stopping"
				}
			}

			// Parse enabled state
			if enabled, ok := props["UnitFileState"]; ok {
				service.Enabled = enabled == "enabled" || enabled == "enabled-runtime"
			}

			// Parse PID
			if mainPID, ok := props["MainPID"]; ok {
				if pid, err := strconv.Atoi(mainPID); err == nil && pid > 0 {
					service.PID = pid
				}
			}

			// Parse memory usage
			if mem, ok := props["MemoryCurrent"]; ok && mem != "[not set]" {
				if memBytes, err := strconv.ParseInt(mem, 10, 64); err == nil {
					service.Memory = memBytes
				}
			}

			// Parse CPU usage (this is cumulative, not percentage)
			if cpuUsage, ok := props["CPUUsageNSec"]; ok && cpuUsage != "[not set]" {
				// This would need more complex calculation for percentage
				// For now, just indicate if CPU is being used
				if usage, err := strconv.ParseInt(cpuUsage, 10, 64); err == nil && usage > 0 {
					service.CPU = 0.1 // Placeholder
				}
			}

			// Parse uptime
			if timestamp, ok := props["ActiveEnterTimestamp"]; ok && timestamp != "" {
				// Parse systemd timestamp format
				if t, err := parseSystemdTimestamp(timestamp); err == nil {
					service.StartTime = t
					service.Uptime = int64(time.Since(t).Seconds())
				}
			}

			// Get last error if failed
			if service.Status == "failed" {
				if result, ok := props["Result"]; ok {
					service.LastError = fmt.Sprintf("Service failed with result: %s", result)
				}
			}
		}

		// Get recent logs (last 10 lines)
		service.Logs = getServiceLogs(serviceName, 10, false)

	} else {
		// Fallback for non-Linux systems
		service.Status = "unknown"
		service.Logs = []string{"Service monitoring not available on this platform"}
	}

	return service
}

// getServiceLogs retrieves recent logs for a service
func getServiceLogs(serviceName string, lines int, follow bool) []string {
	logs := []string{}

	if runtime.GOOS == "linux" {
		args := []string{"-u", serviceName, "-n", strconv.Itoa(lines), "--no-pager"}

		// Add output format
		args = append(args, "-o", "short-iso")

		// Note: follow mode would need special handling with streaming
		// For now, just get static logs

		if output, err := exec.Command("journalctl", args...).Output(); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(output)))
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" && !strings.HasPrefix(line, "-- ") {
					logs = append(logs, line)
				}
			}
		} else {
			logs = append(logs, fmt.Sprintf("Failed to retrieve logs: %v", err))
		}
	} else {
		logs = append(logs, "Log retrieval not available on this platform")
	}

	return logs
}

// parseSystemdTimestamp parses systemd timestamp format
func parseSystemdTimestamp(timestamp string) (time.Time, error) {
	// Systemd format: "Mon 2024-01-15 10:30:45 UTC"
	// Try multiple formats
	formats := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05.000000 MST",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestamp); err == nil {
			return t, nil
		}
	}

	// If none work, try to parse as Unix timestamp
	if strings.HasPrefix(timestamp, "n/a") || timestamp == "" {
		return time.Time{}, fmt.Errorf("no timestamp available")
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestamp)
}

// These functions are already defined in monitoring_handler.go
// We'll use those instead
