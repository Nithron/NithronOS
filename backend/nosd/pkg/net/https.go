package net

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

const (
	caddyConfigPath    = "/etc/caddy/Caddyfile"
	caddyFragmentsPath = "/etc/caddy/Caddyfile.d"
	caddySecretsPath   = "/etc/caddy/secrets"
	acmeChallengeRoot  = "/var/www/acme"
)

// HTTPSManager manages HTTPS/TLS configuration with Caddy
type HTTPSManager struct {
	mu          sync.RWMutex
	config      *HTTPSConfig
	configPath  string
	secretsPath string
}

// NewHTTPSManager creates a new HTTPS manager
func NewHTTPSManager() *HTTPSManager {
	return &HTTPSManager{
		configPath:  caddyConfigPath,
		secretsPath: caddySecretsPath,
	}
}

// GetConfig returns the current HTTPS configuration
func (hm *HTTPSManager) GetConfig() (*HTTPSConfig, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if hm.config == nil {
		// Load current configuration
		config, err := hm.loadConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		hm.config = config
	}

	// Update certificate status
	if hm.config.Mode != HTTPSModeSelfSigned {
		hm.updateCertStatus()
	}

	return hm.config, nil
}

// Configure sets up HTTPS with the specified mode
func (hm *HTTPSManager) Configure(mode HTTPSMode, domain, email, dnsProvider, dnsAPIKey string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Validate inputs based on mode
	switch mode {
	case HTTPSModeHTTP01, HTTPSModeDNS01:
		if domain == "" {
			return fmt.Errorf("domain is required for ACME")
		}
		if email == "" {
			return fmt.Errorf("email is required for ACME")
		}

		// Validate domain points to this server
		if err := hm.validateDomain(domain); err != nil {
			return fmt.Errorf("domain validation failed: %w", err)
		}

		if mode == HTTPSModeDNS01 {
			if dnsProvider == "" {
				return fmt.Errorf("dns provider is required for DNS-01")
			}
			if dnsAPIKey == "" {
				return fmt.Errorf("dns API key is required for DNS-01")
			}
		}

	case HTTPSModeSelfSigned:
		// No additional validation needed

	default:
		return fmt.Errorf("invalid HTTPS mode: %s", mode)
	}

	// Create new configuration
	newConfig := &HTTPSConfig{
		Mode:        mode,
		Domain:      domain,
		Email:       email,
		DNSProvider: dnsProvider,
		DNSAPIKey:   dnsAPIKey,
		Status:      "pending",
	}

	// Generate Caddy configuration
	caddyConfig, err := hm.generateCaddyConfig(newConfig)
	if err != nil {
		return fmt.Errorf("failed to generate Caddy config: %w", err)
	}

	// Validate Caddy configuration
	if err := hm.validateCaddyConfig(caddyConfig); err != nil {
		return fmt.Errorf("caddy config validation failed: %w", err)
	}

	// Save DNS credentials if provided
	if mode == HTTPSModeDNS01 && dnsAPIKey != "" {
		if err := hm.saveCredentials(dnsProvider, dnsAPIKey); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
	}

	// Apply Caddy configuration
	if err := hm.applyCaddyConfig(caddyConfig); err != nil {
		return fmt.Errorf("failed to apply Caddy config: %w", err)
	}

	// Update configuration
	hm.config = newConfig
	hm.config.Status = "active"

	// Save configuration
	if err := hm.saveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Reload Caddy
	if err := hm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %w", err)
	}

	return nil
}

// TestConfiguration tests the current HTTPS configuration
func (hm *HTTPSManager) TestConfiguration() error {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if hm.config == nil {
		return fmt.Errorf("no HTTPS configuration found")
	}

	switch hm.config.Mode {
	case HTTPSModeSelfSigned:
		// Test self-signed certificate
		return hm.testSelfSignedCert()

	case HTTPSModeHTTP01, HTTPSModeDNS01:
		// Test ACME certificate
		return hm.testACMECert()

	default:
		return fmt.Errorf("unknown HTTPS mode")
	}
}

// Private methods

func (hm *HTTPSManager) loadConfig() (*HTTPSConfig, error) {
	// Load configuration from disk
	// Default to self-signed if no config exists
	return &HTTPSConfig{
		Mode:   HTTPSModeSelfSigned,
		Status: "active",
	}, nil
}

func (hm *HTTPSManager) saveConfig() error {
	// Save configuration to disk
	// Store in /var/lib/nos/net/https.json
	return nil
}

func (hm *HTTPSManager) validateDomain(domain string) error {
	// Check if domain resolves to this server
	ips, err := net.LookupHost(domain)
	if err != nil {
		return fmt.Errorf("failed to resolve domain: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("domain does not resolve to any IP")
	}

	// Get local IPs
	localIPs, err := hm.getLocalIPs()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	// Check if any resolved IP matches local IPs
	for _, ip := range ips {
		for _, localIP := range localIPs {
			if ip == localIP {
				return nil // Domain points to this server
			}
		}
	}

	// For DNS-01, we don't need the domain to point to this server
	if hm.config != nil && hm.config.Mode == HTTPSModeDNS01 {
		return nil
	}

	return fmt.Errorf("domain does not point to this server")
}

func (hm *HTTPSManager) generateCaddyConfig(config *HTTPSConfig) (string, error) {
	tmpl := `
{
	{{- if eq .Mode "self_signed" }}
	# Self-signed certificate mode
	local_certs
	{{- else if eq .Mode "http_01" }}
	# Let's Encrypt HTTP-01 challenge
	email {{ .Email }}
	acme_ca https://acme-v02.api.letsencrypt.org/directory
	{{- else if eq .Mode "dns_01" }}
	# Let's Encrypt DNS-01 challenge
	email {{ .Email }}
	acme_ca https://acme-v02.api.letsencrypt.org/directory
	acme_dns {{ .DNSProvider }}
	{{- end }}
}

# HTTP to HTTPS redirect
:80 {
	{{- if ne .Mode "self_signed" }}
	# Allow ACME challenges
	handle /.well-known/acme-challenge/* {
		root * {{ .ACMERoot }}
		file_server
	}
	{{- end }}
	
	# Redirect all other traffic to HTTPS
	redir https://{host}{uri} permanent
}

# Main HTTPS site
{{- if .Domain }}
{{ .Domain }}:443 {
{{- else }}
:443 {
{{- end }}
	{{- if eq .Mode "self_signed" }}
	tls internal
	{{- else if .Domain }}
	tls {
		{{- if eq .Mode "dns_01" }}
		dns {{ .DNSProvider }}
		{{- end }}
	}
	{{- end }}
	
	# Security headers
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		X-XSS-Protection "1; mode=block"
		Referrer-Policy "strict-origin-when-cross-origin"
	}
	
	# API proxy
	handle /api/* {
		reverse_proxy localhost:9000 {
			header_up X-Real-IP {remote_host}
			header_up X-Forwarded-Proto {scheme}
		}
	}
	
	# WebSocket support for live updates
	@websocket {
		header Connection *Upgrade*
		header Upgrade websocket
	}
	handle @websocket {
		reverse_proxy localhost:9000
	}
	
	# Static files (React app)
	handle {
		root * /usr/share/nos/web
		try_files {path} /index.html
		file_server
	}
	
	# Include additional configurations
	import /etc/caddy/Caddyfile.d/*.caddy
}
`

	t, err := template.New("caddyfile").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		Mode        string
		Domain      string
		Email       string
		DNSProvider string
		ACMERoot    string
	}{
		Mode:        string(config.Mode),
		Domain:      config.Domain,
		Email:       config.Email,
		DNSProvider: config.DNSProvider,
		ACMERoot:    acmeChallengeRoot,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (hm *HTTPSManager) validateCaddyConfig(config string) error {
	// Write config to temporary file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("caddy-%d.conf", time.Now().Unix()))
	if err := os.WriteFile(tmpFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	defer os.Remove(tmpFile)

	// Validate using caddy validate
	cmd := exec.Command("caddy", "validate", "--config", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation failed: %s", output)
	}

	return nil
}

func (hm *HTTPSManager) applyCaddyConfig(config string) error {
	// Backup current configuration
	if _, err := os.Stat(hm.configPath); err == nil {
		backupPath := hm.configPath + ".backup"
		if err := copyFile(hm.configPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup config: %w", err)
		}
	}

	// Write new configuration
	if err := os.WriteFile(hm.configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (hm *HTTPSManager) reloadCaddy() error {
	// Reload Caddy service
	cmd := exec.Command("systemctl", "reload", "caddy")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try to restore backup on failure
		backupPath := hm.configPath + ".backup"
		if _, statErr := os.Stat(backupPath); statErr == nil {
			if cpErr := copyFile(backupPath, hm.configPath); cpErr != nil {
				fmt.Printf("Failed to restore Caddy config backup: %v\n", cpErr)
			}
		}
		return fmt.Errorf("reload failed: %s", output)
	}

	return nil
}

func (hm *HTTPSManager) saveCredentials(provider, apiKey string) error {
	// Ensure secrets directory exists with proper permissions
	if err := os.MkdirAll(hm.secretsPath, 0700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	// Save API key to file with restricted permissions
	secretFile := filepath.Join(hm.secretsPath, fmt.Sprintf("%s.key", provider))
	if err := os.WriteFile(secretFile, []byte(apiKey), 0600); err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}

	// Set environment variable for Caddy
	envFile := filepath.Join(hm.secretsPath, fmt.Sprintf("%s.env", provider))
	envContent := hm.getDNSEnvVars(provider, apiKey)
	if err := os.WriteFile(envFile, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	return nil
}

func (hm *HTTPSManager) getDNSEnvVars(provider, apiKey string) string {
	// Map provider to environment variables
	switch strings.ToLower(provider) {
	case "cloudflare":
		return fmt.Sprintf("CF_API_TOKEN=%s\n", apiKey)
	case "route53":
		return fmt.Sprintf("AWS_ACCESS_KEY_ID=%s\n", apiKey)
	case "digitalocean":
		return fmt.Sprintf("DO_AUTH_TOKEN=%s\n", apiKey)
	default:
		return fmt.Sprintf("%s_API_KEY=%s\n", strings.ToUpper(provider), apiKey)
	}
}

func (hm *HTTPSManager) updateCertStatus() {
	if hm.config.Domain == "" {
		return
	}

	// Check certificate expiry
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", hm.config.Domain), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		hm.config.Status = "failed"
		hm.config.ErrorMessage = err.Error()
		return
	}
	defer conn.Close()

	// Get certificate details
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) > 0 {
		cert := certs[0]
		hm.config.Expiry = &cert.NotAfter

		// Calculate renewal time (30 days before expiry)
		renewalTime := cert.NotAfter.Add(-30 * 24 * time.Hour)
		hm.config.NextRenewal = &renewalTime

		// Check if renewal is needed
		if time.Now().After(renewalTime) {
			hm.config.Status = "renewing"
		} else {
			hm.config.Status = "active"
		}
	}
}

func (hm *HTTPSManager) testSelfSignedCert() error {
	// Test HTTPS connectivity with self-signed cert
	tr := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", "localhost:443", tr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	return nil
}

func (hm *HTTPSManager) testACMECert() error {
	if hm.config.Domain == "" {
		return fmt.Errorf("no domain configured")
	}

	// Test HTTPS connectivity with ACME cert
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", hm.config.Domain), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Verify certificate is valid for domain
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return fmt.Errorf("no certificates found")
	}

	cert := certs[0]
	if err := cert.VerifyHostname(hm.config.Domain); err != nil {
		return fmt.Errorf("certificate not valid for domain: %w", err)
	}

	return nil
}

func (hm *HTTPSManager) getLocalIPs() ([]string, error) {
	var ips []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && !ip.IsLoopback() {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips, nil
}

// Helper function to copy file
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0644)
}
