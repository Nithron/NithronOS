package net

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	wgInterface   = "wg0"
	defaultWGPort = 51820
)

// ErrEndpointRequired: clients need an address to reach this system at,
// and nOS doesn't ask an outside service for the public one.
var ErrEndpointRequired = errors.New("enter the address (host name or public IP) devices reach this system at")

// AgentJSON is the part of the nos-agent client the network managers use.
type AgentJSON interface {
	GetJSON(ctx context.Context, path string, out any) error
	PostJSON(ctx context.Context, path string, body any, out any) error
}

// WireGuardManager keeps the tunnel's settings and peers (in statePath) and
// has nos-agent apply them: nosd runs as the nos user, which can neither
// write /etc/wireguard nor run systemctl or read the tunnel's statistics.
// The server's private key never leaves the agent. (The settings used to be
// kept only in memory, so a restart forgot the key and every peer, and an
// empty endpoint was filled in by asking api.ipify.org.)
type WireGuardManager struct {
	mu        sync.RWMutex
	config    *WireGuardConfig
	psk       map[string]string // peer id -> preshared key (not in the API)
	statePath string
	agent     AgentJSON
}

// wgState is what statePath holds.
type wgState struct {
	Config *WireGuardConfig  `json:"config"`
	PSK    map[string]string `json:"psk"`
}

// NewWireGuardManager loads the saved tunnel, if any.
func NewWireGuardManager(statePath string, agent AgentJSON) (*WireGuardManager, error) {
	wm := &WireGuardManager{statePath: statePath, agent: agent, psk: map[string]string{}}
	data, err := os.ReadFile(statePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", statePath, err)
	}
	if err == nil {
		var st wgState
		if err := json.Unmarshal(data, &st); err != nil {
			return nil, fmt.Errorf("parse %s: %w", statePath, err)
		}
		wm.config = st.Config
		if st.PSK != nil {
			wm.psk = st.PSK
		}
	}
	return wm, nil
}

func (wm *WireGuardManager) save() error {
	data, err := json.MarshalIndent(wgState{Config: wm.config, PSK: wm.psk}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(wm.statePath), 0o700); err != nil {
		return err
	}
	tmp := wm.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, wm.statePath)
}

// apply sends the tunnel to the agent and records the server's public key.
func (wm *WireGuardManager) apply(ctx context.Context) error {
	req := map[string]any{"enabled": wm.config.Enabled}
	if wm.config.Enabled {
		peers := []map[string]any{}
		for _, p := range wm.config.Peers {
			if !p.Enabled {
				continue
			}
			peers = append(peers, map[string]any{"public_key": p.PublicKey, "preshared_key": wm.psk[p.ID], "allowed_ips": p.AllowedIPs})
		}
		req["listen_port"] = wm.config.ListenPort
		req["address"] = wm.config.ServerCIDR
		req["peers"] = peers
	}
	var out struct {
		PublicKey string `json:"public_key"`
	}
	if err := wm.agent.PostJSON(ctx, "/v1/wireguard/apply", req, &out); err != nil {
		return err
	}
	if out.PublicKey != "" {
		wm.config.PublicKey = out.PublicKey
	}
	return nil
}

// GetState returns the tunnel with each peer's live statistics.
func (wm *WireGuardManager) GetState() (*WireGuardConfig, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if wm.config == nil {
		return &WireGuardConfig{Enabled: false, Peers: []WireGuardPeer{}}, nil
	}
	cfg := *wm.config
	cfg.Peers = append([]WireGuardPeer(nil), wm.config.Peers...)
	if cfg.Enabled {
		var st struct {
			Peers []struct {
				PublicKey     string `json:"public_key"`
				Endpoint      string `json:"endpoint"`
				LastHandshake int64  `json:"last_handshake"`
				BytesRX       int64  `json:"bytes_rx"`
				BytesTX       int64  `json:"bytes_tx"`
			} `json:"peers"`
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wm.agent.GetJSON(ctx, "/v1/wireguard/status", &st); err == nil {
			for i := range cfg.Peers {
				for _, live := range st.Peers {
					if live.PublicKey != cfg.Peers[i].PublicKey {
						continue
					}
					cfg.Peers[i].Endpoint = live.Endpoint
					cfg.Peers[i].BytesRX, cfg.Peers[i].BytesTX = live.BytesRX, live.BytesTX
					if live.LastHandshake > 0 {
						t := time.Unix(live.LastHandshake, 0)
						cfg.Peers[i].LastHandshake = &t
					}
				}
			}
		}
	}
	return &cfg, nil
}

// Enable starts the tunnel. cidr is the server's address in the tunnel
// (10.8.0.1/24; a network address gets its first host); endpoint is where
// devices reach this system.
func (wm *WireGuardManager) Enable(cidr string, port int, endpoint string, dns []string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if strings.TrimSpace(endpoint) == "" {
		return ErrEndpointRequired
	}
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}
	if ip.Equal(network.IP) {
		ip = append(net.IP(nil), network.IP...)
		incrementIP(ip)
	}
	ones, _ := network.Mask.Size()
	if port == 0 {
		port = defaultWGPort
	}
	peers := []WireGuardPeer{}
	if wm.config != nil {
		peers = wm.config.Peers // re-enabling keeps the devices
	}
	prev := wm.config
	wm.config = &WireGuardConfig{
		Enabled:          true,
		Interface:        wgInterface,
		ListenPort:       port,
		ServerCIDR:       fmt.Sprintf("%s/%d", ip, ones),
		EndpointHostname: strings.TrimSpace(endpoint),
		DNS:              dns,
		Peers:            peers,
	}
	if prev != nil {
		wm.config.PublicKey = prev.PublicKey
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wm.apply(ctx); err != nil {
		wm.config = prev
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}
	return wm.save()
}

// Disable stops the tunnel; its settings and devices are kept.
func (wm *WireGuardManager) Disable() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wm.config == nil || !wm.config.Enabled {
		return fmt.Errorf("wireguard is not enabled")
	}
	wm.config.Enabled = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wm.apply(ctx); err != nil {
		wm.config.Enabled = true
		return fmt.Errorf("failed to stop WireGuard: %w", err)
	}
	return wm.save()
}

// AddPeer adds a device and returns its configuration (with its private
// key, shown once and not kept) and a QR code of it.
func (wm *WireGuardManager) AddPeer(name string, allowedIPs []string, pubkey string) (*WireGuardPeerConfig, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wm.config == nil || !wm.config.Enabled {
		return nil, fmt.Errorf("wireguard is not enabled")
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || strings.ContainsAny(name, "\r\n") {
		return nil, fmt.Errorf("a device name of 1-64 characters on one line")
	}
	for _, peer := range wm.config.Peers {
		if peer.Name == name {
			return nil, fmt.Errorf("peer with name %s already exists", name)
		}
	}
	var priv, pub string
	if pubkey == "" {
		var err error
		if priv, pub, err = GenerateWireGuardKeys(); err != nil {
			return nil, err
		}
	} else {
		k, err := wgtypes.ParseKey(pubkey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key")
		}
		pub = k.String()
	}
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return nil, err
	}
	peerIP, err := wm.allocatePeerIP()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}
	if len(allowedIPs) == 0 {
		allowedIPs = []string{peerIP + "/32"}
	}
	peer := WireGuardPeer{ID: generateID(), Name: name, PublicKey: pub, AllowedIPs: allowedIPs, CreatedAt: time.Now(), Enabled: true}
	wm.config.Peers = append(wm.config.Peers, peer)
	wm.psk[peer.ID] = psk.String()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wm.apply(ctx); err != nil {
		wm.config.Peers = wm.config.Peers[:len(wm.config.Peers)-1]
		delete(wm.psk, peer.ID)
		return nil, fmt.Errorf("failed to apply config: %w", err)
	}
	if err := wm.save(); err != nil {
		return nil, err
	}
	cfg := wm.generateClientConfig(peer, priv, peerIP)
	if cfg.QRCode, err = wm.generateQRCode(cfg.Config); err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	return cfg, nil
}

// RemovePeer removes a device.
func (wm *WireGuardManager) RemovePeer(peerID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wm.config == nil {
		return fmt.Errorf("wireguard is not configured")
	}
	kept := []WireGuardPeer{}
	var removed *WireGuardPeer
	for i, p := range wm.config.Peers {
		if p.ID == peerID {
			removed = &wm.config.Peers[i]
			continue
		}
		kept = append(kept, p)
	}
	if removed == nil {
		return fmt.Errorf("peer not found")
	}
	prev := wm.config.Peers
	wm.config.Peers = kept
	if wm.config.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := wm.apply(ctx); err != nil {
			wm.config.Peers = prev
			return fmt.Errorf("failed to apply config: %w", err)
		}
	}
	delete(wm.psk, peerID)
	return wm.save()
}

// generateClientConfig is the device's wg-quick file. It routes only the
// tunnel's network: the device reaches this system (web interface,
// shares) at its tunnel address, and its other traffic stays local. (It
// used to send all traffic through the tunnel and rely on iptables rules
// masquerading on a hard-coded eth0.)
func (wm *WireGuardManager) generateClientConfig(peer WireGuardPeer, privateKey, clientIP string) *WireGuardPeerConfig {
	_, network, _ := net.ParseCIDR(wm.config.ServerCIDR)
	tunnel := network.String()
	endpoint := net.JoinHostPort(wm.config.EndpointHostname, strconv.Itoa(wm.config.ListenPort))
	cfg := &WireGuardPeerConfig{}
	cfg.Interface.PrivateKey = privateKey
	cfg.Interface.Address = clientIP + "/32"
	cfg.Interface.DNS = wm.config.DNS
	cfg.Peer.PublicKey = wm.config.PublicKey
	cfg.Peer.PresharedKey = wm.psk[peer.ID]
	cfg.Peer.Endpoint = endpoint
	cfg.Peer.AllowedIPs = []string{tunnel}
	cfg.Peer.PersistentKeepalive = 25

	var b strings.Builder
	b.WriteString("[Interface]\n")
	if privateKey != "" {
		fmt.Fprintf(&b, "PrivateKey = %s\n", privateKey)
	}
	fmt.Fprintf(&b, "Address = %s/32\n", clientIP)
	if len(wm.config.DNS) > 0 {
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(wm.config.DNS, ", "))
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", wm.config.PublicKey)
	fmt.Fprintf(&b, "PresharedKey = %s\n", wm.psk[peer.ID])
	fmt.Fprintf(&b, "Endpoint = %s\n", endpoint)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", tunnel)
	b.WriteString("PersistentKeepalive = 25\n")
	cfg.Config = b.String()
	return cfg
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

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
