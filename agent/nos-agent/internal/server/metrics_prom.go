//go:build prommetrics

package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Version and Rev can be overridden at build time via -ldflags
var (
	Version = "dev"
	Rev     = ""
)

var (
	promReg          = prometheus.NewRegistry()
	btrfsStatusCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nos_agent_btrfs_status_calls_total",
			Help: "Total number of btrfs status calls by kind.",
		},
		[]string{"kind"},
	)
	btrfsStatusLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nos_agent_btrfs_status_latency_seconds",
			Help:    "Latency of btrfs status calls by kind in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"kind"},
	)
	buildInfoGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "nos_agent_build_info",
		Help:        "Build info of the agent.",
		ConstLabels: prometheus.Labels{"version": Version, "rev": Rev},
	})
)

func initMetrics() {
	_ = promReg.Register(btrfsStatusCalls)
	_ = promReg.Register(btrfsStatusLatency)
	_ = promReg.Register(buildInfoGauge)
	buildInfoGauge.Set(1)
}

func metricsHandler() http.Handler {
	return promhttp.HandlerFor(promReg, promhttp.HandlerOpts{})
}

func incBtrfsStatus(kind string) {
	btrfsStatusCalls.WithLabelValues(kind).Inc()
}

func observeBtrfsStatus(kind string, seconds float64) {
	btrfsStatusLatency.WithLabelValues(kind).Observe(seconds)
}
