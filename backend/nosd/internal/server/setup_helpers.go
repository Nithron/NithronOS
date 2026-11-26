package server

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"nos/backend/nosd/internal/config"

	// "nos/backend/nosd/pkg/firewall"

	"github.com/gorilla/securecookie"
)

// legacy in-memory rate limiter removed; persisted ratelimit store governs throttling

// setup token helpers using secret.key
func setupEncodeToken(cfg config.Config, payload map[string]any) (string, error) {
	key, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		return "", err
	}
	sc := securecookie.New(key, nil)
	sc.MaxAge(600)
	return sc.Encode("nos_setup", payload)
}

func setupDecodeToken(cfg config.Config, tok string) (map[string]any, error) {
	key, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		return nil, err
	}
	sc := securecookie.New(key, nil)
	var out map[string]any
	if err := sc.Decode("nos_setup", tok, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// validPassword is setup's rule for the first administrator's password.
func validPassword(p string) bool {
	if len(p) < 12 {
		return false
	}
	hasLower, hasUpper, hasDigit, hasOther := false, false, false, false
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasOther = true
		}
	}
	count := 0
	if hasLower {
		count++
	}
	if hasUpper {
		count++
	}
	if hasDigit {
		count++
	}
	if hasOther {
		count++
	}
	return count >= 3
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

// clientIP extracts client IP. If TrustProxy is true, use last untrusted hop from X-Forwarded-For; otherwise use RemoteAddr.

func dirPermInfo(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	st := fi.Mode().Perm()
	// ls -ld-like summary (very simplified)
	ownerGid := ""
	if out, e := exec.Command("/usr/bin/stat", "-c", "%U:%G", path).CombinedOutput(); e == nil {
		ownerGid = strings.TrimSpace(string(out))
	}
	return fmt.Sprintf("ls -ld %s => mode %o owner %s; recommend: chown -R nos:nos %s && chmod 0750 %s", path, st, ownerGid, path, path)
}

// Health monitoring handlers
