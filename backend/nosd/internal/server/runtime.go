package server

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

var (
	rtMu          sync.RWMutex
	rtAllowedOrig []string
	rtTrustProxy  bool
	currentLevel  zerolog.Level
)

func SetRuntimeCORSOrigin(origin string) {
	rtMu.Lock()
	defer rtMu.Unlock()
	if strings.TrimSpace(origin) == "" {
		rtAllowedOrig = []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	} else {
		rtAllowedOrig = []string{origin}
	}
}

func SetRuntimeTrustProxy(v bool) {
	rtMu.Lock()
	rtTrustProxy = v
	rtMu.Unlock()
}

func RuntimeTrustProxy() bool {
	rtMu.RLock()
	v := rtTrustProxy
	rtMu.RUnlock()
	return v
}

func getAllowedOrigins() []string {
	rtMu.RLock()
	out := make([]string, len(rtAllowedOrig))
	copy(out, rtAllowedOrig)
	rtMu.RUnlock()
	return out
}

// DynamicCORS adds CORS headers using the current runtime origin settings.
func DynamicCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range getAllowedOrigins() {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			if v := w.Header().Get("Access-Control-Allow-Origin"); v != "" {
				w.WriteHeader(204)
				_, _ = w.Write([]byte{})
				return
			}
			// not allowed
			w.WriteHeader(403)
			_, _ = w.Write([]byte(strconv.Itoa(http.StatusForbidden)))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetLogLevel updates the process log level dynamically.
// SetLogLevel changes the level of loggers created from now on (SIGHUP reload).
func SetLogLevel(l zerolog.Level) {
	rtMu.Lock()
	defer rtMu.Unlock()
	currentLevel = l
}

func logLevel() zerolog.Level {
	rtMu.RLock()
	defer rtMu.RUnlock()
	return currentLevel
}
