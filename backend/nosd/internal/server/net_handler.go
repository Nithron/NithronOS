package server

import (
	"context"
	"encoding/json"
	"fmt"
	stdnet "net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
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
func NewNetHandler(logger zerolog.Logger, agent AgentClient) (*NetHandler, error) {
	wgMgr, err := net.NewWireGuardManager()
	if err != nil {
		return nil, err
	}
	fw := net.NewFirewallManager()
	fw.SetNFTables(agentNFTables{agent: agent})

	return &NetHandler{
		firewallMgr: fw,
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
			state.WireGuardConfig = wizardWireGuard(stepData)
		}

	case 3: // HTTPS configuration
		if state.AccessMode == net.AccessModePublicHTTPS {
			state.HTTPSConfig = wizardHTTPS(stepData)
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

	// Apply all configurations, stopping at the first failure.
	fail := func(step string, err error) {
		state.Error = step + ": " + err.Error()
		h.storeWizardState(sessionID, state)
		h.writeError(w, http.StatusBadGateway, state.Error)
	}
	if wg := state.WireGuardConfig; wg != nil && wg.Enabled {
		if err := h.wgMgr.Enable(wg.ServerCIDR, wg.ListenPort, wg.EndpointHostname, wg.DNS); err != nil {
			fail("WireGuard", err)
			return
		}
	}
	if hc := state.HTTPSConfig; hc != nil {
		if err := h.httpsMgr.Configure(hc.Mode, hc.Domain, hc.Email, hc.DNSProvider, hc.DNSAPIKey); err != nil {
			fail("HTTPS", err)
			return
		}
	}
	if state.FirewallPlan != nil {
		// Applied with the usual confirm-or-roll-back window.
		if err := h.firewallMgr.ApplyPlan(state.FirewallPlan.ID, 120); err != nil {
			fail("firewall", err)
			return
		}
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

// getSessionID keys the wizard by the signed-in user. (It returned the
// constant "session-123", so every user shared one wizard.)
func (h *NetHandler) getSessionID(r *http.Request) string {
	uid, _ := signedInUID(r)
	return "user-" + uid
}

// getExternalIP is unknown: nOS doesn't ask an outside service for it (it
// returned a made-up documentation address).
func (h *NetHandler) getExternalIP() string { return "" }

// getInternalIPs lists the addresses of the up, non-loopback interfaces.
func (h *NetHandler) getInternalIPs() []string {
	out := []string{}
	ifaces, err := stdnet.Interfaces()
	if err != nil {
		return out
	}
	for _, ifc := range ifaces {
		if ifc.Flags&stdnet.FlagUp == 0 || ifc.Flags&stdnet.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*stdnet.IPNet); ok && !ipn.IP.IsLinkLocalUnicast() {
				out = append(out, ipn.IP.String())
			}
		}
	}
	return out
}

// getOpenPorts lists the TCP ports something listens on (/proc/net/tcp*).
func (h *NetHandler) getOpenPorts() []int {
	return listeningPorts("/proc/net/tcp", "/proc/net/tcp6")
}

func listeningPorts(files ...string) []int {
	seen := map[int]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n")[1:] {
			fields := strings.Fields(line)
			if len(fields) < 4 || fields[3] != "0A" { // 0A = LISTEN
				continue
			}
			if i := strings.LastIndex(fields[1], ":"); i >= 0 {
				if p, err := strconv.ParseInt(fields[1][i+1:], 16, 32); err == nil {
					seen[int(p)] = true
				}
			}
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

// wizardWireGuard reads the wizard's WireGuard step.
func wizardWireGuard(d map[string]interface{}) *net.WireGuardConfig {
	c := &net.WireGuardConfig{Enabled: true, ListenPort: 51820, ServerCIDR: "10.8.0.1/24"}
	if v, ok := d["server_cidr"].(string); ok && v != "" {
		c.ServerCIDR = v
	}
	if v, ok := d["listen_port"].(float64); ok && v > 0 {
		c.ListenPort = int(v)
	}
	if v, ok := d["endpoint_hostname"].(string); ok {
		c.EndpointHostname = v
	}
	if list, ok := d["dns"].([]interface{}); ok {
		for _, x := range list {
			if s, ok := x.(string); ok {
				c.DNS = append(c.DNS, s)
			}
		}
	}
	return c
}

// wizardHTTPS reads the wizard's HTTPS step.
func wizardHTTPS(d map[string]interface{}) *net.HTTPSConfig {
	str := func(k string) string { v, _ := d[k].(string); return v }
	return &net.HTTPSConfig{Mode: net.HTTPSMode(str("mode")), Domain: str("domain"), Email: str("email"),
		DNSProvider: str("dns_provider"), DNSAPIKey: str("dns_api_key")}
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

// agentNFTables reads and loads rulesets through nos-agent (it checks a
// ruleset with nft -c and keeps a backup before loading it).
type agentNFTables struct{ agent AgentClient }

func (a agentNFTables) List(json bool) ([]byte, error) {
	path := "/v1/firewall/ruleset"
	if json {
		path += "?json=1"
	}
	var out struct {
		Ruleset string `json:"ruleset"`
	}
	if err := a.agent.GetJSON(context.Background(), path, &out); err != nil {
		return nil, err
	}
	return []byte(out.Ruleset), nil
}

func (a agentNFTables) Apply(script string, persist bool) error {
	return a.agent.PostJSON(context.Background(), "/v1/firewall/apply", map[string]any{"ruleset_text": script, "persist": persist}, nil)
}
