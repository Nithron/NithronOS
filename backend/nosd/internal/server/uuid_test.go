package server

import (
	"regexp"
	"testing"
)

// Regression (S15): IDs were derived from the clock with the last 8 bytes
// always zero, so per-agent tokens were predictable.
func TestGenerateUUIDIsRandomV4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := generateUUID()
		if !re.MatchString(id) {
			t.Fatalf("not a v4 UUID: %q", id)
		}
		if id[24:] == "000000000000" {
			t.Fatalf("predictable tail: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate: %q", id)
		}
		seen[id] = true
	}
}
