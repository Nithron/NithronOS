package net

// HTTPS for the web interface. The LAN site always uses this machine's
// self-signed certificate (nos-certgen). "Public HTTPS" adds a site for a
// domain with a Let's Encrypt certificate (HTTP-01 challenge, which
// Debian's caddy can do); nos-agent writes that one site block and reloads
// Caddy. (This used to regenerate the whole Caddyfile, dropping the API and
// WebDAV proxy, the apps and the admin socket, write /etc/caddy as the nos
// user, and offer DNS challenges that need Caddy plugins Debian's caddy
// doesn't have.)

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrDNSChallengeUnsupported: Debian's caddy has no DNS provider plugins.
var ErrDNSChallengeUnsupported = errors.New("the DNS challenge needs Caddy plugins Debian's caddy doesn't have; use the HTTP challenge (ports 80 and 443 open to the internet)")

var (
	reDomain = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))+$`)
	reEmail  = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,63}$`)
)

// HTTPSManager keeps the HTTPS settings (in statePath) and applies them
// through nos-agent.
type HTTPSManager struct {
	mu        sync.RWMutex
	config    *HTTPSConfig
	statePath string
	agent     AgentJSON
}

// NewHTTPSManager loads the saved settings; none means self-signed only.
func NewHTTPSManager(statePath string, agent AgentJSON) *HTTPSManager {
	hm := &HTTPSManager{statePath: statePath, agent: agent}
	if data, err := os.ReadFile(statePath); err == nil {
		var c HTTPSConfig
		if json.Unmarshal(data, &c) == nil {
			hm.config = &c
		}
	}
	if hm.config == nil {
		hm.config = &HTTPSConfig{Mode: HTTPSModeSelfSigned, Status: "active"}
	}
	return hm
}

// GetConfig returns the HTTPS settings.
func (hm *HTTPSManager) GetConfig() (*HTTPSConfig, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	c := *hm.config
	return &c, nil
}

// Configure switches between the LAN's self-signed certificate only and a
// public domain with a Let's Encrypt certificate.
func (hm *HTTPSManager) Configure(mode HTTPSMode, domain, email, dnsProvider, _ string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	domain = strings.ToLower(strings.TrimSpace(domain))
	email = strings.TrimSpace(email)
	next := &HTTPSConfig{Mode: mode}
	switch mode {
	case HTTPSModeSelfSigned:
	case HTTPSModeHTTP01:
		if !reDomain.MatchString(domain) || len(domain) > 253 {
			return fmt.Errorf("enter a domain name such as nas.example.org")
		}
		if !reEmail.MatchString(email) {
			return fmt.Errorf("enter the email address Let's Encrypt writes to about the certificate")
		}
		next.Domain, next.Email = domain, email
	case HTTPSModeDNS01:
		return ErrDNSChallengeUnsupported
	default:
		return fmt.Errorf("unknown HTTPS mode %q", mode)
	}
	_ = dnsProvider
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := hm.agent.PostJSON(ctx, "/v1/caddy/public-site", map[string]string{"domain": next.Domain, "email": next.Email}, nil); err != nil {
		return fmt.Errorf("failed to configure Caddy: %w", err)
	}
	next.Status = "active"
	if mode == HTTPSModeHTTP01 {
		// Caddy asks Let's Encrypt in the background; TestConfiguration
		// shows when the certificate is there.
		next.Status = "pending"
	}
	hm.config = next
	return hm.save()
}

func (hm *HTTPSManager) save() error {
	data, err := json.MarshalIndent(hm.config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(hm.statePath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(hm.statePath, data, 0o600)
}

// TestConfiguration checks that the public site answers with a valid
// certificate for its domain (it has nothing to check for self-signed).
func (hm *HTTPSManager) TestConfiguration() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.config.Mode != HTTPSModeHTTP01 {
		return nil
	}
	dialer := &tls.Dialer{Config: &tls.Config{ServerName: hm.config.Domain}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", hm.config.Domain+":443")
	if err != nil {
		hm.config.Status = "failed"
		hm.config.ErrorMessage = err.Error()
		return fmt.Errorf("https://%s isn't answering with a valid certificate yet: %w", hm.config.Domain, err)
	}
	defer conn.Close()
	cert := conn.(*tls.Conn).ConnectionState().PeerCertificates[0]
	exp := cert.NotAfter
	hm.config.Status, hm.config.ErrorMessage, hm.config.Expiry = "active", "", &exp
	_ = hm.save()
	return nil
}
