package server

import (
	"context"
	"testing"
	"time"
)

// The dashboard's disk health is SMART's answer per disk, read in the
// background; before the first read it claims nothing.
func TestSmartCacheCountsHealth(t *testing.T) {
	calls := 0
	c := &smartCache{collect: func(context.Context) []SMARTDevice {
		calls++
		return []SMARTDevice{{Health: "good"}, {Health: "good"}, {Health: "critical"}, {Health: "unknown"}, {Health: "warning"}}
	}}
	if s := c.get(); s.Total != 0 || s.LastScanISO != "" {
		t.Fatalf("before the first check: %+v", s)
	}
	deadline := time.Now().Add(2 * time.Second)
	for c.get().LastScanISO == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	s := c.get()
	if s.Total != 5 || s.Healthy != 2 || s.Critical != 1 || s.Warning != 1 || s.Unknown != 1 {
		t.Fatalf("counts: %+v", s)
	}
	if calls != 1 {
		t.Fatalf("a fresh result must be reused: %d reads", calls)
	}
}
