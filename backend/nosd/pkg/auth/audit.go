package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// AuditLogger handles audit event logging
type AuditLogger struct {
	logger   zerolog.Logger
	dataPath string
	events   []AuditEvent
	mu       sync.RWMutex

	// File handles for streaming
	currentFile *os.File
	currentDate string
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger zerolog.Logger, dataPath string) *AuditLogger {
	al := &AuditLogger{
		logger:   logger.With().Str("component", "audit").Logger(),
		dataPath: dataPath,
		events:   []AuditEvent{},
	}

	// Create audit directory
	_ = os.MkdirAll(dataPath, 0700)

	// Load recent events
	al.loadRecentEvents()

	// Start rotation routine
	go al.rotationRoutine()

	return al
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event *AuditEvent) {
	// Set defaults
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Redact sensitive data
	al.redactSensitiveData(event)

	al.mu.Lock()
	defer al.mu.Unlock()

	// Add to memory cache
	al.events = append(al.events, *event)

	// Keep only recent events in memory (last 1000)
	if len(al.events) > 1000 {
		al.events = al.events[len(al.events)-1000:]
	}

	// Write to file
	al.writeToFile(event)

	// Log to system logger based on severity
	switch event.Severity {
	case "critical":
		al.logger.Error().
			Str("code", event.Code).
			Str("user", event.Username).
			Str("target", event.Target).
			Bool("success", event.Success).
			Msg(event.Message)
	case "warning":
		al.logger.Warn().
			Str("code", event.Code).
			Str("user", event.Username).
			Str("target", event.Target).
			Bool("success", event.Success).
			Msg(event.Message)
	default:
		al.logger.Info().
			Str("code", event.Code).
			Str("user", event.Username).
			Str("target", event.Target).
			Bool("success", event.Success).
			Msg(event.Message)
	}
}

// Query retrieves audit events
func (al *AuditLogger) Query(query AuditLogQuery) ([]AuditEvent, int, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	// Start with all events (from memory and files)
	allEvents := al.getAllEvents(query.From, query.To)

	// Filter events
	filtered := []AuditEvent{}
	for _, event := range allEvents {
		if al.matchesQuery(event, query) {
			filtered = append(filtered, event)
		}
	}

	total := len(filtered)

	// Apply pagination
	start := query.Offset
	if start < 0 {
		start = 0
	}
	if start >= len(filtered) {
		return []AuditEvent{}, total, nil
	}

	end := start + query.Limit
	if query.Limit <= 0 || end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

// GetEvent retrieves a specific event
func (al *AuditLogger) GetEvent(eventID string) (*AuditEvent, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	// Check memory cache first
	for _, event := range al.events {
		if event.ID == eventID {
			return &event, nil
		}
	}

	// Search in files
	files, _ := filepath.Glob(filepath.Join(al.dataPath, "audit-*.json"))
	for _, file := range files {
		events := al.loadEventsFromFile(file)
		for _, event := range events {
			if event.ID == eventID {
				return &event, nil
			}
		}
	}

	return nil, fmt.Errorf("event not found")
}

// ExportCSV exports events to CSV format
func (al *AuditLogger) ExportCSV(query AuditLogQuery) ([]byte, error) {
	events, _, err := al.Query(query)
	if err != nil {
		return nil, err
	}

	// Build CSV
	csv := "Timestamp,User,IP,Code,Category,Severity,Success,Target,Message\n"

	for _, event := range events {
		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%t,%s,\"%s\"\n",
			event.Timestamp.Format(time.RFC3339),
			event.Username,
			event.IP,
			event.Code,
			event.Category,
			event.Severity,
			event.Success,
			event.Target,
			event.Message,
		)
	}

	return []byte(csv), nil
}

// GetStatistics returns audit statistics
func (al *AuditLogger) GetStatistics(from, to time.Time) map[string]interface{} {
	al.mu.RLock()
	defer al.mu.RUnlock()

	events := al.getAllEvents(from, to)

	stats := map[string]interface{}{
		"total_events": len(events),
		"by_category":  make(map[string]int),
		"by_severity":  make(map[string]int),
		"by_success":   map[string]int{"success": 0, "failure": 0},
		"by_code":      make(map[string]int),
		"top_users":    make(map[string]int),
		"top_ips":      make(map[string]int),
	}

	byCategory := stats["by_category"].(map[string]int)
	bySeverity := stats["by_severity"].(map[string]int)
	bySuccess := stats["by_success"].(map[string]int)
	byCode := stats["by_code"].(map[string]int)
	topUsers := stats["top_users"].(map[string]int)
	topIPs := stats["top_ips"].(map[string]int)

	for _, event := range events {
		// Category
		byCategory[event.Category]++

		// Severity
		bySeverity[event.Severity]++

		// Success
		if event.Success {
			bySuccess["success"]++
		} else {
			bySuccess["failure"]++
		}

		// Code
		byCode[event.Code]++

		// Users
		if event.Username != "" {
			topUsers[event.Username]++
		}

		// IPs
		if event.IP != "" {
			topIPs[event.IP]++
		}
	}

	return stats
}

// Private methods

func (al *AuditLogger) redactSensitiveData(event *AuditEvent) {
	// Redact passwords and tokens
	redactKeys := []string{"password", "token", "secret", "key", "credential"}

	// Redact in details
	if event.Details != nil {
		for key := range event.Details {
			for _, redactKey := range redactKeys {
				if strings.Contains(strings.ToLower(key), redactKey) {
					event.Details[key] = "***REDACTED***"
				}
			}
		}
	}

	// Redact in old values
	if event.OldValues != nil {
		for key := range event.OldValues {
			for _, redactKey := range redactKeys {
				if strings.Contains(strings.ToLower(key), redactKey) {
					event.OldValues[key] = "***REDACTED***"
				}
			}
		}
	}

	// Redact in new values
	if event.NewValues != nil {
		for key := range event.NewValues {
			for _, redactKey := range redactKeys {
				if strings.Contains(strings.ToLower(key), redactKey) {
					event.NewValues[key] = "***REDACTED***"
				}
			}
		}
	}

	// Partially mask email addresses
	if email, ok := event.Details["email"].(string); ok {
		event.Details["email"] = al.maskEmail(email)
	}
	if email, ok := event.NewValues["email"].(string); ok {
		event.NewValues["email"] = al.maskEmail(email)
	}
}

func (al *AuditLogger) maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	local := parts[0]
	if len(local) > 3 {
		local = local[:2] + "***"
	}

	return local + "@" + parts[1]
}

func (al *AuditLogger) writeToFile(event *AuditEvent) {
	// Get current date
	dateStr := event.Timestamp.Format("2006-01-02")

	// Check if we need to rotate
	if al.currentFile != nil && al.currentDate != dateStr {
		al.currentFile.Close()
		al.currentFile = nil
	}

	// Open file if needed
	if al.currentFile == nil {
		filename := filepath.Join(al.dataPath, fmt.Sprintf("audit-%s.json", dateStr))
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			al.logger.Error().Err(err).Msg("Failed to open audit file")
			return
		}
		al.currentFile = file
		al.currentDate = dateStr
	}

	// Write event
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	_, _ = al.currentFile.Write(data)
	_, _ = al.currentFile.Write([]byte("\n"))
	_ = al.currentFile.Sync()
}

func (al *AuditLogger) matchesQuery(event AuditEvent, query AuditLogQuery) bool {
	// User filter
	if query.UserID != "" && event.UserID != query.UserID {
		return false
	}
	if query.Username != "" && !strings.Contains(strings.ToLower(event.Username), strings.ToLower(query.Username)) {
		return false
	}

	// IP filter
	if query.IP != "" && !strings.Contains(event.IP, query.IP) {
		return false
	}

	// Code filter
	if query.Code != "" && !strings.HasPrefix(event.Code, query.Code) {
		return false
	}

	// Category filter
	if query.Category != "" && event.Category != query.Category {
		return false
	}

	// Time filter
	if !query.From.IsZero() && event.Timestamp.Before(query.From) {
		return false
	}
	if !query.To.IsZero() && event.Timestamp.After(query.To) {
		return false
	}

	return true
}

func (al *AuditLogger) getAllEvents(from, to time.Time) []AuditEvent {
	events := []AuditEvent{}

	// Add events from memory
	for _, event := range al.events {
		if (from.IsZero() || event.Timestamp.After(from)) &&
			(to.IsZero() || event.Timestamp.Before(to)) {
			events = append(events, event)
		}
	}

	// Load events from files
	if !from.IsZero() || !to.IsZero() {
		files, _ := filepath.Glob(filepath.Join(al.dataPath, "audit-*.json"))
		for _, file := range files {
			// Check if file is in date range
			base := filepath.Base(file)
			dateStr := strings.TrimPrefix(strings.TrimSuffix(base, ".json"), "audit-")
			fileDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}

			// Skip files outside date range
			if !from.IsZero() && fileDate.Before(from.Truncate(24*time.Hour)) {
				continue
			}
			if !to.IsZero() && fileDate.After(to.Add(24*time.Hour)) {
				continue
			}

			fileEvents := al.loadEventsFromFile(file)
			for _, event := range fileEvents {
				if (from.IsZero() || event.Timestamp.After(from)) &&
					(to.IsZero() || event.Timestamp.Before(to)) {
					events = append(events, event)
				}
			}
		}
	}

	return events
}

func (al *AuditLogger) loadEventsFromFile(filename string) []AuditEvent {
	events := []AuditEvent{}

	data, err := os.ReadFile(filename)
	if err != nil {
		return events
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}

	return events
}

func (al *AuditLogger) loadRecentEvents() {
	// Load today's events into memory
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(al.dataPath, fmt.Sprintf("audit-%s.json", today))

	al.events = al.loadEventsFromFile(filename)

	// Keep only last 1000
	if len(al.events) > 1000 {
		al.events = al.events[len(al.events)-1000:]
	}
}

func (al *AuditLogger) rotationRoutine() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		al.rotate()
	}
}

func (al *AuditLogger) rotate() {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Close current file
	if al.currentFile != nil {
		al.currentFile.Close()
		al.currentFile = nil
	}

	// Clean old files (keep 90 days)
	cutoff := time.Now().AddDate(0, 0, -90)
	files, _ := filepath.Glob(filepath.Join(al.dataPath, "audit-*.json"))

	for _, file := range files {
		base := filepath.Base(file)
		dateStr := strings.TrimPrefix(strings.TrimSuffix(base, ".json"), "audit-")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if fileDate.Before(cutoff) {
			os.Remove(file)
			al.logger.Info().
				Str("file", file).
				Msg("Removed old audit file")
		}
	}
}

// Close closes the audit logger
func (al *AuditLogger) Close() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.currentFile != nil {
		al.currentFile.Close()
	}
}
