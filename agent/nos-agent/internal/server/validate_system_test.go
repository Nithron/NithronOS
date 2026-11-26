package server

import "testing"

// Regression (S8): hostname, interface name, addresses and NTP/DNS servers
// went unvalidated into /etc/hosts, a file path under /etc/systemd/network
// and config files, as root.
func TestSystemInputValidation(t *testing.T) {
	for _, h := range []string{"nos", "nas-01", "nas.example.com"} {
		if !validHostname(h) {
			t.Errorf("hostname %q rejected", h)
		}
	}
	for _, h := range []string{"", "-nas", "nas-", "nas\n10.0.0.1 evil", "a b", "x..y", "ex/../../etc"} {
		if validHostname(h) {
			t.Errorf("hostname %q accepted", h)
		}
	}
	for _, i := range []string{"eth0", "enp3s0", "br0.10", "wlan0"} {
		if !validIfaceName(i) {
			t.Errorf("iface %q rejected", i)
		}
	}
	for _, i := range []string{"", ".", "..", "../../etc/x", "eth0\nDNS=1.2.3.4", "averyveryverylongname0"} {
		if validIfaceName(i) {
			t.Errorf("iface %q accepted", i)
		}
	}
	if !validIPv4CIDR("192.168.1.5/24") || validIPv4CIDR("192.168.1.5") || validIPv4CIDR("1.2.3.4/24\nDNS=x") || validIPv4CIDR("fd00::1/64") {
		t.Error("ipv4 cidr validation")
	}
	if !validNTPServer("pool.ntp.org") || !validNTPServer("192.0.2.1") || validNTPServer("pool.ntp.org\n[Time]") {
		t.Error("ntp server validation")
	}
}

// ipv4_address may be one "address/prefix" string (what nosd sends) or a list.
func TestStringOrList(t *testing.T) {
	if got := stringOrList([]interface{}{"10.0.0.5/24", "", "10.0.1.5/24"}); len(got) != 2 || got[1] != "10.0.1.5/24" {
		t.Fatalf("list: %v", got)
	}
	if got := stringOrList("10.0.0.5/24"); len(got) != 1 {
		t.Fatalf("string: %v", got)
	}
	if got := stringOrList(42); got != nil {
		t.Fatalf("number: %v", got)
	}
}
