package server

import (
	"nos/backend/nosd/pkg/firewall"
	"strings"
	"testing"
)

func TestBuildRulesLanOnly(t *testing.T) {
	rules := firewall.BuildRules("lan-only")
	must := []string{"policy drop", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, m := range must {
		if !strings.Contains(rules, m) {
			t.Fatalf("expected rules to contain %q", m)
		}
	}
}
