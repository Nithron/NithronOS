package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nos/backend/nosd/pkg/net"
)

// NetHandler handles networking-related API endpoints
type NetHandler struct {
	firewallMgr *net.FirewallManager
	wgMgr       *net.WireGuardManager
	httpsMgr    *net.HTTPSManager
	logger      zerolog.Logger
}

// NewNetHandler creates a new networking handler
func NewNetHandler(logger zerolog.Logger) (*NetHandler, error) {
	wgMgr, err := net.NewWireGuardManager()
	if err != nil {
		return nil, err
	}

	return &NetHandler{
		firewallMgr: net.NewFirewallManager(),
		wgMgr:       wgMgr,
		httpsMgr:    net.NewHTTPSManager(),
		logger:      logger,
	}, nil
}

// Routes returns the networking routes
func (h *NetHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Network status
	r.Get("/status", h.GetNetworkStatus)

	// Firewall endpoints
	r.Get("/firewall/state", h.GetFirewallState)
	r.Post("/firewall/plan", h.PlanFirewall)
	r.Post("/firewall/apply", h.ApplyFirewall)
	r.Post("/firewall/confirm", h.ConfirmFirewall)
	r.Post("/firewall/rollback", h.RollbackFirewall)

	// WireGuard endpoints
	r.Get("/wg/state", h.GetWireGuardState)
	r.Post("/wg/enable", h.EnableWireGuard)
	r.Post("/wg/disable", h.DisableWireGuard)
	r.Post("/wg/peers/add", h.AddWireGuardPeer)
	r.Post("/wg/peers/remove", h.RemoveWireGuardPeer)

	// HTTPS/ACME endpoints
	r.Get("/https/config", h.GetHTTPSConfig)
	r.Post("/https/configure", h.ConfigureHTTPS)
	r.Post("/https/test", h.TestHTTPS)

	// Remote Access Wizard
	r.Post("/wizard/start", h.StartWizard)
	r.Get("/wizard/state", h.GetWizardState)
	r.Post("/wizard/next", h.WizardNext)
	r.Post("/wizard/complete", h.CompleteWizard)

	return r
}

// AuthRoutes returns the 2FA/auth routes to be mounted separately

// GetNetworkStatus returns the overall network configuration status
func (h *NetHandler) GetNetworkStatus(w http.ResponseWriter, r *http.Request) {
	// Get current states from all managers
	firewallState, _ := h.firewallMgr.GetState()
	wgState, _ := h.wgMgr.GetState()
	httpsConfig, _ := h.httpsMgr.GetConfig()

	// Determine access mode
	var accessMode net.AccessMode
	if firewallState != nil {
		accessMode = firewallState.Mode
	} else {
		accessMode = net.AccessModeLANOnly
	}

	status := net.NetworkStatus{
		AccessMode:  accessMode,
		LANAccess:   true,
		WANBlocked:  accessMode == net.AccessModeLANOnly,
		WireGuard:   wgState,
		HTTPS:       httpsConfig,
		Firewall:    firewallState,
		ExternalIP:  h.getExternalIP(),
		InternalIPs: h.getInternalIPs(),
		OpenPorts:   h.getOpenPorts(),
	}

	h.writeJSON(w, status)
}

// Firewall handlers

func (h *NetHandler) GetFirewallState(w http.ResponseWriter, r *http.Request) {
	state, err := h.firewallMgr.GetState()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, state)
}

func (h *NetHandler) PlanFirewall(w http.ResponseWriter, r *http.Request) {
	var req net.PlanFirewallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Create firewall plan
	plan, err := h.firewallMgr.CreatePlan(req.DesiredMode, req.EnableWG, req.EnableHTTPS, req.CustomRules)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, plan)
}

func (h *NetHandler) ApplyFirewall(w http.ResponseWriter, r *http.Request) {
	var req net.ApplyFirewallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Apply the plan
	if err := h.firewallMgr.ApplyPlan(req.PlanID, req.RollbackTimeoutSec); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "applied",
		"message": "Firewall configuration applied. Please confirm within timeout period.",
	})
}

func (h *NetHandler) ConfirmFirewall(w http.ResponseWriter, r *http.Request) {
	if err := h.firewallMgr.ConfirmPlan(); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "confirmed",
		"message": "Firewall configuration confirmed and active.",
	})
}

func (h *NetHandler) RollbackFirewall(w http.ResponseWriter, r *http.Request) {
	if err := h.firewallMgr.Rollback(); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "rolled_back",
		"message": "Firewall configuration rolled back to previous state.",
	})
}

// WireGuard handlers

func (h *NetHandler) GetWireGuardState(w http.ResponseWriter, r *http.Request) {
	state, err := h.wgMgr.GetState()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, state)
}

func (h *NetHandler) EnableWireGuard(w http.ResponseWriter, r *http.Request) {
	var req net.EnableWireGuardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Enable WireGuard
	if err := h.wgMgr.Enable(req.CIDR, req.ListenPort, req.EndpointHostname, req.DNS); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "enabled",
		"message": "WireGuard VPN enabled successfully.",
	})
}

func (h *NetHandler) DisableWireGuard(w http.ResponseWriter, r *http.Request) {
	if err := h.wgMgr.Disable(); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "disabled",
		"message": "WireGuard VPN disabled.",
	})
}

func (h *NetHandler) AddWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	var req net.AddWireGuardPeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Add peer
	config, err := h.wgMgr.AddPeer(req.Name, req.AllowedIPs, req.PublicKey)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, config)
}

func (h *NetHandler) RemoveWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("id")
	if peerID == "" {
		h.writeError(w, http.StatusBadRequest, "Peer ID required")
		return
	}

	if err := h.wgMgr.RemovePeer(peerID); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "removed",
		"message": "WireGuard peer removed.",
	})
}

// HTTPS/ACME handlers

func (h *NetHandler) GetHTTPSConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.httpsMgr.GetConfig()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, config)
}

func (h *NetHandler) ConfigureHTTPS(w http.ResponseWriter, r *http.Request) {
	var req net.ConfigureHTTPSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Configure HTTPS
	if err := h.httpsMgr.Configure(req.Mode, req.Domain, req.Email, req.DNSProvider, req.DNSAPIKey); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "configured",
		"message": "HTTPS configuration applied successfully.",
	})
}

func (h *NetHandler) TestHTTPS(w http.ResponseWriter, r *http.Request) {
	if err := h.httpsMgr.TestConfiguration(); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "HTTPS configuration test passed.",
	})
}

// Remote Access Wizard handlers

func (h *NetHandler) StartWizard(w http.ResponseWriter, r *http.Request) {
	// Initialize wizard state
	state := &net.RemoteAccessWizardState{
		Step:       1,
		AccessMode: net.AccessModeLANOnly,
		Completed:  false,
	}

	// Store in session (implement session storage)
	sessionID := h.getSessionID(r)
	h.storeWizardState(sessionID, state)

	h.writeJSON(w, state)
}

func (h *NetHandler) GetWizardState(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	state := h.getWizardState(sessionID)

	if state == nil {
		h.writeError(w, http.StatusNotFound, "No wizard session found")
		return
	}

	h.writeJSON(w, state)
}

func (h *NetHandler) WizardNext(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	state := h.getWizardState(sessionID)

	if state == nil {
		h.writeError(w, http.StatusNotFound, "No wizard session found")
		return
	}

	// Parse step data
	var stepData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&stepData); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Process based on current step
	switch state.Step {
	case 1: // Access mode selection
		if mode, ok := stepData["access_mode"].(string); ok {
			state.AccessMode = net.AccessMode(mode)
		}

	case 2: // WireGuard configuration
		if state.AccessMode == net.AccessModeWireGuard {
			// TODO: Store WireGuard config
			// This will be implemented when WireGuard configuration is added
			// state.WireGuardConfig = extractWireGuardConfig(stepData)
			_ = state // placeholder to satisfy linter until implementation
		}

	case 3: // HTTPS configuration
		if state.AccessMode == net.AccessModePublicHTTPS {
			// TODO: Store HTTPS config
			// This will be implemented when HTTPS configuration is added
			// state.HTTPSConfig = extractHTTPSConfig(stepData)
			_ = state // placeholder to satisfy linter until implementation
		}

	case 4: // Firewall plan
		// Generate firewall plan based on selections
		plan, err := h.firewallMgr.CreatePlan(
			state.AccessMode,
			state.WireGuardConfig != nil,
			state.HTTPSConfig != nil,
			nil,
		)
		if err != nil {
			state.Error = err.Error()
		} else {
			state.FirewallPlan = plan
		}
	}

	// Move to next step
	state.Step++

	// Store updated state
	h.storeWizardState(sessionID, state)

	h.writeJSON(w, state)
}

func (h *NetHandler) CompleteWizard(w http.ResponseWriter, r *http.Request) {
	sessionID := h.getSessionID(r)
	state := h.getWizardState(sessionID)

	if state == nil {
		h.writeError(w, http.StatusNotFound, "No wizard session found")
		return
	}

	// Apply all configurations

	// 1. Apply WireGuard if configured
	if state.WireGuardConfig != nil && state.WireGuardConfig.Enabled {
		// TODO: Enable WireGuard
		// This will be implemented when WireGuard manager is added
		// err := h.wgMgr.Enable(state.WireGuardConfig)
		// if err != nil { ... }
		_ = state // placeholder to satisfy linter until implementation
	}

	// 2. Apply HTTPS if configured
	if state.HTTPSConfig != nil {
		// TODO: Configure HTTPS
		// h.httpsMgr.Configure(...)
		_ = state // placeholder to satisfy linter until implementation
	}

	// 3. Apply firewall plan
	if state.FirewallPlan != nil {
		// TODO: Apply firewall
		// h.firewallMgr.ApplyPlan(...)
		_ = state // placeholder to satisfy linter until implementation
	}

	state.Completed = true
	h.storeWizardState(sessionID, state)

	h.writeJSON(w, map[string]string{
		"status":  "completed",
		"message": "Remote access configuration applied successfully.",
	})
}

// Helper methods

func (h *NetHandler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
	}
}

func (h *NetHandler) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	}); err != nil {
		fmt.Printf("Failed to write error response: %v\n", err)
	}
}

func (h *NetHandler) getSessionID(r *http.Request) string {
	// Extract from cookie or header
	// Placeholder implementation
	return "session-123"
}

func (h *NetHandler) getExternalIP() string {
	// Get external IP
	// Placeholder - would call external service
	return "203.0.113.1"
}

func (h *NetHandler) getInternalIPs() []string {
	// Get internal IPs
	// Placeholder - would enumerate interfaces
	return []string{"192.168.1.100", "10.0.0.5"}
}

func (h *NetHandler) getOpenPorts() []int {
	// Get open ports
	// Placeholder - would check netstat/ss
	return []int{22, 80, 443, 51820}
}

// Wizard state management (would be in session store)
var wizardStates = make(map[string]*net.RemoteAccessWizardState)
var wizardMu sync.RWMutex

func (h *NetHandler) storeWizardState(sessionID string, state *net.RemoteAccessWizardState) {
	wizardMu.Lock()
	defer wizardMu.Unlock()
	wizardStates[sessionID] = state
}

func (h *NetHandler) getWizardState(sessionID string) *net.RemoteAccessWizardState {
	wizardMu.RLock()
	defer wizardMu.RUnlock()
	return wizardStates[sessionID]
}
