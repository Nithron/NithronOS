package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"nos/backend/nosd/pkg/monitor"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Engine manages alert rules and notifications
type Engine struct {
	logger     zerolog.Logger
	configPath string
	collector  *monitor.Collector
	storage    *monitor.TimeSeriesStorage
	notifier   *Notifier

	rules    map[string]*AlertRule
	channels map[string]*NotificationChannel
	events   []AlertEvent

	mu         sync.RWMutex
	cancel     context.CancelFunc
	evalTicker *time.Ticker
}

// NewEngine creates a new alerts engine
func NewEngine(logger zerolog.Logger, configPath string, collector *monitor.Collector, storage *monitor.TimeSeriesStorage) *Engine {
	return &Engine{
		logger:     logger.With().Str("component", "alerts-engine").Logger(),
		configPath: configPath,
		collector:  collector,
		storage:    storage,
		notifier:   NewNotifier(logger),
		rules:      make(map[string]*AlertRule),
		channels:   make(map[string]*NotificationChannel),
		events:     []AlertEvent{},
	}
}

// Start begins alert evaluation
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info().Msg("Starting alerts engine")

	// Create config directory
	if err := os.MkdirAll(e.configPath, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load configuration
	if err := e.loadConfig(); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to load alerts config")
	}

	// If no rules exist, create defaults
	if len(e.rules) == 0 {
		e.createDefaultRules()
	}

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	// Start evaluation loop
	e.evalTicker = time.NewTicker(30 * time.Second)
	go e.evaluationLoop(ctx)

	return nil
}

// Stop halts alert evaluation
func (e *Engine) Stop() error {
	e.logger.Info().Msg("Stopping alerts engine")

	if e.cancel != nil {
		e.cancel()
	}

	if e.evalTicker != nil {
		e.evalTicker.Stop()
	}

	// Save configuration
	return e.saveConfig()
}

// evaluationLoop runs rule evaluation
func (e *Engine) evaluationLoop(ctx context.Context) {
	// Evaluate immediately
	e.evaluateRules()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.evalTicker.C:
			e.evaluateRules()
		}
	}
}

// evaluateRules evaluates all enabled rules
func (e *Engine) evaluateRules() {
	e.mu.RLock()
	rules := make([]*AlertRule, 0, len(e.rules))
	for _, rule := range e.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	e.mu.RUnlock()

	for _, rule := range rules {
		e.evaluateRule(rule)
	}
}

// evaluateRule evaluates a single rule
func (e *Engine) evaluateRule(rule *AlertRule) {
	// Get current metric value
	value, err := e.getMetricValue(rule.Metric, rule.Filters)
	if err != nil {
		e.logger.Error().Err(err).Str("rule", rule.Name).Msg("Failed to get metric value")
		return
	}

	// Check condition
	conditionMet := e.checkCondition(value, rule.Operator, rule.Threshold)

	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	// Update rule state
	previousState := rule.CurrentState.Firing
	rule.CurrentState.Value = value
	rule.CurrentState.LastChecked = now

	if conditionMet {
		if !rule.CurrentState.Firing {
			// Condition just became true
			rule.CurrentState.Since = now
		}

		// Check if duration requirement is met
		if now.Sub(rule.CurrentState.Since) >= rule.Duration {
			if !previousState {
				// Fire alert
				e.fireAlert(rule, value)
			}
			rule.CurrentState.Firing = true
		}
	} else {
		// Check hysteresis for clearing
		clearThreshold := rule.Threshold
		if rule.Hysteresis > 0 {
			switch rule.Operator {
			case ">":
				clearThreshold = rule.Threshold * (1 - rule.Hysteresis/100)
			case "<":
				clearThreshold = rule.Threshold * (1 + rule.Hysteresis/100)
			}
		}

		if !e.checkCondition(value, rule.Operator, clearThreshold) {
			if previousState {
				// Clear alert
				e.clearAlert(rule, value)
			}
			rule.CurrentState.Firing = false
			rule.CurrentState.Since = time.Time{}
		}
	}
}

// getMetricValue retrieves the current value for a metric
func (e *Engine) getMetricValue(metric string, filters map[string]string) (float64, error) {
	// Try to get from last known values first
	switch metric {
	case "cpu":
		if val, ok := e.collector.GetLastValue("cpu_percent"); ok {
			if f, ok := val.(float64); ok {
				return f, nil
			}
		}
	case "memory":
		if val, ok := e.collector.GetLastValue("memory_percent"); ok {
			if f, ok := val.(float64); ok {
				return f, nil
			}
		}
	case "disk_space":
		key := "disk_root_percent"
		if mp, ok := filters["mountpoint"]; ok {
			key = fmt.Sprintf("disk_%s_percent", mp)
		}
		if val, ok := e.collector.GetLastValue(key); ok {
			if f, ok := val.(float64); ok {
				return f, nil
			}
		}
	case "service_health":
		// Query from time series
		query := monitor.TimeSeriesQuery{
			Metric:    monitor.MetricTypeServiceHealth,
			StartTime: time.Now().Add(-2 * time.Minute),
			EndTime:   time.Now(),
			Filters:   filters,
		}
		ts, err := e.storage.Query(query)
		if err == nil && len(ts.DataPoints) > 0 {
			return ts.DataPoints[len(ts.DataPoints)-1].Value, nil
		}
	}

	// Fall back to querying time series
	query := monitor.TimeSeriesQuery{
		Metric:    monitor.MetricType(metric),
		StartTime: time.Now().Add(-5 * time.Minute),
		EndTime:   time.Now(),
		Filters:   filters,
	}

	ts, err := e.storage.Query(query)
	if err != nil {
		return 0, err
	}

	if len(ts.DataPoints) == 0 {
		return 0, fmt.Errorf("no data points found")
	}

	// Return most recent value
	return ts.DataPoints[len(ts.DataPoints)-1].Value, nil
}

// checkCondition evaluates a condition
func (e *Engine) checkCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

// fireAlert fires an alert
func (e *Engine) fireAlert(rule *AlertRule, value float64) {
	now := time.Now()

	// Check cooldown
	if rule.LastFired != nil && now.Sub(*rule.LastFired) < rule.Cooldown {
		return
	}

	event := AlertEvent{
		ID:        uuid.New().String(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		State:     "firing",
		Metric:    rule.Metric,
		Value:     value,
		Threshold: rule.Threshold,
		FiredAt:   now,
		Message:   e.formatMessage(rule, value, "firing"),
	}

	// Send notifications
	if len(rule.Channels) > 0 {
		msg := e.createNotificationMessage(rule, value, "firing")
		for _, channelID := range rule.Channels {
			if channel, ok := e.channels[channelID]; ok && channel.Enabled {
				if err := e.notifier.Send(channel, msg); err != nil {
					e.logger.Error().Err(err).Str("channel", channelID).Msg("Failed to send notification")
					event.NotifyError = err.Error()
				} else {
					event.Notified = true
					event.Channels = append(event.Channels, channelID)
				}
			}
		}
	}

	// Update rule
	rule.LastFired = &now

	// Store event
	e.events = append(e.events, event)
	if len(e.events) > 1000 {
		e.events = e.events[len(e.events)-1000:]
	}

	e.logger.Warn().
		Str("rule", rule.Name).
		Float64("value", value).
		Float64("threshold", rule.Threshold).
		Msg("Alert fired")
}

// clearAlert clears an alert
func (e *Engine) clearAlert(rule *AlertRule, value float64) {
	now := time.Now()

	event := AlertEvent{
		ID:        uuid.New().String(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		State:     "cleared",
		Metric:    rule.Metric,
		Value:     value,
		Threshold: rule.Threshold,
		FiredAt:   now,
		ClearedAt: &now,
		Message:   e.formatMessage(rule, value, "cleared"),
	}

	// Send recovery notifications
	if len(rule.Channels) > 0 {
		msg := e.createNotificationMessage(rule, value, "cleared")
		for _, channelID := range rule.Channels {
			if channel, ok := e.channels[channelID]; ok && channel.Enabled {
				if err := e.notifier.Send(channel, msg); err != nil {
					e.logger.Error().Err(err).Str("channel", channelID).Msg("Failed to send recovery notification")
				} else {
					event.Notified = true
					event.Channels = append(event.Channels, channelID)
				}
			}
		}
	}

	// Update rule
	rule.LastCleared = &now

	// Store event
	e.events = append(e.events, event)

	e.logger.Info().
		Str("rule", rule.Name).
		Float64("value", value).
		Msg("Alert cleared")
}

// formatMessage formats an alert message
func (e *Engine) formatMessage(rule *AlertRule, value float64, state string) string {
	if rule.Template != "" {
		// Fields: .Name .Metric .Value .Threshold .State .Host
		if msg, err := renderAlertTemplate(rule, value, state); err == nil {
			return msg
		}
		return rule.Template
	}

	if state == "firing" {
		return fmt.Sprintf("%s: %s %.2f (threshold: %.2f)",
			rule.Name, rule.Metric, value, rule.Threshold)
	}

	return fmt.Sprintf("%s: CLEARED - %s %.2f (threshold: %.2f)",
		rule.Name, rule.Metric, value, rule.Threshold)
}

// createNotificationMessage creates a notification message
func (e *Engine) createNotificationMessage(rule *AlertRule, value float64, state string) NotificationMessage {
	hostname, _ := os.Hostname()

	title := fmt.Sprintf("[%s] %s", rule.Severity, rule.Name)
	if state == "cleared" {
		title = fmt.Sprintf("[CLEARED] %s", rule.Name)
	}

	return NotificationMessage{
		Title:     title,
		Body:      e.formatMessage(rule, value, state),
		Severity:  rule.Severity,
		Timestamp: time.Now(),
		RuleName:  rule.Name,
		Metric:    rule.Metric,
		Value:     value,
		Threshold: rule.Threshold,
		State:     state,
		Hostname:  hostname,
		Labels:    rule.Filters,
	}
}

// Rule management

// CreateRule creates a new alert rule
func (e *Engine) CreateRule(rule *AlertRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.CurrentState = RuleState{}

	e.rules[rule.ID] = rule

	return e.saveConfig()
}

// UpdateRule updates an existing rule
func (e *Engine) UpdateRule(id string, update *AlertRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	// Preserve certain fields
	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()
	update.CurrentState = existing.CurrentState
	update.LastFired = existing.LastFired
	update.LastCleared = existing.LastCleared

	e.rules[id] = update

	return e.saveConfig()
}

// DeleteRule deletes a rule
func (e *Engine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(e.rules, id)

	return e.saveConfig()
}

// GetRule returns a rule by ID
func (e *Engine) GetRule(id string) (*AlertRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	return rule, nil
}

// ListRules returns all rules
func (e *Engine) ListRules() []*AlertRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}

	return rules
}

// Channel management

// CreateChannel creates a notification channel
func (e *Engine) CreateChannel(channel *NotificationChannel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if channel.ID == "" {
		channel.ID = uuid.New().String()
	}

	now := time.Now()
	channel.CreatedAt = now
	channel.UpdatedAt = now

	e.channels[channel.ID] = channel

	return e.saveConfig()
}

// UpdateChannel updates a channel
func (e *Engine) UpdateChannel(id string, update *NotificationChannel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.channels[id]
	if !ok {
		return fmt.Errorf("channel not found: %s", id)
	}

	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()

	e.channels[id] = update

	return e.saveConfig()
}

// DeleteChannel deletes a channel
func (e *Engine) DeleteChannel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.channels[id]; !ok {
		return fmt.Errorf("channel not found: %s", id)
	}

	delete(e.channels, id)

	return e.saveConfig()
}

// GetChannel returns a channel by ID
func (e *Engine) GetChannel(id string) (*NotificationChannel, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	channel, ok := e.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", id)
	}

	return channel, nil
}

// ListChannels returns all channels
func (e *Engine) ListChannels() []*NotificationChannel {
	e.mu.RLock()
	defer e.mu.RUnlock()

	channels := make([]*NotificationChannel, 0, len(e.channels))
	for _, channel := range e.channels {
		channels = append(channels, channel)
	}

	return channels
}

// TestChannel sends a test notification
func (e *Engine) TestChannel(id string) error {
	e.mu.RLock()
	channel, ok := e.channels[id]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channel not found: %s", id)
	}

	hostname, _ := os.Hostname()
	msg := NotificationMessage{
		Title:     "Test Alert",
		Body:      "This is a test notification from nOS monitoring.",
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
		Hostname:  hostname,
	}

	return e.notifier.Send(channel, msg)
}

// Events

// ListEvents returns alert events
func (e *Engine) ListEvents(limit int) []AlertEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.events) {
		limit = len(e.events)
	}

	// Return most recent events
	start := len(e.events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AlertEvent, limit)
	copy(result, e.events[start:])

	// Reverse to get newest first
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// Configuration

// loadConfig loads rules and channels from disk
func (e *Engine) loadConfig() error {
	// Load rules
	rulesPath := filepath.Join(e.configPath, "rules.json")
	if data, err := os.ReadFile(rulesPath); err == nil {
		var rules map[string]*AlertRule
		if err := json.Unmarshal(data, &rules); err == nil {
			e.rules = rules
		}
	}

	// Load channels
	channelsPath := filepath.Join(e.configPath, "channels.json")
	if data, err := os.ReadFile(channelsPath); err == nil {
		var channels map[string]*NotificationChannel
		if err := json.Unmarshal(data, &channels); err == nil {
			e.channels = channels
		}
	}

	// Load events
	eventsPath := filepath.Join(e.configPath, "events.json")
	if data, err := os.ReadFile(eventsPath); err == nil {
		var events []AlertEvent
		if err := json.Unmarshal(data, &events); err == nil {
			e.events = events
		}
	}

	return nil
}

// saveConfig saves rules and channels to disk
func (e *Engine) saveConfig() error {
	// Save rules
	rulesPath := filepath.Join(e.configPath, "rules.json")
	if data, err := json.MarshalIndent(e.rules, "", "  "); err == nil {
		if err := os.WriteFile(rulesPath, data, 0600); err != nil {
			return err
		}
	}

	// Save channels
	channelsPath := filepath.Join(e.configPath, "channels.json")
	if data, err := json.MarshalIndent(e.channels, "", "  "); err == nil {
		if err := os.WriteFile(channelsPath, data, 0600); err != nil {
			return err
		}
	}

	// Save recent events (last 100)
	eventsToSave := e.events
	if len(eventsToSave) > 100 {
		eventsToSave = e.events[len(e.events)-100:]
	}

	eventsPath := filepath.Join(e.configPath, "events.json")
	if data, err := json.MarshalIndent(eventsToSave, "", "  "); err == nil {
		_ = os.WriteFile(eventsPath, data, 0600)
	}

	return nil
}

// createDefaultRules creates default alert rules
func (e *Engine) createDefaultRules() {
	defaults := PredefinedRules()

	for _, rule := range defaults {
		rule.ID = uuid.New().String()
		rule.Enabled = false // Start disabled
		now := time.Now()
		rule.CreatedAt = now
		rule.UpdatedAt = now
		e.rules[rule.ID] = &rule
	}

	_ = e.saveConfig()
}

// renderAlertTemplate fills a rule's message template (text/template, so a
// template can only print the fields it is given).
func renderAlertTemplate(rule *AlertRule, value float64, state string) (string, error) {
	t, err := template.New("alert").Option("missingkey=error").Parse(rule.Template)
	if err != nil {
		return "", err
	}
	host, _ := os.Hostname()
	var b strings.Builder
	err = t.Execute(&b, map[string]any{
		"Name": rule.Name, "Metric": rule.Metric, "Value": fmt.Sprintf("%.2f", value),
		"Threshold": fmt.Sprintf("%.2f", rule.Threshold), "State": state, "Host": host,
	})
	return b.String(), err
}
