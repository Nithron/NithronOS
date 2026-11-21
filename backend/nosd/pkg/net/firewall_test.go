package net

import (
	"net"
	"testing"
	"time"
)

func TestFirewallManager_CreatePlan(t *testing.T) {
	fm := NewFirewallManager()

	tests := []struct {
		name        string
		mode        AccessMode
		enableWG    bool
		enableHTTPS bool
		wantErr     bool
	}{
		{
			name:        "LAN only mode",
			mode:        AccessModeLANOnly,
			enableWG:    false,
			enableHTTPS: false,
			wantErr:     false,
		},
		{
			name:        "WireGuard mode",
			mode:        AccessModeWireGuard,
			enableWG:    true,
			enableHTTPS: false,
			wantErr:     false,
		},
		{
			name:        "Public HTTPS mode",
			mode:        AccessModePublicHTTPS,
			enableWG:    false,
			enableHTTPS: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := fm.CreatePlan(tt.mode, tt.enableWG, tt.enableHTTPS, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePlan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if plan == nil {
					t.Error("Expected plan to be non-nil")
					return
				}

				if plan.DesiredState.Mode != tt.mode {
					t.Errorf("Expected mode %v, got %v", tt.mode, plan.DesiredState.Mode)
				}

				if len(plan.DryRunOutput) == 0 {
					t.Error("Expected dry run output to be non-empty")
				}

				if time.Until(plan.ExpiresAt) > 5*time.Minute {
					t.Error("Plan expiry time too far in future")
				}
			}
		})
	}
}

func TestFirewallManager_CalculateDiff(t *testing.T) {
	fm := NewFirewallManager()

	currentState := &FirewallState{
		Mode: AccessModeLANOnly,
		Rules: []FirewallRule{
			{
				ID:          "allow-loopback",
				Type:        "allow",
				Description: "Allow loopback",
				Enabled:     true,
			},
			{
				ID:          "allow-lan-http",
				Type:        "allow",
				Protocol:    "tcp",
				DestPort:    "80",
				Description: "Allow HTTP from LAN",
				Enabled:     true,
			},
		},
	}

	desiredState := &FirewallState{
		Mode: AccessModeWireGuard,
		Rules: []FirewallRule{
			{
				ID:          "allow-loopback",
				Type:        "allow",
				Description: "Allow loopback",
				Enabled:     true,
			},
			{
				ID:          "allow-wireguard",
				Type:        "allow",
				Protocol:    "udp",
				DestPort:    "51820",
				Description: "Allow WireGuard",
				Enabled:     true,
			},
		},
	}

	diffs := fm.calculateDiff(currentState, desiredState)

	if len(diffs) != 2 {
		t.Errorf("Expected 2 diffs (1 remove, 1 add), got %d", len(diffs))
	}

	// Check for expected diff types
	var hasRemove, hasAdd bool
	for _, diff := range diffs {
		if diff.Type == "remove" && diff.OldRule != nil && diff.OldRule.ID == "allow-lan-http" {
			hasRemove = true
		}
		if diff.Type == "add" && diff.Rule != nil && diff.Rule.ID == "allow-wireguard" {
			hasAdd = true
		}
	}

	if !hasRemove {
		t.Error("Expected to find remove diff for allow-lan-http")
	}
	if !hasAdd {
		t.Error("Expected to find add diff for allow-wireguard")
	}
}

func TestIsLANIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.100", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"fc00::1", true},      // IPv6 ULA
		{"8.8.8.8", false},     // Public IP
		{"203.0.113.1", false}, // Public IP
		{"2001:db8::1", false}, // IPv6 public
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			if got := IsLANIP(ip); got != tt.want {
				t.Errorf("IsLANIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// Helper function to parse IP
func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
