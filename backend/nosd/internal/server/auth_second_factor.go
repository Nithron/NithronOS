package server

import (
	"crypto/subtle"
	"strings"

	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/auth"
)

// userHasTOTP reports whether two-factor sign-in is enabled for u. "pending"
// marks an enrollment that was started but never confirmed.
func userHasTOTP(u userstore.User) bool {
	return u.TOTPEnc != "" && u.TOTPEnc != "pending"
}

// verifySecondFactor checks code against u's TOTP secret, or against one of
// u's recovery codes. A recovery code works once: it is removed from u, and
// the caller must save u. It reports whether the code was accepted.
func verifySecondFactor(cfg config.Config, u *userstore.User, code string) bool {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if code == "" {
		return false
	}
	if len(code) == 6 {
		if secret, err := decryptWithSecretKey(cfg.SecretPath, u.TOTPEnc); err == nil && auth.VerifyTOTP(string(secret), code) {
			return true
		}
	}
	h := hashRecovery(code)
	for i, stored := range u.RecoveryHashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(stored)) == 1 {
			u.RecoveryHashes = append(append([]string{}, u.RecoveryHashes[:i]...), u.RecoveryHashes[i+1:]...)
			return true
		}
	}
	return false
}
