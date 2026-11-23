package server

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// NetworkOverview represents network system overview
type NetworkOverview struct {
	Hostname   string                 `json:"hostname"`
	Interfaces []NetworkInterfaceInfo `json:"interfaces"`
	Routes     []Route                `json:"routes"`
	DNS        DNSConfig              `json:"dns"`
	Firewall   FirewallStatus         `json:"firewall"`
	WireGuard  WireGuardStatus        `json:"wireguard"`
	HTTPS      HTTPSStatus            `json:"https"`
}

// NetworkInterfaceInfo represents a network interface
type NetworkInterfaceInfo struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`   // ethernet, wifi, bridge, virtual
	Status    string   `json:"status"` // up, down
	IPv4      []string `json:"ipv4"`
	IPv6      []string `json:"ipv6"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Speed     int      `json:"speed"` // Mbps
	RxBytes   int64    `json:"rx_bytes"`
	TxBytes   int64    `json:"tx_bytes"`
	RxPackets int64    `json:"rx_packets"`
	TxPackets int64    `json:"tx_packets"`
	RxErrors  int64    `json:"rx_errors"`
	TxErrors  int64    `json:"tx_errors"`
}

// Route represents a network route
type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
	Flags       string `json:"flags"`
}

// DNSConfig represents DNS configuration
type DNSConfig struct {
	Servers []string `json:"servers"`
	Search  []string `json:"search"`
	Domain  string   `json:"domain"`
}

// FirewallStatus represents firewall status
type FirewallStatus struct {
	Enabled bool                    `json:"enabled"`
	Mode    string                  `json:"mode"` // strict, permissive
	Rules   int                     `json:"rules_count"`
	Zones   map[string]FirewallZone `json:"zones"`
}

// FirewallZone represents a firewall zone
type FirewallZone struct {
	Name       string   `json:"name"`
	Interfaces []string `json:"interfaces"`
	Policy     string   `json:"policy"` // accept, drop, reject
	Services   []string `json:"services"`
	Ports      []string `json:"ports"`
}

// FirewallRule represents a firewall rule
type FirewallRule struct {
	ID          string `json:"id"`
	Priority    int    `json:"priority"`
	Direction   string `json:"direction"` // inbound, outbound
	Action      string `json:"action"`    // allow, deny, reject
	Protocol    string `json:"protocol"`  // tcp, udp, icmp, any
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Port        string `json:"port"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// WireGuardStatus represents WireGuard VPN status
type WireGuardStatus struct {
	Enabled    bool          `json:"enabled"`
	Interfaces []WGInterface `json:"interfaces"`
	Peers      []WGPeer      `json:"peers"`
}

// WGInterface represents a WireGuard interface
type WGInterface struct {
	Name       string   `json:"name"`
	PublicKey  string   `json:"public_key"`
	ListenPort int      `json:"listen_port"`
	Addresses  []string `json:"addresses"`
}

// WGPeer represents a WireGuard peer
type WGPeer struct {
	Name          string    `json:"name"`
	PublicKey     string    `json:"public_key"`
	Endpoint      string    `json:"endpoint"`
	AllowedIPs    []string  `json:"allowed_ips"`
	LastHandshake time.Time `json:"last_handshake"`
	TransferRx    int64     `json:"transfer_rx"`
	TransferTx    int64     `json:"transfer_tx"`
}

// HTTPSStatus represents HTTPS/TLS configuration status
type HTTPSStatus struct {
	Enabled     bool     `json:"enabled"`
	Certificate CertInfo `json:"certificate"`
	Domains     []string `json:"domains"`
	AutoRenew   bool     `json:"auto_renew"`
	Provider    string   `json:"provider"` // letsencrypt, self-signed, custom
}

// CertInfo represents certificate information
type CertInfo struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	DaysLeft  int       `json:"days_left"`
}

// NetworkConfigHandler handles network configuration
type NetworkConfigHandler struct {
	config     config.Config
	configPath string
}

// NewNetworkConfigHandler creates a new network config handler
func NewNetworkConfigHandler(cfg config.Config) *NetworkConfigHandler {
	return &NetworkConfigHandler{
		config:     cfg,
		configPath: filepath.Join(cfg.EtcDir, "nos", "network-config.json"),
	}
}

// GetNetworkOverview returns network system overview
func (h *NetworkConfigHandler) GetNetworkOverview(w http.ResponseWriter, r *http.Request) {
	overview := NetworkOverview{
		Hostname:   h.getHostname(),
		Interfaces: h.getInterfaces(),
		Routes:     h.getRoutes(),
		DNS:        h.getDNSConfig(),
		Firewall:   h.getFirewallStatus(),
		WireGuard:  h.getWireGuardStatus(),
		HTTPS:      h.getHTTPSStatus(),
	}

	writeJSON(w, overview)
}

// Firewall endpoints

// GetFirewallRules returns all firewall rules
func (h *NetworkConfigHandler) GetFirewallRules(w http.ResponseWriter, r *http.Request) {
	rules := h.loadFirewallRules()
	writeJSON(w, rules)
}

// CreateFirewallRule creates a new firewall rule
func (h *NetworkConfigHandler) CreateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var rule FirewallRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "firewall.invalid_rule", "Invalid rule format", 0)
		return
	}

	rule.ID = generateUUID()

	rules := h.loadFirewallRules()
	rules = append(rules, rule)

	if err := h.saveFirewallRules(rules); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.save_failed", "Failed to save rule", 0)
		return
	}

	// Apply firewall rules
	if err := h.applyFirewallRules(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.apply_failed", "Failed to apply rules", 0)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, rule)
}

// UpdateFirewallRule updates an existing firewall rule
func (h *NetworkConfigHandler) UpdateFirewallRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "id")

	var updatedRule FirewallRule
	if err := json.NewDecoder(r.Body).Decode(&updatedRule); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "firewall.invalid_rule", "Invalid rule format", 0)
		return
	}

	rules := h.loadFirewallRules()
	found := false
	for i, rule := range rules {
		if rule.ID == ruleID {
			updatedRule.ID = ruleID
			rules[i] = updatedRule
			found = true
			break
		}
	}

	if !found {
		httpx.WriteTypedError(w, http.StatusNotFound, "firewall.rule_not_found", "Rule not found", 0)
		return
	}

	if err := h.saveFirewallRules(rules); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.save_failed", "Failed to save rule", 0)
		return
	}

	// Apply firewall rules
	if err := h.applyFirewallRules(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.apply_failed", "Failed to apply rules", 0)
		return
	}

	writeJSON(w, updatedRule)
}

// DeleteFirewallRule deletes a firewall rule
func (h *NetworkConfigHandler) DeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "id")

	rules := h.loadFirewallRules()
	newRules := []FirewallRule{}
	found := false

	for _, rule := range rules {
		if rule.ID != ruleID {
			newRules = append(newRules, rule)
		} else {
			found = true
		}
	}

	if !found {
		httpx.WriteTypedError(w, http.StatusNotFound, "firewall.rule_not_found", "Rule not found", 0)
		return
	}

	if err := h.saveFirewallRules(newRules); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.save_failed", "Failed to save rules", 0)
		return
	}

	// Apply firewall rules
	if err := h.applyFirewallRules(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "firewall.apply_failed", "Failed to apply rules", 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// WireGuard endpoints

// GetWireGuardConfig returns WireGuard configuration
func (h *NetworkConfigHandler) GetWireGuardConfig(w http.ResponseWriter, r *http.Request) {
	config := h.loadWireGuardConfig()
	writeJSON(w, config)
}

// CreateWireGuardPeer adds a new WireGuard peer
func (h *NetworkConfigHandler) CreateWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	var peer WGPeer
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "wg.invalid_peer", "Invalid peer configuration", 0)
		return
	}

	// Generate keys if not provided
	if peer.PublicKey == "" {
		privateKey, publicKey, err := h.generateWGKeys()
		if err != nil {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "wg.keygen_failed", "Could not generate WireGuard keys", 0)
			return
		}
		peer.PublicKey = publicKey
		// Store private key securely
		h.storeWGPrivateKey(peer.Name, privateKey)
	}

	config := h.loadWireGuardConfig()
	config.Peers = append(config.Peers, peer)

	if err := h.saveWireGuardConfig(config); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "wg.save_failed", "Failed to save configuration", 0)
		return
	}

	// Apply WireGuard configuration
	if err := h.applyWireGuardConfig(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "wg.apply_failed", "Failed to apply configuration", 0)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, peer)
}

// HTTPS/TLS endpoints

// GetHTTPSConfig returns HTTPS configuration
func (h *NetworkConfigHandler) GetHTTPSConfig(w http.ResponseWriter, r *http.Request) {
	config := h.loadHTTPSConfig()
	writeJSON(w, config)
}

// UpdateHTTPSConfig updates HTTPS configuration
func (h *NetworkConfigHandler) UpdateHTTPSConfig(w http.ResponseWriter, r *http.Request) {
	var config HTTPSConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "https.invalid_config", "Invalid configuration", 0)
		return
	}

	if err := h.saveHTTPSConfig(config); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "https.save_failed", "Failed to save configuration", 0)
		return
	}

	// Apply HTTPS configuration
	if err := h.applyHTTPSConfig(config); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "https.apply_failed", "Failed to apply configuration", 0)
		return
	}

	writeJSON(w, config)
}

// Helper methods

func (h *NetworkConfigHandler) getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func (h *NetworkConfigHandler) getInterfaces() []NetworkInterfaceInfo {
	interfaces := []NetworkInterfaceInfo{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return interfaces
	}

	for _, iface := range ifaces {
		ni := NetworkInterfaceInfo{
			Name: iface.Name,
			MAC:  iface.HardwareAddr.String(),
			MTU:  iface.MTU,
		}

		// Get status
		if iface.Flags&net.FlagUp != 0 {
			ni.Status = "up"
		} else {
			ni.Status = "down"
		}

		// Get type
		if iface.Flags&net.FlagLoopback != 0 {
			ni.Type = "loopback"
		} else if strings.HasPrefix(iface.Name, "eth") || strings.HasPrefix(iface.Name, "en") {
			ni.Type = "ethernet"
		} else if strings.HasPrefix(iface.Name, "wl") {
			ni.Type = "wifi"
		} else if strings.HasPrefix(iface.Name, "br") {
			ni.Type = "bridge"
		} else {
			ni.Type = "virtual"
		}

		// Get addresses
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					ni.IPv4 = append(ni.IPv4, addr.String())
				} else {
					ni.IPv6 = append(ni.IPv6, addr.String())
				}
			}
		}

		// Get statistics (Linux-specific)
		if runtime.GOOS == "linux" {
			h.getInterfaceStats(&ni)
		}

		interfaces = append(interfaces, ni)
	}

	return interfaces
}

func (h *NetworkConfigHandler) getInterfaceStats(ni *NetworkInterfaceInfo) {
	// Read from /sys/class/net/<interface>/statistics/
	basePath := fmt.Sprintf("/sys/class/net/%s/statistics", ni.Name)

	if data, err := os.ReadFile(filepath.Join(basePath, "rx_bytes")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.RxBytes)
	}
	if data, err := os.ReadFile(filepath.Join(basePath, "tx_bytes")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.TxBytes)
	}
	if data, err := os.ReadFile(filepath.Join(basePath, "rx_packets")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.RxPackets)
	}
	if data, err := os.ReadFile(filepath.Join(basePath, "tx_packets")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.TxPackets)
	}
	if data, err := os.ReadFile(filepath.Join(basePath, "rx_errors")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.RxErrors)
	}
	if data, err := os.ReadFile(filepath.Join(basePath, "tx_errors")); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.TxErrors)
	}

	// Get speed
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", ni.Name)
	if data, err := os.ReadFile(speedPath); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &ni.Speed)
	}
}

func (h *NetworkConfigHandler) getRoutes() []Route {
	routes := []Route{}

	if runtime.GOOS != "linux" {
		return routes
	}

	// Parse /proc/net/route
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return routes
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || line == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		route := Route{
			Interface: fields[0],
		}

		// Parse destination and gateway (hex to IP)
		if dest, err := hexToIP(fields[1]); err == nil {
			route.Destination = dest
		}
		if gw, err := hexToIP(fields[2]); err == nil {
			route.Gateway = gw
		}

		// Parse metric
		_, _ = fmt.Sscanf(fields[6], "%d", &route.Metric)

		routes = append(routes, route)
	}

	return routes
}

func hexToIP(hex string) (string, error) {
	if len(hex) != 8 {
		return "", fmt.Errorf("invalid hex IP")
	}

	var ip net.IP = make([]byte, 4)
	for i := 0; i < 4; i++ {
		var b byte
		_, _ = fmt.Sscanf(hex[i*2:i*2+2], "%02x", &b)
		ip[3-i] = b // Little-endian
	}

	return ip.String(), nil
}

func (h *NetworkConfigHandler) getDNSConfig() DNSConfig {
	config := DNSConfig{
		Servers: []string{},
		Search:  []string{},
	}

	// Parse /etc/resolv.conf
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return config
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "nameserver":
			config.Servers = append(config.Servers, fields[1])
		case "search":
			config.Search = fields[1:]
		case "domain":
			config.Domain = fields[1]
		}
	}

	return config
}

func (h *NetworkConfigHandler) getFirewallStatus() FirewallStatus {
	status := FirewallStatus{
		Enabled: false,
		Mode:    "permissive",
		Zones:   make(map[string]FirewallZone),
	}

	if runtime.GOOS == "linux" {
		// Check if firewall is enabled
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check nftables
		if cmd := exec.CommandContext(ctx, "nft", "list", "ruleset"); cmd.Run() == nil {
			status.Enabled = true
			// Count rules
			if output, err := cmd.Output(); err == nil {
				status.Rules = strings.Count(string(output), "\n")
			}
		}
	}

	return status
}

func (h *NetworkConfigHandler) getWireGuardStatus() WireGuardStatus {
	status := WireGuardStatus{
		Enabled:    false,
		Interfaces: []WGInterface{},
		Peers:      []WGPeer{},
	}

	if runtime.GOOS == "linux" {
		// Check if WireGuard is available
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "wg", "show", "all")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			status.Enabled = true
			// Parse WireGuard output
			// This is simplified - real implementation would parse the output properly
		}
	}

	return status
}

func (h *NetworkConfigHandler) getHTTPSStatus() HTTPSStatus {
	status := HTTPSStatus{
		Enabled:   false,
		AutoRenew: false,
		Provider:  "none",
		Domains:   []string{},
	}

	// Check for certificate files
	certPath := filepath.Join(h.config.EtcDir, "nos", "tls", "cert.pem")
	if _, err := os.Stat(certPath); err == nil {
		status.Enabled = true
		status.Provider = "custom"

		// Parse certificate for details
		// This is simplified - real implementation would parse the certificate
		status.Certificate = CertInfo{
			Subject:   "NithronOS",
			Issuer:    "Self-Signed",
			NotBefore: time.Now().AddDate(0, -1, 0),
			NotAfter:  time.Now().AddDate(1, 0, 0),
			DaysLeft:  365,
		}
	}

	return status
}

// Configuration persistence

func (h *NetworkConfigHandler) loadFirewallRules() []FirewallRule {
	rulesFile := filepath.Join(h.config.EtcDir, "nos", "firewall-rules.json")
	var rules []FirewallRule

	if data, err := os.ReadFile(rulesFile); err == nil {
		_ = json.Unmarshal(data, &rules)
	}

	if rules == nil {
		rules = []FirewallRule{}
	}

	return rules
}

func (h *NetworkConfigHandler) saveFirewallRules(rules []FirewallRule) error {
	rulesFile := filepath.Join(h.config.EtcDir, "nos", "firewall-rules.json")
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(rulesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(rulesFile, data, 0644)
}

func (h *NetworkConfigHandler) applyFirewallRules() error {
	// Apply firewall rules using nftables or iptables
	// This is a simplified implementation
	return nil
}

func (h *NetworkConfigHandler) loadWireGuardConfig() WireGuardStatus {
	configFile := filepath.Join(h.config.EtcDir, "nos", "wireguard-config.json")
	var config WireGuardStatus

	if data, err := os.ReadFile(configFile); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	return config
}

func (h *NetworkConfigHandler) saveWireGuardConfig(config WireGuardStatus) error {
	configFile := filepath.Join(h.config.EtcDir, "nos", "wireguard-config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

func (h *NetworkConfigHandler) applyWireGuardConfig() error {
	// Apply WireGuard configuration
	// This is a simplified implementation
	return nil
}

// generateWGKeys returns a new WireGuard key pair (base64). It used to return
// fixed placeholder strings, which every peer would have shared.
func (h *NetworkConfigHandler) generateWGKeys() (privateKey, publicKey string, err error) {
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("generate wireguard key: %w", err)
	}
	return k.String(), k.PublicKey().String(), nil
}

func (h *NetworkConfigHandler) storeWGPrivateKey(name, key string) {
	// Store private key securely
	// This is a simplified implementation
}

// HTTPSConfig represents HTTPS configuration
type HTTPSConfig struct {
	Enabled   bool     `json:"enabled"`
	Domains   []string `json:"domains"`
	Provider  string   `json:"provider"`
	AutoRenew bool     `json:"auto_renew"`
	Email     string   `json:"email"`
}

func (h *NetworkConfigHandler) loadHTTPSConfig() HTTPSConfig {
	configFile := filepath.Join(h.config.EtcDir, "nos", "https-config.json")
	var config HTTPSConfig

	if data, err := os.ReadFile(configFile); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	return config
}

func (h *NetworkConfigHandler) saveHTTPSConfig(config HTTPSConfig) error {
	configFile := filepath.Join(h.config.EtcDir, "nos", "https-config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

func (h *NetworkConfigHandler) applyHTTPSConfig(config HTTPSConfig) error {
	// Apply HTTPS configuration
	// This would integrate with Caddy or nginx
	return nil
}

// Routes returns the routes for the network config handler
func (h *NetworkConfigHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Network overview
	r.Get("/overview", h.GetNetworkOverview)

	// Firewall management
	r.Get("/firewall/rules", h.GetFirewallRules)
	r.Post("/firewall/rules", h.CreateFirewallRule)
	r.Put("/firewall/rules/{id}", h.UpdateFirewallRule)
	r.Delete("/firewall/rules/{id}", h.DeleteFirewallRule)

	// WireGuard VPN
	r.Get("/wireguard/config", h.GetWireGuardConfig)
	r.Post("/wireguard/peers", h.CreateWireGuardPeer)

	// HTTPS/TLS configuration
	r.Get("/https/config", h.GetHTTPSConfig)
	r.Put("/https/config", h.UpdateHTTPSConfig)

	return r
}
