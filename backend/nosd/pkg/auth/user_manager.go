package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/argon2"
)

// UserManager handles user management
type UserManager struct {
	logger      zerolog.Logger
	dataPath    string
	users       map[string]*User
	sessions    map[string]*Session
	resetTokens map[string]*PasswordResetToken
	lockouts    map[string]*Lockout
	auditLog    *AuditLogger
	mu          sync.RWMutex

	// Password policy
	passwordPolicy PasswordPolicy

	// Rate limiting
	loginAttempts []LoginAttempt
	attemptsMu    sync.RWMutex
}

// NewUserManager creates a new user manager
func NewUserManager(logger zerolog.Logger, dataPath string) *UserManager {
	um := &UserManager{
		logger:        logger.With().Str("component", "user-manager").Logger(),
		dataPath:      dataPath,
		users:         make(map[string]*User),
		sessions:      make(map[string]*Session),
		resetTokens:   make(map[string]*PasswordResetToken),
		lockouts:      make(map[string]*Lockout),
		loginAttempts: []LoginAttempt{},
		passwordPolicy: PasswordPolicy{
			MinLength:        12,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   false,
			MinEntropy:       3.0,
			ProhibitCommon:   true,
			ProhibitUsername: true,
			ProhibitReuse:    3,
			MaxAge:           90,
			WarnAge:          14,
		},
	}

	// Initialize audit logger
	um.auditLog = NewAuditLogger(logger, filepath.Join(dataPath, "audit"))

	// Load existing data
	um.loadData()

	// Create default admin if no users exist
	if len(um.users) == 0 {
		um.createDefaultAdmin()
	}

	// Start cleanup routine
	go um.cleanupRoutine()

	return um
}

// User CRUD operations

// CreateUser creates a new user
func (um *UserManager) CreateUser(req UserCreateRequest, actorID string) (*User, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if username already exists
	for _, u := range um.users {
		if strings.EqualFold(u.Username, req.Username) {
			return nil, fmt.Errorf("username already exists")
		}
		if req.Email != "" && strings.EqualFold(u.Email, req.Email) {
			return nil, fmt.Errorf("email already in use")
		}
	}

	// Validate password
	if err := um.validatePassword(req.Password, req.Username); err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword := um.hashPassword(req.Password)

	// Create user
	user := &User{
		ID:                uuid.New().String(),
		Username:          req.Username,
		Email:             req.Email,
		Role:              req.Role,
		Enabled:           true,
		TwoFactorEnabled:  false,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		PasswordChangedAt: time.Now(),
		FailedLogins:      0,
	}

	// Store user
	um.users[user.ID] = user

	// Store password separately
	um.storePassword(user.ID, hashedPassword)

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   actorID,
		Code:     AuditUserCreate,
		Category: "user",
		Severity: "info",
		Success:  true,
		Target:   user.Username,
		Message:  fmt.Sprintf("Created user %s with role %s", user.Username, user.Role),
		NewValues: map[string]interface{}{
			"username": user.Username,
			"role":     user.Role,
			"email":    user.Email,
		},
	})

	// Save data
	um.saveData()

	return user, nil
}

// GetUser returns a user by ID
func (um *UserManager) GetUser(userID string) (*User, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// GetUserByUsername returns a user by username
func (um *UserManager) GetUserByUsername(username string) (*User, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	for _, user := range um.users {
		if strings.EqualFold(user.Username, username) {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// UpdateUser updates a user
func (um *UserManager) UpdateUser(userID string, req UserUpdateRequest, actorID string) (*User, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	oldValues := make(map[string]interface{})
	newValues := make(map[string]interface{})

	// Update fields
	if req.Email != nil {
		oldValues["email"] = user.Email
		user.Email = *req.Email
		newValues["email"] = *req.Email
	}

	if req.Role != nil {
		oldValues["role"] = user.Role
		user.Role = *req.Role
		newValues["role"] = *req.Role
	}

	if req.Enabled != nil {
		oldValues["enabled"] = user.Enabled
		user.Enabled = *req.Enabled
		newValues["enabled"] = *req.Enabled

		// Log enable/disable event
		if *req.Enabled {
			um.auditLog.LogEvent(&AuditEvent{
				UserID:   actorID,
				Code:     AuditUserEnable,
				Category: "user",
				Severity: "info",
				Success:  true,
				Target:   user.Username,
				Message:  fmt.Sprintf("Enabled user %s", user.Username),
			})
		} else {
			um.auditLog.LogEvent(&AuditEvent{
				UserID:   actorID,
				Code:     AuditUserDisable,
				Category: "user",
				Severity: "warning",
				Success:  true,
				Target:   user.Username,
				Message:  fmt.Sprintf("Disabled user %s", user.Username),
			})

			// Revoke all sessions when disabling user
			um.revokeUserSessionsInternal(userID)
		}
	}

	user.UpdatedAt = time.Now()

	// Audit log
	if len(oldValues) > 0 {
		um.auditLog.LogEvent(&AuditEvent{
			UserID:    actorID,
			Code:      AuditUserUpdate,
			Category:  "user",
			Severity:  "info",
			Success:   true,
			Target:    user.Username,
			Message:   fmt.Sprintf("Updated user %s", user.Username),
			OldValues: oldValues,
			NewValues: newValues,
		})
	}

	// Save data
	um.saveData()

	return user, nil
}

// DeleteUser deletes a user
func (um *UserManager) DeleteUser(userID string, actorID string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Prevent deleting last admin
	if user.Role == RoleAdmin {
		adminCount := 0
		for _, u := range um.users {
			if u.Role == RoleAdmin && u.Enabled {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot delete last admin user")
		}
	}

	// Revoke all sessions
	um.revokeUserSessionsInternal(userID)

	// Delete user
	delete(um.users, userID)

	// Delete password
	um.deletePassword(userID)

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   actorID,
		Code:     AuditUserDelete,
		Category: "user",
		Severity: "warning",
		Success:  true,
		Target:   user.Username,
		Message:  fmt.Sprintf("Deleted user %s", user.Username),
	})

	// Save data
	um.saveData()

	return nil
}

// ListUsers returns all users
func (um *UserManager) ListUsers() []*User {
	um.mu.RLock()
	defer um.mu.RUnlock()

	users := make([]*User, 0, len(um.users))
	for _, user := range um.users {
		users = append(users, user)
	}

	return users
}

// Password management

// ChangePassword changes a user's password
func (um *UserManager) ChangePassword(userID string, req PasswordChangeRequest) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Verify current password
	if !um.verifyPassword(userID, req.CurrentPassword) {
		um.auditLog.LogEvent(&AuditEvent{
			UserID:   userID,
			Code:     AuditPasswordChange,
			Category: "password",
			Severity: "warning",
			Success:  false,
			Target:   user.Username,
			Message:  "Failed to change password: incorrect current password",
		})
		return fmt.Errorf("current password is incorrect")
	}

	// Validate new password
	if err := um.validatePassword(req.NewPassword, user.Username); err != nil {
		return err
	}

	// Hash and store new password
	hashedPassword := um.hashPassword(req.NewPassword)
	um.storePassword(userID, hashedPassword)

	// Update user
	user.PasswordChangedAt = time.Now()
	user.ForcePasswordChange = false

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   userID,
		Code:     AuditPasswordChange,
		Category: "password",
		Severity: "info",
		Success:  true,
		Target:   user.Username,
		Message:  "Password changed successfully",
	})

	// Save data
	um.saveData()

	return nil
}

// RequestPasswordReset initiates password reset
func (um *UserManager) RequestPasswordReset(req PasswordResetRequest, ip string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Find user
	var user *User
	for _, u := range um.users {
		if strings.EqualFold(u.Username, req.UsernameOrEmail) ||
			strings.EqualFold(u.Email, req.UsernameOrEmail) {
			user = u
			break
		}
	}

	if user == nil {
		// Don't reveal if user exists
		return nil
	}

	// Check for existing unexpired token
	for _, token := range um.resetTokens {
		if token.UserID == user.ID && token.ExpiresAt.After(time.Now()) && token.UsedAt == nil {
			// Already has valid token
			return nil
		}
	}

	// Generate token
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

	// Create reset token
	resetToken := &PasswordResetToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     um.hashPassword(tokenStr), // Store hashed
		Method:    req.Method,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		RequestIP: ip,
	}

	um.resetTokens[resetToken.ID] = resetToken

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   user.ID,
		Code:     AuditPasswordReset,
		Category: "password",
		Severity: "info",
		Success:  true,
		Target:   user.Username,
		Message:  fmt.Sprintf("Password reset requested via %s", req.Method),
		IP:       ip,
	})

	// Save data
	um.saveData()

	// Return token for console method
	if req.Method == "console" {
		// Log token to console
		um.logger.Warn().
			Str("user", user.Username).
			Str("token", tokenStr).
			Msg("Password reset token (valid for 30 minutes)")
	}

	// TODO: Send email for email method

	return nil
}

// VerifyPasswordReset completes password reset
func (um *UserManager) VerifyPasswordReset(req PasswordResetVerify, ip string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Find valid token
	var resetToken *PasswordResetToken
	for _, token := range um.resetTokens {
		if token.UsedAt == nil &&
			token.ExpiresAt.After(time.Now()) &&
			um.verifyPasswordHash(token.Token, req.Token) {
			resetToken = token
			break
		}
	}

	if resetToken == nil {
		return fmt.Errorf("invalid or expired token")
	}

	// Get user
	user, exists := um.users[resetToken.UserID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Validate new password
	if err := um.validatePassword(req.NewPassword, user.Username); err != nil {
		return err
	}

	// Update password
	hashedPassword := um.hashPassword(req.NewPassword)
	um.storePassword(user.ID, hashedPassword)

	// Mark token as used
	now := time.Now()
	resetToken.UsedAt = &now
	resetToken.ResetIP = ip

	// Update user
	user.PasswordChangedAt = time.Now()
	user.ForcePasswordChange = false

	// Revoke all sessions (force re-login)
	um.revokeUserSessionsInternal(user.ID)

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   user.ID,
		Code:     AuditPasswordResetApply,
		Category: "password",
		Severity: "warning",
		Success:  true,
		Target:   user.Username,
		Message:  "Password reset completed",
		IP:       ip,
	})

	// Save data
	um.saveData()

	return nil
}

// 2FA Management

// EnrollTwoFactor starts 2FA enrollment
func (um *UserManager) EnrollTwoFactor(userID string, password string) (*TOTPSecret, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Verify password
	if !um.verifyPassword(userID, password) {
		return nil, fmt.Errorf("incorrect password")
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "NithronOS",
		AccountName: user.Username,
		Period:      30,
		SecretSize:  32,
	})
	if err != nil {
		return nil, err
	}

	// Generate recovery codes
	recoveryCodes := um.generateRecoveryCodes()

	// Store temporarily (not verified yet)
	totpSecret := &TOTPSecret{
		Secret:      key.Secret(),
		URL:         key.URL(),
		Verified:    false,
		BackupCodes: recoveryCodes,
	}

	// Store in temp location
	um.storeTempTOTP(userID, totpSecret)

	return &TOTPSecret{
		URL:         key.URL(),
		BackupCodes: recoveryCodes, // Return codes once
	}, nil
}

// VerifyTwoFactor verifies TOTP code and enables 2FA
func (um *UserManager) VerifyTwoFactor(userID string, code string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Get temp TOTP
	totpSecret := um.getTempTOTP(userID)
	if totpSecret == nil {
		return fmt.Errorf("no pending 2FA enrollment")
	}

	// Verify code
	valid := totp.Validate(code, totpSecret.Secret)
	if !valid {
		return fmt.Errorf("invalid code")
	}

	// Enable 2FA
	user.TwoFactorEnabled = true
	now := time.Now()
	user.TwoFactorSetupAt = &now
	user.UpdatedAt = now

	// Store TOTP secret permanently
	um.storeTOTPSecret(userID, totpSecret)

	// Clear temp
	um.clearTempTOTP(userID)

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   userID,
		Code:     AuditAuth2FAEnabled,
		Category: "auth",
		Severity: "info",
		Success:  true,
		Target:   user.Username,
		Message:  "Two-factor authentication enabled",
	})

	// Save data
	um.saveData()

	return nil
}

// DisableTwoFactor disables 2FA
func (um *UserManager) DisableTwoFactor(userID string, password string, code string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Verify password
	if !um.verifyPassword(userID, password) {
		return fmt.Errorf("incorrect password")
	}

	// Verify TOTP or recovery code if provided
	if code != "" {
		if !um.verifyTOTP(userID, code) && !um.verifyRecoveryCode(userID, code) {
			return fmt.Errorf("invalid verification code")
		}
	}

	// Disable 2FA
	user.TwoFactorEnabled = false
	user.UpdatedAt = time.Now()

	// Remove TOTP secret
	um.deleteTOTPSecret(userID)

	// Audit log
	um.auditLog.LogEvent(&AuditEvent{
		UserID:   userID,
		Code:     AuditAuth2FADisabled,
		Category: "auth",
		Severity: "warning",
		Success:  true,
		Target:   user.Username,
		Message:  "Two-factor authentication disabled",
	})

	// Save data
	um.saveData()

	return nil
}

// GetRecoveryCodes returns recovery codes (admin only)
func (um *UserManager) GetRecoveryCodes(userID string) ([]RecoveryCode, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if !user.TwoFactorEnabled {
		return nil, fmt.Errorf("2FA not enabled")
	}

	totpSecret := um.getTOTPSecret(userID)
	if totpSecret == nil {
		return nil, fmt.Errorf("TOTP secret not found")
	}

	return totpSecret.BackupCodes, nil
}

// RegenerateRecoveryCodes regenerates recovery codes
func (um *UserManager) RegenerateRecoveryCodes(userID string, password string) ([]RecoveryCode, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	user, exists := um.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Verify password
	if !um.verifyPassword(userID, password) {
		return nil, fmt.Errorf("incorrect password")
	}

	if !user.TwoFactorEnabled {
		return nil, fmt.Errorf("2FA not enabled")
	}

	// Generate new codes
	recoveryCodes := um.generateRecoveryCodes()

	// Update TOTP secret
	totpSecret := um.getTOTPSecret(userID)
	if totpSecret != nil {
		totpSecret.BackupCodes = recoveryCodes
		um.storeTOTPSecret(userID, totpSecret)
	}

	// Save data
	um.saveData()

	return recoveryCodes, nil
}

// Helper methods

func (um *UserManager) hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, 64*1024, 1, 4, b64Salt, b64Hash)
}

func (um *UserManager) verifyPasswordHash(hash, password string) bool {
	// Parse hash
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return false
	}

	salt, _ := base64.RawStdEncoding.DecodeString(parts[4])
	expectedHash, _ := base64.RawStdEncoding.DecodeString(parts[5])

	// Compute hash
	computedHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Compare
	return string(expectedHash) == string(computedHash)
}

func (um *UserManager) verifyPassword(userID, password string) bool {
	hash := um.getPassword(userID)
	if hash == "" {
		return false
	}
	return um.verifyPasswordHash(hash, password)
}

func (um *UserManager) validatePassword(password, username string) error {
	// Check minimum length
	if len(password) < um.passwordPolicy.MinLength {
		return fmt.Errorf("password must be at least %d characters", um.passwordPolicy.MinLength)
	}

	// Check username similarity
	if um.passwordPolicy.ProhibitUsername {
		if strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
			return fmt.Errorf("password cannot contain username")
		}
	}

	// Check character requirements
	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasNumber = true
		default:
			hasSpecial = true
		}
	}

	if um.passwordPolicy.RequireUppercase && !hasUpper {
		return fmt.Errorf("password must contain uppercase letter")
	}
	if um.passwordPolicy.RequireLowercase && !hasLower {
		return fmt.Errorf("password must contain lowercase letter")
	}
	if um.passwordPolicy.RequireNumbers && !hasNumber {
		return fmt.Errorf("password must contain number")
	}
	if um.passwordPolicy.RequireSpecial && !hasSpecial {
		return fmt.Errorf("password must contain special character")
	}

	return nil
}

func (um *UserManager) generateRecoveryCodes() []RecoveryCode {
	codes := make([]RecoveryCode, 10)
	for i := range codes {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		code := fmt.Sprintf("%X-%X", b[:2], b[2:])
		codes[i] = RecoveryCode{
			Code:      code,
			CreatedAt: time.Now(),
		}
	}
	return codes
}

func (um *UserManager) verifyTOTP(userID, code string) bool {
	totpSecret := um.getTOTPSecret(userID)
	if totpSecret == nil {
		return false
	}
	return totp.Validate(code, totpSecret.Secret)
}

func (um *UserManager) verifyRecoveryCode(userID, code string) bool {
	totpSecret := um.getTOTPSecret(userID)
	if totpSecret == nil {
		return false
	}

	for i, rc := range totpSecret.BackupCodes {
		if rc.Code == code && rc.UsedAt == nil {
			// Mark as used
			now := time.Now()
			totpSecret.BackupCodes[i].UsedAt = &now
			um.storeTOTPSecret(userID, totpSecret)
			return true
		}
	}

	return false
}

func (um *UserManager) revokeUserSessionsInternal(userID string) {
	// Remove all sessions for user
	for id, session := range um.sessions {
		if session.UserID == userID {
			delete(um.sessions, id)
		}
	}
}

func (um *UserManager) createDefaultAdmin() {
	// Generate random password
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	password := base64.URLEncoding.EncodeToString(b)

	// Create admin user
	user := &User{
		ID:                  uuid.New().String(),
		Username:            "admin",
		Role:                RoleAdmin,
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
		PasswordChangedAt:   time.Now(),
		ForcePasswordChange: true,
	}

	um.users[user.ID] = user

	// Store password
	hashedPassword := um.hashPassword(password)
	um.storePassword(user.ID, hashedPassword)

	// Log credentials
	um.logger.Warn().
		Str("username", "admin").
		Str("password", password).
		Msg("Default admin user created. CHANGE THIS PASSWORD IMMEDIATELY!")

	// Save data
	um.saveData()
}

func (um *UserManager) cleanupRoutine() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		um.cleanup()
	}
}

func (um *UserManager) cleanup() {
	um.mu.Lock()
	defer um.mu.Unlock()

	now := time.Now()

	// Clean expired sessions
	for id, session := range um.sessions {
		if session.ExpiresAt.Before(now) {
			delete(um.sessions, id)
		}
	}

	// Clean expired reset tokens
	for id, token := range um.resetTokens {
		if token.ExpiresAt.Before(now) {
			delete(um.resetTokens, id)
		}
	}

	// Clean expired lockouts
	for id, lockout := range um.lockouts {
		if lockout.LockedUntil.Before(now) {
			delete(um.lockouts, id)
		}
	}

	// Clean old login attempts
	um.attemptsMu.Lock()
	cutoff := now.Add(-24 * time.Hour)
	newAttempts := []LoginAttempt{}
	for _, attempt := range um.loginAttempts {
		if attempt.Timestamp.After(cutoff) {
			newAttempts = append(newAttempts, attempt)
		}
	}
	um.loginAttempts = newAttempts
	um.attemptsMu.Unlock()
}

// Data persistence

func (um *UserManager) loadData() {
	// Load users
	usersPath := filepath.Join(um.dataPath, "users.json")
	if data, err := os.ReadFile(usersPath); err == nil {
		_ = json.Unmarshal(data, &um.users)
	}

	// Load sessions
	sessionsPath := filepath.Join(um.dataPath, "sessions.json")
	if data, err := os.ReadFile(sessionsPath); err == nil {
		_ = json.Unmarshal(data, &um.sessions)
	}

	// Load reset tokens
	tokensPath := filepath.Join(um.dataPath, "reset_tokens.json")
	if data, err := os.ReadFile(tokensPath); err == nil {
		_ = json.Unmarshal(data, &um.resetTokens)
	}

	// Load lockouts
	lockoutsPath := filepath.Join(um.dataPath, "lockouts.json")
	if data, err := os.ReadFile(lockoutsPath); err == nil {
		_ = json.Unmarshal(data, &um.lockouts)
	}
}

func (um *UserManager) saveData() {
	// Create directory if needed
	_ = os.MkdirAll(um.dataPath, 0700)

	// Save users
	if data, err := json.MarshalIndent(um.users, "", "  "); err == nil {
		usersPath := filepath.Join(um.dataPath, "users.json")
		_ = os.WriteFile(usersPath, data, 0600)
	}

	// Save sessions
	if data, err := json.MarshalIndent(um.sessions, "", "  "); err == nil {
		sessionsPath := filepath.Join(um.dataPath, "sessions.json")
		_ = os.WriteFile(sessionsPath, data, 0600)
	}

	// Save reset tokens
	if data, err := json.MarshalIndent(um.resetTokens, "", "  "); err == nil {
		tokensPath := filepath.Join(um.dataPath, "reset_tokens.json")
		_ = os.WriteFile(tokensPath, data, 0600)
	}

	// Save lockouts
	if data, err := json.MarshalIndent(um.lockouts, "", "  "); err == nil {
		lockoutsPath := filepath.Join(um.dataPath, "lockouts.json")
		_ = os.WriteFile(lockoutsPath, data, 0600)
	}
}

// Password storage helpers

func (um *UserManager) storePassword(userID, hash string) {
	passwordsPath := filepath.Join(um.dataPath, "passwords.json")
	passwords := make(map[string]string)

	if data, err := os.ReadFile(passwordsPath); err == nil {
		_ = json.Unmarshal(data, &passwords)
	}

	passwords[userID] = hash

	if data, err := json.Marshal(passwords); err == nil {
		_ = os.WriteFile(passwordsPath, data, 0600)
	}
}

func (um *UserManager) getPassword(userID string) string {
	passwordsPath := filepath.Join(um.dataPath, "passwords.json")
	passwords := make(map[string]string)

	if data, err := os.ReadFile(passwordsPath); err == nil {
		_ = json.Unmarshal(data, &passwords)
	}

	return passwords[userID]
}

func (um *UserManager) deletePassword(userID string) {
	passwordsPath := filepath.Join(um.dataPath, "passwords.json")
	passwords := make(map[string]string)

	if data, err := os.ReadFile(passwordsPath); err == nil {
		_ = json.Unmarshal(data, &passwords)
	}

	delete(passwords, userID)

	if data, err := json.Marshal(passwords); err == nil {
		_ = os.WriteFile(passwordsPath, data, 0600)
	}
}

// TOTP storage helpers

func (um *UserManager) storeTOTPSecret(userID string, secret *TOTPSecret) {
	totpPath := filepath.Join(um.dataPath, "totp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(totpPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	secrets[userID] = secret

	if data, err := json.Marshal(secrets); err == nil {
		_ = os.WriteFile(totpPath, data, 0600)
	}
}

func (um *UserManager) getTOTPSecret(userID string) *TOTPSecret {
	totpPath := filepath.Join(um.dataPath, "totp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(totpPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	return secrets[userID]
}

func (um *UserManager) deleteTOTPSecret(userID string) {
	totpPath := filepath.Join(um.dataPath, "totp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(totpPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	delete(secrets, userID)

	if data, err := json.Marshal(secrets); err == nil {
		_ = os.WriteFile(totpPath, data, 0600)
	}
}

func (um *UserManager) storeTempTOTP(userID string, secret *TOTPSecret) {
	tempPath := filepath.Join(um.dataPath, "totp_temp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(tempPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	secrets[userID] = secret

	if data, err := json.Marshal(secrets); err == nil {
		_ = os.WriteFile(tempPath, data, 0600)
	}
}

func (um *UserManager) getTempTOTP(userID string) *TOTPSecret {
	tempPath := filepath.Join(um.dataPath, "totp_temp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(tempPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	return secrets[userID]
}

func (um *UserManager) clearTempTOTP(userID string) {
	tempPath := filepath.Join(um.dataPath, "totp_temp.json")
	secrets := make(map[string]*TOTPSecret)

	if data, err := os.ReadFile(tempPath); err == nil {
		_ = json.Unmarshal(data, &secrets)
	}

	delete(secrets, userID)

	if data, err := json.Marshal(secrets); err == nil {
		_ = os.WriteFile(tempPath, data, 0600)
	}
}
