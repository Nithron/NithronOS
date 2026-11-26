package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The request logger must not hide Flush and Hijack: SSE streams and
// WebSocket upgrades need them.
func TestStatusWriterPassesThroughStreaming(t *testing.T) {
	var w http.ResponseWriter = &statusWriter{ResponseWriter: httptest.NewRecorder()}
	if _, ok := w.(http.Flusher); !ok {
		t.Fatal("not a Flusher")
	}
	if _, ok := w.(http.Hijacker); !ok {
		t.Fatal("not a Hijacker")
	}
	w.(http.Flusher).Flush()
	if !w.(*statusWriter).ResponseWriter.(*httptest.ResponseRecorder).Flushed {
		t.Fatal("Flush didn't reach the real writer")
	}
}

func TestAverageCPUPercent(t *testing.T) {
	// 30 s of CPU over 10 minutes: 5% of one core.
	if got := averageCPUPercent(30e9, 600); got != 5 {
		t.Fatalf("got %v", got)
	}
	if got := averageCPUPercent(1e9, 3); got != 33.3 {
		t.Fatalf("rounding: %v", got)
	}
}
