package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SessionManager handles session management and rate limiting
type SessionManager struct {
	logger      zerolog.Logger
	userManager *UserManager
	auditLog    *AuditLogger
	sessions    map[string]*Session
	mu          sync.RWMutex

	// Rate limiting
	loginAttempts map[string][]time.Time // IP -> timestamps
	attemptsMu    sync.RWMutex

	// Configuration
	sessionTTL        time.Duration
	elevatedTTL       time.Duration
	maxFailedAttempts int
	lockoutDuration   time.Duration
	rateWindow        time.Duration
	maxAttemptsPerIP  int
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger zerolog.Logger, userManager *UserManager, auditLog *AuditLogger) *SessionManager {
	sm := &SessionManager{
		logger:            logger.With().Str("component", "session-manager").Logger(),
		userManager:       userManager,
		auditLog:          auditLog,
		sessions:          make(map[string]*Session),
		loginAttempts:     make(map[string][]time.Time),
		sessionTTL:        24 * time.Hour,
		elevatedTTL:       15 * time.Minute,
		maxFailedAttempts: 5,
		lockoutDuration:   30 * time.Minute,
		rateWindow:        15 * time.Minute,
		maxAttemptsPerIP:  10,
	}

	// Start cleanup routine
	go sm.cleanupRoutine()

	return sm
}

// CreateSession creates a new session after successful authentication
func (sm *SessionManager) CreateSession(user *User, ip, userAgent string, twoFactorVerified bool) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate session ID
	sessionID := sm.generateSessionID()

	// Generate refresh token
	refreshToken := sm.generateRefreshToken()

	// Create session
	session := &Session{
		ID:                sessionID,
		UserID:            user.ID,
		Username:          user.Username,
		Role:              user.Role,
		IssuedAt:          time.Now(),
		ExpiresAt:         time.Now().Add(sm.sessionTTL),
		LastSeenAt:        time.Now(),
		IP:                ip,
		UserAgent:         userAgent,
		TwoFactorVerified: twoFactorVerified,
		RefreshToken:      refreshToken,
		TokenVersion:      1,
		Scopes:            user.Role.GetPermissions(),
	}

	// Set elevated status if 2FA verified
	if twoFactorVerified {
		elevatedUntil := time.Now().Add(sm.elevatedTTL)
		session.ElevatedUntil = &elevatedUntil
	}

	// Store session
	sm.sessions[sessionID] = session

	// Update user last login
	user.LastLoginAt = &session.IssuedAt
	user.LastLoginIP = ip

	// Audit log
	sm.auditLog.LogEvent(&AuditEvent{
		UserID:    user.ID,
		Username:  user.Username,
		SessionID: sessionID,
		IP:        ip,
		UserAgent: userAgent,
		Code:      AuditSessionCreate,
		Category:  "session",
		Severity:  "info",
		Success:   true,
		Message:   fmt.Sprintf("Session created for user %s", user.Username),
		Details: map[string]interface{}{
			"two_factor_verified": twoFactorVerified,
		},
	})

	return session, nil
}

// ValidateSession validates and returns a session
func (sm *SessionManager) ValidateSession(sessionID string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check expiration
	if session.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("session expired")
	}

	// Check if user is still enabled
	user, err := sm.userManager.GetUser(session.UserID)
	if err != nil || !user.Enabled {
		return nil, fmt.Errorf("user disabled or not found")
	}

	// Update last seen
	session.LastSeenAt = time.Now()

	return session, nil
}

// RefreshSession refreshes a session with a refresh token
func (sm *SessionManager) RefreshSession(refreshToken string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find session by refresh token
	var session *Session
	for _, s := range sm.sessions {
		if s.RefreshToken == refreshToken {
			session = s
			break
		}
	}

	if session == nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if user is still enabled
	user, err := sm.userManager.GetUser(session.UserID)
	if err != nil || !user.Enabled {
		return nil, fmt.Errorf("user disabled or not found")
	}

	// Extend expiration
	session.ExpiresAt = time.Now().Add(sm.sessionTTL)
	session.LastSeenAt = time.Now()

	// Rotate refresh token
	session.RefreshToken = sm.generateRefreshToken()
	session.TokenVersion++

	// Audit log
	sm.auditLog.LogEvent(&AuditEvent{
		UserID:    session.UserID,
		Username:  session.Username,
		SessionID: session.ID,
		Code:      AuditAuthTokenRefresh,
		Category:  "auth",
		Severity:  "info",
		Success:   true,
		Message:   "Session refreshed",
	})

	return session, nil
}

// ElevateSession elevates a session after 2FA verification
func (sm *SessionManager) ElevateSession(sessionID string, code string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	// Verify TOTP
	if !sm.userManager.verifyTOTP(session.UserID, code) &&
		!sm.userManager.verifyRecoveryCode(session.UserID, code) {
		sm.auditLog.LogEvent(&AuditEvent{
			UserID:    session.UserID,
			Username:  session.Username,
			SessionID: sessionID,
			Code:      AuditAuth2FAFailed,
			Category:  "auth",
			Severity:  "warning",
			Success:   false,
			Message:   "2FA verification failed",
		})
		return fmt.Errorf("invalid 2FA code")
	}

	// Elevate session
	elevatedUntil := time.Now().Add(sm.elevatedTTL)
	session.ElevatedUntil = &elevatedUntil
	session.TwoFactorVerified = true

	// Audit log
	sm.auditLog.LogEvent(&AuditEvent{
		UserID:    session.UserID,
		Username:  session.Username,
		SessionID: sessionID,
		Code:      AuditSessionElevate,
		Category:  "session",
		Severity:  "info",
		Success:   true,
		Message:   "Session elevated with 2FA",
	})

	return nil
}

// RevokeSession revokes a specific session
func (sm *SessionManager) RevokeSession(sessionID string, actorID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	// Delete session
	delete(sm.sessions, sessionID)

	// Audit log
	sm.auditLog.LogEvent(&AuditEvent{
		UserID:    actorID,
		SessionID: sessionID,
		Code:      AuditSessionRevoke,
		Category:  "session",
		Severity:  "warning",
		Success:   true,
		Target:    session.Username,
		Message:   fmt.Sprintf("Session revoked for user %s", session.Username),
	})

	return nil
}

// RevokeUserSessions revokes all sessions for a user
func (sm *SessionManager) RevokeUserSessions(userID string, actorID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	for id, session := range sm.sessions {
		if session.UserID == userID {
			delete(sm.sessions, id)
			count++
		}
	}

	if count > 0 {
		user, _ := sm.userManager.GetUser(userID)
		username := "unknown"
		if user != nil {
			username = user.Username
		}

		// Audit log
		sm.auditLog.LogEvent(&AuditEvent{
			UserID:   actorID,
			Code:     AuditSessionRevoke,
			Category: "session",
			Severity: "warning",
			Success:  true,
			Target:   username,
			Message:  fmt.Sprintf("Revoked %d sessions for user %s", count, username),
		})
	}

	return nil
}

// ListUserSessions returns all sessions for a user
func (sm *SessionManager) ListUserSessions(userID string) []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := []*Session{}
	for _, session := range sm.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// ListAllSessions returns all active sessions (admin only)
func (sm *SessionManager) ListAllSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// CheckRateLimit checks if an IP has exceeded rate limits
func (sm *SessionManager) CheckRateLimit(ip string) (bool, error) {
	sm.attemptsMu.Lock()
	defer sm.attemptsMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sm.rateWindow)

	// Clean old attempts
	attempts := sm.loginAttempts[ip]
	newAttempts := []time.Time{}
	for _, t := range attempts {
		if t.After(cutoff) {
			newAttempts = append(newAttempts, t)
		}
	}

	// Check if exceeded
	if len(newAttempts) >= sm.maxAttemptsPerIP {
		return false, fmt.Errorf("rate limit exceeded: %d attempts in %v", len(newAttempts), sm.rateWindow)
	}

	// Add new attempt
	newAttempts = append(newAttempts, now)
	sm.loginAttempts[ip] = newAttempts

	return true, nil
}

// RecordFailedLogin records a failed login attempt
func (sm *SessionManager) RecordFailedLogin(username, ip, reason string) {
	// Update user failed login count
	if user, err := sm.userManager.GetUserByUsername(username); err == nil {
		user.FailedLogins++

		// Check if should lock
		if user.FailedLogins >= sm.maxFailedAttempts {
			lockedUntil := time.Now().Add(sm.lockoutDuration)
			user.LockedUntil = &lockedUntil

			// Audit log
			sm.auditLog.LogEvent(&AuditEvent{
				UserID:   user.ID,
				Username: username,
				IP:       ip,
				Code:     AuditAuthLocked,
				Category: "auth",
				Severity: "critical",
				Success:  true,
				Message:  fmt.Sprintf("User %s locked after %d failed attempts", username, user.FailedLogins),
			})
		}
	}

	// Audit log failed attempt
	sm.auditLog.LogEvent(&AuditEvent{
		Username: username,
		IP:       ip,
		Code:     AuditAuthFailed,
		Category: "auth",
		Severity: "warning",
		Success:  false,
		Message:  fmt.Sprintf("Login failed: %s", reason),
		Details: map[string]interface{}{
			"reason": reason,
		},
	})
}

// RecordSuccessfulLogin records a successful login
func (sm *SessionManager) RecordSuccessfulLogin(user *User, ip string) {
	// Reset failed login count
	user.FailedLogins = 0
	user.LockedUntil = nil

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip

	// Audit log
	sm.auditLog.LogEvent(&AuditEvent{
		UserID:   user.ID,
		Username: user.Username,
		IP:       ip,
		Code:     AuditAuthLogin,
		Category: "auth",
		Severity: "info",
		Success:  true,
		Message:  fmt.Sprintf("User %s logged in", user.Username),
	})
}

// IsIPLocked checks if an IP is locked
func (sm *SessionManager) IsIPLocked(ip string) bool {
	sm.attemptsMu.RLock()
	defer sm.attemptsMu.RUnlock()

	attempts := sm.loginAttempts[ip]
	if len(attempts) < sm.maxAttemptsPerIP {
		return false
	}

	// Check if all attempts are within rate window
	cutoff := time.Now().Add(-sm.rateWindow)
	recentCount := 0
	for _, t := range attempts {
		if t.After(cutoff) {
			recentCount++
		}
	}

	return recentCount >= sm.maxAttemptsPerIP
}

// GetLockouts returns current lockouts
func (sm *SessionManager) GetLockouts() []Lockout {
	lockouts := []Lockout{}

	// User lockouts
	for _, user := range sm.userManager.ListUsers() {
		if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
			lockouts = append(lockouts, Lockout{
				ID:          user.ID,
				Type:        "user",
				Target:      user.Username,
				LockedAt:    user.LockedUntil.Add(-sm.lockoutDuration),
				LockedUntil: *user.LockedUntil,
				Reason:      fmt.Sprintf("%d failed login attempts", sm.maxFailedAttempts),
			})
		}
	}

	// IP lockouts
	sm.attemptsMu.RLock()
	defer sm.attemptsMu.RUnlock()

	for ip, attempts := range sm.loginAttempts {
		if len(attempts) >= sm.maxAttemptsPerIP {
			// Find most recent attempt
			var mostRecent time.Time
			for _, t := range attempts {
				if t.After(mostRecent) {
					mostRecent = t
				}
			}

			lockouts = append(lockouts, Lockout{
				ID:          ip,
				Type:        "ip",
				Target:      ip,
				LockedAt:    mostRecent,
				LockedUntil: mostRecent.Add(sm.rateWindow),
				Reason:      fmt.Sprintf("%d attempts in %v", len(attempts), sm.rateWindow),
			})
		}
	}

	return lockouts
}

// ClearLockout clears a lockout
func (sm *SessionManager) ClearLockout(lockoutID string, actorID string, reason string) error {
	// Check if it's a user lockout
	if user, err := sm.userManager.GetUser(lockoutID); err == nil {
		user.LockedUntil = nil
		user.FailedLogins = 0

		// Audit log
		sm.auditLog.LogEvent(&AuditEvent{
			UserID:   actorID,
			Code:     AuditAuthUnlocked,
			Category: "auth",
			Severity: "info",
			Success:  true,
			Target:   user.Username,
			Message:  fmt.Sprintf("User %s unlocked: %s", user.Username, reason),
		})

		return nil
	}

	// Check if it's an IP lockout
	sm.attemptsMu.Lock()
	defer sm.attemptsMu.Unlock()

	// Try to parse as IP
	if net.ParseIP(lockoutID) != nil {
		delete(sm.loginAttempts, lockoutID)

		// Audit log
		sm.auditLog.LogEvent(&AuditEvent{
			UserID:   actorID,
			Code:     AuditAuthUnlocked,
			Category: "auth",
			Severity: "info",
			Success:  true,
			Target:   lockoutID,
			Message:  fmt.Sprintf("IP %s unlocked: %s", lockoutID, reason),
		})

		return nil
	}

	return fmt.Errorf("lockout not found")
}

// Private methods

func (sm *SessionManager) generateSessionID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (sm *SessionManager) generateRefreshToken() string {
	b := make([]byte, 64)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (sm *SessionManager) cleanupRoutine() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanup()
	}
}

func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()

	// Clean expired sessions
	for id, session := range sm.sessions {
		if session.ExpiresAt.Before(now) {
			delete(sm.sessions, id)

			// Audit log
			sm.auditLog.LogEvent(&AuditEvent{
				UserID:    session.UserID,
				Username:  session.Username,
				SessionID: id,
				Code:      AuditSessionExpired,
				Category:  "session",
				Severity:  "info",
				Success:   true,
				Message:   "Session expired",
			})
		}
	}

	// Clean old login attempts
	sm.attemptsMu.Lock()
	defer sm.attemptsMu.Unlock()

	cutoff := now.Add(-24 * time.Hour)
	for ip, attempts := range sm.loginAttempts {
		newAttempts := []time.Time{}
		for _, t := range attempts {
			if t.After(cutoff) {
				newAttempts = append(newAttempts, t)
			}
		}

		if len(newAttempts) == 0 {
			delete(sm.loginAttempts, ip)
		} else {
			sm.loginAttempts[ip] = newAttempts
		}
	}
}
