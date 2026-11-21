package net

import (
	"net"
	"strings"
	"testing"
)

func TestWireGuardManager_AllocatePeerIP(t *testing.T) {
	wm := &WireGuardManager{
		config: &WireGuardConfig{
			ServerCIDR: "10.8.0.1/24",
			Peers: []WireGuardPeer{
				{
					ID:         "peer1",
					AllowedIPs: []string{"10.8.0.2/32"},
				},
				{
					ID:         "peer2",
					AllowedIPs: []string{"10.8.0.3/32"},
				},
			},
		},
	}

	// Allocate new IP
	ip, err := wm.allocatePeerIP()
	if err != nil {
		t.Fatalf("Failed to allocate IP: %v", err)
	}

	// Should not be server IP or existing peer IPs
	if ip == "10.8.0.1" || ip == "10.8.0.2" || ip == "10.8.0.3" {
		t.Errorf("Allocated IP %s conflicts with existing IPs", ip)
	}

	// Should be in the correct subnet
	_, network, _ := net.ParseCIDR("10.8.0.0/24")
	allocatedIP := net.ParseIP(ip)
	if !network.Contains(allocatedIP) {
		t.Errorf("Allocated IP %s is not in subnet %s", ip, "10.8.0.0/24")
	}
}

func TestWireGuardManager_GenerateWireGuardConfig(t *testing.T) {
	wm := &WireGuardManager{
		config: &WireGuardConfig{
			PrivateKey: "test-private-key",
			ServerCIDR: "10.8.0.1/24",
			ListenPort: 51820,
			Peers: []WireGuardPeer{
				{
					Name:         "TestPeer",
					PublicKey:    "peer-public-key",
					PresharedKey: "peer-preshared-key",
					AllowedIPs:   []string{"10.8.0.2/32"},
					Enabled:      true,
				},
				{
					Name:       "DisabledPeer",
					PublicKey:  "disabled-public-key",
					AllowedIPs: []string{"10.8.0.3/32"},
					Enabled:    false,
				},
			},
		},
	}

	config := wm.generateWireGuardConfig()

	// Check Interface section
	if !strings.Contains(config, "[Interface]") {
		t.Error("Config missing [Interface] section")
	}
	if !strings.Contains(config, "PrivateKey = test-private-key") {
		t.Error("Config missing PrivateKey")
	}
	if !strings.Contains(config, "Address = 10.8.0.1/24") {
		t.Error("Config missing Address")
	}
	if !strings.Contains(config, "ListenPort = 51820") {
		t.Error("Config missing ListenPort")
	}

	// Check enabled peer is included
	if !strings.Contains(config, "# TestPeer") {
		t.Error("Config missing enabled peer comment")
	}
	if !strings.Contains(config, "PublicKey = peer-public-key") {
		t.Error("Config missing enabled peer public key")
	}
	if !strings.Contains(config, "PresharedKey = peer-preshared-key") {
		t.Error("Config missing enabled peer preshared key")
	}
	if !strings.Contains(config, "AllowedIPs = 10.8.0.2/32") {
		t.Error("Config missing enabled peer allowed IPs")
	}

	// Check disabled peer is not included
	if strings.Contains(config, "DisabledPeer") {
		t.Error("Config should not include disabled peer")
	}
	if strings.Contains(config, "disabled-public-key") {
		t.Error("Config should not include disabled peer public key")
	}
}

func TestWireGuardManager_GenerateClientConfig(t *testing.T) {
	wm := &WireGuardManager{
		config: &WireGuardConfig{
			PublicKey:        "server-public-key",
			ListenPort:       51820,
			EndpointHostname: "vpn.example.com",
			DNS:              []string{"1.1.1.1", "1.0.0.1"},
		},
	}

	peer := WireGuardPeer{
		ID:           "test-peer",
		Name:         "TestPeer",
		PublicKey:    "peer-public-key",
		PresharedKey: "peer-preshared-key",
		AllowedIPs:   []string{"10.8.0.2/32"},
	}

	clientConfig := wm.generateClientConfig(peer, "peer-private-key", "10.8.0.2")

	// Check Interface configuration
	if clientConfig.Interface.PrivateKey != "peer-private-key" {
		t.Error("Client config missing private key")
	}
	if clientConfig.Interface.Address != "10.8.0.2/32" {
		t.Error("Client config has wrong address")
	}
	if len(clientConfig.Interface.DNS) != 2 {
		t.Error("Client config missing DNS servers")
	}

	// Check Peer (server) configuration
	if clientConfig.Peer.PublicKey != "server-public-key" {
		t.Error("Client config missing server public key")
	}
	if clientConfig.Peer.PresharedKey != "peer-preshared-key" {
		t.Error("Client config missing preshared key")
	}
	if clientConfig.Peer.Endpoint != "vpn.example.com:51820" {
		t.Error("Client config has wrong endpoint")
	}
	if len(clientConfig.Peer.AllowedIPs) != 2 ||
		clientConfig.Peer.AllowedIPs[0] != "0.0.0.0/0" ||
		clientConfig.Peer.AllowedIPs[1] != "::/0" {
		t.Error("Client config has wrong allowed IPs")
	}
	if clientConfig.Peer.PersistentKeepalive != 25 {
		t.Error("Client config missing persistent keepalive")
	}

	// Check generated config text
	if !strings.Contains(clientConfig.Config, "[Interface]") {
		t.Error("Config text missing [Interface] section")
	}
	if !strings.Contains(clientConfig.Config, "[Peer]") {
		t.Error("Config text missing [Peer] section")
	}
	if !strings.Contains(clientConfig.Config, "DNS = 1.1.1.1, 1.0.0.1") {
		t.Error("Config text missing DNS servers")
	}
}

func TestIncrementIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"10.0.0.1", "10.0.0.2"},
		{"10.0.0.255", "10.0.1.0"},
		{"10.0.255.255", "10.1.0.0"},
		{"192.168.1.99", "192.168.1.100"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ip := net.ParseIP(tt.input)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.input)
			}

			// Make a copy since incrementIP modifies in place
			ipCopy := make(net.IP, len(ip))
			copy(ipCopy, ip)

			incrementIP(ipCopy)

			if ipCopy.String() != tt.expected {
				t.Errorf("incrementIP(%s) = %s, want %s", tt.input, ipCopy.String(), tt.expected)
			}
		})
	}
}
