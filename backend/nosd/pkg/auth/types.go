package auth

import (
	"time"
)

// UserRole defines user permission levels
type UserRole string

const (
	RoleAdmin    UserRole = "admin"    // Full access
	RoleOperator UserRole = "operator" // No user/role edits
	RoleViewer   UserRole = "viewer"   // Read-only
)

// User represents a system user
type User struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email,omitempty"`
	Role     UserRole `json:"role"`
	Enabled  bool     `json:"enabled"`

	// 2FA settings
	TwoFactorEnabled bool       `json:"two_factor_enabled"`
	TwoFactorSetupAt *time.Time `json:"two_factor_setup_at,omitempty"`
	LastTOTPUsed     *time.Time `json:"last_totp_used,omitempty"`

	// Metadata
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP string     `json:"last_login_ip,omitempty"`

	// Password metadata (not the password itself)
	PasswordChangedAt   time.Time `json:"password_changed_at"`
	ForcePasswordChange bool      `json:"force_password_change"`

	// Lockout status
	LockedUntil  *time.Time `json:"locked_until,omitempty"`
	FailedLogins int        `json:"failed_logins"`
}

// Session represents an active user session
type Session struct {
	ID       string   `json:"id"`
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Role     UserRole `json:"role"`

	// Session metadata
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`

	// Client info
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`

	// Security
	TwoFactorVerified bool       `json:"two_factor_verified"`
	ElevatedUntil     *time.Time `json:"elevated_until,omitempty"`
	Scopes            []string   `json:"scopes,omitempty"`

	// Token metadata
	RefreshToken string `json:"-"` // Never expose
	TokenVersion int    `json:"token_version"`
}

// PasswordResetToken represents a password reset request
type PasswordResetToken struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Token  string `json:"-"`      // Stored hashed
	Method string `json:"method"` // email or console

	// Lifecycle
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`

	// Metadata
	RequestIP string `json:"request_ip"`
	ResetIP   string `json:"reset_ip,omitempty"`
}

// RecoveryCode represents a 2FA recovery code
type RecoveryCode struct {
	Code      string     `json:"code"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// AuditEvent represents an auditable action
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Actor
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent,omitempty"`

	// Event
	Code     string `json:"code"`     // e.g., "auth.login"
	Category string `json:"category"` // auth, user, password, session, acl
	Severity string `json:"severity"` // info, warning, critical
	Success  bool   `json:"success"`

	// Details
	Target    string                 `json:"target,omitempty"` // Target user/resource
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	OldValues map[string]interface{} `json:"old_values,omitempty"`
	NewValues map[string]interface{} `json:"new_values,omitempty"`
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength        int     `json:"min_length"`
	RequireUppercase bool    `json:"require_uppercase"`
	RequireLowercase bool    `json:"require_lowercase"`
	RequireNumbers   bool    `json:"require_numbers"`
	RequireSpecial   bool    `json:"require_special"`
	MinEntropy       float64 `json:"min_entropy"` // Using zxcvbn-like scoring
	ProhibitCommon   bool    `json:"prohibit_common"`
	ProhibitUsername bool    `json:"prohibit_username"`
	ProhibitReuse    int     `json:"prohibit_reuse"` // Number of previous passwords to check
	MaxAge           int     `json:"max_age_days"`
	WarnAge          int     `json:"warn_age_days"`
}

// LoginAttempt tracks login attempts for rate limiting
type LoginAttempt struct {
	IP        string    `json:"ip"`
	Username  string    `json:"username,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason,omitempty"`
}

// Lockout represents a user or IP lockout
type Lockout struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`   // user or ip
	Target      string     `json:"target"` // username or IP
	LockedAt    time.Time  `json:"locked_at"`
	LockedUntil time.Time  `json:"locked_until"`
	Reason      string     `json:"reason"`
	ClearedAt   *time.Time `json:"cleared_at,omitempty"`
	ClearedBy   string     `json:"cleared_by,omitempty"`
	ClearReason string     `json:"clear_reason,omitempty"`
}

// TOTPSecret holds TOTP configuration
type TOTPSecret struct {
	Secret      string         `json:"-"`             // Never expose
	URL         string         `json:"url,omitempty"` // otpauth:// URL for QR
	Verified    bool           `json:"verified"`
	BackupCodes []RecoveryCode `json:"-"` // Never expose in normal responses
}

// UserCreateRequest for creating new users
type UserCreateRequest struct {
	Username string   `json:"username" validate:"required,min=3,max=32,alphanum"`
	Email    string   `json:"email,omitempty" validate:"omitempty,email"`
	Password string   `json:"password" validate:"required"`
	Role     UserRole `json:"role" validate:"required,oneof=admin operator viewer"`
}

// UserUpdateRequest for updating users
type UserUpdateRequest struct {
	Email   *string   `json:"email,omitempty"`
	Role    *UserRole `json:"role,omitempty"`
	Enabled *bool     `json:"enabled,omitempty"`
}

// PasswordChangeRequest for changing passwords
type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required"`
}

// PasswordResetRequest initiates password reset
type PasswordResetRequest struct {
	UsernameOrEmail string `json:"username_or_email" validate:"required"`
	Method          string `json:"method,omitempty"` // email or console
}

// PasswordResetVerify completes password reset
type PasswordResetVerify struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

// SessionRevokeRequest for revoking sessions
type SessionRevokeRequest struct {
	SessionID string `json:"session_id,omitempty"` // Empty = revoke all
	UserID    string `json:"user_id,omitempty"`    // Admin revoking another user's sessions
}

// TwoFactorEnrollRequest starts 2FA enrollment
type TwoFactorEnrollRequest struct {
	Password string `json:"password" validate:"required"` // Verify identity
}

// TwoFactorVerifyRequest verifies TOTP code
type TwoFactorVerifyRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// TwoFactorDisableRequest disables 2FA
type TwoFactorDisableRequest struct {
	Password string `json:"password" validate:"required"`
	Code     string `json:"code,omitempty"` // Current TOTP or recovery code
}

// AuditLogQuery for filtering audit logs
type AuditLogQuery struct {
	UserID   string    `json:"user_id,omitempty"`
	Username string    `json:"username,omitempty"`
	IP       string    `json:"ip,omitempty"`
	Code     string    `json:"code,omitempty"`
	Category string    `json:"category,omitempty"`
	From     time.Time `json:"from,omitempty"`
	To       time.Time `json:"to,omitempty"`
	Limit    int       `json:"limit,omitempty"`
	Offset   int       `json:"offset,omitempty"`
}

// Standard audit event codes
const (
	// Authentication events
	AuditAuthLogin        = "auth.login"
	AuditAuthLogout       = "auth.logout"
	AuditAuthFailed       = "auth.failed"
	AuditAuthLocked       = "auth.locked"
	AuditAuthUnlocked     = "auth.unlocked"
	AuditAuth2FAEnabled   = "auth.2fa.enabled"
	AuditAuth2FADisabled  = "auth.2fa.disabled"
	AuditAuth2FAFailed    = "auth.2fa.failed"
	AuditAuthTokenRefresh = "auth.token.refresh"

	// User management events
	AuditUserCreate     = "user.create"
	AuditUserUpdate     = "user.update"
	AuditUserDelete     = "user.delete"
	AuditUserEnable     = "user.enable"
	AuditUserDisable    = "user.disable"
	AuditUserRoleChange = "user.role.change"

	// Password events
	AuditPasswordChange     = "password.change"
	AuditPasswordReset      = "password.reset.request"
	AuditPasswordResetApply = "password.reset.apply"
	AuditPasswordExpired    = "password.expired"

	// Session events
	AuditSessionCreate  = "session.create"
	AuditSessionRevoke  = "session.revoke"
	AuditSessionExpired = "session.expired"
	AuditSessionElevate = "session.elevate"

	// Access control events
	AuditACLDenied = "acl.denied"
	AuditACLGrant  = "acl.grant"
	AuditACLRevoke = "acl.revoke"
)

// GetRolePermissions returns permissions for a role
func (r UserRole) GetPermissions() []string {
	switch r {
	case RoleAdmin:
		return []string{
			"users:read", "users:write", "users:delete",
			"roles:read", "roles:write",
			"system:read", "system:write",
			"storage:read", "storage:write",
			"apps:read", "apps:write",
			"backup:read", "backup:write",
			"monitor:read", "monitor:write",
			"audit:read", "audit:clear",
		}
	case RoleOperator:
		return []string{
			"users:read", // No write
			"roles:read", // No write
			"system:read", "system:write",
			"storage:read", "storage:write",
			"apps:read", "apps:write",
			"backup:read", "backup:write",
			"monitor:read", "monitor:write",
			"audit:read",
		}
	case RoleViewer:
		return []string{
			"users:read",
			"roles:read",
			"system:read",
			"storage:read",
			"apps:read",
			"backup:read",
			"monitor:read",
			"audit:read",
		}
	default:
		return []string{}
	}
}

// CanManageUsers checks if role can manage users
func (r UserRole) CanManageUsers() bool {
	return r == RoleAdmin
}

// CanWrite checks if role has write permissions
func (r UserRole) CanWrite() bool {
	return r == RoleAdmin || r == RoleOperator
}

// IsValid checks if role is valid
func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}
