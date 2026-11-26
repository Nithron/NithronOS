package server

import (
	"encoding/json"
	"errors"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"

	"nithronos/backend/nosd/internal/auth/hash"
	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// UserAccount represents a user account in the API
type UserAccount struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name,omitempty"`
	Roles            []string  `json:"roles"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	LastLoginAt      time.Time `json:"last_login_at,omitempty"`
	Enabled          bool      `json:"enabled"`
	TwoFactorEnabled bool      `json:"two_factor_enabled"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	DisplayName *string   `json:"display_name,omitempty"`
	Email       *string   `json:"email,omitempty"`
	Roles       *[]string `json:"roles,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UsersHandler handles user management endpoints
type UsersHandler struct {
	store    *userstore.Store
	sessions userSessions
	config   config.Config
}

// userSessions is what UsersHandler needs from the session store.
type userSessions interface {
	RevokeAll(uid string) error
	RevokeOthers(uid, keepSID string) error
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(store *userstore.Store, sessions userSessions, cfg config.Config) *UsersHandler {
	return &UsersHandler{
		store:    store,
		sessions: sessions,
		config:   cfg,
	}
}

// ListUsers returns all users
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.List()
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "users.list_failed", "Failed to list users", 0)
		return
	}

	// Convert to API response format
	apiUsers := make([]UserAccount, 0, len(users))
	for _, u := range users {
		apiUser := UserAccount{
			ID:               u.ID,
			Username:         u.Username,
			Email:            u.Username, // Username is email in current implementation
			DisplayName:      "",         // Not in current store
			Roles:            u.Roles,
			CreatedAt:        parseTime(u.CreatedAt),
			UpdatedAt:        parseTime(u.UpdatedAt),
			Enabled:          true, // Not in current store
			TwoFactorEnabled: u.TOTPEnc != "",
		}
		if u.LastLoginAt != "" {
			apiUser.LastLoginAt = parseTime(u.LastLoginAt)
		}
		apiUsers = append(apiUsers, apiUser)
	}

	writeJSON(w, apiUsers)
}

// GetUser returns a specific user
func (h *UsersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	apiUser := UserAccount{
		ID:               user.ID,
		Username:         user.Username,
		Email:            user.Username,
		DisplayName:      "", // Not in current store
		Roles:            user.Roles,
		CreatedAt:        parseTime(user.CreatedAt),
		UpdatedAt:        parseTime(user.UpdatedAt),
		Enabled:          true, // Not in current store
		TwoFactorEnabled: user.TOTPEnc != "",
	}
	if user.LastLoginAt != "" {
		apiUser.LastLoginAt = parseTime(user.LastLoginAt)
	}

	writeJSON(w, apiUser)
}

// CreateUser creates a new user
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}

	// Validate request
	if req.Username == "" || req.Email == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.missing_fields", "Username and email are required", 0)
		return
	}

	if req.Password == "" || len(req.Password) < 8 {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.weak_password", "Password must be at least 8 characters", 0)
		return
	}

	// Check if user already exists
	if _, err := h.store.FindByUsername(req.Email); err == nil {
		httpx.WriteTypedError(w, http.StatusConflict, "user.already_exists", "User with this email already exists", 0)
		return
	}

	// Hash password
	hashedPassword, err := hash.HashPassword(req.Password)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.hash_failed", "Failed to hash password", 0)
		return
	}

	// Create user
	now := time.Now().UTC().Format(time.RFC3339)
	newUser := userstore.User{
		ID:           generateUUID(),
		Username:     req.Email,
		PasswordHash: hashedPassword,
		Roles:        req.Roles,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if len(newUser.Roles) == 0 {
		newUser.Roles = []string{"user"}
	}

	// Save user
	if err := h.store.UpsertUser(newUser); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.create_failed", "Failed to create user", 0)
		return
	}

	// Return created user
	apiUser := UserAccount{
		ID:               newUser.ID,
		Username:         newUser.Username,
		Email:            newUser.Username,
		DisplayName:      req.DisplayName,
		Roles:            newUser.Roles,
		CreatedAt:        parseTime(newUser.CreatedAt),
		UpdatedAt:        parseTime(newUser.UpdatedAt),
		Enabled:          true,
		TwoFactorEnabled: false,
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, apiUser)
}

// UpdateUser updates an existing user
func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}

	// Get existing user
	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	// Update fields
	// DisplayName not in store
	if req.Email != nil {
		user.Username = *req.Email
	}
	if req.Roles != nil {
		user.Roles = *req.Roles
	}
	// Enabled/Disabled not in store

	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Save updated user
	if err := h.store.UpsertUser(user); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update user", 0)
		return
	}

	// Return updated user
	apiUser := UserAccount{
		ID:               user.ID,
		Username:         user.Username,
		Email:            user.Username,
		DisplayName:      "", // Not in store
		Roles:            user.Roles,
		CreatedAt:        parseTime(user.CreatedAt),
		UpdatedAt:        parseTime(user.UpdatedAt),
		Enabled:          true, // Not in store
		TwoFactorEnabled: user.TOTPEnc != "",
	}
	if user.LastLoginAt != "" {
		apiUser.LastLoginAt = parseTime(user.LastLoginAt)
	}

	writeJSON(w, apiUser)
}

// DeleteUser deletes a user
func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	// The store refuses to delete the last administrator. (This used to refuse
	// every admin, and for other users it re-saved the remaining users without
	// ever removing the deleted one, so nobody was actually deleted.)
	if err := h.store.DeleteUser(userID); err != nil {
		switch {
		case errors.Is(err, userstore.ErrUserNotFound):
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		case errors.Is(err, userstore.ErrLastAdmin):
			httpx.WriteTypedError(w, http.StatusConflict, "user.last_admin", "Cannot delete the last administrator", 0)
		default:
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.delete_failed", "Failed to delete user", 0)
		}
		return
	}
	// A deleted user's browsers are signed out at once.
	if err := h.sessions.RevokeAll(userID); err != nil {
		log.Error().Err(err).Str("uid", userID).Msg("revoke sessions of deleted user")
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword changes a user's password
func (h *UsersHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}

	// The caller. (This read a context key nothing set, and the unchecked
	// type assertion panicked, so changing a password always failed.)
	caller, ok := signedInUser(r)
	if !ok {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Sign in required", 0)
		return
	}
	currentUserID := caller.UID

	// Users can only change their own password (unless admin)
	if currentUserID != userID {
		currentUser, _ := h.store.FindByID(currentUserID)
		isAdmin := false
		for _, role := range currentUser.Roles {
			if role == "admin" {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			httpx.WriteTypedError(w, http.StatusForbidden, "user.forbidden", "You can only change your own password", 0)
			return
		}
	}

	// Get user
	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	// Verify current password (if not admin changing another user's password)
	if currentUserID == userID {
		if !hash.VerifyPassword(user.PasswordHash, req.CurrentPassword) {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "user.invalid_password", "Current password is incorrect", 0)
			return
		}
	}

	// Validate new password
	if len(req.NewPassword) < 8 {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.weak_password", "Password must be at least 8 characters", 0)
		return
	}

	// Hash new password
	hashedPassword, err := hash.HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.hash_failed", "Failed to hash password", 0)
		return
	}

	// Update password
	user.PasswordHash = hashedPassword
	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := h.store.UpsertUser(user); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update password", 0)
		return
	}
	// The old password's sessions end; the browser that changed its own
	// password stays signed in.
	keep := ""
	if currentUserID == userID {
		keep = caller.SID
	}
	if err := h.sessions.RevokeOthers(userID, keep); err != nil {
		log.Error().Err(err).Str("uid", userID).Msg("revoke sessions after password change")
	}

	writeJSON(w, map[string]bool{"success": true})
}

// Helper function to parse time
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// Helper function to check if string is in slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Routes returns the routes for the users handler
func (h *UsersHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// User CRUD operations
	r.Get("/", h.ListUsers)
	r.Post("/", h.CreateUser)
	r.Get("/{id}", h.GetUser)
	r.Put("/{id}", h.UpdateUser)
	r.Delete("/{id}", h.DeleteUser)

	// Password management
	r.Post("/{id}/password", h.ChangePassword)

	// Role management
	r.Post("/{id}/roles", h.SetUserRoles)

	// 2FA management
	r.Post("/{id}/2fa/toggle", h.ToggleUser2FA)
	r.Post("/{id}/recovery-codes", h.GenerateRecoveryCodes)

	return r
}

// SetUserRoles updates a user's roles
func (h *UsersHandler) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	var req struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}

	// Get user
	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	// Check if removing admin role from last admin
	if contains(user.Roles, "admin") && !contains(req.Roles, "admin") {
		// Check if this is the last admin
		users, _ := h.store.List()
		adminCount := 0
		for _, u := range users {
			if contains(u.Roles, "admin") {
				adminCount++
			}
		}
		if adminCount <= 1 {
			httpx.WriteTypedError(w, http.StatusForbidden, "user.last_admin", "Cannot remove admin role from the last admin", 0)
			return
		}
	}

	// Update roles
	user.Roles = req.Roles
	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := h.store.UpsertUser(user); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update user roles", 0)
		return
	}

	writeJSON(w, map[string]any{"success": true, "roles": user.Roles})
}

// ToggleUser2FA enables or disables 2FA for a user
func (h *UsersHandler) ToggleUser2FA(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	var req struct {
		Enable bool   `json:"enable"`
		Code   string `json:"code,omitempty"` // Required when disabling
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}

	// Get user
	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	// Check current user permissions
	currentUserID := r.Context().Value("user_id")
	if currentUserID != nil && currentUserID.(string) != userID {
		// Only the user themselves or an admin can toggle 2FA
		currentUser, _ := h.store.FindByID(currentUserID.(string))
		if !contains(currentUser.Roles, "admin") {
			httpx.WriteTypedError(w, http.StatusForbidden, "user.forbidden", "You can only manage your own 2FA settings", 0)
			return
		}
	}

	if req.Enable {
		// Enable 2FA - mark as pending until verified
		user.TOTPEnc = "pending"
	} else {
		// Disable 2FA - clear TOTP and recovery codes
		user.TOTPEnc = ""
		user.RecoveryHashes = nil
	}

	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := h.store.UpsertUser(user); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update 2FA settings", 0)
		return
	}

	writeJSON(w, map[string]any{
		"success": true,
		"enabled": req.Enable,
	})
}

// GenerateRecoveryCodes generates new recovery codes for a user
func (h *UsersHandler) GenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.id_required", "User ID is required", 0)
		return
	}

	// Get user
	user, err := h.store.FindByID(userID)
	if err != nil {
		if err == userstore.ErrUserNotFound {
			httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		} else {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		}
		return
	}

	// Check if 2FA is enabled
	if user.TOTPEnc == "" || user.TOTPEnc == "pending" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.2fa_not_enabled", "2FA must be enabled to generate recovery codes", 0)
		return
	}

	// Check current user permissions
	currentUserID := r.Context().Value("user_id")
	if currentUserID != nil && currentUserID.(string) != userID {
		// Only the user themselves or an admin can generate recovery codes
		currentUser, _ := h.store.FindByID(currentUserID.(string))
		if !contains(currentUser.Roles, "admin") {
			httpx.WriteTypedError(w, http.StatusForbidden, "user.forbidden", "You can only manage your own recovery codes", 0)
			return
		}
	}

	// Generate new recovery codes
	codes := make([]string, 10)
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code := generateRecoveryCode()
		codes[i] = code
		// Hash the recovery code for storage
		hashedCode, _ := hash.HashPassword(code)
		hashes[i] = hashedCode
	}

	// Update user with new recovery code hashes
	user.RecoveryHashes = hashes
	user.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := h.store.UpsertUser(user); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to save recovery codes", 0)
		return
	}

	// Return the plain codes (only shown once)
	writeJSON(w, map[string]any{
		"success": true,
		"codes":   codes,
		"message": "Save these codes in a safe place. They will not be shown again.",
	})
}

// generateRecoveryCode generates a random recovery code
func generateRecoveryCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
