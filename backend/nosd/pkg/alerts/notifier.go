package alerts

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rs/zerolog"
)

// Notifier handles sending notifications
type Notifier struct {
	logger     zerolog.Logger
	httpClient *http.Client
}

// NewNotifier creates a new notifier
func NewNotifier(logger zerolog.Logger) *Notifier {
	return &Notifier{
		logger: logger.With().Str("component", "notifier").Logger(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends a notification through a channel
func (n *Notifier) Send(channel *NotificationChannel, msg NotificationMessage) error {
	// Check quiet hours
	if n.isQuietHours(channel.QuietHours) {
		n.logger.Debug().Str("channel", channel.Name).Msg("Skipping notification during quiet hours")
		return nil
	}

	// Check rate limit
	if !n.checkRateLimit(channel) {
		return fmt.Errorf("rate limit exceeded")
	}

	// Send based on channel type
	var err error
	switch channel.Type {
	case "email":
		err = n.sendEmail(channel, msg)
	case "webhook":
		err = n.sendWebhook(channel, msg)
	case "ntfy":
		err = n.sendNtfy(channel, msg)
	default:
		err = fmt.Errorf("unsupported channel type: %s", channel.Type)
	}

	// Update channel metadata
	now := time.Now()
	channel.LastUsed = &now
	if err != nil {
		channel.LastError = err.Error()
	} else {
		channel.LastError = ""
	}

	return err
}

// sendEmail sends an email notification
func (n *Notifier) sendEmail(channel *NotificationChannel, msg NotificationMessage) error {
	config := EmailConfig{}
	if err := n.mapConfig(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}

	// Build email
	subject := config.Subject
	if subject == "" {
		subject = msg.Title
	}

	body := n.formatEmailBody(msg)

	// Build message
	var message bytes.Buffer
	message.WriteString("From: " + config.From + "\r\n")
	message.WriteString("To: " + strings.Join(config.To, ", ") + "\r\n")
	message.WriteString("Subject: " + subject + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)

	var auth smtp.Auth
	if config.SMTPUser != "" && config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)
	}

	// Send email
	var err error
	if config.UseTLS || config.SMTPPort == 465 {
		// TLS connection
		err = n.sendEmailTLS(addr, auth, config.From, config.To, message.Bytes())
	} else if config.UseSTARTTLS || config.SMTPPort == 587 {
		// STARTTLS
		err = smtp.SendMail(addr, auth, config.From, config.To, message.Bytes())
	} else {
		// Plain SMTP
		err = smtp.SendMail(addr, nil, config.From, config.To, message.Bytes())
	}

	return err
}

// sendEmailTLS sends email over TLS
func (n *Notifier) sendEmailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: strings.Split(addr, ":")[0],
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, tlsConfig.ServerName)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}

// formatEmailBody formats the email body
func (n *Notifier) formatEmailBody(msg NotificationMessage) string {
	severityColor := "#17a2b8" // info
	switch msg.Severity {
	case SeverityWarning:
		severityColor = "#ffc107"
	case SeverityCritical:
		severityColor = "#dc3545"
	}

	stateIcon := "⚠️"
	if msg.State == "cleared" {
		stateIcon = "✅"
		severityColor = "#28a745"
	}

	htmlTemplate := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: {{.SeverityColor}}; color: white; padding: 15px; border-radius: 5px 5px 0 0; }
        .content { background-color: #f8f9fa; padding: 20px; border: 1px solid #dee2e6; border-top: none; }
        .metric { display: inline-block; margin: 10px 20px 10px 0; }
        .metric-label { color: #6c757d; font-size: 12px; }
        .metric-value { font-size: 18px; font-weight: bold; }
        .footer { margin-top: 20px; padding-top: 20px; border-top: 1px solid #dee2e6; color: #6c757d; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>{{.StateIcon}} {{.Title}}</h2>
        </div>
        <div class="content">
            <p>{{.Body}}</p>
            
            <div class="metrics">
                <div class="metric">
                    <div class="metric-label">Metric</div>
                    <div class="metric-value">{{.Metric}}</div>
                </div>
                <div class="metric">
                    <div class="metric-label">Value</div>
                    <div class="metric-value">{{.Value}}</div>
                </div>
                <div class="metric">
                    <div class="metric-label">Threshold</div>
                    <div class="metric-value">{{.Threshold}}</div>
                </div>
            </div>
            
            <div class="footer">
                <p>Host: {{.Hostname}}<br>
                Time: {{.Timestamp}}</p>
            </div>
        </div>
    </div>
</body>
</html>
`

	tmpl, err := template.New("email").Parse(htmlTemplate)
	if err != nil {
		return msg.Body
	}

	data := map[string]interface{}{
		"StateIcon":     stateIcon,
		"SeverityColor": severityColor,
		"Title":         msg.Title,
		"Body":          msg.Body,
		"Metric":        msg.Metric,
		"Value":         fmt.Sprintf("%.2f", msg.Value),
		"Threshold":     fmt.Sprintf("%.2f", msg.Threshold),
		"Hostname":      msg.Hostname,
		"Timestamp":     msg.Timestamp.Format(time.RFC3339),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return msg.Body
	}

	return buf.String()
}

// sendWebhook sends a webhook notification
func (n *Notifier) sendWebhook(channel *NotificationChannel, msg NotificationMessage) error {
	config := WebhookConfig{}
	if err := n.mapConfig(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}

	// Prepare payload
	var payload []byte
	var err error

	if config.Template != "" {
		// Use custom template
		tmpl, err := template.New("webhook").Parse(config.Template)
		if err != nil {
			return fmt.Errorf("invalid template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, msg); err != nil {
			return fmt.Errorf("template execution failed: %w", err)
		}

		payload = buf.Bytes()
	} else {
		// Default JSON payload
		payload, err = json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	// Create request
	method := config.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// Add HMAC signature if secret is provided
	if config.Secret != "" {
		h := hmac.New(sha256.New, []byte(config.Secret))
		h.Write(payload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Signature", signature)
	}

	// Send request
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// sendNtfy sends a notification via ntfy
func (n *Notifier) sendNtfy(channel *NotificationChannel, msg NotificationMessage) error {
	config := NtfyConfig{}
	if err := n.mapConfig(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid ntfy config: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/%s", strings.TrimRight(config.ServerURL, "/"), config.Topic)

	// Create request
	req, err := http.NewRequest("POST", url, strings.NewReader(msg.Body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Title", msg.Title)

	// Priority
	priority := config.Priority
	if priority == 0 {
		switch msg.Severity {
		case SeverityCritical:
			priority = 5
		case SeverityWarning:
			priority = 4
		default:
			priority = 3
		}
	}
	req.Header.Set("Priority", strconv.Itoa(priority))

	// Tags
	tags := config.Tags
	if len(tags) == 0 {
		tags = []string{string(msg.Severity)}
		if msg.State == "cleared" {
			tags = append(tags, "white_check_mark")
		} else {
			tags = append(tags, "warning")
		}
	}
	req.Header.Set("Tags", strings.Join(tags, ","))

	// Authentication
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	} else if config.Username != "" && config.Password != "" {
		req.SetBasicAuth(config.Username, config.Password)
	}

	// Send request
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}

// isQuietHours checks if current time is within quiet hours
func (n *Notifier) isQuietHours(qh *QuietHours) bool {
	if qh == nil || !qh.Enabled {
		return false
	}

	now := time.Now()

	// Check weekends
	if qh.Weekends {
		weekday := now.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return false // Not quiet on weekends
		}
	}

	// Parse quiet hours
	startTime, err := time.Parse("15:04", qh.StartTime)
	if err != nil {
		return false
	}

	endTime, err := time.Parse("15:04", qh.EndTime)
	if err != nil {
		return false
	}

	// Adjust to today's date
	startTime = time.Date(now.Year(), now.Month(), now.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, now.Location())
	endTime = time.Date(now.Year(), now.Month(), now.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, now.Location())

	// Handle overnight quiet hours
	if endTime.Before(startTime) {
		// Quiet hours span midnight
		return now.After(startTime) || now.Before(endTime)
	}

	return now.After(startTime) && now.Before(endTime)
}

// checkRateLimit checks if rate limit is exceeded
func (n *Notifier) checkRateLimit(channel *NotificationChannel) bool {
	if channel.RateLimit <= 0 {
		return true // No rate limit
	}

	// Simple rate limit check based on last used time
	if channel.LastUsed != nil {
		timeSinceLastUse := time.Since(*channel.LastUsed)
		minInterval := time.Hour / time.Duration(channel.RateLimit)

		if timeSinceLastUse < minInterval {
			return false
		}
	}

	return true
}

// mapConfig maps interface config to struct
func (n *Notifier) mapConfig(src map[string]interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dst)
}
