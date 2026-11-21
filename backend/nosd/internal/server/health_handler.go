package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nithronos/backend/nosd/internal/version"
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

// GetSmartSummary returns a summary of SMART health across all devices
// GET /api/v1/health/smart/summary
func (h *HealthHandler) GetSmartSummary(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would query the stored SMART data
	// For now, return a sample response

	lastScan := time.Now().Format(time.RFC3339)
	summary := SmartSummary{
		TotalDevices:    4,
		HealthyDevices:  3,
		WarningDevices:  1,
		CriticalDevices: 0,
		LastScan:        &lastScan,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		log.Error().Err(err).Msg("Failed to encode SMART summary")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetSmartDevice returns SMART data for a specific device
// GET /api/v1/health/smart/{device}
func (h *HealthHandler) GetSmartDevice(w http.ResponseWriter, r *http.Request) {
	device := chi.URLParam(r, "device")

	// In a real implementation, this would query smartctl via nos-agent
	// For now, return sample data

	temp := 35
	powerOn := 8760
	data := SmartData{
		Device:       device,
		Status:       "healthy",
		Temperature:  &temp,
		PowerOnHours: &powerOn,
		Attributes: []SmartAttribute{
			{
				ID:        5,
				Name:      "Reallocated_Sector_Ct",
				Value:     100,
				Worst:     100,
				Threshold: 50,
				RawValue:  "0",
				Status:    "ok",
			},
			{
				ID:        9,
				Name:      "Power_On_Hours",
				Value:     99,
				Worst:     99,
				Threshold: 0,
				RawValue:  "8760",
				Status:    "ok",
			},
			{
				ID:        194,
				Name:      "Temperature_Celsius",
				Value:     65,
				Worst:     45,
				Threshold: 0,
				RawValue:  "35 (Min/Max 20/45)",
				Status:    "ok",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode SMART data")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
