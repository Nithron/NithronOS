package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"nos/backend/nosd/internal/auth/hash"
	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

// The roles a user can have. Admins manage the system; users reach the
// shares, apps and sync they are given.
const (
	roleAdmin = "admin"
	roleUser  = "user"
)

// UserAccount is a user as the API shows it. (It used to also report an
// email, a display name and an "enabled" flag that weren't stored: the email
// was the username and every user was "enabled".)
type UserAccount struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	Roles            []string   `json:"roles"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	TwoFactorEnabled bool       `json:"two_factor_enabled"`
	// LockedUntil is set while too many failed sign-ins keep the user out.
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// CreateUserRequest is the body of POST /users.
type CreateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

// UpdateUserRequest is the body of PUT /users/{id}: only roles change. (It
// used to take an "email" that silently renamed the account, which cut it
// off from its SMB account and its share access.)
type UpdateUserRequest struct {
	Roles []string `json:"roles"`
}

// ChangePasswordRequest is a signed-in user changing their own password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// systemAccounts keeps a user's Unix/SMB account (file sharing) in step with
// their nOS account: same name, same password.
type systemAccounts interface {
	SetPassword(ctx context.Context, username, password string) error
	Remove(ctx context.Context, username string) error
}

// agentAccounts does it through nos-agent.
type agentAccounts struct{ agent AgentClient }

func (a agentAccounts) SetPassword(ctx context.Context, username, password string) error {
	return a.agent.PostJSON(ctx, "/v1/smb/user-create", map[string]string{"username": username, "password": password}, nil)
}

func (a agentAccounts) Remove(ctx context.Context, username string) error {
	return a.agent.PostJSON(ctx, "/v1/smb/user-delete", map[string]string{"username": username}, nil)
}

// UsersHandler serves /api/v1/users (admins) and a user's own password change.
type UsersHandler struct {
	store    *userstore.Store
	sessions userSessions
	config   config.Config
	// system keeps the SMB account in step. Nil skips it (tests, no agent).
	system systemAccounts
	// onDeleted runs after a user is deleted (takes them out of groups and
	// share access lists). Nil does nothing.
	onDeleted func(ctx context.Context, username string)
}

// userSessions is what UsersHandler needs from the session store.
type userSessions interface {
	RevokeAll(uid string) error
	RevokeOthers(uid, keepSID string) error
}

// NewUsersHandler creates a new users handler.
func NewUsersHandler(store *userstore.Store, sessions userSessions, cfg config.Config) *UsersHandler {
	return &UsersHandler{store: store, sessions: sessions, config: cfg}
}

// Routes are for admins.
func (h *UsersHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListUsers)
	r.Post("/", h.CreateUser)
	r.Get("/{id}", h.GetUser)
	r.Put("/{id}", h.UpdateUser)
	r.Delete("/{id}", h.DeleteUser)
	r.Post("/{id}/reset-password", h.ResetPassword)
	r.Post("/{id}/roles", h.UpdateUser)
	// An admin can turn off a user's two-factor sign-in (a lost phone); only
	// the user can turn it on. (A "toggle" here stored the word "pending" as
	// the secret, and a recovery-code generator made codes from the clock.)
	r.Post("/{id}/2fa/disable", h.DisableTwoFactor)
	return r
}

// accountNameRe is a name that works as a Unix account (Samba, ACLs, groups).
var accountNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{2,31}$`)

func validAccountName(s string) bool { return accountNameRe.MatchString(s) }

// passwordProblem says what is wrong with a new password, or "". SMB takes it
// on one line, so line breaks are refused.
func passwordProblem(p string) string {
	switch {
	case len(p) < 8:
		return "Password must be at least 8 characters"
	case len(p) > 256:
		return "Password must be at most 256 characters"
	case strings.ContainsAny(p, "\r\n\x00"):
		return "Password can't contain line breaks"
	}
	return ""
}

func validRoles(roles []string) bool {
	if len(roles) == 0 {
		return false
	}
	for _, r := range roles {
		if r != roleAdmin && r != roleUser {
			return false
		}
	}
	return true
}

func toAccount(u userstore.User) UserAccount {
	a := UserAccount{
		ID:               u.ID,
		Username:         u.Username,
		Roles:            u.Roles,
		CreatedAt:        parseTime(u.CreatedAt),
		UpdatedAt:        parseTime(u.UpdatedAt),
		TwoFactorEnabled: userHasTOTP(u),
	}
	if a.Roles == nil {
		a.Roles = []string{}
	}
	if t := parseTime(u.LastLoginAt); !t.IsZero() {
		a.LastLoginAt = &t
	}
	if t := parseTime(u.LockedUntil); t.After(time.Now()) {
		a.LockedUntil = &t
	}
	return a
}

func (h *UsersHandler) findOr404(w http.ResponseWriter, id string) (userstore.User, bool) {
	u, err := h.store.FindByID(id)
	switch {
	case errors.Is(err, userstore.ErrUserNotFound):
		httpx.WriteTypedError(w, http.StatusNotFound, "user.not_found", "User not found", 0)
		return u, false
	case err != nil:
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.get_failed", "Failed to get user", 0)
		return u, false
	}
	return u, true
}

// ListUsers returns all users.
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.List()
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "users.list_failed", "Failed to list users", 0)
		return
	}
	out := make([]UserAccount, 0, len(users))
	for _, u := range users {
		out = append(out, toAccount(u))
	}
	writeJSON(w, out)
}

// GetUser returns one user.
func (h *UsersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	if u, ok := h.findOr404(w, chi.URLParam(r, "id")); ok {
		writeJSON(w, toAccount(u))
	}
}

// CreateUser creates a user and their SMB account. (The username used to be
// replaced by an "email" field, so new users got names no share could use.)
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !validAccountName(req.Username) {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_username",
			"Username: 3-32 lowercase letters, digits, '-' or '_', starting with a letter", 0)
		return
	}
	if p := passwordProblem(req.Password); p != "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.weak_password", p, 0)
		return
	}
	if len(req.Roles) == 0 {
		req.Roles = []string{roleUser}
	}
	if !validRoles(req.Roles) {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_roles", "Roles must be admin or user", 0)
		return
	}
	if _, err := h.store.FindByUsername(req.Username); err == nil {
		httpx.WriteTypedError(w, http.StatusConflict, "user.already_exists", "A user with this name already exists", 0)
		return
	}
	hashed, err := hash.HashPassword(req.Password)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.hash_failed", "Failed to hash password", 0)
		return
	}
	// The SMB account first: a name the system refuses (root, a daemon
	// account) must not become a nOS user either.
	if h.system != nil {
		if err := h.system.SetPassword(r.Context(), req.Username, req.Password); err != nil {
			log.Error().Err(err).Str("user", req.Username).Msg("create SMB account")
			httpx.WriteTypedError(w, http.StatusBadGateway, "user.system_account_failed",
				"The file-sharing account could not be created: "+agentErrorMessage(err), 0)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	u := userstore.User{ID: generateUUID(), Username: req.Username, PasswordHash: hashed, Roles: req.Roles, CreatedAt: now, UpdatedAt: now}
	if err := h.store.UpsertUser(u); err != nil {
		if h.system != nil {
			_ = h.system.Remove(context.Background(), req.Username)
		}
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.create_failed", "Failed to create user", 0)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, toAccount(u))
}

// UpdateUser changes a user's roles. The last administrator stays one.
func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var req UpdateUserRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}
	if !validRoles(req.Roles) {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_roles", "Roles must be admin or user", 0)
		return
	}
	u, ok := h.findOr404(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if contains(u.Roles, roleAdmin) && !contains(req.Roles, roleAdmin) && h.adminCount() <= 1 {
		httpx.WriteTypedError(w, http.StatusConflict, "user.last_admin", "The last administrator must stay an administrator", 0)
		return
	}
	u.Roles = req.Roles
	u.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := h.store.UpsertUser(u); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update user", 0)
		return
	}
	writeJSON(w, toAccount(u))
}

func (h *UsersHandler) adminCount() int {
	users, _ := h.store.List()
	n := 0
	for _, u := range users {
		if contains(u.Roles, roleAdmin) {
			n++
		}
	}
	return n
}

// DeleteUser deletes a user, signs them out, takes them out of groups and
// shares, and removes their SMB access.
func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	gone, findErr := h.store.FindByID(userID)
	// The store refuses to delete the last administrator.
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
	if err := h.sessions.RevokeAll(userID); err != nil {
		log.Error().Err(err).Str("uid", userID).Msg("revoke sessions of deleted user")
	}
	if findErr == nil {
		if h.onDeleted != nil {
			h.onDeleted(r.Context(), gone.Username)
		}
		if h.system != nil {
			if err := h.system.Remove(r.Context(), gone.Username); err != nil {
				log.Error().Err(err).Str("user", gone.Username).Msg("remove SMB account of deleted user")
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// setPassword sets u's password everywhere it is used: SMB first (so a
// refused SMB password leaves nothing half-changed), then nOS.
func (h *UsersHandler) setPassword(ctx context.Context, u *userstore.User, password string) (code, msg string, status int) {
	hashed, err := hash.HashPassword(password)
	if err != nil {
		return "user.hash_failed", "Failed to hash password", http.StatusInternalServerError
	}
	if h.system != nil {
		if err := h.system.SetPassword(ctx, u.Username, password); err != nil {
			log.Error().Err(err).Str("user", u.Username).Msg("set SMB password")
			return "user.system_account_failed", "The file-sharing password could not be set: " + agentErrorMessage(err), http.StatusBadGateway
		}
	}
	u.PasswordHash = hashed
	u.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := h.store.UpsertUser(*u); err != nil {
		return "user.save_failed", "Failed to save the user", http.StatusInternalServerError
	}
	return "", "", 0
}

// ResetPassword sets another user's password (an admin). The user's sessions
// end and a lockout is cleared.
func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_body", "Invalid request body", 0)
		return
	}
	if p := passwordProblem(req.Password); p != "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.weak_password", p, 0)
		return
	}
	u, ok := h.findOr404(w, userID)
	if !ok {
		return
	}
	u.FailedAttempts, u.LockedUntil = 0, ""
	if code, msg, status := h.setPassword(r.Context(), &u, req.Password); status != 0 {
		httpx.WriteTypedError(w, status, code, msg, 0)
		return
	}
	if err := h.sessions.RevokeAll(userID); err != nil {
		log.Error().Err(err).Str("uid", userID).Msg("revoke sessions after password reset")
	}
	w.WriteHeader(http.StatusNoContent)
}

// ChangeOwnPassword is POST /api/v1/auth/password: any signed-in user, with
// their current password. Their other sessions end; this one stays. (The
// only password change used to be under the admin-only users API.)
func (h *UsersHandler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	caller, ok := signedInUser(r)
	if !ok {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Sign in required", 0)
		return
	}
	var req ChangePasswordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.invalid_request", "Invalid request body", 0)
		return
	}
	u, ok := h.findOr404(w, caller.UID)
	if !ok {
		return
	}
	if !hash.VerifyPassword(u.PasswordHash, req.CurrentPassword) {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "user.invalid_password", "Current password is incorrect", 0)
		return
	}
	if p := passwordProblem(req.NewPassword); p != "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "user.weak_password", p, 0)
		return
	}
	if code, msg, status := h.setPassword(r.Context(), &u, req.NewPassword); status != 0 {
		httpx.WriteTypedError(w, status, code, msg, 0)
		return
	}
	if err := h.sessions.RevokeOthers(u.ID, caller.SID); err != nil {
		log.Error().Err(err).Str("uid", u.ID).Msg("revoke sessions after password change")
	}
	writeJSON(w, map[string]bool{"success": true})
}

// DisableTwoFactor turns off a user's two-factor sign-in and recovery codes.
func (h *UsersHandler) DisableTwoFactor(w http.ResponseWriter, r *http.Request) {
	u, ok := h.findOr404(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	u.TOTPEnc, u.RecoveryHashes = "", nil
	u.TOTPPendingEnc, u.RecoveryPendingHashes = "", nil
	u.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := h.store.UpsertUser(u); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "user.update_failed", "Failed to update the user", 0)
		return
	}
	writeJSON(w, toAccount(u))
}

// agentErrorMessage is the part of an agent error worth showing.
func agentErrorMessage(err error) string {
	var he *agentclient.HTTPError
	if errors.As(err, &he) {
		var body struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(he.Body), &body) == nil && body.Error != "" {
			return body.Error
		}
	}
	return "the system agent did not answer"
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
