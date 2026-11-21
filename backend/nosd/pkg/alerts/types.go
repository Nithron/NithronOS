package alerts

import (
	"time"
)

// Severity levels for alerts
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AlertRule defines a monitoring alert rule
type AlertRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`

	// Condition
	Metric    string            `json:"metric"`
	Operator  string            `json:"operator"` // >, <, ==, !=
	Threshold float64           `json:"threshold"`
	Duration  time.Duration     `json:"duration"` // How long condition must be true
	Filters   map[string]string `json:"filters,omitempty"`

	// Alert properties
	Severity   Severity      `json:"severity"`
	Cooldown   time.Duration `json:"cooldown"`             // Min time between alerts
	Hysteresis float64       `json:"hysteresis,omitempty"` // % change needed to clear

	// Notification
	Channels []string `json:"channels"`           // Channel IDs to notify
	Template string   `json:"template,omitempty"` // Custom message template

	// Metadata
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastFired   *time.Time `json:"last_fired,omitempty"`
	LastCleared *time.Time `json:"last_cleared,omitempty"`

	// State
	CurrentState RuleState `json:"current_state"`
}

// RuleState represents the current state of a rule
type RuleState struct {
	Firing      bool      `json:"firing"`
	Since       time.Time `json:"since,omitempty"`
	Value       float64   `json:"value"`
	LastChecked time.Time `json:"last_checked"`
}

// NotificationChannel defines a notification destination
type NotificationChannel struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Type    string                 `json:"type"` // email, webhook, ntfy
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`

	// Rate limiting
	RateLimit  int         `json:"rate_limit,omitempty"` // Max alerts per hour
	QuietHours *QuietHours `json:"quiet_hours,omitempty"`

	// Metadata
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

// QuietHours defines when notifications should be suppressed
type QuietHours struct {
	Enabled   bool   `json:"enabled"`
	StartTime string `json:"start_time"` // HH:MM format
	EndTime   string `json:"end_time"`   // HH:MM format
	Weekends  bool   `json:"weekends"`   // Apply to weekends
}

// AlertEvent represents a fired alert
type AlertEvent struct {
	ID       string   `json:"id"`
	RuleID   string   `json:"rule_id"`
	RuleName string   `json:"rule_name"`
	Severity Severity `json:"severity"`
	State    string   `json:"state"` // firing, cleared

	// Alert details
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`

	// Timestamps
	FiredAt   time.Time  `json:"fired_at"`
	ClearedAt *time.Time `json:"cleared_at,omitempty"`

	// Notification status
	Notified    bool     `json:"notified"`
	Channels    []string `json:"channels,omitempty"`
	NotifyError string   `json:"notify_error,omitempty"`
}

// EmailConfig holds email channel configuration
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUser     string   `json:"smtp_user,omitempty"`
	SMTPPassword string   `json:"smtp_password,omitempty"`
	UseTLS       bool     `json:"use_tls"`
	UseSTARTTLS  bool     `json:"use_starttls"`
	From         string   `json:"from"`
	To           []string `json:"to"`
	Subject      string   `json:"subject,omitempty"`
}

// WebhookConfig holds webhook channel configuration
type WebhookConfig struct {
	URL      string            `json:"url"`
	Method   string            `json:"method,omitempty"` // Default: POST
	Headers  map[string]string `json:"headers,omitempty"`
	Template string            `json:"template,omitempty"` // JSON template
	Secret   string            `json:"secret,omitempty"`   // For HMAC signing
}

// NtfyConfig holds ntfy channel configuration
type NtfyConfig struct {
	ServerURL string   `json:"server_url"`
	Topic     string   `json:"topic"`
	Priority  int      `json:"priority,omitempty"` // 1-5
	Tags      []string `json:"tags,omitempty"`
	Username  string   `json:"username,omitempty"`
	Password  string   `json:"password,omitempty"`
	Token     string   `json:"token,omitempty"`
}

// NotificationMessage represents a notification to be sent
type NotificationMessage struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Severity  Severity  `json:"severity"`
	Timestamp time.Time `json:"timestamp"`

	// Alert details
	RuleName  string  `json:"rule_name"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	State     string  `json:"state"`

	// Additional context
	Hostname string            `json:"hostname"`
	Labels   map[string]string `json:"labels,omitempty"`
	URL      string            `json:"url,omitempty"`
}

// PredefinedRules returns common alert rules
func PredefinedRules() []AlertRule {
	return []AlertRule{
		{
			Name:        "High CPU Usage",
			Description: "CPU usage exceeds 90% for 5 minutes",
			Metric:      "cpu",
			Operator:    ">",
			Threshold:   90,
			Duration:    5 * time.Minute,
			Severity:    SeverityWarning,
			Cooldown:    15 * time.Minute,
		},
		{
			Name:        "High Memory Usage",
			Description: "Memory usage exceeds 90% for 2 minutes",
			Metric:      "memory",
			Operator:    ">",
			Threshold:   90,
			Duration:    2 * time.Minute,
			Severity:    SeverityWarning,
			Cooldown:    15 * time.Minute,
		},
		{
			Name:        "Disk Space Critical",
			Description: "Root filesystem usage exceeds 85%",
			Metric:      "disk_space",
			Operator:    ">",
			Threshold:   85,
			Duration:    1 * time.Minute,
			Severity:    SeverityCritical,
			Cooldown:    30 * time.Minute,
			Filters:     map[string]string{"mountpoint": "/"},
		},
		{
			Name:        "Service Down",
			Description: "Critical service is not running",
			Metric:      "service_health",
			Operator:    "==",
			Threshold:   0,
			Duration:    1 * time.Minute,
			Severity:    SeverityCritical,
			Cooldown:    5 * time.Minute,
		},
		{
			Name:        "SMART Failure",
			Description: "Disk SMART health check failed",
			Metric:      "disk_smart",
			Operator:    "==",
			Threshold:   0,
			Duration:    1 * time.Minute,
			Severity:    SeverityCritical,
			Cooldown:    60 * time.Minute,
		},
		{
			Name:        "High Disk Temperature",
			Description: "Disk temperature exceeds 60°C",
			Metric:      "disk_temp",
			Operator:    ">",
			Threshold:   60,
			Duration:    5 * time.Minute,
			Severity:    SeverityWarning,
			Cooldown:    30 * time.Minute,
		},
		{
			Name:        "Backup Job Failed",
			Description: "Backup job has failed",
			Metric:      "backup_jobs",
			Operator:    "==",
			Threshold:   0,
			Duration:    1 * time.Minute,
			Severity:    SeverityWarning,
			Cooldown:    60 * time.Minute,
			Filters:     map[string]string{"state": "failed"},
		},
		{
			Name:        "Btrfs Errors",
			Description: "Btrfs filesystem has errors",
			Metric:      "btrfs_errors",
			Operator:    ">",
			Threshold:   0,
			Duration:    1 * time.Minute,
			Severity:    SeverityCritical,
			Cooldown:    60 * time.Minute,
		},
	}
}
