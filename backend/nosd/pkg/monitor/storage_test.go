package monitor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// The nosd package is built with CGO_ENABLED=0; the storage used a cgo-only
// SQLite driver and failed at runtime there. This test runs in CI's normal
// build and with CGO_ENABLED=0.
func TestStorageStoresAndQueries(t *testing.T) {
	s, err := NewTimeSeriesStorage(zerolog.Nop(), filepath.Join(t.TempDir(), "monitor"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	for i := 0; i < 5; i++ {
		if err := s.Store(MetricTypeCPU, now.Add(time.Duration(-i)*time.Minute), float64(10*i), map[string]string{"host": "nos"}); err != nil {
			t.Fatal(err)
		}
	}
	ts, err := s.Query(TimeSeriesQuery{Metric: MetricTypeCPU, StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if len(ts.DataPoints) != 5 {
		t.Fatalf("points: %+v", ts)
	}
}
