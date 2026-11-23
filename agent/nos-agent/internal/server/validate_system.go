package server

import (
	"net"
	"regexp"
	"strings"
)

// Input checks for the system configuration actions. The agent runs as root
// and these values end up in commands, file paths (/etc/systemd/network/
// 10-<iface>.network) and config files (/etc/hosts, timesyncd, networkd), so
// they are validated here even though nosd checks them too: any process that
// can reach the agent socket can call the agent directly.

var (
	hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	ifaceName     = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,15}$`)
)

// validHostname accepts RFC 1123 host names (one or more dot-separated labels).
func validHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if !hostnameLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// validIfaceName accepts Linux interface names (at most 15 characters, no
// slashes or whitespace) and rejects "." and "..".
func validIfaceName(s string) bool {
	return ifaceName.MatchString(s) && s != "." && s != ".."
}

// validIPv4CIDR accepts an IPv4 address with a prefix length, e.g. 192.168.1.5/24.
func validIPv4CIDR(s string) bool {
	ip, _, err := net.ParseCIDR(s)
	return err == nil && ip.To4() != nil
}

// validIP accepts a single IPv4 or IPv6 address.
func validIP(s string) bool { return net.ParseIP(s) != nil }

// validNTPServer accepts a host name or an IP address.
func validNTPServer(s string) bool { return validIP(s) || validHostname(s) }

// stringOrList returns a JSON value that is either one string or a list of
// strings as a list, skipping empty entries. Anything else yields nil.
func stringOrList(v interface{}) []string {
	switch t := v.(type) {
	case string:
		if t != "" {
			return []string{t}
		}
	case []interface{}:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}
	return nil
}
