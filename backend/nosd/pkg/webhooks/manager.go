package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Manager handles webhook subscriptions and delivery
type Manager struct {
	logger     zerolog.Logger
	webhooks   map[string]*Webhook
	deliveries map[string][]*Delivery
	eventQueue chan Event
	httpClient *http.Client
	mu         sync.RWMutex

	// Configuration
	maxRetries   int
	retryDelay   time.Duration
	timeout      time.Duration
	maxQueueSize int
}

// NewManager creates a new webhook manager
func NewManager(logger zerolog.Logger) *Manager {
	m := &Manager{
		logger:     logger.With().Str("component", "webhook-manager").Logger(),
		webhooks:   make(map[string]*Webhook),
		deliveries: make(map[string][]*Delivery),
		eventQueue: make(chan Event, 1000),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries:   3,
		retryDelay:   5 * time.Second,
		timeout:      30 * time.Second,
		maxQueueSize: 1000,
	}

	// Start delivery workers
	for i := 0; i < 5; i++ {
		go m.deliveryWorker()
	}

	// Start cleanup routine
	go m.cleanupRoutine()

	return m
}

// CreateWebhook creates a new webhook subscription
func (m *Manager) CreateWebhook(req CreateWebhookRequest) (*Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate events
	if err := m.validateEvents(req.Events); err != nil {
		return nil, err
	}

	// Create webhook
	webhook := &Webhook{
		ID:          uuid.New().String(),
		URL:         req.URL,
		Name:        req.Name,
		Description: req.Description,
		Events:      req.Events,
		Enabled:     req.Enabled,
		Secret:      req.Secret,
		Headers:     req.Headers,
		MaxRetries:  m.maxRetries,
		RetryDelay:  m.retryDelay,
		Timeout:     m.timeout,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store webhook
	m.webhooks[webhook.ID] = webhook

	m.logger.Info().
		Str("webhook_id", webhook.ID).
		Str("name", webhook.Name).
		Str("url", webhook.URL).
		Msg("Webhook created")

	return webhook, nil
}

// UpdateWebhook updates an existing webhook
func (m *Manager) UpdateWebhook(id string, req UpdateWebhookRequest) (*Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	webhook, exists := m.webhooks[id]
	if !exists {
		return nil, fmt.Errorf("webhook not found")
	}

	// Update fields
	if req.URL != nil {
		webhook.URL = *req.URL
	}
	if req.Name != nil {
		webhook.Name = *req.Name
	}
	if req.Description != nil {
		webhook.Description = *req.Description
	}
	if len(req.Events) > 0 {
		if err := m.validateEvents(req.Events); err != nil {
			return nil, err
		}
		webhook.Events = req.Events
	}
	if req.Secret != nil {
		webhook.Secret = *req.Secret
	}
	if req.Headers != nil {
		webhook.Headers = req.Headers
	}
	if req.Enabled != nil {
		webhook.Enabled = *req.Enabled
	}

	webhook.UpdatedAt = time.Now()

	m.logger.Info().
		Str("webhook_id", webhook.ID).
		Str("name", webhook.Name).
		Msg("Webhook updated")

	return webhook, nil
}

// DeleteWebhook deletes a webhook subscription
func (m *Manager) DeleteWebhook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	webhook, exists := m.webhooks[id]
	if !exists {
		return fmt.Errorf("webhook not found")
	}

	// Delete webhook
	delete(m.webhooks, id)

	// Delete delivery history
	delete(m.deliveries, id)

	m.logger.Info().
		Str("webhook_id", id).
		Str("name", webhook.Name).
		Msg("Webhook deleted")

	return nil
}

// GetWebhook returns a webhook by ID
func (m *Manager) GetWebhook(id string) (*Webhook, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	webhook, exists := m.webhooks[id]
	if !exists {
		return nil, fmt.Errorf("webhook not found")
	}

	// Return copy without secret
	webhookCopy := *webhook
	webhookCopy.Secret = ""

	return &webhookCopy, nil
}

// ListWebhooks returns all webhooks
func (m *Manager) ListWebhooks() []*Webhook {
	m.mu.RLock()
	defer m.mu.RUnlock()

	webhooks := make([]*Webhook, 0, len(m.webhooks))
	for _, webhook := range m.webhooks {
		// Copy without secret
		webhookCopy := *webhook
		webhookCopy.Secret = ""
		webhooks = append(webhooks, &webhookCopy)
	}

	return webhooks
}

// PublishEvent publishes an event to trigger webhooks
func (m *Manager) PublishEvent(event Event) error {
	// Set defaults
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Queue event
	select {
	case m.eventQueue <- event:
		m.logger.Debug().
			Str("event_id", event.ID).
			Str("type", string(event.Type)).
			Msg("Event queued for delivery")
		return nil
	default:
		// Queue full, log to dead letter
		m.logDeadLetter(event, "queue full")
		return fmt.Errorf("event queue full")
	}
}

// TestWebhook sends a test event to a webhook
func (m *Manager) TestWebhook(id string) error {
	m.mu.RLock()
	webhook, exists := m.webhooks[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("webhook not found")
	}

	// Create test event
	event := Event{
		ID:        uuid.New().String(),
		Type:      "webhook.test",
		Timestamp: time.Now(),
		Message:   "Test webhook delivery",
		Severity:  "info",
		Source:    "webhook-manager",
		Data: map[string]interface{}{
			"test": true,
		},
	}

	// Deliver directly
	delivery := m.deliver(webhook, event, 1)

	// Update webhook status
	m.mu.Lock()
	webhook.LastDelivery = &delivery.AttemptedAt
	webhook.LastStatus = delivery.Status
	if delivery.Error != "" {
		webhook.LastError = delivery.Error
	} else {
		webhook.LastError = ""
	}
	m.mu.Unlock()

	if delivery.Status >= 200 && delivery.Status < 300 {
		return nil
	}

	return fmt.Errorf("test delivery failed: %s", delivery.Error)
}

// GetDeliveries returns delivery history for a webhook
func (m *Manager) GetDeliveries(webhookID string, limit int) []*Delivery {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deliveries := m.deliveries[webhookID]
	if len(deliveries) == 0 {
		return []*Delivery{}
	}

	// Return most recent deliveries
	if limit <= 0 || limit > len(deliveries) {
		limit = len(deliveries)
	}

	start := len(deliveries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Delivery, limit)
	copy(result, deliveries[start:])

	return result
}

// GetDeliveryStatus returns delivery statistics for a webhook
func (m *Manager) GetDeliveryStatus(webhookID string) (*DeliveryStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	webhook, exists := m.webhooks[webhookID]
	if !exists {
		return nil, fmt.Errorf("webhook not found")
	}

	status := &DeliveryStatus{
		WebhookID:            webhookID,
		TotalDeliveries:      webhook.DeliveryCount,
		SuccessfulDeliveries: webhook.SuccessCount,
		FailedDeliveries:     webhook.FailureCount,
	}

	// Get delivery history
	deliveries := m.deliveries[webhookID]
	if len(deliveries) > 0 {
		lastDelivery := deliveries[len(deliveries)-1]
		status.LastDelivery = &lastDelivery.AttemptedAt

		// Find last success and failure
		for i := len(deliveries) - 1; i >= 0; i-- {
			if deliveries[i].Status >= 200 && deliveries[i].Status < 300 {
				status.LastSuccess = &deliveries[i].AttemptedAt
				break
			}
		}

		for i := len(deliveries) - 1; i >= 0; i-- {
			if deliveries[i].Status < 200 || deliveries[i].Status >= 300 {
				status.LastFailure = &deliveries[i].AttemptedAt
				break
			}
		}
	}

	// Count queued events (approximate)
	status.QueuedEvents = len(m.eventQueue)

	return status, nil
}

// Private methods

func (m *Manager) deliveryWorker() {
	for event := range m.eventQueue {
		m.processEvent(event)
	}
}

func (m *Manager) processEvent(event Event) {
	m.mu.RLock()
	webhooks := m.getMatchingWebhooks(event.Type)
	m.mu.RUnlock()

	for _, webhook := range webhooks {
		if !webhook.Enabled {
			continue
		}

		// Deliver with retries
		m.deliverWithRetry(webhook, event)
	}
}

func (m *Manager) getMatchingWebhooks(eventType EventType) []*Webhook {
	var matching []*Webhook

	for _, webhook := range m.webhooks {
		for _, subscribedEvent := range webhook.Events {
			if subscribedEvent == eventType {
				matching = append(matching, webhook)
				break
			}
		}
	}

	return matching
}

func (m *Manager) deliverWithRetry(webhook *Webhook, event Event) {
	var delivery *Delivery

	for attempt := 1; attempt <= webhook.MaxRetries; attempt++ {
		delivery = m.deliver(webhook, event, attempt)

		// Update webhook stats
		m.mu.Lock()
		webhook.DeliveryCount++
		webhook.LastDelivery = &delivery.AttemptedAt
		webhook.LastStatus = delivery.Status

		if delivery.Status >= 200 && delivery.Status < 300 {
			webhook.SuccessCount++
			webhook.LastError = ""
			m.mu.Unlock()

			// Store successful delivery
			m.storeDelivery(webhook.ID, delivery)

			m.logger.Debug().
				Str("webhook_id", webhook.ID).
				Str("event_id", event.ID).
				Int("status", delivery.Status).
				Msg("Webhook delivered successfully")

			return
		}

		webhook.FailureCount++
		webhook.LastError = delivery.Error
		m.mu.Unlock()

		// Store failed delivery
		m.storeDelivery(webhook.ID, delivery)

		m.logger.Warn().
			Str("webhook_id", webhook.ID).
			Str("event_id", event.ID).
			Int("attempt", attempt).
			Str("error", delivery.Error).
			Msg("Webhook delivery failed")

		// Wait before retry
		if attempt < webhook.MaxRetries {
			time.Sleep(webhook.RetryDelay * time.Duration(attempt))
		}
	}

	// All retries failed, log to dead letter
	m.logDeadLetter(event, fmt.Sprintf("webhook %s failed after %d attempts", webhook.ID, webhook.MaxRetries))
}

func (m *Manager) deliver(webhook *Webhook, event Event, attempt int) *Delivery {
	delivery := &Delivery{
		ID:          uuid.New().String(),
		WebhookID:   webhook.ID,
		EventID:     event.ID,
		URL:         webhook.URL,
		Method:      "POST",
		Headers:     make(map[string]string),
		Attempt:     attempt,
		AttemptedAt: time.Now(),
	}

	// Build payload
	payload := WebhookPayload{
		Event: event,
		Webhook: WebhookInfo{
			ID:   webhook.ID,
			Name: webhook.Name,
		},
		Timestamp: time.Now(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		delivery.Error = fmt.Sprintf("failed to marshal payload: %v", err)
		return delivery
	}

	delivery.Payload = payloadBytes

	// Create request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		delivery.Error = fmt.Sprintf("failed to create request: %v", err)
		return delivery
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nOS-Webhook/1.0")
	req.Header.Set("X-NOS-Event", string(event.Type))
	req.Header.Set("X-NOS-Event-ID", event.ID)
	req.Header.Set("X-NOS-Webhook-ID", webhook.ID)
	req.Header.Set("X-NOS-Delivery-ID", delivery.ID)

	// Add HMAC signature if secret is configured
	if webhook.Secret != "" {
		signature := m.generateSignature(webhook.Secret, payloadBytes)
		req.Header.Set("X-NOS-Signature", signature)
		delivery.Headers["X-NOS-Signature"] = signature
	}

	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
		delivery.Headers[key] = value
	}

	// Send request
	start := time.Now()
	resp, err := m.httpClient.Do(req)
	delivery.Duration = time.Since(start)

	if err != nil {
		delivery.Error = fmt.Sprintf("request failed: %v", err)
		return delivery
	}
	defer resp.Body.Close()

	delivery.Status = resp.StatusCode

	// Read response (limited)
	responseBytes := make([]byte, 1024)
	n, _ := resp.Body.Read(responseBytes)
	delivery.Response = responseBytes[:n]

	// Check status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		delivery.Error = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}

	return delivery
}

func (m *Manager) generateSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func (m *Manager) storeDelivery(webhookID string, delivery *Delivery) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deliveries[webhookID] == nil {
		m.deliveries[webhookID] = []*Delivery{}
	}

	m.deliveries[webhookID] = append(m.deliveries[webhookID], delivery)

	// Keep only last 100 deliveries per webhook
	if len(m.deliveries[webhookID]) > 100 {
		m.deliveries[webhookID] = m.deliveries[webhookID][len(m.deliveries[webhookID])-100:]
	}
}

func (m *Manager) validateEvents(events []EventType) error {
	validEvents := make(map[EventType]bool)
	for _, event := range GetAllEventTypes() {
		validEvents[event] = true
	}

	for _, event := range events {
		if !validEvents[event] {
			return fmt.Errorf("invalid event type: %s", event)
		}
	}

	return nil
}

func (m *Manager) logDeadLetter(event Event, reason string) {
	m.logger.Error().
		Str("event_id", event.ID).
		Str("type", string(event.Type)).
		Str("reason", reason).
		Msg("Event sent to dead letter queue")

	// TODO: Write to dead letter file
}

func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean old deliveries (keep last 7 days)
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	for webhookID, deliveries := range m.deliveries {
		newDeliveries := []*Delivery{}
		for _, delivery := range deliveries {
			if delivery.AttemptedAt.After(cutoff) {
				newDeliveries = append(newDeliveries, delivery)
			}
		}

		if len(newDeliveries) == 0 {
			delete(m.deliveries, webhookID)
		} else {
			m.deliveries[webhookID] = newDeliveries
		}
	}
}
