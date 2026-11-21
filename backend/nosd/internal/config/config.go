package config

import (
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Port          int
	LogLevel      zerolog.Level
	UsersPath     string
	SecretPath    string
	FirstBootPath string
	// FirstBootIssuePath is the console banner (agetty issue snippet) that
	// shows the setup code; empty disables it.
	FirstBootIssuePath string
	SessionsPath       string
	RateLimitPath      string
	SharesPath         string
	SessionHashKey     []byte
	SessionBlockKey    []byte
	EtcDir             string
	AppsDataDir        string
	AppsInstallDir     string
	TrustProxy         bool
	RateOTPPerMin      int
	RateLoginPer15m    int
	RateOTPWindowSec   int
	RateLoginWindowSec int
	// new fields
	Bind                     string
	CORSOrigin               string
	SessionAccessTTLSeconds  int
	SessionRefreshTTLSeconds int
	MetricsEnabled           bool
	PprofEnabled             bool
	MetricsAllowlist         []string
	AllowAgentRegistration   bool
	RecoveryMode             bool
}

type fileYAML struct {
	HTTP struct {
		Bind string `yaml:"bind"`
	} `yaml:"http"`
	CORS struct {
		Origin string `yaml:"origin"`
	} `yaml:"cors"`
	Rate struct {
		OTPPerMin      int `yaml:"otpPerMin"`
		LoginPer15m    int `yaml:"loginPer15m"`
		OTPWindowSec   int `yaml:"otpWindowSec"`
		LoginWindowSec int `yaml:"loginWindowSec"`
	} `yaml:"rate"`
	TrustProxy bool `yaml:"trustProxy"`
	Sessions   struct {
		AccessTTL  string `yaml:"accessTTL"`
		RefreshTTL string `yaml:"refreshTTL"`
	} `yaml:"sessions"`
	Logging struct{ Level string } `yaml:"logging"`
	Metrics struct {
		Enabled   bool     `yaml:"enabled"`
		Pprof     bool     `yaml:"pprof"`
		Allowlist []string `yaml:"allowlist"`
	} `yaml:"metrics"`
	Agents struct {
		AllowRegistration bool `yaml:"allowRegistration"`
	} `yaml:"agents"`
}

func Defaults() Config {
	return Config{
		Port:                     9000,
		LogLevel:                 zerolog.InfoLevel,
		UsersPath:                "/etc/nos/users.json",
		SecretPath:               "/etc/nos/secret.key",
		FirstBootPath:            "/var/lib/nos/state/firstboot.json",
		FirstBootIssuePath:       "/run/nos/firstboot.issue",
		SessionsPath:             "/var/lib/nos/sessions.json",
		RateLimitPath:            "/var/lib/nos/ratelimit.json",
		SharesPath:               "/etc/nos/shares.json",
		SessionHashKey:           nil,
		SessionBlockKey:          nil,
		EtcDir:                   "/etc",
		AppsDataDir:              "/var/lib/nos/apps",
		AppsInstallDir:           "/opt/nos/apps",
		TrustProxy:               false,
		RateOTPPerMin:            5,
		RateLoginPer15m:          5,
		RateOTPWindowSec:         60,
		RateLoginWindowSec:       900,
		Bind:                     "127.0.0.1:9000",
		CORSOrigin:               "http://localhost:5173",
		SessionAccessTTLSeconds:  int((15 * time.Minute).Seconds()),
		SessionRefreshTTLSeconds: int((7 * 24 * time.Hour).Seconds()),
		MetricsEnabled:           false,
		PprofEnabled:             false,
		MetricsAllowlist:         nil,
		AllowAgentRegistration:   true,
		RecoveryMode:             false,
	}
}

func Load(path string) Config {
	cfg := Defaults()
	if b, err := os.ReadFile(path); err == nil {
		var fy fileYAML
		if yaml.Unmarshal(b, &fy) == nil {
			if fy.HTTP.Bind != "" {
				cfg.Bind = fy.HTTP.Bind
			}
			if fy.CORS.Origin != "" {
				cfg.CORSOrigin = fy.CORS.Origin
			}
			if fy.TrustProxy {
				cfg.TrustProxy = true
			}
			if fy.Rate.OTPPerMin > 0 {
				cfg.RateOTPPerMin = fy.Rate.OTPPerMin
			}
			if fy.Rate.LoginPer15m > 0 {
				cfg.RateLoginPer15m = fy.Rate.LoginPer15m
			}
			if fy.Rate.OTPWindowSec > 0 {
				cfg.RateOTPWindowSec = fy.Rate.OTPWindowSec
			}
			if fy.Rate.LoginWindowSec > 0 {
				cfg.RateLoginWindowSec = fy.Rate.LoginWindowSec
			}
			if fy.Logging.Level != "" {
				if l, err := zerolog.ParseLevel(fy.Logging.Level); err == nil {
					cfg.LogLevel = l
				}
			}
			if d, err := time.ParseDuration(fy.Sessions.AccessTTL); err == nil && d > 0 {
				cfg.SessionAccessTTLSeconds = int(d.Seconds())
			}
			if d, err := time.ParseDuration(fy.Sessions.RefreshTTL); err == nil && d > 0 {
				cfg.SessionRefreshTTLSeconds = int(d.Seconds())
			}
			cfg.MetricsEnabled = fy.Metrics.Enabled
			cfg.PprofEnabled = fy.Metrics.Pprof
			if len(fy.Metrics.Allowlist) > 0 {
				cfg.MetricsAllowlist = append([]string{}, fy.Metrics.Allowlist...)
			}
			if fy.Agents.AllowRegistration {
				cfg.AllowAgentRegistration = true
			}
		}
	}
	return applyEnv(cfg)
}

func FromEnv() Config { return applyEnv(Defaults()) }

func applyEnv(cfg Config) Config {
	if v := os.Getenv("NOS_HTTP_BIND"); v != "" {
		cfg.Bind = v
	}
	if v := os.Getenv("NOS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Port = p
			if cfg.Bind == "" || cfg.Bind == "127.0.0.1:9000" {
				cfg.Bind = "127.0.0.1:" + strconv.Itoa(p)
			}
		}
	}
	if v := os.Getenv("NOS_LOG"); v != "" {
		if l, err := zerolog.ParseLevel(v); err == nil {
			cfg.LogLevel = l
		}
	}
	if v := os.Getenv("NOS_USERS_PATH"); v != "" {
		cfg.UsersPath = v
	}
	if v := os.Getenv("NOS_SESSIONS_PATH"); v != "" {
		cfg.SessionsPath = v
	}
	if v := os.Getenv("NOS_SECRET_PATH"); v != "" {
		cfg.SecretPath = v
	}
	if v := os.Getenv("NOS_FIRSTBOOT_PATH"); v != "" {
		cfg.FirstBootPath = v
	}
	if v, ok := os.LookupEnv("NOS_FIRSTBOOT_ISSUE_PATH"); ok {
		cfg.FirstBootIssuePath = v
	}
	if v := os.Getenv("NOS_RL_PATH"); v != "" {
		cfg.RateLimitPath = v
	}
	if v := os.Getenv("NOS_SHARES_PATH"); v != "" {
		cfg.SharesPath = v
	}
	if v := os.Getenv("NOS_SESSION_HASH_KEY"); v != "" {
		cfg.SessionHashKey = []byte(v)
	} else if len(cfg.SessionHashKey) == 0 {
		cfg.SessionHashKey = []byte("0123456789abcdef0123456789abcdef")
	}
	if v := os.Getenv("NOS_SESSION_BLOCK_KEY"); v != "" {
		cfg.SessionBlockKey = []byte(v)
	} else if len(cfg.SessionBlockKey) == 0 {
		cfg.SessionBlockKey = []byte("abcdef0123456789abcdef0123456789")
	}
	if v := os.Getenv("NOS_ETC_DIR"); v != "" {
		cfg.EtcDir = v
	}
	if v := os.Getenv("NOS_APPS_DATA_DIR"); v != "" {
		cfg.AppsDataDir = v
	}
	if v := os.Getenv("NOS_APPS_INSTALL_DIR"); v != "" {
		cfg.AppsInstallDir = v
	}
	if v := os.Getenv("NOS_TRUST_PROXY"); v != "" {
		switch v {
		case "1", "true", "yes":
			cfg.TrustProxy = true
		case "0", "false", "no":
			cfg.TrustProxy = false
		}
	}
	if v := os.Getenv("NOS_CORS_ORIGIN"); v != "" {
		cfg.CORSOrigin = v
	} else if v := os.Getenv("NOS_UI_ORIGIN"); v != "" {
		cfg.CORSOrigin = v
	}
	if v := os.Getenv("NOS_RATE_OTP_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RateOTPPerMin = n
		}
	}
	if v := os.Getenv("NOS_RATE_LOGIN_PER_15M"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RateLoginPer15m = n
		}
	}
	if v := os.Getenv("NOS_RATE_OTP_WINDOW_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RateOTPWindowSec = n
		}
	}
	if v := os.Getenv("NOS_RATE_LOGIN_WINDOW_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RateLoginWindowSec = n
		}
	}
	if v := os.Getenv("NOS_SESSION_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionAccessTTLSeconds = int(d.Seconds())
		}
	}
	if v := os.Getenv("NOS_SESSION_REFRESH_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionRefreshTTLSeconds = int(d.Seconds())
		}
	}
	if v := os.Getenv("NOS_METRICS"); v != "" {
		cfg.MetricsEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("NOS_PPROF"); v != "" {
		cfg.PprofEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("NOS_METRICS_ALLOWLIST"); v != "" {
		parts := []string{}
		cur := ""
		for i := 0; i < len(v); i++ {
			if v[i] == ',' {
				if cur != "" {
					parts = append(parts, cur)
				}
				cur = ""
			} else {
				cur += string(v[i])
			}
		}
		if cur != "" {
			parts = append(parts, cur)
		}
		cfg.MetricsAllowlist = parts
	}
	if v := os.Getenv("NOS_ALLOW_AGENT_REG"); v != "" {
		cfg.AllowAgentRegistration = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("NOS_RECOVERY"); v != "" {
		cfg.RecoveryMode = v == "1" || v == "true" || v == "yes"
	}
	return cfg
}
