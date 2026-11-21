package webhooks

import (
	"time"
)

// EventType defines types of events that can trigger webhooks
type EventType string

const (
	// App events
	EventAppInstall      EventType = "app.install"
	EventAppUpdate       EventType = "app.update"
	EventAppRemove       EventType = "app.remove"
	EventAppStart        EventType = "app.start"
	EventAppStop         EventType = "app.stop"
	EventAppHealthChange EventType = "app.health.change"

	// Backup events
	EventBackupStarted   EventType = "backup.started"
	EventBackupFinished  EventType = "backup.finished"
	EventBackupFailed    EventType = "backup.failed"
	EventRestoreStarted  EventType = "restore.started"
	EventRestoreFinished EventType = "restore.finished"
	EventRestoreFailed   EventType = "restore.failed"

	// Alert events
	EventAlertFired   EventType = "alert.fired"
	EventAlertCleared EventType = "alert.cleared"

	// System events
	EventSystemStartup   EventType = "system.startup"
	EventSystemShutdown  EventType = "system.shutdown"
	EventServiceChange   EventType = "system.service.change"
	EventUpdateAvailable EventType = "system.update.available"
	EventUpdateApplied   EventType = "system.update.applied"

	// Storage events
	EventPoolCreated     EventType = "storage.pool.created"
	EventPoolDeleted     EventType = "storage.pool.deleted"
	EventPoolDegraded    EventType = "storage.pool.degraded"
	EventSnapshotCreated EventType = "storage.snapshot.created"
	EventSnapshotDeleted EventType = "storage.snapshot.deleted"

	// Auth events
	EventAuthLogin   EventType = "auth.login"
	EventAuthLogout  EventType = "auth.logout"
	EventAuthFailed  EventType = "auth.failed"
	EventUserCreated EventType = "auth.user.created"
	EventUserDeleted EventType = "auth.user.deleted"
	EventUserLocked  EventType = "auth.user.locked"
)

// Webhook represents a webhook endpoint configuration
type Webhook struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Configuration
	Events  []EventType       `json:"events"`
	Enabled bool              `json:"enabled"`
	Secret  string            `json:"-"` // Never expose
	Headers map[string]string `json:"headers,omitempty"`

	// Retry configuration
	MaxRetries int           `json:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay"`
	Timeout    time.Duration `json:"timeout"`

	// Metadata
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastDelivery *time.Time `json:"last_delivery,omitempty"`
	LastStatus   int        `json:"last_status,omitempty"`
	LastError    string     `json:"last_error,omitempty"`

	// Statistics
	DeliveryCount int `json:"delivery_count"`
	SuccessCount  int `json:"success_count"`
	FailureCount  int `json:"failure_count"`
}

// Event represents an event that triggers webhooks
type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// Event data
	Actor    string `json:"actor,omitempty"`  // User or system component
	Target   string `json:"target,omitempty"` // Resource affected
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"` // info, warning, critical

	// Payload
	Data map[string]interface{} `json:"data,omitempty"`

	// Metadata
	Source    string `json:"source"` // Component that generated event
	RequestID string `json:"request_id,omitempty"`
}

// Delivery represents a webhook delivery attempt
type Delivery struct {
	ID        string `json:"id"`
	WebhookID string `json:"webhook_id"`
	EventID   string `json:"event_id"`

	// Delivery details
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Payload []byte            `json:"-"` // Don't expose in JSON

	// Response
	Status   int    `json:"status"`
	Response []byte `json:"-"` // Don't expose in JSON
	Error    string `json:"error,omitempty"`

	// Timing
	AttemptedAt time.Time     `json:"attempted_at"`
	Duration    time.Duration `json:"duration"`

	// Retry info
	Attempt   int        `json:"attempt"`
	NextRetry *time.Time `json:"next_retry,omitempty"`
}

// WebhookPayload is the standard payload sent to webhooks
type WebhookPayload struct {
	Event     Event       `json:"event"`
	Webhook   WebhookInfo `json:"webhook"`
	Timestamp time.Time   `json:"timestamp"`
	Signature string      `json:"-"` // Added as header, not in body
}

// WebhookInfo is minimal webhook info included in payload
type WebhookInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateWebhookRequest for creating new webhooks
type CreateWebhookRequest struct {
	URL         string            `json:"url" validate:"required,url"`
	Name        string            `json:"name" validate:"required,min=1,max=100"`
	Description string            `json:"description,omitempty"`
	Events      []EventType       `json:"events" validate:"required,min=1"`
	Secret      string            `json:"secret,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// UpdateWebhookRequest for updating webhooks
type UpdateWebhookRequest struct {
	URL         *string           `json:"url,omitempty" validate:"omitempty,url"`
	Name        *string           `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Description *string           `json:"description,omitempty"`
	Events      []EventType       `json:"events,omitempty"`
	Secret      *string           `json:"secret,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

// DeliveryStatus represents the status of webhook deliveries
type DeliveryStatus struct {
	WebhookID            string     `json:"webhook_id"`
	TotalDeliveries      int        `json:"total_deliveries"`
	SuccessfulDeliveries int        `json:"successful_deliveries"`
	FailedDeliveries     int        `json:"failed_deliveries"`
	LastDelivery         *time.Time `json:"last_delivery,omitempty"`
	LastSuccess          *time.Time `json:"last_success,omitempty"`
	LastFailure          *time.Time `json:"last_failure,omitempty"`
	QueuedEvents         int        `json:"queued_events"`
}

// GetEventDescription returns a human-readable description of an event
func GetEventDescription(event EventType) string {
	descriptions := map[EventType]string{
		EventAppInstall:      "Application installed",
		EventAppUpdate:       "Application updated",
		EventAppRemove:       "Application removed",
		EventAppStart:        "Application started",
		EventAppStop:         "Application stopped",
		EventAppHealthChange: "Application health status changed",

		EventBackupStarted:   "Backup started",
		EventBackupFinished:  "Backup completed successfully",
		EventBackupFailed:    "Backup failed",
		EventRestoreStarted:  "Restore started",
		EventRestoreFinished: "Restore completed successfully",
		EventRestoreFailed:   "Restore failed",

		EventAlertFired:   "Alert triggered",
		EventAlertCleared: "Alert cleared",

		EventSystemStartup:   "System started",
		EventSystemShutdown:  "System shutting down",
		EventServiceChange:   "Service state changed",
		EventUpdateAvailable: "System update available",
		EventUpdateApplied:   "System update applied",

		EventPoolCreated:     "Storage pool created",
		EventPoolDeleted:     "Storage pool deleted",
		EventPoolDegraded:    "Storage pool degraded",
		EventSnapshotCreated: "Snapshot created",
		EventSnapshotDeleted: "Snapshot deleted",

		EventAuthLogin:   "User logged in",
		EventAuthLogout:  "User logged out",
		EventAuthFailed:  "Authentication failed",
		EventUserCreated: "User account created",
		EventUserDeleted: "User account deleted",
		EventUserLocked:  "User account locked",
	}

	if desc, ok := descriptions[event]; ok {
		return desc
	}

	return string(event)
}

// GetAllEventTypes returns all available event types
func GetAllEventTypes() []EventType {
	return []EventType{
		EventAppInstall,
		EventAppUpdate,
		EventAppRemove,
		EventAppStart,
		EventAppStop,
		EventAppHealthChange,
		EventBackupStarted,
		EventBackupFinished,
		EventBackupFailed,
		EventRestoreStarted,
		EventRestoreFinished,
		EventRestoreFailed,
		EventAlertFired,
		EventAlertCleared,
		EventSystemStartup,
		EventSystemShutdown,
		EventServiceChange,
		EventUpdateAvailable,
		EventUpdateApplied,
		EventPoolCreated,
		EventPoolDeleted,
		EventPoolDegraded,
		EventSnapshotCreated,
		EventSnapshotDeleted,
		EventAuthLogin,
		EventAuthLogout,
		EventAuthFailed,
		EventUserCreated,
		EventUserDeleted,
		EventUserLocked,
	}
}
