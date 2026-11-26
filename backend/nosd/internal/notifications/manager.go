package notifications

import (
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nos/backend/nosd/internal/fsatomic"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Notification represents a system notification
type Notification struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`     // info, warning, error, success
	Category  string                 `json:"category"` // system, backup, storage, network, security
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Read      bool                   `json:"read"`
	Timestamp time.Time              `json:"timestamp"`
	Actions   []Action               `json:"actions,omitempty"`
}

// Action represents an action that can be taken on a notification
type Action struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Type  string `json:"type"` // link, dismiss, resolve
}

// Channel represents a notification channel
type Channel struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Type    string                 `json:"type"` // email, webhook, syslog
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
	Filters []Filter               `json:"filters"`
}

// Filter defines what notifications to send to a channel
type Filter struct {
	Type       string   `json:"type,omitempty"`
	Categories []string `json:"categories,omitempty"`
	MinLevel   string   `json:"minLevel,omitempty"` // info, warning, error
}

// Manager handles notifications
type Manager struct {
	storePath     string
	notifications map[string]*Notification
	channels      map[string]*Channel
	subscribers   map[string][]chan *Notification
	mu            sync.RWMutex
}

// NewManager creates a new notification manager
func NewManager(storePath string) (*Manager, error) {
	m := &Manager{
		storePath:     storePath,
		notifications: make(map[string]*Notification),
		channels:      make(map[string]*Channel),
		subscribers:   make(map[string][]chan *Notification),
	}

	// Ensure directory exists
	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, err
	}

	// Load existing data
	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Start cleanup routine
	go m.cleanupOldNotifications()

	return m, nil
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load notifications
	notifPath := filepath.Join(m.storePath, "notifications.json")
	var notifications []*Notification
	if ok, err := fsatomic.LoadJSON(notifPath, &notifications); err != nil {
		return err
	} else if ok {
		for _, n := range notifications {
			m.notifications[n.ID] = n
		}
	}

	// Load channels
	channelsPath := filepath.Join(m.storePath, "channels.json")
	var channels []*Channel
	if ok, err := fsatomic.LoadJSON(channelsPath, &channels); err != nil {
		return err
	} else if ok {
		for _, c := range channels {
			m.channels[c.ID] = c
		}
	}

	// Add default channels if none exist
	if len(m.channels) == 0 {
		m.addDefaultChannels()
	}

	return nil
}

func (m *Manager) save() error {
	// Save notifications (keep last 1000)
	notifications := make([]*Notification, 0, len(m.notifications))
	for _, n := range m.notifications {
		notifications = append(notifications, n)
	}

	// Sort by timestamp and keep recent ones
	if len(notifications) > 1000 {
		notifications = notifications[len(notifications)-1000:]
	}

	notifPath := filepath.Join(m.storePath, "notifications.json")
	if err := fsatomic.SaveJSON(notifPath, notifications, 0600); err != nil {
		return err
	}

	// Save channels
	channels := make([]*Channel, 0, len(m.channels))
	for _, c := range m.channels {
		channels = append(channels, c)
	}

	channelsPath := filepath.Join(m.storePath, "channels.json")
	return fsatomic.SaveJSON(channelsPath, channels, 0600)
}

func (m *Manager) addDefaultChannels() {
	// Add system log channel
	syslogChannel := &Channel{
		ID:      "system-log",
		Name:    "System Log",
		Type:    "syslog",
		Enabled: true,
		Config: map[string]interface{}{
			"facility": "local0",
			"tag":      "nos",
		},
		Filters: []Filter{
			{MinLevel: "info"},
		},
	}
	m.channels[syslogChannel.ID] = syslogChannel
}

// Send creates and dispatches a notification
func (m *Manager) Send(notif *Notification) error {
	if notif.ID == "" {
		notif.ID = uuid.New().String()
	}
	if notif.Timestamp.IsZero() {
		notif.Timestamp = time.Now()
	}

	m.mu.Lock()
	m.notifications[notif.ID] = notif
	_ = m.save()

	// Notify subscribers
	for _, subs := range m.subscribers {
		for _, ch := range subs {
			select {
			case ch <- notif:
			default:
				// Channel full, skip
			}
		}
	}
	m.mu.Unlock()

	// Send to channels
	m.sendToChannels(notif)

	return nil
}

// sendToChannels sends notification to configured channels
func (m *Manager) sendToChannels(notif *Notification) {
	m.mu.RLock()
	channels := make([]*Channel, 0, len(m.channels))
	for _, c := range m.channels {
		if c.Enabled && m.matchesFilters(notif, c.Filters) {
			channels = append(channels, c)
		}
	}
	m.mu.RUnlock()

	for _, channel := range channels {
		switch channel.Type {
		case "email":
			go m.sendEmail(channel, notif)
		case "webhook":
			go m.sendWebhook(channel, notif)
		case "syslog":
			go m.sendSyslog(channel, notif)
		}
	}
}

// matchesFilters checks if notification matches channel filters
func (m *Manager) matchesFilters(notif *Notification, filters []Filter) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		// Check type
		if filter.Type != "" && filter.Type != notif.Type {
			continue
		}

		// Check categories
		if len(filter.Categories) > 0 {
			found := false
			for _, cat := range filter.Categories {
				if cat == notif.Category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check level
		if filter.MinLevel != "" {
			if !m.meetsMinLevel(notif.Type, filter.MinLevel) {
				continue
			}
		}

		return true
	}

	return false
}

// meetsMinLevel checks if notification type meets minimum level
func (m *Manager) meetsMinLevel(notifType, minLevel string) bool {
	levels := map[string]int{
		"info":    1,
		"success": 2,
		"warning": 3,
		"error":   4,
	}

	notifLevel, ok1 := levels[notifType]
	minLevelVal, ok2 := levels[minLevel]

	if !ok1 || !ok2 {
		return false
	}

	return notifLevel >= minLevelVal
}

// sendEmail sends notification via email
func (m *Manager) sendEmail(channel *Channel, notif *Notification) {
	host, _ := channel.Config["host"].(string)
	port, _ := channel.Config["port"].(string)
	from, _ := channel.Config["from"].(string)
	to, _ := channel.Config["to"].(string)
	username, _ := channel.Config["username"].(string)
	password, _ := channel.Config["password"].(string)

	if host == "" || from == "" || to == "" {
		log.Error().Str("channel", channel.ID).Msg("Invalid email configuration")
		return
	}

	if port == "" {
		port = "587"
	}

	// Build message
	subject := fmt.Sprintf("[nOS %s] %s", strings.ToUpper(notif.Type), notif.Title)
	body := fmt.Sprintf("Time: %s\n\n%s\n", notif.Timestamp.Format(time.RFC3339), notif.Message)

	if len(notif.Details) > 0 {
		body += "\n\nDetails:\n"
		for k, v := range notif.Details {
			body += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}

	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body))

	// Send email
	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		log.Error().Err(err).Str("channel", channel.ID).Msg("Failed to send email")
	}
}

// sendWebhook sends notification via webhook
func (m *Manager) sendWebhook(channel *Channel, notif *Notification) {
	url, _ := channel.Config["url"].(string)
	if url == "" {
		log.Error().Str("channel", channel.ID).Msg("Invalid webhook configuration")
		return
	}

	// TODO: Implement webhook sending
	log.Debug().Str("channel", channel.ID).Str("url", url).Msg("Webhook notification not yet implemented")
}

// sendSyslog sends notification to syslog
func (m *Manager) sendSyslog(channel *Channel, notif *Notification) {
	// TODO: Implement syslog sending
	log.Info().
		Str("type", notif.Type).
		Str("category", notif.Category).
		Str("title", notif.Title).
		Msg(notif.Message)
}

// List returns all notifications
func (m *Manager) List(unreadOnly bool) []*Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Notification, 0, len(m.notifications))
	for _, n := range m.notifications {
		if !unreadOnly || !n.Read {
			list = append(list, n)
		}
	}

	return list
}

// Get returns a specific notification
func (m *Manager) Get(id string) (*Notification, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notif, ok := m.notifications[id]
	return notif, ok
}

// MarkRead marks a notification as read
func (m *Manager) MarkRead(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	notif, ok := m.notifications[id]
	if !ok {
		return fmt.Errorf("notification not found")
	}

	notif.Read = true
	return m.save()
}

// MarkAllRead marks all notifications as read
func (m *Manager) MarkAllRead() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, n := range m.notifications {
		n.Read = true
	}

	return m.save()
}

// Delete removes a notification
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.notifications, id)
	return m.save()
}

// Subscribe creates a subscription for real-time notifications
func (m *Manager) Subscribe(clientID string) chan *Notification {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *Notification, 10)
	m.subscribers[clientID] = append(m.subscribers[clientID], ch)

	return ch
}

// Unsubscribe removes a subscription
func (m *Manager) Unsubscribe(clientID string, ch chan *Notification) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subscribers[clientID]
	for i, sub := range subs {
		if sub == ch {
			close(sub)
			m.subscribers[clientID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	if len(m.subscribers[clientID]) == 0 {
		delete(m.subscribers, clientID)
	}
}

// Channels
func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Channel, 0, len(m.channels))
	for _, c := range m.channels {
		list = append(list, c)
	}
	return list
}

func (m *Manager) GetChannel(id string) (*Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[id]
	return channel, ok
}

func (m *Manager) CreateChannel(channel *Channel) error {
	if channel.ID == "" {
		channel.ID = uuid.New().String()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels[channel.ID] = channel
	return m.save()
}

func (m *Manager) UpdateChannel(id string, updates *Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, ok := m.channels[id]
	if !ok {
		return fmt.Errorf("channel not found")
	}

	// Update fields
	if updates.Name != "" {
		channel.Name = updates.Name
	}
	channel.Enabled = updates.Enabled
	if updates.Config != nil {
		channel.Config = updates.Config
	}
	if updates.Filters != nil {
		channel.Filters = updates.Filters
	}

	return m.save()
}

func (m *Manager) DeleteChannel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.channels, id)
	return m.save()
}

// TestChannel tests a notification channel
func (m *Manager) TestChannel(id string) error {
	channel, ok := m.GetChannel(id)
	if !ok {
		return fmt.Errorf("channel not found")
	}

	// Send test notification
	testNotif := &Notification{
		Type:     "info",
		Category: "system",
		Title:    "Test Notification",
		Message:  fmt.Sprintf("This is a test notification for channel: %s", channel.Name),
		Details: map[string]interface{}{
			"channel_id":   channel.ID,
			"channel_type": channel.Type,
			"test":         true,
		},
	}

	switch channel.Type {
	case "email":
		m.sendEmail(channel, testNotif)
	case "webhook":
		m.sendWebhook(channel, testNotif)
	case "syslog":
		m.sendSyslog(channel, testNotif)
	default:
		return fmt.Errorf("unknown channel type: %s", channel.Type)
	}

	return nil
}

// cleanupOldNotifications removes old notifications periodically
func (m *Manager) cleanupOldNotifications() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()

		// Remove notifications older than 30 days
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		for id, n := range m.notifications {
			if n.Timestamp.Before(cutoff) {
				delete(m.notifications, id)
			}
		}

		_ = m.save()
		m.mu.Unlock()
	}
}

// SendSystemNotification is a helper to send system notifications
func (m *Manager) SendSystemNotification(title, message string, notifType string) {
	_ = m.Send(&Notification{
		Type:     notifType,
		Category: "system",
		Title:    title,
		Message:  message,
	})
}

// SendBackupNotification is a helper to send backup notifications
func (m *Manager) SendBackupNotification(jobName string, status string, details map[string]interface{}) {
	notifType := "success"
	if status == "failed" {
		notifType = "error"
	}

	_ = m.Send(&Notification{
		Type:     notifType,
		Category: "backup",
		Title:    fmt.Sprintf("Backup Job: %s", jobName),
		Message:  fmt.Sprintf("Backup job '%s' %s", jobName, status),
		Details:  details,
	})
}

// SendStorageNotification is a helper to send storage notifications
func (m *Manager) SendStorageNotification(title, message string, notifType string, details map[string]interface{}) {
	_ = m.Send(&Notification{
		Type:     notifType,
		Category: "storage",
		Title:    title,
		Message:  message,
		Details:  details,
	})
}
