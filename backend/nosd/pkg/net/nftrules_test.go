package net

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func allModes() map[string][]FirewallRule {
	return map[string][]FirewallRule{
		"lan":       BaseRules(),
		"wireguard": append(BaseRules(), WireGuardRules(51821, "wg0")...),
		"public":    append(BaseRules(), PublicHTTPSRules()...),
	}
}

// The script replaces only table inet nos, with no forward chain (Docker
// routes app traffic) and without flushing anyone else's tables.
func TestRenderScriptOwnsOneTable(t *testing.T) {
	s, err := RenderScript(BaseRules())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"flush ruleset", "hook forward", "table inet filter"} {
		if strings.Contains(s, bad) {
			t.Errorf("script contains %q:\n%s", bad, s)
		}
	}
	for _, want := range []string{
		"table inet nos\ndelete table inet nos\n",
		"policy drop;",
		`iifname "lo" accept`,
		"ct state established,related accept",
		"meta l4proto { icmp, ipv6-icmp } accept",
		"ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } tcp dport { 139, 445 } accept",
		"ip6 saddr { fc00::/7, fe80::/10 } tcp dport { 139, 445 } accept",
		"ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } udp dport 137-138 accept",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script lacks %q:\n%s", want, s)
		}
	}
	wg, _ := RenderScript(allModes()["wireguard"])
	if !strings.Contains(wg, "udp dport 51821 accept") || !strings.Contains(wg, `iifname "wg0" accept`) {
		t.Errorf("wireguard rules:\n%s", wg)
	}
	if !strings.Contains(wg, "tcp dport { 80, 443 }") {
		t.Error("WireGuard mode must keep the LAN rules")
	}
}

// No rule may accept everything (the old loopback rule did), and every field
// that lands in the root-run script is checked.
func TestRenderRuleRejects(t *testing.T) {
	for name, r := range map[string]FirewallRule{
		"match-all":   {ID: "x", Action: "accept", Enabled: true},
		"bad-source":  {ID: "x", SourceCIDR: "10.0.0.0/33", Action: "accept", Enabled: true},
		"bad-port":    {ID: "x", Protocol: "tcp", DestPort: "22; flush ruleset", Action: "accept", Enabled: true},
		"bad-range":   {ID: "x", Protocol: "tcp", DestPort: "90-80", Action: "accept", Enabled: true},
		"bad-proto":   {ID: "x", Protocol: "sctp", DestPort: "22", Action: "accept", Enabled: true},
		"bad-action":  {ID: "x", Protocol: "tcp", DestPort: "22", Action: "jump evil", Enabled: true},
		"bad-iface":   {ID: "x", Interface: `wg0" accept #`, Action: "accept", Enabled: true},
		"icmp-port":   {ID: "x", Protocol: "icmp", DestPort: "1", Action: "accept", Enabled: true},
		"bad-id":      {ID: "X Y", Protocol: "tcp", DestPort: "22", Action: "accept", Enabled: true},
		"other-chain": {ID: "x", Chain: "forward", Protocol: "tcp", DestPort: "22", Action: "accept", Enabled: true},
	} {
		if _, err := RenderScript([]FirewallRule{r}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A description can't break out of its comment.
	s, err := RenderScript([]FirewallRule{{ID: "x", Protocol: "tcp", DestPort: "22", Action: "accept", Description: "a\"; flush ruleset; #\nb", Enabled: true}})
	if err != nil || strings.Contains(s, `";`) || strings.Count(s, "\n") != 9 {
		t.Fatalf("description: %v\n%s", err, s)
	}
	// Disabled rules are left out.
	s, _ = RenderScript([]FirewallRule{{ID: "x", Protocol: "tcp", DestPort: "22", Action: "accept"}})
	if strings.Contains(s, "dport 22") {
		t.Fatal("a disabled rule was rendered")
	}
}

// NOS_NFT_CHECK=1 (as root, e.g. in a container with nftables) checks every
// mode's script with nft itself.
func TestRenderScriptPassesNFT(t *testing.T) {
	if os.Getenv("NOS_NFT_CHECK") != "1" {
		t.Skip("set NOS_NFT_CHECK=1 where nft can check rulesets")
	}
	for mode, rules := range allModes() {
		s, err := RenderScript(rules)
		if err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(t.TempDir(), mode+".nft")
		if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("nft", "-c", "-f", f).CombinedOutput(); err != nil {
			t.Errorf("%s: nft -c: %v\n%s\n%s", mode, err, out, s)
		}
	}
}
