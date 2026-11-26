package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AgentRequest represents a request to execute an action
type AgentRequest struct {
	Action string                 `json:"Action"`
	Params map[string]interface{} `json:"Params"`
}

// handleExecute handles the /execute endpoint for system configuration
func handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req AgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}

	switch req.Action {
	case "system.hostname.set":
		handleSetHostname(w, req.Params)
	case "system.timezone.set":
		handleSetTimezone(w, req.Params)
	case "system.ntp.set", "system.ntp.configure":
		handleSetNTP(w, req.Params)
	case "system.network.configure", "network.interface.configure":
		handleConfigureNetwork(w, req.Params)
	default:
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s", req.Action))
	}
}

func handleSetHostname(w http.ResponseWriter, params map[string]interface{}) {
	hostname, ok := params["hostname"].(string)
	if !ok || hostname == "" {
		writeErr(w, http.StatusBadRequest, "hostname required")
		return
	}
	// It is written to /etc/hostname and /etc/hosts: reject anything that is
	// not a plain host name (a newline would add arbitrary hosts entries).
	if !validHostname(hostname) {
		writeErr(w, http.StatusBadRequest, "invalid hostname")
		return
	}

	// Set hostname using hostnamectl
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hostnamectl", "set-hostname", hostname)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback: write directly to /etc/hostname
		if err := os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("failed to set hostname: %v", err))
			return
		}
		// Also try to set it immediately
		_ = exec.Command("hostname", hostname).Run()
	}

	// Update /etc/hosts
	hostsPath := "/etc/hosts"
	hostsData, _ := os.ReadFile(hostsPath)
	lines := strings.Split(string(hostsData), "\n")
	newLines := []string{}

	for _, line := range lines {
		if strings.Contains(line, "127.0.1.1") {
			newLines = append(newLines, fmt.Sprintf("127.0.1.1\t%s", hostname))
		} else {
			newLines = append(newLines, line)
		}
	}

	_ = os.WriteFile(hostsPath, []byte(strings.Join(newLines, "\n")), 0644)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "result": "success"})
}

func handleSetTimezone(w http.ResponseWriter, params map[string]interface{}) {
	timezone, ok := params["timezone"].(string)
	if !ok || timezone == "" {
		writeErr(w, http.StatusBadRequest, "timezone required")
		return
	}

	// Verify timezone exists
	tzPath := filepath.Join("/usr/share/zoneinfo", timezone)
	if _, err := os.Stat(tzPath); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid timezone: %s", timezone))
		return
	}

	// Set timezone using timedatectl
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "timedatectl", "set-timezone", timezone)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback: create symlink manually
		_ = os.Remove("/etc/localtime")
		if err := os.Symlink(tzPath, "/etc/localtime"); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("failed to set timezone: %v", err))
			return
		}
		// Also write to /etc/timezone
		_ = os.WriteFile("/etc/timezone", []byte(timezone+"\n"), 0644)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "result": "success"})
}

func handleSetNTP(w http.ResponseWriter, params map[string]interface{}) {
	enabled, _ := params["enabled"].(bool)
	servers, _ := params["servers"].([]interface{})
	for _, s := range servers {
		if str, ok := s.(string); !ok || !validNTPServer(str) {
			writeErr(w, http.StatusBadRequest, "invalid NTP server")
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if enabled {
		// Enable NTP
		_ = exec.CommandContext(ctx, "timedatectl", "set-ntp", "true").Run()

		// Configure NTP servers if provided
		if len(servers) > 0 {
			// Create timesyncd config
			configDir := "/etc/systemd/timesyncd.conf.d"
			_ = os.MkdirAll(configDir, 0755)

			var serverList []string
			for _, s := range servers {
				if str, ok := s.(string); ok {
					serverList = append(serverList, str)
				}
			}

			if len(serverList) > 0 {
				config := "[Time]\n"
				config += fmt.Sprintf("NTP=%s\n", strings.Join(serverList, " "))
				_ = os.WriteFile(filepath.Join(configDir, "nos.conf"), []byte(config), 0644)
			}
		}

		// Restart timesyncd
		_ = exec.CommandContext(ctx, "systemctl", "restart", "systemd-timesyncd").Run()
	} else {
		// Disable NTP
		_ = exec.CommandContext(ctx, "timedatectl", "set-ntp", "false").Run()
		_ = exec.CommandContext(ctx, "systemctl", "stop", "systemd-timesyncd").Run()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "result": "success"})
}

func handleConfigureNetwork(w http.ResponseWriter, params map[string]interface{}) {
	// Extract network configuration parameters
	iface, _ := params["interface"].(string)
	dhcp, _ := params["dhcp"].(bool)
	// One "address/prefix" string (what nosd sends) or a list of them.
	ipv4Addresses := stringOrList(params["ipv4_address"])
	ipv4Gateway, _ := params["ipv4_gateway"].(string)
	dnsServers, _ := params["dns"].([]interface{})

	if iface == "" {
		writeErr(w, http.StatusBadRequest, "interface name required")
		return
	}
	// iface becomes part of a file path and every value ends up in a config
	// file or command line: validate all of them.
	if !validIfaceName(iface) {
		writeErr(w, http.StatusBadRequest, "invalid interface name")
		return
	}
	if !dhcp {
		for _, a := range ipv4Addresses {
			if !validIPv4CIDR(a) {
				writeErr(w, http.StatusBadRequest, "invalid ipv4_address (want address/prefix)")
				return
			}
		}
		if ipv4Gateway != "" && !validIP(ipv4Gateway) {
			writeErr(w, http.StatusBadRequest, "invalid ipv4_gateway")
			return
		}
	}
	for _, d := range dnsServers {
		if str, ok := d.(string); !ok || !validIP(str) {
			writeErr(w, http.StatusBadRequest, "invalid dns server")
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Different approaches based on the network manager
	// First, try NetworkManager if available
	if _, err := exec.LookPath("nmcli"); err == nil {
		// Use NetworkManager
		if dhcp {
			// Configure DHCP
			_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.method", "auto").Run()
			_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.addresses", "").Run()
			_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.gateway", "").Run()
			_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.dns", "").Run()
		} else {
			// Configure static IP
			if len(ipv4Addresses) > 0 {
				_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.method", "manual").Run()
				_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.addresses", strings.Join(ipv4Addresses, ",")).Run()

				if ipv4Gateway != "" {
					_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.gateway", ipv4Gateway).Run()
				}

				if len(dnsServers) > 0 {
					var dnsList []string
					for _, dns := range dnsServers {
						if str, ok := dns.(string); ok {
							dnsList = append(dnsList, str)
						}
					}
					if len(dnsList) > 0 {
						_ = exec.CommandContext(ctx, "nmcli", "con", "mod", iface, "ipv4.dns", strings.Join(dnsList, " ")).Run()
					}
				}
			}
		}

		// Restart the connection
		_ = exec.CommandContext(ctx, "nmcli", "con", "down", iface).Run()
		_ = exec.CommandContext(ctx, "nmcli", "con", "up", iface).Run()

	} else if _, err := exec.LookPath("systemctl"); err == nil {
		// Use systemd-networkd
		// Create a network configuration file
		configDir := "/etc/systemd/network"
		_ = os.MkdirAll(configDir, 0755)

		configFile := filepath.Join(configDir, fmt.Sprintf("10-%s.network", iface))

		var config strings.Builder
		config.WriteString("[Match]\n")
		fmt.Fprintf(&config, "Name=%s\n\n", iface)
		config.WriteString("[Network]\n")

		if dhcp {
			config.WriteString("DHCP=yes\n")
		} else {
			for _, ipv4Address := range ipv4Addresses {
				fmt.Fprintf(&config, "Address=%s\n", ipv4Address)
			}
			if ipv4Gateway != "" {
				fmt.Fprintf(&config, "Gateway=%s\n", ipv4Gateway)
			}
			for _, dns := range dnsServers {
				if str, ok := dns.(string); ok {
					fmt.Fprintf(&config, "DNS=%s\n", str)
				}
			}
		}

		_ = os.WriteFile(configFile, []byte(config.String()), 0644)

		// Restart systemd-networkd
		_ = exec.CommandContext(ctx, "systemctl", "restart", "systemd-networkd").Run()

	} else {
		// Fallback: Use traditional ifconfig/route commands
		if dhcp {
			// Try to start dhclient
			_ = exec.CommandContext(ctx, "dhclient", "-r", iface).Run() // Release any existing lease
			_ = exec.CommandContext(ctx, "dhclient", iface).Run()
		} else {
			// Configure static IP using ip command
			if len(ipv4Addresses) > 0 {
				// Bring interface down
				_ = exec.CommandContext(ctx, "ip", "link", "set", iface, "down").Run()

				// Remove existing addresses
				_ = exec.CommandContext(ctx, "ip", "addr", "flush", "dev", iface).Run()

				// Add the new addresses
				for _, ipv4Address := range ipv4Addresses {
					_ = exec.CommandContext(ctx, "ip", "addr", "add", ipv4Address, "dev", iface).Run()
				}

				// Bring interface up
				_ = exec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run()

				// Add default route if gateway provided
				if ipv4Gateway != "" {
					_ = exec.CommandContext(ctx, "ip", "route", "add", "default", "via", ipv4Gateway).Run()
				}

				// Configure DNS in /etc/resolv.conf
				if len(dnsServers) > 0 {
					var resolvConf strings.Builder
					for _, dns := range dnsServers {
						if str, ok := dns.(string); ok {
							fmt.Fprintf(&resolvConf, "nameserver %s\n", str)
						}
					}
					_ = os.WriteFile("/etc/resolv.conf", []byte(resolvConf.String()), 0644)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "result": "success"})
}
