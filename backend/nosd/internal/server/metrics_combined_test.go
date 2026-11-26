package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type fakeAgentCombined struct {
	payload []byte
	err     error
}

func (f fakeAgentCombined) FetchMetrics(ctx context.Context) ([]byte, error) { return f.payload, f.err }

func TestCombinedMetricsHandler(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "nos_dummy_total",
		Help: "dummy",
	})
	reg.MustRegister(c)
	c.Inc()

	fa := fakeAgentCombined{payload: []byte("nos_agent_build_info 1\n")}

	req := httptest.NewRequest("GET", "/metrics/all", nil)
	rr := httptest.NewRecorder()
	NewCombinedMetricsHandler(reg, fa).ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "nos_dummy_total") {
		t.Fatalf("missing nosd metric")
	}
	if !strings.Contains(body, "nos_agent_build_info") {
		t.Fatalf("missing agent metric")
	}
}
