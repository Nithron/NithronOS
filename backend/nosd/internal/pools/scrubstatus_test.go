package pools

import (
	"testing"
	"time"
)

func TestParseScrubStatus(t *testing.T) {
	running := `UUID:             0b7fd1e2-5f3c-4b1e-9d4a-2c1f0a9e8b7d
Scrub started:    Thu Nov 26 10:00:00 2026
Status:           running
Duration:         0:01:30
Time left:        0:03:00
ETA:              Thu Nov 26 10:04:30 2026
Total to scrub:   2.00GiB
Bytes scrubbed:   512.00MiB  (25.00%)
Rate:             5.69MiB/s
Error summary:    no errors found
`
	info := ParseScrubStatus(running)
	if info.Status != "running" || info.Percent != 25 || info.BytesTotal != 2<<30 || info.BytesScanned != 512<<20 {
		t.Fatalf("running: %+v", info)
	}
	if info.Duration != 90*time.Second || info.StartedAt == nil || info.Errors != 0 {
		t.Fatalf("running: %+v", info)
	}

	finished := `UUID:             0b7fd1e2
Scrub started:    Thu Nov 26 10:00:00 2026
Status:           finished
Duration:         0:04:30
Total to scrub:   2.00GiB
Rate:             7.58MiB/s
Error summary:    csum=2 verify=1
  Corrected:      3
  Uncorrectable:  0
  Unverified:     0
`
	info = ParseScrubStatus(finished)
	if info.Status != "finished" || info.Percent != 100 || info.Errors != 3 {
		t.Fatalf("finished: %+v", info)
	}

	if info := ParseScrubStatus("UUID: x\n\tno stats available\n"); info.Status != "idle" {
		t.Fatalf("never scrubbed: %+v", info)
	}
	if ParseSize("1.5KiB") != 1536 || ParseSize("4096") != 4096 || ParseSize("x") != 0 {
		t.Fatal("ParseSize")
	}
}
