package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	pwhash "nithronos/backend/nosd/internal/auth/hash"
	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/auth"
	"nithronos/backend/nosd/pkg/httpx"
)

// twoFactor is the one implementation of TOTP two-factor sign-in. Secrets
// and recovery codes live in the user store (the secret encrypted with the
// server key, recovery codes hashed), which is what login checks.
//
// Enrolling stores a *pending* secret; it becomes active only after a valid
// code confirms it, so abandoning enrollment can't lock anyone out.
//
// (The settings page used to talk to an in-memory TOTP manager that treated
// every request as user "user-123", accepted any password, was forgotten on
// restart and was never consulted at login.)
type twoFactor struct {
	cfg   config.Config
	users *userstore.Store
}

var (
	errNoPendingTOTP = errors.New("no two-factor enrollment in progress")
	errBadTOTPCode   = errors.New("invalid code")
)

// currentUser loads the signed-in user (session cookie, or the legacy session
// codec via withUser).
func (t twoFactor) currentUser(r *http.Request) (userstore.User, bool) {
	uid, ok := decodeSessionUID(r, t.cfg)
	if !ok {
		uid, ok = r.Context().Value(ctxUserID).(string)
	}
	if !ok || uid == "" || t.users == nil {
		return userstore.User{}, false
	}
	u, err := t.users.FindByID(uid)
	return u, err == nil
}

// begin starts enrollment: a new secret and recovery codes are stored as
// pending. It returns the secret, its otpauth:// URI and the recovery codes.
func (t twoFactor) begin(u *userstore.User) (secret, uri string, recovery []string, err error) {
	secret, uri, err = auth.GenerateTOTPSecret("NithronOS", u.Username)
	if err != nil {
		return "", "", nil, err
	}
	enc, err := encryptWithSecretKey(t.cfg.SecretPath, []byte(secret))
	if err != nil {
		return "", "", nil, err
	}
	recovery, hashes := generateRecoveryCodes()
	u.TOTPPendingEnc = enc
	u.RecoveryPendingHashes = hashes
	return secret, uri, recovery, t.users.UpsertUser(*u)
}

// confirm activates the pending secret when code is valid for it.
func (t twoFactor) confirm(u *userstore.User, code string) error {
	if u.TOTPPendingEnc == "" {
		return errNoPendingTOTP
	}
	secret, err := decryptWithSecretKey(t.cfg.SecretPath, u.TOTPPendingEnc)
	if err != nil {
		return err
	}
	if !auth.VerifyTOTP(string(secret), strings.TrimSpace(code)) {
		return errBadTOTPCode
	}
	u.TOTPEnc, u.TOTPPendingEnc = u.TOTPPendingEnc, ""
	if len(u.RecoveryPendingHashes) > 0 {
		u.RecoveryHashes, u.RecoveryPendingHashes = u.RecoveryPendingHashes, nil
	}
	return t.users.UpsertUser(*u)
}

func (t twoFactor) disable(u *userstore.User) error {
	u.TOTPEnc, u.TOTPPendingEnc = "", ""
	u.RecoveryHashes, u.RecoveryPendingHashes = nil, nil
	return t.users.UpsertUser(*u)
}

func qrDataURI(uri string) string {
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// SetupRoutes serves the setup wizard's /api/v1/auth/totp/{enroll,verify}.
func (t twoFactor) registerTOTPRoutes(r chi.Router) {
	enroll := func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, uri, _, err := t.begin(&u)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "totp error")
			return
		}
		png := strings.TrimPrefix(qrDataURI(uri), "data:image/png;base64,")
		writeJSON(w, map[string]any{"otpauth_url": uri, "qr_png_base64": png})
	}
	r.Get("/api/v1/auth/totp/enroll", enroll)
	r.Post("/api/v1/auth/totp/enroll", enroll)
	r.Post("/api/v1/auth/totp/verify", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct{ Code string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Recovery codes are shown once, at confirmation; generate fresh ones
		// here so the wizard can display them.
		plain, hashes := generateRecoveryCodes()
		u.RecoveryPendingHashes = hashes
		switch err := t.confirm(&u, body.Code); {
		case err == nil:
			writeJSON(w, map[string]any{"ok": true, "recovery_codes": plain})
		case errors.Is(err, errBadTOTPCode):
			httpx.WriteError(w, http.StatusUnauthorized, "invalid code")
		case errors.Is(err, errNoPendingTOTP):
			httpx.WriteError(w, http.StatusBadRequest, "no enrollment in progress")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "persist error")
		}
	})
}

// settingsRoutes serves the settings page's /api/v1/auth/2fa/*.
func (t twoFactor) settingsRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/2fa/status", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		on := userHasTOTP(u)
		writeJSON(w, map[string]bool{"enrolled": on, "verified": on, "required": on})
	})
	r.Post("/2fa/enroll", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !pwhash.VerifyPassword(u.PasswordHash, body.Password) {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.password.invalid", "Invalid password", 0)
			return
		}
		secret, uri, recovery, err := t.begin(&u)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "totp error")
			return
		}
		writeJSON(w, map[string]any{"secret": secret, "uri": uri, "qr_code": qrDataURI(uri), "backup_codes": recovery})
	})
	r.Post("/2fa/verify", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		err := t.confirm(&u, body.Code)
		if errors.Is(err, errNoPendingTOTP) && userHasTOTP(u) {
			// Already enabled: just check the code.
			if verifySecondFactor(t.cfg, &u, body.Code) {
				err = t.users.UpsertUser(u) // a recovery code may have been spent
			} else {
				err = errBadTOTPCode
			}
		}
		switch {
		case err == nil:
			writeJSON(w, map[string]string{"status": "verified", "message": "Two-factor sign-in is enabled."})
		case errors.Is(err, errBadTOTPCode):
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.totp.invalid", "Invalid code", 0)
		case errors.Is(err, errNoPendingTOTP):
			httpx.WriteTypedError(w, http.StatusBadRequest, "auth.totp.not_enrolling", "Start enrollment first", 0)
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "persist error")
		}
	})
	r.Post("/2fa/disable", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := t.disable(&u); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "persist error")
			return
		}
		writeJSON(w, map[string]string{"status": "disabled", "message": "Two-factor sign-in is disabled."})
	})
	r.Post("/2fa/backup-codes", func(w http.ResponseWriter, r *http.Request) {
		u, ok := t.currentUser(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !userHasTOTP(u) {
			httpx.WriteTypedError(w, http.StatusConflict, "auth.totp.not_enabled", "Two-factor sign-in is not enabled", 0)
			return
		}
		plain, hashes := generateRecoveryCodes()
		u.RecoveryHashes = hashes
		if err := t.users.UpsertUser(u); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "persist error")
			return
		}
		writeJSON(w, map[string][]string{"backup_codes": plain})
	})
	return r
}
