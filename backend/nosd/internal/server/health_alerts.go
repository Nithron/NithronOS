package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/disks"
	"nos/backend/nosd/internal/fsatomic"
)

type healthConfig struct {
	SMART struct {
		TempWarn        int `json:"tempWarn"`
		TempCrit        int `json:"tempCrit"`
		ReallocatedWarn int `json:"reallocatedWarn"`
		MediaErrWarn    int `json:"mediaErrWarn"`
	} `json:"smart"`
}

type alert struct {
	ID        string   `json:"id"`
	Severity  string   `json:"severity"` // warn|crit
	Kind      string   `json:"kind"`     // smart
	Device    string   `json:"device"`
	Messages  []string `json:"messages"`
	CreatedAt string   `json:"createdAt"`
}

func alertsPath() string {
	base := os.Getenv("NOS_STATE_DIR")
	if base == "" {
		base = "/var/lib/nos"
	}
	return filepath.Join(base, "alerts.json")
}

func loadHealthConfig(cfg config.Config) healthConfig {
	// default thresholds
	hc := healthConfig{}
	hc.SMART.TempWarn = 60
	hc.SMART.TempCrit = 70
	hc.SMART.ReallocatedWarn = 1
	hc.SMART.MediaErrWarn = 1
	// best-effort load from /etc/nos/health.yaml or health.json
	pathY := filepath.Join(cfg.EtcDir, "nos", "health.yaml")
	pathJ := filepath.Join(cfg.EtcDir, "nos", "health.json")
	if b, err := os.ReadFile(pathJ); err == nil {
		_ = json.Unmarshal(b, &hc)
		return hc
	}
	if b, err := os.ReadFile(pathY); err == nil {
		// very small YAML subset: try to parse numbers by string search
		s := string(b)
		// not robust; acceptable for minimal dev flow
		_ = s // keep defaults for now
	}
	return hc
}

func handleAlertsGet(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list []alert
		if b, err := os.ReadFile(alertsPath()); err == nil {
			_ = json.Unmarshal(b, &list)
		}
		writeJSON(w, map[string]any{"alerts": list})
	}
}

func handleHealthScan(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hc := loadHealthConfig(cfg)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		devs, _ := disks.Collect(ctx)
		out := []alert{}
		for _, d := range devs {
			if d.Path == "" {
				continue
			}
			s := disks.SmartSummaryFor(ctx, d.Path)
			sev := ""
			msgs := []string{}
			if s != nil {
				if s.TempCelsius != nil {
					if *s.TempCelsius >= hc.SMART.TempCrit {
						sev = "crit"
						msgs = append(msgs, "temperature critical")
					} else if *s.TempCelsius >= hc.SMART.TempWarn {
						if sev == "" {
							sev = "warn"
						}
						msgs = append(msgs, "temperature high")
					}
				}
				if s.Reallocated != nil && *s.Reallocated >= hc.SMART.ReallocatedWarn {
					if sev == "" {
						sev = "warn"
					}
					msgs = append(msgs, "reallocated sectors")
				}
				if s.MediaErrors != nil && *s.MediaErrors >= hc.SMART.MediaErrWarn {
					if sev == "" {
						sev = "warn"
					}
					msgs = append(msgs, "media errors")
				}
				if s.Healthy != nil && !*s.Healthy {
					sev = "crit"
					msgs = append(msgs, "SMART failed")
				}
			}
			if sev != "" {
				out = append(out, alert{
					ID: generateUUID(), Severity: sev, Kind: "smart", Device: d.Path, Messages: msgs, CreatedAt: time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
		_ = os.MkdirAll(filepath.Dir(alertsPath()), 0o755)
		_ = fsatomic.SaveJSON(alertsPath(), out, 0o600)
		writeJSON(w, map[string]any{"ok": true, "alerts": out})
	}
}
