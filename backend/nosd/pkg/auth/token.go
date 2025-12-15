package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// TokenType defines the type of API token
type TokenType string

const (
	TokenTypePersonal TokenType = "personal"
	TokenTypeService  TokenType = "service"
	TokenTypeDevice   TokenType = "device" // Sync device tokens
)

// APIToken represents an API access token
type APIToken struct {
	ID          string      `json:"id"`
	Type        TokenType   `json:"type"`
	OwnerUserID string      `json:"owner_user_id,omitempty"`
	Name        string      `json:"name"`
	Hash        string      `json:"-"` // Never expose
	
	// Metadata
	CreatedAt   time.Time   `json:"created_at"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time  `json:"last_used_at,omitempty"`
	LastUsedIP  string      `json:"last_used_ip,omitempty"`
	
	// Permissions
	Scopes      []string    `json:"scopes"`
	IPAllowlist []string    `json:"ip_allowlist,omitempty"`
	
	// Stats
	UseCount    int         `json:"use_count"`
}

// TokenScope defines available API scopes
type TokenScope string

const (
	// System scopes
	ScopeSystemRead    TokenScope = "system.read"
	ScopeSystemWrite   TokenScope = "system.write"
	
	// User scopes
	ScopeUsersRead     TokenScope = "users.read"
	ScopeUsersWrite    TokenScope = "users.write"
	
	// Storage scopes
	ScopeStorageRead   TokenScope = "storage.read"
	ScopeStorageWrite  TokenScope = "storage.write"
	
	// Apps scopes
	ScopeAppsRead      TokenScope = "apps.read"
	ScopeAppsWrite     TokenScope = "apps.write"
	
	// Metrics scopes
	ScopeMetricsRead   TokenScope = "metrics.read"
	
	// Alerts scopes
	ScopeAlertsRead    TokenScope = "alerts.read"
	ScopeAlertsWrite   TokenScope = "alerts.write"
	
	// Backup scopes
	ScopeBackupsRead   TokenScope = "backups.read"
	ScopeBackupsWrite  TokenScope = "backups.write"
	
	// Sync scopes (for NithronSync client devices)
	ScopeSyncRead    TokenScope = "sync.read"    // Read files via sync API
	ScopeSyncWrite   TokenScope = "sync.write"   // Write files via sync API
	ScopeSyncDevices TokenScope = "sync.devices" // Manage sync devices
	ScopeSyncAdmin   TokenScope = "sync.admin"   // Full sync administration
	
	// Admin scope (all permissions)
	ScopeAdminAll      TokenScope = "admin.*"
)

// TokenManager manages API tokens
type TokenManager struct {
	logger    zerolog.Logger
	dataPath  string
	tokens    map[string]*APIToken
	mu        sync.RWMutex
	auditLog  *AuditLogger
}

// NewTokenManager creates a new token manager
func NewTokenManager(logger zerolog.Logger, dataPath string, auditLog *AuditLogger) *TokenManager {
	tm := &TokenManager{
		logger:   logger.With().Str("component", "token-manager").Logger(),
		dataPath: dataPath,
		tokens:   make(map[string]*APIToken),
		auditLog: auditLog,
	}
	
	// Load existing tokens
	tm.loadTokens()
	
	// Start cleanup routine
	go tm.cleanupRoutine()
	
	return tm
}

// CreateToken creates a new API token
func (tm *TokenManager) CreateToken(req CreateTokenRequest, actorID string) (*APIToken, string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	// Validate scopes
	if err := tm.validateScopes(req.Scopes); err != nil {
		return nil, "", err
	}
	
	// Validate IP allowlist
	if err := tm.validateIPAllowlist(req.IPAllowlist); err != nil {
		return nil, "", err
	}
	
	// Generate token
	tokenValue := tm.generateTokenValue(req.Type)
	
	// Hash token
	hash, err := bcrypt.GenerateFromPassword([]byte(tokenValue), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash token: %w", err)
	}
	
	// Create token object
	token := &APIToken{
		ID:          uuid.New().String(),
		Type:        req.Type,
		OwnerUserID: req.OwnerUserID,
		Name:        req.Name,
		Hash:        string(hash),
		CreatedAt:   time.Now(),
		Scopes:      req.Scopes,
		IPAllowlist: req.IPAllowlist,
		UseCount:    0,
	}
	
	if req.ExpiresIn > 0 {
		expiresAt := time.Now().Add(req.ExpiresIn)
		token.ExpiresAt = &expiresAt
	}
	
	// Store token
	tm.tokens[token.ID] = token
	
	// Audit log
	tm.auditLog.LogEvent(&AuditEvent{
		UserID:   actorID,
		Code:     "token.create",
		Category: "auth",
		Severity: "info",
		Success:  true,
		Target:   token.Name,
		Message:  fmt.Sprintf("Created %s token '%s'", token.Type, token.Name),
		Details: map[string]interface{}{
			"token_id": token.ID,
			"type":     token.Type,
			"scopes":   token.Scopes,
		},
	})
	
	// Save tokens
	tm.saveTokens()
	
	return token, tokenValue, nil
}

// ValidateToken validates an API token
func (tm *TokenManager) ValidateToken(tokenValue string, ip string) (*APIToken, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	// Extract token type from prefix
	var tokenType TokenType
	if strings.HasPrefix(tokenValue, "nos_pt_") {
		tokenType = TokenTypePersonal
	} else if strings.HasPrefix(tokenValue, "nos_st_") {
		tokenType = TokenTypeService
	} else if strings.HasPrefix(tokenValue, "nos_dt_") {
		tokenType = TokenTypeDevice
	} else {
		return nil, fmt.Errorf("invalid token format")
	}
	
	// Find matching token
	for _, token := range tm.tokens {
		if token.Type != tokenType {
			continue
		}
		
		// Check hash
		if err := bcrypt.CompareHashAndPassword([]byte(token.Hash), []byte(tokenValue)); err == nil {
			// Check expiration
			if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("token expired")
			}
			
			// Check IP allowlist
			if len(token.IPAllowlist) > 0 && !tm.isIPAllowed(ip, token.IPAllowlist) {
				return nil, fmt.Errorf("IP not allowed")
			}
			
			// Update usage stats
			now := time.Now()
			token.LastUsedAt = &now
			token.LastUsedIP = ip
			token.UseCount++
			
			// Save async
			go tm.saveTokens()
			
			return token, nil
		}
	}
	
	return nil, fmt.Errorf("invalid token")
}

// DeleteToken deletes a token
func (tm *TokenManager) DeleteToken(tokenID string, actorID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	token, exists := tm.tokens[tokenID]
	if !exists {
		return fmt.Errorf("token not found")
	}
	
	// Delete token
	delete(tm.tokens, tokenID)
	
	// Audit log
	tm.auditLog.LogEvent(&AuditEvent{
		UserID:   actorID,
		Code:     "token.delete",
		Category: "auth",
		Severity: "warning",
		Success:  true,
		Target:   token.Name,
		Message:  fmt.Sprintf("Deleted %s token '%s'", token.Type, token.Name),
		Details: map[string]interface{}{
			"token_id": token.ID,
			"type":     token.Type,
		},
	})
	
	// Save tokens
	tm.saveTokens()
	
	return nil
}

// ListTokens returns all tokens for a user
func (tm *TokenManager) ListTokens(userID string, includeAll bool) []*APIToken {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	tokens := []*APIToken{}
	for _, token := range tm.tokens {
		if includeAll || token.OwnerUserID == userID {
			// Don't expose hash
			tokenCopy := *token
			tokenCopy.Hash = ""
			tokens = append(tokens, &tokenCopy)
		}
	}
	
	return tokens
}

// RevokeUserTokens revokes all tokens for a user
func (tm *TokenManager) RevokeUserTokens(userID string, actorID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	count := 0
	for id, token := range tm.tokens {
		if token.OwnerUserID == userID {
			delete(tm.tokens, id)
			count++
		}
	}
	
	if count > 0 {
		// Audit log
		tm.auditLog.LogEvent(&AuditEvent{
			UserID:   actorID,
			Code:     "token.revoke",
			Category: "auth",
			Severity: "warning",
			Success:  true,
			Target:   userID,
			Message:  fmt.Sprintf("Revoked %d tokens for user", count),
		})
		
		// Save tokens
		tm.saveTokens()
	}
	
	return nil
}

// HasScope checks if a token has a specific scope
func (tm *TokenManager) HasScope(token *APIToken, scope string) bool {
	// Check for admin scope
	for _, s := range token.Scopes {
		if s == string(ScopeAdminAll) {
			return true
		}
		if s == scope {
			return true
		}
		// Check wildcard scopes
		if strings.HasSuffix(s, ".*") {
			prefix := strings.TrimSuffix(s, ".*")
			if strings.HasPrefix(scope, prefix) {
				return true
			}
		}
	}
	
	return false
}

// Private methods

func (tm *TokenManager) generateTokenValue(tokenType TokenType) string {
	// Generate random bytes
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	
	// Encode to base64
	encoded := base64.URLEncoding.EncodeToString(b)
	
	// Add prefix based on type
	var prefix string
	switch tokenType {
	case TokenTypePersonal:
		prefix = "nos_pt_"
	case TokenTypeService:
		prefix = "nos_st_"
	case TokenTypeDevice:
		prefix = "nos_dt_"
	default:
		prefix = "nos_"
	}
	
	return prefix + encoded
}

func (tm *TokenManager) validateScopes(scopes []string) error {
	validScopes := map[string]bool{
		string(ScopeSystemRead):    true,
		string(ScopeSystemWrite):   true,
		string(ScopeUsersRead):     true,
		string(ScopeUsersWrite):    true,
		string(ScopeStorageRead):   true,
		string(ScopeStorageWrite):  true,
		string(ScopeAppsRead):      true,
		string(ScopeAppsWrite):     true,
		string(ScopeMetricsRead):   true,
		string(ScopeAlertsRead):    true,
		string(ScopeAlertsWrite):   true,
		string(ScopeBackupsRead):   true,
		string(ScopeBackupsWrite):  true,
		string(ScopeSyncRead):      true,
		string(ScopeSyncWrite):     true,
		string(ScopeSyncDevices):   true,
		string(ScopeSyncAdmin):     true,
		string(ScopeAdminAll):      true,
	}
	
	for _, scope := range scopes {
		// Allow wildcard scopes like "system.*"
		if strings.HasSuffix(scope, ".*") {
			prefix := strings.TrimSuffix(scope, ".*")
			hasPrefix := false
			for validScope := range validScopes {
				if strings.HasPrefix(validScope, prefix+".") {
					hasPrefix = true
					break
				}
			}
			if !hasPrefix && scope != string(ScopeAdminAll) {
				return fmt.Errorf("invalid scope: %s", scope)
			}
		} else if !validScopes[scope] {
			return fmt.Errorf("invalid scope: %s", scope)
		}
	}
	
	return nil
}

func (tm *TokenManager) validateIPAllowlist(ips []string) error {
	for _, ip := range ips {
		// Check if it's a valid IP or CIDR
		if strings.Contains(ip, "/") {
			_, _, err := net.ParseCIDR(ip)
			if err != nil {
				return fmt.Errorf("invalid CIDR: %s", ip)
			}
		} else {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid IP: %s", ip)
			}
		}
	}
	
	return nil
}

func (tm *TokenManager) isIPAllowed(ip string, allowlist []string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}
	
	for _, allowed := range allowlist {
		if strings.Contains(allowed, "/") {
			_, cidr, err := net.ParseCIDR(allowed)
			if err == nil && cidr.Contains(clientIP) {
				return true
			}
		} else {
			if allowed == ip {
				return true
			}
		}
	}
	
	return false
}

func (tm *TokenManager) cleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		tm.cleanup()
	}
}

func (tm *TokenManager) cleanup() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	now := time.Now()
	for id, token := range tm.tokens {
		if token.ExpiresAt != nil && token.ExpiresAt.Before(now) {
			delete(tm.tokens, id)
			
			// Audit log
			tm.auditLog.LogEvent(&AuditEvent{
				Code:     "token.expired",
				Category: "auth",
				Severity: "info",
				Success:  true,
				Target:   token.Name,
				Message:  fmt.Sprintf("Token '%s' expired and removed", token.Name),
			})
		}
	}
	
	tm.saveTokens()
}

func (tm *TokenManager) loadTokens() {
	// Implementation would load from disk
	// For now, tokens are stored in memory only
}

func (tm *TokenManager) saveTokens() {
	// Implementation would save to disk
	// For now, tokens are stored in memory only
}

// CreateTokenRequest for creating new tokens
type CreateTokenRequest struct {
	Type        TokenType     `json:"type" validate:"required,oneof=personal service"`
	Name        string        `json:"name" validate:"required,min=1,max=100"`
	OwnerUserID string        `json:"owner_user_id,omitempty"`
	Scopes      []string      `json:"scopes" validate:"required,min=1"`
	ExpiresIn   time.Duration `json:"expires_in,omitempty"`
	IPAllowlist []string      `json:"ip_allowlist,omitempty"`
}

// GetScopeDescription returns a human-readable description of a scope
func GetScopeDescription(scope string) string {
	descriptions := map[string]string{
		string(ScopeSystemRead):    "Read system information and status",
		string(ScopeSystemWrite):   "Modify system configuration",
		string(ScopeUsersRead):     "View user accounts and sessions",
		string(ScopeUsersWrite):    "Manage user accounts and roles",
		string(ScopeStorageRead):   "View storage pools and snapshots",
		string(ScopeStorageWrite):  "Manage storage and snapshots",
		string(ScopeAppsRead):      "View installed applications",
		string(ScopeAppsWrite):     "Install and manage applications",
		string(ScopeMetricsRead):   "View system metrics and monitoring",
		string(ScopeAlertsRead):    "View alerts and rules",
		string(ScopeAlertsWrite):   "Configure alert rules and channels",
		string(ScopeBackupsRead):   "View backup schedules and jobs",
		string(ScopeBackupsWrite):  "Manage backups and restoration",
		string(ScopeAdminAll):      "Full administrative access",
	}
	
	if desc, ok := descriptions[scope]; ok {
		return desc
	}
	
	// Handle wildcard scopes
	if strings.HasSuffix(scope, ".*") {
		prefix := strings.TrimSuffix(scope, ".*")
		return fmt.Sprintf("All %s permissions", prefix)
	}
	
	return scope
}

// GetAllScopes returns all available scopes
func GetAllScopes() []string {
	return []string{
		string(ScopeSystemRead),
		string(ScopeSystemWrite),
		string(ScopeUsersRead),
		string(ScopeUsersWrite),
		string(ScopeStorageRead),
		string(ScopeStorageWrite),
		string(ScopeAppsRead),
		string(ScopeAppsWrite),
		string(ScopeMetricsRead),
		string(ScopeAlertsRead),
		string(ScopeAlertsWrite),
		string(ScopeBackupsRead),
		string(ScopeBackupsWrite),
		string(ScopeSyncRead),
		string(ScopeSyncWrite),
		string(ScopeSyncDevices),
		string(ScopeSyncAdmin),
		string(ScopeAdminAll),
	}
}
