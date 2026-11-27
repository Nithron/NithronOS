package net

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// deploy/firewall/nos-default.nft, which the agent package ships and boot
// loads before any rules are confirmed, is BaseRules rendered. Regenerate:
// NOS_WRITE_DEFAULT_RULES=1 go test ./pkg/net -run DefaultRulesFile
func TestDefaultRulesFile(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "deploy", "firewall", "nos-default.nft")
	want, err := RenderScript(BaseRules())
	if err != nil {
		t.Fatal(err)
	}
	want = "#!/usr/sbin/nft -f\n# nOS default firewall: generated from BaseRules (backend/nosd/pkg/net).\n" + want
	if os.Getenv("NOS_WRITE_DEFAULT_RULES") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s is out of date (err %v); regenerate it: NOS_WRITE_DEFAULT_RULES=1 go test ./pkg/net -run DefaultRulesFile", path, err)
	}
}
