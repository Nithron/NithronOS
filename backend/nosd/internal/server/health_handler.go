package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nithronos/backend/nosd/internal/version"
	"nithronos/backend/nosd/pkg/httpx"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// SmartSummary represents a summary of SMART health across all devices
type SmartSummary struct {
	TotalDevices    int     `json:"totalDevices"`
	HealthyDevices  int     `json:"healthyDevices"`
	WarningDevices  int     `json:"warningDevices"`
	CriticalDevices int     `json:"criticalDevices"`
	LastScan        *string `json:"lastScan,omitempty"`
}

// SmartData represents SMART data for a single device
type SmartData struct {
	Device       string           `json:"device"`
	Status       string           `json:"status"` // healthy, warning, critical
	Temperature  *int             `json:"temperature,omitempty"`
	PowerOnHours *int             `json:"powerOnHours,omitempty"`
	Attributes   []SmartAttribute `json:"attributes,omitempty"`
}

// SmartAttribute represents a SMART attribute
type SmartAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  string `json:"rawValue"`
	Status    string `json:"status"` // ok, warning, critical
}

// HealthHandler handles health-related endpoints
type HealthHandler struct {
	agentClient AgentClient
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(agentClient AgentClient) *HealthHandler {
	return &HealthHandler{
		agentClient: agentClient,
	}
}

// Routes registers the health routes
func (h *HealthHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Root health endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "version": version.Version})
	})

	r.Get("/smart/summary", h.GetSmartSummary)
	r.Get("/smart/{device}", h.GetSmartDevice)
	r.Post("/smart/scan", h.StartSmartScan)

	return r
}

// smartStatus maps the agent's health values to this API's statuses.
func smartStatus(health string) string {
	switch health {
	case "good", "healthy", "passed", "PASSED", "OK":
		return "healthy"
	case "warning", "critical":
		return health
	}
	return "unknown"
}

// GetSmartSummary returns a summary of SMART health across all devices
// GET /api/v1/health/smart/summary
// (This used to return a fixed "4 disks, 3 healthy, 1 warning" whatever the
// hardware said.)
func (h *HealthHandler) GetSmartSummary(w http.ResponseWriter, r *http.Request) {
	devices := collectSMARTDevices(r.Context())
	lastScan := time.Now().Format(time.RFC3339)
	summary := SmartSummary{TotalDevices: len(devices), LastScan: &lastScan}
	for _, d := range devices {
		switch smartStatus(d.Health) {
		case "healthy":
			summary.HealthyDevices++
		case "warning":
			summary.WarningDevices++
		case "critical":
			summary.CriticalDevices++
		}
	}
	writeJSON(w, summary)
}

// GetSmartDevice returns SMART data for a specific device
// GET /api/v1/health/smart/{device}
// (This used to return a made-up healthy disk for any device name.)
func (h *HealthHandler) GetSmartDevice(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(chi.URLParam(r, "device"), "/dev/")
	for _, d := range collectSMARTDevices(r.Context()) {
		if strings.TrimPrefix(d.Device, "/dev/") != name {
			continue
		}
		out := SmartData{Device: d.Device, Status: smartStatus(d.Health)}
		if d.Temperature > 0 {
			t := d.Temperature
			out.Temperature = &t
		}
		if d.PowerOnHours > 0 {
			p := d.PowerOnHours
			out.PowerOnHours = &p
		}
		writeJSON(w, out)
		return
	}
	httpx.WriteTypedError(w, http.StatusNotFound, "smart.device_not_found", "No such disk", 0)
}

// StartSmartScan initiates a SMART scan on all devices
// POST /api/v1/health/smart/scan
func (h *HealthHandler) StartSmartScan(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would trigger a SMART scan via nos-agent
	log.Info().Msg("Starting SMART scan on all devices")

	// Return success
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "started",
		"message": "SMART scan initiated on all devices",
	}); err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
	}
}
