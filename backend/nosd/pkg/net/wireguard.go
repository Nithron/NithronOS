package net

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	wgConfigPath    = "/etc/wireguard/wg0.conf"
	wgInterface     = "wg0"
	defaultWGPort   = 51820
	defaultWGSubnet = "10.8.0.0/24"
)

// WireGuardManager manages WireGuard VPN configuration
type WireGuardManager struct {
	mu         sync.RWMutex
	config     *WireGuardConfig
	configPath string
	client     *wgctrl.Client
}

// NewWireGuardManager creates a new WireGuard manager
func NewWireGuardManager() (*WireGuardManager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create WireGuard client: %w", err)
	}

	wm := &WireGuardManager{
		configPath: wgConfigPath,
		client:     client,
	}

	// Load existing config if available
	if err := wm.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return wm, nil
}

// GetState returns the current WireGuard configuration and runtime state
func (wm *WireGuardManager) GetState() (*WireGuardConfig, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if wm.config == nil {
		return &WireGuardConfig{
			Enabled: false,
		}, nil
	}

	// Update runtime stats if enabled
	if wm.config.Enabled {
		device, err := wm.client.Device(wgInterface)
		if err == nil {
			wm.updateRuntimeStats(device)
		}
	}

	return wm.config, nil
}

// Enable enables WireGuard with the specified configuration
func (wm *WireGuardManager) Enable(cidr string, port int, endpoint string, dns []string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.config != nil && wm.config.Enabled {
		return fmt.Errorf("wireguard is already enabled")
	}

	// Parse and validate CIDR
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	// Generate server keys
	privateKey, publicKey, err := GenerateWireGuardKeys()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}

	// Set defaults
	if port == 0 {
		port = defaultWGPort
	}
	if endpoint == "" {
		endpoint = getPublicIP()
	}
	if len(dns) == 0 {
		dns = []string{"1.1.1.1", "1.0.0.1"}
	}

	// Create configuration
	wm.config = &WireGuardConfig{
		Enabled:          true,
		Interface:        wgInterface,
		PrivateKey:       privateKey,
		PublicKey:        publicKey,
		ListenPort:       port,
		ServerCIDR:       cidr,
		EndpointHostname: endpoint,
		DNS:              dns,
		Peers:            []WireGuardPeer{},
	}

	// Generate and apply configuration
	if err := wm.applyConfig(); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Enable and start systemd service
	if err := wm.enableService(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Save configuration
	if err := wm.saveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Disable disables WireGuard
func (wm *WireGuardManager) Disable() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.config == nil || !wm.config.Enabled {
		return fmt.Errorf("wireguard is not enabled")
	}

	// Stop and disable systemd service
	if err := wm.disableService(); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	// Update configuration
	wm.config.Enabled = false

	// Save configuration
	if err := wm.saveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// AddPeer adds a new WireGuard peer
func (wm *WireGuardManager) AddPeer(name string, allowedIPs []string, pubkey string) (*WireGuardPeerConfig, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.config == nil || !wm.config.Enabled {
		return nil, fmt.Errorf("wireguard is not enabled")
	}

	// Check for duplicate name
	for _, peer := range wm.config.Peers {
		if peer.Name == name {
			return nil, fmt.Errorf("peer with name %s already exists", name)
		}
	}

	// Generate peer keys if not provided
	var peerPrivateKey, peerPublicKey string
	if pubkey == "" {
		var err error
		peerPrivateKey, peerPublicKey, err = GenerateWireGuardKeys()
		if err != nil {
			return nil, fmt.Errorf("failed to generate peer keys: %w", err)
		}
	} else {
		peerPublicKey = pubkey
	}

	// Generate preshared key for additional security
	presharedKey, err := generatePresharedKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate preshared key: %w", err)
	}

	// Allocate IP address for peer
	peerIP, err := wm.allocatePeerIP()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Set default allowed IPs if not specified
	if len(allowedIPs) == 0 {
		allowedIPs = []string{fmt.Sprintf("%s/32", peerIP)}
	}

	// Create peer
	peer := WireGuardPeer{
		ID:           generateID(),
		Name:         name,
		PublicKey:    peerPublicKey,
		PresharedKey: presharedKey,
		AllowedIPs:   allowedIPs,
		CreatedAt:    time.Now(),
		Enabled:      true,
	}

	// Add to configuration
	wm.config.Peers = append(wm.config.Peers, peer)

	// Apply configuration
	if err := wm.applyConfig(); err != nil {
		return nil, fmt.Errorf("failed to apply config: %w", err)
	}

	// Save configuration
	if err := wm.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Generate client configuration
	clientConfig := wm.generateClientConfig(peer, peerPrivateKey, peerIP)

	// Generate QR code
	qrCode, err := wm.generateQRCode(clientConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	clientConfig.QRCode = qrCode

	return clientConfig, nil
}

// RemovePeer removes a WireGuard peer
func (wm *WireGuardManager) RemovePeer(peerID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.config == nil || !wm.config.Enabled {
		return fmt.Errorf("wireguard is not enabled")
	}

	// Find and remove peer
	found := false
	var updatedPeers []WireGuardPeer
	for _, peer := range wm.config.Peers {
		if peer.ID != peerID {
			updatedPeers = append(updatedPeers, peer)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("peer not found")
	}

	// Update configuration
	wm.config.Peers = updatedPeers

	// Apply configuration
	if err := wm.applyConfig(); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Save configuration
	if err := wm.saveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Private methods

func (wm *WireGuardManager) loadConfig() error {
	// Load configuration from disk
	// This would parse the saved JSON config
	return nil
}

func (wm *WireGuardManager) saveConfig() error {
	// Save configuration to disk as JSON
	// Store in /var/lib/nos/net/wireguard.json
	return nil
}

func (wm *WireGuardManager) applyConfig() error {
	// Generate wg0.conf from configuration
	configContent := wm.generateWireGuardConfig()

	// Ensure directory exists
	configDir := filepath.Dir(wm.configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write configuration file
	if err := os.WriteFile(wm.configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Reload WireGuard if running
	if wm.isServiceRunning() {
		cmd := exec.Command("systemctl", "reload", "wg-quick@wg0")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to reload WireGuard: %w", err)
		}
	}

	return nil
}

func (wm *WireGuardManager) generateWireGuardConfig() string {
	var buf bytes.Buffer

	// Interface section
	buf.WriteString("[Interface]\n")
	fmt.Fprintf(&buf, "PrivateKey = %s\n", wm.config.PrivateKey)
	fmt.Fprintf(&buf, "Address = %s\n", wm.config.ServerCIDR)
	fmt.Fprintf(&buf, "ListenPort = %d\n", wm.config.ListenPort)
	buf.WriteString("PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE\n")
	buf.WriteString("PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE\n")
	buf.WriteString("\n")

	// Peer sections
	for _, peer := range wm.config.Peers {
		if !peer.Enabled {
			continue
		}

		fmt.Fprintf(&buf, "# %s\n", peer.Name)
		buf.WriteString("[Peer]\n")
		fmt.Fprintf(&buf, "PublicKey = %s\n", peer.PublicKey)
		if peer.PresharedKey != "" {
			fmt.Fprintf(&buf, "PresharedKey = %s\n", peer.PresharedKey)
		}
		fmt.Fprintf(&buf, "AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", "))
		buf.WriteString("\n")
	}

	return buf.String()
}

func (wm *WireGuardManager) generateClientConfig(peer WireGuardPeer, privateKey, clientIP string) *WireGuardPeerConfig {
	config := &WireGuardPeerConfig{}

	// Set interface configuration
	config.Interface.PrivateKey = privateKey
	config.Interface.Address = fmt.Sprintf("%s/32", clientIP)
	config.Interface.DNS = wm.config.DNS

	// Set peer (server) configuration
	config.Peer.PublicKey = wm.config.PublicKey
	config.Peer.PresharedKey = peer.PresharedKey
	config.Peer.Endpoint = fmt.Sprintf("%s:%d", wm.config.EndpointHostname, wm.config.ListenPort)
	config.Peer.AllowedIPs = []string{"0.0.0.0/0", "::/0"} // Route all traffic through VPN
	config.Peer.PersistentKeepalive = 25

	// Generate config file content
	var buf bytes.Buffer
	buf.WriteString("[Interface]\n")
	fmt.Fprintf(&buf, "PrivateKey = %s\n", privateKey)
	fmt.Fprintf(&buf, "Address = %s/32\n", clientIP)
	fmt.Fprintf(&buf, "DNS = %s\n", strings.Join(wm.config.DNS, ", "))
	buf.WriteString("\n[Peer]\n")
	fmt.Fprintf(&buf, "PublicKey = %s\n", wm.config.PublicKey)
	fmt.Fprintf(&buf, "PresharedKey = %s\n", peer.PresharedKey)
	fmt.Fprintf(&buf, "Endpoint = %s:%d\n", wm.config.EndpointHostname, wm.config.ListenPort)
	buf.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	buf.WriteString("PersistentKeepalive = 25\n")

	config.Config = buf.String()

	return config
}

func (wm *WireGuardManager) generateQRCode(content string) (string, error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}

	// Generate PNG image
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}

	// Encode as base64
	encoded := base64.StdEncoding.EncodeToString(png)
	return fmt.Sprintf("data:image/png;base64,%s", encoded), nil
}

func (wm *WireGuardManager) allocatePeerIP() (string, error) {
	// Parse server CIDR
	ip, network, err := net.ParseCIDR(wm.config.ServerCIDR)
	if err != nil {
		return "", err
	}

	// Get all used IPs
	usedIPs := make(map[string]bool)
	usedIPs[ip.String()] = true // Server IP

	for _, peer := range wm.config.Peers {
		for _, allowedIP := range peer.AllowedIPs {
			if strings.Contains(allowedIP, "/32") {
				ip := strings.TrimSuffix(allowedIP, "/32")
				usedIPs[ip] = true
			}
		}
	}

	// Find next available IP
	for ip := ip.Mask(network.Mask); network.Contains(ip); incrementIP(ip) {
		if !usedIPs[ip.String()] && !ip.Equal(network.IP) {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet")
}

func (wm *WireGuardManager) updateRuntimeStats(device *wgtypes.Device) {
	// Update peer statistics
	for i, peer := range wm.config.Peers {
		for _, devicePeer := range device.Peers {
			if peer.PublicKey == devicePeer.PublicKey.String() {
				if !devicePeer.LastHandshakeTime.IsZero() {
					handshake := devicePeer.LastHandshakeTime
					wm.config.Peers[i].LastHandshake = &handshake
				}
				wm.config.Peers[i].BytesRX = devicePeer.ReceiveBytes
				wm.config.Peers[i].BytesTX = devicePeer.TransmitBytes
				break
			}
		}
	}
}

func (wm *WireGuardManager) enableService() error {
	// Enable and start wg-quick@wg0 service
	cmd := exec.Command("systemctl", "enable", "--now", "wg-quick@wg0")
	return cmd.Run()
}

func (wm *WireGuardManager) disableService() error {
	// Stop and disable wg-quick@wg0 service
	cmd := exec.Command("systemctl", "disable", "--now", "wg-quick@wg0")
	return cmd.Run()
}

func (wm *WireGuardManager) isServiceRunning() bool {
	cmd := exec.Command("systemctl", "is-active", "wg-quick@wg0")
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "active"
}

// Helper functions

func generatePresharedKey() (string, error) {
	cmd := exec.Command("wg", "genpsk")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate preshared key: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func getPublicIP() string {
	// Try to get public IP from external service
	cmd := exec.Command("curl", "-s", "https://api.ipify.org")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// Fallback to hostname
	hostname, _ := os.Hostname()
	return hostname
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
