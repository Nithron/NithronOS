package net

// The firewall is the table "inet nos": one input chain, policy drop. Each
// apply replaces that table atomically and touches nothing else, so Docker's
// tables (app networking and published ports) and any other table survive.
// (The script used to start with "flush ruleset", which deleted Docker's
// rules, and to drop all forwarded traffic, which cut apps off.)

import (
	"fmt"
	stdnet "net"
	"regexp"
	"strconv"
	"strings"
)

// NFTTable is the table nOS owns.
const NFTTable = "nos"

// lanSources are the networks treated as "the LAN": private IPv4, IPv6
// unique-local and link-local.
var lanSources = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7", "fe80::/10"}

var (
	reRuleID     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	reIface      = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)
	reDescUnsafe = regexp.MustCompile(`[^A-Za-z0-9 .,:()/+'-]`)
)

// BaseRules are always applied: loopback, replies, ICMP, and the LAN
// services (SSH, the web UI, SMB, NFS, mDNS) from the LAN only.
func BaseRules() []FirewallRule {
	lan := strings.Join(lanSources, ",")
	return []FirewallRule{
		{ID: "allow-loopback", Chain: "input", Interface: "lo", Action: "accept", Description: "Loopback", Enabled: true},
		{ID: "allow-established", Chain: "input", CTState: "established,related", Action: "accept", Description: "Replies to connections this system opened", Enabled: true},
		{ID: "allow-icmp", Chain: "input", Protocol: "icmp", Action: "accept", Description: "Ping and ICMP errors", Enabled: true},
		{ID: "allow-lan-ssh", Chain: "input", Protocol: "tcp", SourceCIDR: lan, DestPort: "22", Action: "accept", Description: "SSH from the LAN", Enabled: true},
		{ID: "allow-lan-web", Chain: "input", Protocol: "tcp", SourceCIDR: lan, DestPort: "80,443", Action: "accept", Description: "Web interface from the LAN", Enabled: true},
		{ID: "allow-lan-smb", Chain: "input", Protocol: "tcp", SourceCIDR: lan, DestPort: "139,445", Action: "accept", Description: "SMB file sharing from the LAN", Enabled: true},
		{ID: "allow-lan-netbios", Chain: "input", Protocol: "udp", SourceCIDR: lan, DestPort: "137-138", Action: "accept", Description: "NetBIOS name service from the LAN", Enabled: true},
		{ID: "allow-lan-nfs-tcp", Chain: "input", Protocol: "tcp", SourceCIDR: lan, DestPort: "111,2049", Action: "accept", Description: "NFS from the LAN", Enabled: true},
		{ID: "allow-lan-nfs-udp", Chain: "input", Protocol: "udp", SourceCIDR: lan, DestPort: "111,2049", Action: "accept", Description: "NFS from the LAN", Enabled: true},
		{ID: "allow-mdns", Chain: "input", Protocol: "udp", SourceCIDR: lan, DestPort: "5353", Action: "accept", Description: "mDNS discovery from the LAN", Enabled: true},
	}
}

// WireGuardRules open the tunnel's UDP port to everyone and treat traffic
// from the tunnel like the LAN.
func WireGuardRules(listenPort int, iface string) []FirewallRule {
	if listenPort <= 0 {
		listenPort = 51820
	}
	if iface == "" {
		iface = "wg0"
	}
	return []FirewallRule{
		{ID: "allow-wireguard", Chain: "input", Protocol: "udp", DestPort: strconv.Itoa(listenPort), Action: "accept", Description: "WireGuard", Enabled: true},
		{ID: "allow-wireguard-clients", Chain: "input", Interface: iface, Action: "accept", Description: "Devices connected over WireGuard", Enabled: true},
	}
}

// PublicHTTPSRules open the web interface to everyone (80 for ACME's HTTP
// challenge and the redirect).
func PublicHTTPSRules() []FirewallRule {
	return []FirewallRule{
		{ID: "allow-public-web", Chain: "input", Protocol: "tcp", DestPort: "80,443", Action: "accept", Description: "Web interface from the internet", Enabled: true},
	}
}

// RenderScript is the nft script that replaces table inet nos with rules.
func RenderScript(rules []FirewallRule) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "table inet %s\ndelete table inet %s\n", NFTTable, NFTTable)
	fmt.Fprintf(&b, "table inet %s {\n\tchain input {\n\t\ttype filter hook input priority 0; policy drop;\n\t\tct state invalid drop\n", NFTTable)
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if r.Chain != "" && r.Chain != "input" {
			return "", fmt.Errorf("rule %s: only input rules are supported", r.ID)
		}
		lines, err := renderRule(r)
		if err != nil {
			return "", err
		}
		for _, l := range lines {
			b.WriteString("\t\t" + l + "\n")
		}
	}
	b.WriteString("\t}\n}\n")
	return b.String(), nil
}

// renderRule is one rule as nft statements (two when its sources mix IPv4
// and IPv6). Every field is checked: the script runs as root.
func renderRule(r FirewallRule) ([]string, error) {
	fail := func(f string, a ...any) ([]string, error) {
		return nil, fmt.Errorf("rule %s: %s", r.ID, fmt.Sprintf(f, a...))
	}
	if !reRuleID.MatchString(r.ID) {
		return fail("bad id")
	}
	switch r.Action {
	case "accept", "drop", "reject":
	default:
		return fail("action must be accept, drop or reject")
	}
	var common []string
	if r.Interface != "" {
		if !reIface.MatchString(r.Interface) {
			return fail("bad interface %q", r.Interface)
		}
		common = append(common, fmt.Sprintf("iifname %q", r.Interface))
	}
	if r.CTState != "" {
		if r.CTState != "established,related" {
			return fail("connection state must be established,related")
		}
		common = append(common, "ct state established,related")
	}
	v4, v6, err := splitSources(r.SourceCIDR)
	if err != nil {
		return fail("%v", err)
	}
	ports, err := portSet(r.DestPort)
	if err != nil {
		return fail("%v", err)
	}
	var proto string
	switch r.Protocol {
	case "":
		if ports != "" {
			proto = "meta l4proto { tcp, udp } th dport " + ports
		}
	case "tcp", "udp":
		proto = r.Protocol
		if ports != "" {
			proto += " dport " + ports
		} else {
			proto = "meta l4proto " + r.Protocol
		}
	case "icmp":
		if ports != "" {
			return fail("icmp has no ports")
		}
		proto = "meta l4proto { icmp, ipv6-icmp }"
	default:
		return fail("protocol must be tcp, udp or icmp")
	}
	desc := reDescUnsafe.ReplaceAllString(r.Description, "")
	if len(desc) > 64 {
		desc = desc[:64]
	}
	build := func(src string) string {
		parts := append([]string{}, common...)
		if src != "" {
			parts = append(parts, src)
		}
		if proto != "" {
			parts = append(parts, proto)
		}
		parts = append(parts, r.Action)
		if desc != "" {
			parts = append(parts, fmt.Sprintf("comment %q", desc))
		}
		return strings.Join(parts, " ")
	}
	var out []string
	if len(v4) > 0 {
		out = append(out, build("ip saddr "+set(v4)))
	}
	if len(v6) > 0 {
		out = append(out, build("ip6 saddr "+set(v6)))
	}
	if len(v4) == 0 && len(v6) == 0 {
		// A rule that matches everything would open the firewall; it was
		// how "allow loopback" (source 127.0.0.0/8, dropped when rendered)
		// accepted all traffic.
		if len(common) == 0 && proto == "" && r.Action == "accept" {
			return fail("it would accept all traffic")
		}
		out = append(out, build(""))
	}
	return out, nil
}

func splitSources(list string) (v4, v6 []string, err error) {
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ip, n, err := stdnet.ParseCIDR(s)
		if err != nil {
			if ip = stdnet.ParseIP(s); ip == nil {
				return nil, nil, fmt.Errorf("bad source %q", s)
			}
			if ip.To4() != nil {
				v4 = append(v4, ip.String())
			} else {
				v6 = append(v6, ip.String())
			}
			continue
		}
		if ip.To4() != nil {
			v4 = append(v4, n.String())
		} else {
			v6 = append(v6, n.String())
		}
	}
	return v4, v6, nil
}

func portSet(list string) (string, error) {
	var ports []string
	for _, p := range strings.Split(list, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(p, "-")
		a, err1 := strconv.Atoi(lo)
		b := a
		var err2 error
		if isRange {
			b, err2 = strconv.Atoi(hi)
		}
		if err1 != nil || err2 != nil || a < 1 || b > 65535 || a > b {
			return "", fmt.Errorf("bad port %q", p)
		}
		ports = append(ports, p)
	}
	if len(ports) == 0 {
		return "", nil
	}
	return set(ports), nil
}

func set(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return "{ " + strings.Join(items, ", ") + " }"
}
