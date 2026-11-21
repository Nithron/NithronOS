package net

import (
	"net"
	"time"
)

// AccessMode represents the system's network access configuration
type AccessMode string

const (
	AccessModeLANOnly     AccessMode = "lan_only"     // Default: only LAN can access UI/API
	AccessModeWireGuard   AccessMode = "wireguard"    // WireGuard VPN for remote admin
	AccessModePublicHTTPS AccessMode = "public_https" // Public HTTPS on custom domain
)

// HTTPSMode represents the HTTPS/TLS configuration mode
type HTTPSMode string

const (
	HTTPSModeSelfSigned HTTPSMode = "self_signed" // Internal self-signed cert
	HTTPSModeHTTP01     HTTPSMode = "http_01"     // Let's Encrypt HTTP-01 challenge
	HTTPSModeDNS01      HTTPSMode = "dns_01"      // Let's Encrypt DNS-01 challenge
)

// FirewallState represents the current firewall configuration
type FirewallState struct {
	Mode        AccessMode     `json:"mode"`
	Rules       []FirewallRule `json:"rules"`
	LastApplied time.Time      `json:"last_applied"`
	Checksum    string         `json:"checksum"`
	Status      string         `json:"status"` // active, pending_confirm, rolling_back
	RollbackAt  *time.Time     `json:"rollback_at,omitempty"`
}

// FirewallRule represents a single nftables rule
type FirewallRule struct {
	ID          string `json:"id"`
	Table       string `json:"table"`
	Chain       string `json:"chain"`
	Priority    int    `json:"priority"`
	Type        string `json:"type"` // allow, deny, nat
	Protocol    string `json:"protocol,omitempty"`
	SourceCIDR  string `json:"source_cidr,omitempty"`
	DestPort    string `json:"dest_port,omitempty"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// FirewallPlan represents a planned firewall configuration change
type FirewallPlan struct {
	ID           string         `json:"id"`
	CurrentState *FirewallState `json:"current_state"`
	DesiredState *FirewallState `json:"desired_state"`
	Changes      []FirewallDiff `json:"changes"`
	DryRunOutput string         `json:"dry_run_output"`
	CreatedAt    time.Time      `json:"created_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

// FirewallDiff represents a single change in the firewall configuration
type FirewallDiff struct {
	Type        string        `json:"type"` // add, remove, modify
	Rule        *FirewallRule `json:"rule,omitempty"`
	OldRule     *FirewallRule `json:"old_rule,omitempty"`
	Description string        `json:"description"`
}

// WireGuardConfig represents the WireGuard server configuration
type WireGuardConfig struct {
	Enabled          bool            `json:"enabled"`
	Interface        string          `json:"interface"`
	PrivateKey       string          `json:"-"` // Never expose in API
	PublicKey        string          `json:"public_key"`
	ListenPort       int             `json:"listen_port"`
	ServerCIDR       string          `json:"server_cidr"`
	EndpointHostname string          `json:"endpoint_hostname"`
	DNS              []string        `json:"dns,omitempty"`
	Peers            []WireGuardPeer `json:"peers"`
	LastHandshake    *time.Time      `json:"last_handshake,omitempty"`
	BytesRX          int64           `json:"bytes_rx"`
	BytesTX          int64           `json:"bytes_tx"`
}

// WireGuardPeer represents a WireGuard peer/client
type WireGuardPeer struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	PublicKey     string     `json:"public_key"`
	PresharedKey  string     `json:"-"` // Never expose in API
	AllowedIPs    []string   `json:"allowed_ips"`
	Endpoint      string     `json:"endpoint,omitempty"`
	LastHandshake *time.Time `json:"last_handshake,omitempty"`
	BytesRX       int64      `json:"bytes_rx"`
	BytesTX       int64      `json:"bytes_tx"`
	CreatedAt     time.Time  `json:"created_at"`
	Enabled       bool       `json:"enabled"`
}

// WireGuardPeerConfig represents the client configuration for a peer
type WireGuardPeerConfig struct {
	Interface struct {
		PrivateKey string   `json:"private_key"`
		Address    string   `json:"address"`
		DNS        []string `json:"dns,omitempty"`
	} `json:"interface"`
	Peer struct {
		PublicKey           string   `json:"public_key"`
		PresharedKey        string   `json:"preshared_key,omitempty"`
		Endpoint            string   `json:"endpoint"`
		AllowedIPs          []string `json:"allowed_ips"`
		PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
	} `json:"peer"`
	QRCode string `json:"qr_code"` // Base64 encoded PNG
	Config string `json:"config"`  // Raw wg-quick config file content
}

// HTTPSConfig represents the HTTPS/TLS configuration
type HTTPSConfig struct {
	Mode         HTTPSMode  `json:"mode"`
	Domain       string     `json:"domain,omitempty"`
	Email        string     `json:"email,omitempty"` // For Let's Encrypt
	DNSProvider  string     `json:"dns_provider,omitempty"`
	DNSAPIKey    string     `json:"-"` // Never expose in API
	CertPath     string     `json:"cert_path,omitempty"`
	KeyPath      string     `json:"key_path,omitempty"`
	Status       string     `json:"status"` // pending, active, failed, renewing
	Expiry       *time.Time `json:"expiry,omitempty"`
	LastRenewal  *time.Time `json:"last_renewal,omitempty"`
	NextRenewal  *time.Time `json:"next_renewal,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// TOTPConfig represents TOTP configuration for a user
type TOTPConfig struct {
	UserID      string     `json:"user_id"`
	Secret      string     `json:"-"` // Base32 encoded secret, never exposed
	BackupCodes []string   `json:"-"` // Encrypted backup codes
	Enabled     bool       `json:"enabled"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
}

// TOTPEnrollment represents the TOTP enrollment response
type TOTPEnrollment struct {
	Secret      string   `json:"secret"`  // Base32 encoded
	QRCode      string   `json:"qr_code"` // Base64 encoded PNG/SVG
	BackupCodes []string `json:"backup_codes"`
	URI         string   `json:"uri"` // otpauth:// URI
}

// NetworkStatus represents overall network configuration status
type NetworkStatus struct {
	AccessMode  AccessMode       `json:"access_mode"`
	LANAccess   bool             `json:"lan_access"`
	WANBlocked  bool             `json:"wan_blocked"`
	WireGuard   *WireGuardConfig `json:"wireguard,omitempty"`
	HTTPS       *HTTPSConfig     `json:"https,omitempty"`
	Firewall    *FirewallState   `json:"firewall,omitempty"`
	ExternalIP  string           `json:"external_ip,omitempty"`
	InternalIPs []string         `json:"internal_ips,omitempty"`
	OpenPorts   []int            `json:"open_ports,omitempty"`
}

// RemoteAccessWizardState represents the wizard progress
type RemoteAccessWizardState struct {
	Step            int              `json:"step"`
	AccessMode      AccessMode       `json:"access_mode"`
	WireGuardConfig *WireGuardConfig `json:"wireguard_config,omitempty"`
	HTTPSConfig     *HTTPSConfig     `json:"https_config,omitempty"`
	FirewallPlan    *FirewallPlan    `json:"firewall_plan,omitempty"`
	Completed       bool             `json:"completed"`
	Error           string           `json:"error,omitempty"`
}

// API Request/Response types

type EnableWireGuardRequest struct {
	CIDR             string   `json:"cidr" validate:"required,cidr"`
	ListenPort       int      `json:"listen_port,omitempty"`
	EndpointHostname string   `json:"endpoint_hostname,omitempty"`
	DNS              []string `json:"dns,omitempty"`
}

type AddWireGuardPeerRequest struct {
	Name       string   `json:"name" validate:"required,min=1,max=64"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	PublicKey  string   `json:"public_key,omitempty"`
}

type ConfigureHTTPSRequest struct {
	Mode        HTTPSMode `json:"mode" validate:"required"`
	Domain      string    `json:"domain,omitempty" validate:"omitempty,fqdn"`
	Email       string    `json:"email,omitempty" validate:"omitempty,email"`
	DNSProvider string    `json:"dns_provider,omitempty"`
	DNSAPIKey   string    `json:"dns_api_key,omitempty"`
}

type PlanFirewallRequest struct {
	DesiredMode AccessMode     `json:"desired_mode" validate:"required"`
	EnableWG    bool           `json:"enable_wg"`
	EnableHTTPS bool           `json:"enable_https"`
	CustomRules []FirewallRule `json:"custom_rules,omitempty"`
}

type ApplyFirewallRequest struct {
	PlanID             string `json:"plan_id" validate:"required"`
	RollbackTimeoutSec int    `json:"rollback_timeout_sec,omitempty"`
}

type VerifyTOTPRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

type EnrollTOTPRequest struct {
	Password string `json:"password" validate:"required"` // Re-verify password for security
}

// Helper functions

// IsLANIP checks if an IP address is in a private/LAN range
func IsLANIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check RFC1918 ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7", // IPv6 ULA
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// GenerateWireGuardKeys generates a new WireGuard keypair
func GenerateWireGuardKeys() (privateKey, publicKey string, err error) {
	// This would call wg genkey | wg pubkey
	// Implementation will be in the actual service
	return "", "", nil
}
