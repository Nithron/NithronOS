package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// vitalsInterval is how often /api/v1/metrics/stream sends.
const vitalsInterval = 2 * time.Second

// systemVitals is one sample for the dashboard's live gauges.
func systemVitals() map[string]any {
	v := map[string]any{"timestamp": time.Now().Unix()}
	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		v["cpu_pct"] = p[0]
	}
	if m, err := mem.VirtualMemory(); err == nil {
		v["mem_used"], v["mem_total"] = m.Used, m.Total
	}
	if s, err := mem.SwapMemory(); err == nil {
		v["swap_used"], v["swap_total"] = s.Used, s.Total
	}
	if l, err := load.Avg(); err == nil {
		v["load_1"], v["load_5"], v["load_15"] = l.Load1, l.Load5, l.Load15
	}
	if u, err := host.Uptime(); err == nil {
		v["uptime"] = u
	}
	return v
}

// handleMetricsStream sends system vitals as server-sent events until the
// client goes away.
func handleMetricsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	tick := time.NewTicker(vitalsInterval)
	defer tick.Stop()
	for {
		b, _ := json.Marshal(systemVitals())
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}
