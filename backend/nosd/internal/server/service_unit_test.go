package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a lightweight assertion that the shipped unit contains the
// sandbox directives we expect. It doesn't require systemd.
func TestNosdServiceUnitContainsSandboxing(t *testing.T) {
	candidates := []string{
		"../../../deploy/systemd/nosd.service",    // from internal/server
		"../../../../deploy/systemd/nosd.service", // in case working dir differs
		"../../deploy/systemd/nosd.service",       // from internal
		"deploy/systemd/nosd.service",             // repo root (CI)
	}
	var path string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Skip("nosd.service not found in expected relative paths; skipping")
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read unit: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"ProtectSystem=full",
		"NoNewPrivileges=yes",
		"ReadWritePaths=/etc/nos /var/lib/nos",
		"RuntimeDirectory=nos",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK",
		"StateDirectory=nos",
		"ConfigurationDirectory=nos",
		"User=nos",
		"Group=nos",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("unit missing %q", want)
		}
	}
}
