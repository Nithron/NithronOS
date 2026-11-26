package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	prom "github.com/prometheus/client_golang/prometheus"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
	// "nos/backend/nosd/pkg/firewall"
)

// registerObservability serves Prometheus metrics and pprof when enabled (pprof from localhost only).
func (d *routeDeps) registerObservability(r chi.Router) {
	cfg := d.cfg
	// Observability endpoints: metrics and pprof
	if cfg.MetricsEnabled {
		r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
			// very simple allowlist by exact ip match or prefix
			if len(cfg.MetricsAllowlist) > 0 {
				ip := clientIP(r, cfg)
				allowed := false
				for _, a := range cfg.MetricsAllowlist {
					if a == ip || (strings.HasSuffix(a, ".") && strings.HasPrefix(ip, a)) {
						allowed = true
						break
					}
				}
				if !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			var b strings.Builder
			b.WriteString("nosd_up 1\n")
			// pool metrics (best-effort)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if list, err := pools.ListPools(ctx); err == nil {
				var total uint64
				var used uint64
				for _, p := range list {
					total += p.Size
					used += p.Used
				}
				fmt.Fprintf(&b, "pool_total_bytes %d\n", total)
				fmt.Fprintf(&b, "pool_used_bytes %d\n", used)
			}
			// SMART metrics for common devices (best-effort)
			for _, dev := range []string{"/dev/sda", "/dev/nvme0n1"} {
				client := agentclient.New("/run/nos-agent.sock")
				var out map[string]any
				if err := client.GetJSON(r.Context(), "/v1/smart?device="+dev, &out); err == nil {
					if t, ok := out["temperature_c"].(float64); ok {
						fmt.Fprintf(&b, "smart_disk_temp_celsius{dev=\"%s\"} %g\n", dev, t)
					}
					if st, ok := out["passed"].(bool); ok {
						if st {
							fmt.Fprintf(&b, "smart_pass{dev=\"%s\"} 1\n", dev)
						} else {
							fmt.Fprintf(&b, "smart_pass{dev=\"%s\"} 0\n", dev)
						}
					}
				}
			}
			// Btrfs tx progress (best-effort gauges set by executor)
			if p := currentBalancePercent(); p >= 0 {
				fmt.Fprintf(&b, "btrfs_balance_percent %g\n", p)
			}
			if p := currentReplacePercent(); p >= 0 {
				fmt.Fprintf(&b, "btrfs_replace_percent %g\n", p)
			}
			_, _ = w.Write([]byte(b.String()))
		})
		// Combined metrics endpoint: nosd + agent
		r.Get("/metrics/all", func(w http.ResponseWriter, r *http.Request) {
			NewCombinedMetricsHandler(prom.DefaultGatherer, agentMetricsClient{socket: cfg.AgentSocket()}).ServeHTTP(w, r)
		})
	}

	if cfg.PprofEnabled {
		// Guard pprof: localhost only
		r.Mount("/debug/pprof", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isLocalRequest(r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.DefaultServeMux.ServeHTTP(w, r)
		}))
	}

}
