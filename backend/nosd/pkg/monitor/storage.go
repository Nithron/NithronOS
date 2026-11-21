package monitor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// TimeSeriesStorage handles metric storage with downsampling
type TimeSeriesStorage struct {
	logger   zerolog.Logger
	db       *sql.DB
	dataPath string
	mu       sync.RWMutex

	// Retention settings
	rawRetention    time.Duration // Keep raw data for this long
	minuteRetention time.Duration // Keep 1-minute rollups
	hourRetention   time.Duration // Keep 1-hour rollups
}

// NewTimeSeriesStorage creates a new time series storage
func NewTimeSeriesStorage(logger zerolog.Logger, dataPath string) (*TimeSeriesStorage, error) {
	dbPath := filepath.Join(dataPath, "metrics.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set pragmas for performance
	pragmas := []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA journal_mode = WAL",
		"PRAGMA cache_size = -64000", // 64MB cache
		"PRAGMA temp_store = MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	s := &TimeSeriesStorage{
		logger:          logger.With().Str("component", "timeseries-storage").Logger(),
		db:              db,
		dataPath:        dataPath,
		rawRetention:    24 * time.Hour,      // Keep raw data for 24 hours
		minuteRetention: 7 * 24 * time.Hour,  // Keep 1-minute rollups for 7 days
		hourRetention:   30 * 24 * time.Hour, // Keep hourly rollups for 30 days
	}

	if err := s.createTables(); err != nil {
		return nil, err
	}

	// Start background tasks
	go s.downsampleLoop()
	go s.cleanupLoop()

	return s, nil
}

// createTables creates the necessary database tables
func (s *TimeSeriesStorage) createTables() error {
	schemas := []string{
		// Raw metrics table
		`CREATE TABLE IF NOT EXISTS metrics_raw (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			metric TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			value REAL NOT NULL,
			labels TEXT,
			INDEX idx_metric_time (metric, timestamp)
		)`,

		// 1-minute rollup table
		`CREATE TABLE IF NOT EXISTS metrics_1m (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			metric TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			value_avg REAL NOT NULL,
			value_min REAL NOT NULL,
			value_max REAL NOT NULL,
			value_count INTEGER NOT NULL,
			labels TEXT,
			UNIQUE(metric, timestamp, labels),
			INDEX idx_metric_time_1m (metric, timestamp)
		)`,

		// 1-hour rollup table
		`CREATE TABLE IF NOT EXISTS metrics_1h (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			metric TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			value_avg REAL NOT NULL,
			value_min REAL NOT NULL,
			value_max REAL NOT NULL,
			value_count INTEGER NOT NULL,
			labels TEXT,
			UNIQUE(metric, timestamp, labels),
			INDEX idx_metric_time_1h (metric, timestamp)
		)`,

		// Metadata table
		`CREATE TABLE IF NOT EXISTS metrics_metadata (
			metric TEXT PRIMARY KEY,
			description TEXT,
			unit TEXT,
			last_updated INTEGER
		)`,
	}

	for _, schema := range schemas {
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_raw_metric_time ON metrics_raw(metric, timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_raw_timestamp ON metrics_raw(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_1m_metric_time ON metrics_1m(metric, timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_1m_timestamp ON metrics_1m(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_1h_metric_time ON metrics_1h(metric, timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_1h_timestamp ON metrics_1h(timestamp)",
	}

	for _, index := range indexes {
		if _, err := s.db.Exec(index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Store saves a metric data point
func (s *TimeSeriesStorage) Store(metric MetricType, timestamp time.Time, value float64, labels map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var labelsJSON []byte
	if len(labels) > 0 {
		var err error
		labelsJSON, err = json.Marshal(labels)
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}
	}

	query := `INSERT INTO metrics_raw (metric, timestamp, value, labels) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, string(metric), timestamp.Unix(), value, string(labelsJSON))

	if err != nil {
		return fmt.Errorf("failed to store metric: %w", err)
	}

	return nil
}

// Query retrieves time series data
func (s *TimeSeriesStorage) Query(q TimeSeriesQuery) (*TimeSeries, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Determine which table to query based on time range and step
	table := s.selectTable(q.StartTime, q.EndTime, q.Step)

	// Build query
	var query string
	var args []interface{}

	if table == "metrics_raw" {
		query = `SELECT timestamp, value, labels FROM metrics_raw 
				WHERE metric = ? AND timestamp >= ? AND timestamp <= ?`
		args = []interface{}{string(q.Metric), q.StartTime.Unix(), q.EndTime.Unix()}
	} else {
		// Use rollup tables
		valueCol := "value_avg"
		switch q.Aggregate {
		case "min":
			valueCol = "value_min"
		case "max":
			valueCol = "value_max"
		}

		query = fmt.Sprintf(`SELECT timestamp, %s, labels FROM %s 
				WHERE metric = ? AND timestamp >= ? AND timestamp <= ?`,
			valueCol, table)
		args = []interface{}{string(q.Metric), q.StartTime.Unix(), q.EndTime.Unix()}
	}

	// Add label filters if specified
	if len(q.Filters) > 0 {
		filterJSON, _ := json.Marshal(q.Filters)
		query += " AND labels LIKE ?"
		args = append(args, "%"+string(filterJSON)+"%")
	}

	query += " ORDER BY timestamp ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	ts := &TimeSeries{
		Metric:     q.Metric,
		DataPoints: []DataPoint{},
	}

	for rows.Next() {
		var timestamp int64
		var value float64
		var labelsJSON sql.NullString

		if err := rows.Scan(&timestamp, &value, &labelsJSON); err != nil {
			continue
		}

		dp := DataPoint{
			Timestamp: time.Unix(timestamp, 0),
			Value:     value,
		}

		if labelsJSON.Valid {
			_ = json.Unmarshal([]byte(labelsJSON.String), &dp.Labels)
		}

		ts.DataPoints = append(ts.DataPoints, dp)
	}

	// Apply step aggregation if needed
	if q.Step > 0 {
		ts.DataPoints = s.aggregateByStep(ts.DataPoints, q.Step, q.Aggregate)
	}

	return ts, nil
}

// selectTable determines which table to query based on the time range
func (s *TimeSeriesStorage) selectTable(start, end time.Time, step time.Duration) string {
	duration := end.Sub(start)

	// Use raw data for recent queries or high resolution
	if duration <= 24*time.Hour && step < time.Minute {
		return "metrics_raw"
	}

	// Use 1-minute rollups for medium range
	if duration <= 7*24*time.Hour && step < time.Hour {
		return "metrics_1m"
	}

	// Use hourly rollups for long range
	return "metrics_1h"
}

// aggregateByStep aggregates data points by the specified step
func (s *TimeSeriesStorage) aggregateByStep(points []DataPoint, step time.Duration, aggregate string) []DataPoint {
	if len(points) == 0 {
		return points
	}

	var result []DataPoint
	var bucket []DataPoint

	bucketStart := points[0].Timestamp.Truncate(step)
	bucketEnd := bucketStart.Add(step)

	for _, p := range points {
		if p.Timestamp.Before(bucketEnd) {
			bucket = append(bucket, p)
		} else {
			// Process current bucket
			if len(bucket) > 0 {
				result = append(result, s.aggregateBucket(bucket, bucketStart, aggregate))
			}

			// Start new bucket
			bucketStart = p.Timestamp.Truncate(step)
			bucketEnd = bucketStart.Add(step)
			bucket = []DataPoint{p}
		}
	}

	// Process last bucket
	if len(bucket) > 0 {
		result = append(result, s.aggregateBucket(bucket, bucketStart, aggregate))
	}

	return result
}

// aggregateBucket aggregates a bucket of data points
func (s *TimeSeriesStorage) aggregateBucket(bucket []DataPoint, timestamp time.Time, aggregate string) DataPoint {
	if len(bucket) == 0 {
		return DataPoint{Timestamp: timestamp, Value: 0}
	}

	switch aggregate {
	case "min":
		min := bucket[0].Value
		for _, p := range bucket[1:] {
			if p.Value < min {
				min = p.Value
			}
		}
		return DataPoint{Timestamp: timestamp, Value: min, Labels: bucket[0].Labels}

	case "max":
		max := bucket[0].Value
		for _, p := range bucket[1:] {
			if p.Value > max {
				max = p.Value
			}
		}
		return DataPoint{Timestamp: timestamp, Value: max, Labels: bucket[0].Labels}

	case "sum":
		sum := 0.0
		for _, p := range bucket {
			sum += p.Value
		}
		return DataPoint{Timestamp: timestamp, Value: sum, Labels: bucket[0].Labels}

	default: // avg
		sum := 0.0
		for _, p := range bucket {
			sum += p.Value
		}
		return DataPoint{Timestamp: timestamp, Value: sum / float64(len(bucket)), Labels: bucket[0].Labels}
	}
}

// downsampleLoop periodically creates rollup tables
func (s *TimeSeriesStorage) downsampleLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.downsample()
	}
}

// downsample creates rollup data
func (s *TimeSeriesStorage) downsample() {
	now := time.Now()

	// Create 1-minute rollups from raw data
	s.createRollups("metrics_raw", "metrics_1m", time.Minute, now.Add(-s.rawRetention))

	// Create 1-hour rollups from 1-minute data
	s.createRollups("metrics_1m", "metrics_1h", time.Hour, now.Add(-s.minuteRetention))
}

// createRollups creates rollup data from source table
func (s *TimeSeriesStorage) createRollups(sourceTable, destTable string, interval time.Duration, since time.Time) {
	query := fmt.Sprintf(`
		INSERT OR REPLACE INTO %s (metric, timestamp, value_avg, value_min, value_max, value_count, labels)
		SELECT 
			metric,
			(timestamp / %d) * %d as bucket,
			AVG(value) as value_avg,
			MIN(value) as value_min,
			MAX(value) as value_max,
			COUNT(*) as value_count,
			labels
		FROM %s
		WHERE timestamp >= ?
		GROUP BY metric, bucket, labels
	`, destTable, int64(interval.Seconds()), int64(interval.Seconds()), sourceTable)

	if _, err := s.db.Exec(query, since.Unix()); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create rollups")
	}
}

// cleanupLoop periodically removes old data
func (s *TimeSeriesStorage) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes old data based on retention policies
func (s *TimeSeriesStorage) cleanup() {
	now := time.Now()

	// Clean raw data
	cutoff := now.Add(-s.rawRetention).Unix()
	if _, err := s.db.Exec("DELETE FROM metrics_raw WHERE timestamp < ?", cutoff); err != nil {
		s.logger.Error().Err(err).Msg("Failed to cleanup raw metrics")
	}

	// Clean 1-minute rollups
	cutoff = now.Add(-s.minuteRetention).Unix()
	if _, err := s.db.Exec("DELETE FROM metrics_1m WHERE timestamp < ?", cutoff); err != nil {
		s.logger.Error().Err(err).Msg("Failed to cleanup 1m metrics")
	}

	// Clean 1-hour rollups
	cutoff = now.Add(-s.hourRetention).Unix()
	if _, err := s.db.Exec("DELETE FROM metrics_1h WHERE timestamp < ?", cutoff); err != nil {
		s.logger.Error().Err(err).Msg("Failed to cleanup 1h metrics")
	}

	// Vacuum database occasionally
	if now.Hour() == 3 && now.Minute() < 5 {
		if _, err := s.db.Exec("VACUUM"); err != nil {
			s.logger.Error().Err(err).Msg("Failed to vacuum database")
		}
	}
}

// Close closes the storage
func (s *TimeSeriesStorage) Close() error {
	return s.db.Close()
}

// GetStorageStats returns storage statistics
func (s *TimeSeriesStorage) GetStorageStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get table sizes
	tables := []string{"metrics_raw", "metrics_1m", "metrics_1h"}
	for _, table := range tables {
		var count int64
		row := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
		if err := row.Scan(&count); err == nil {
			stats[table+"_count"] = count
		}
	}

	// Get database size
	var pageCount, pageSize int64
	_ = s.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	_ = s.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	stats["db_size_bytes"] = pageCount * pageSize

	return stats, nil
}
