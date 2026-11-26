package pools

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ScrubInfo is what `btrfs scrub status` says about the last scrub.
type ScrubInfo struct {
	Status       string // idle (never scrubbed), running, finished, aborted, interrupted
	StartedAt    *time.Time
	Duration     time.Duration
	BytesTotal   uint64
	BytesScanned uint64
	Percent      int // while running
	Errors       int // errors found (corrected or not)
}

var (
	rePercent = regexp.MustCompile(`\(([0-9.]+)%\)`)
	reErrKV   = regexp.MustCompile(`([a-z_]+)=(\d+)`)
)

// ParseScrubStatus reads the output of `btrfs scrub status <mount>`
// (btrfs-progs 6.x).
func ParseScrubStatus(out string) ScrubInfo {
	info := ScrubInfo{Status: "idle"}
	if strings.Contains(out, "no stats available") {
		return info
	}
	for _, line := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "status":
			info.Status = strings.ToLower(val)
		case "scrub started", "scrub resumed":
			if t, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", val, time.Local); err == nil {
				t = t.UTC()
				info.StartedAt = &t
			}
		case "duration":
			info.Duration = parseClock(val)
		case "total to scrub":
			info.BytesTotal = ParseSize(val)
		case "bytes scrubbed":
			info.BytesScanned = ParseSize(strings.Fields(val + " ")[0])
			if m := rePercent.FindStringSubmatch(val); m != nil {
				if f, err := strconv.ParseFloat(m[1], 64); err == nil {
					info.Percent = int(f)
				}
			}
		case "error summary":
			for _, m := range reErrKV.FindAllStringSubmatch(val, -1) {
				n, _ := strconv.Atoi(m[2])
				info.Errors += n
			}
		}
	}
	if info.Status == "finished" {
		info.Percent = 100
	}
	return info
}

// parseClock reads h:mm:ss.
func parseClock(s string) time.Duration {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 3 {
		return 0
	}
	var d time.Duration
	for i, unit := range []time.Duration{time.Hour, time.Minute, time.Second} {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0
		}
		d += time.Duration(n) * unit
	}
	return d
}

// ParseSize reads btrfs sizes such as 1.50GiB, 512.00MiB or 4096 (bytes).
func ParseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		mult   float64
	}{
		{"EiB", 1 << 60}, {"PiB", 1 << 50}, {"TiB", 1 << 40}, {"GiB", 1 << 30},
		{"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1},
	}
	for _, u := range units {
		if num, ok := strings.CutSuffix(s, u.suffix); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
			if err != nil {
				return 0
			}
			return uint64(f * u.mult)
		}
	}
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}
