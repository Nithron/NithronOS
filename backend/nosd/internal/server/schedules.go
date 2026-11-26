package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/fsatomic"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

type Schedules struct {
	SmartScan  string `yaml:"smartScan" json:"smartScan"`
	BtrfsScrub string `yaml:"btrfsScrub" json:"btrfsScrub"`
}

func schedulesPath(cfg config.Config) string {
	return filepath.Join(cfg.EtcDir, "nos", "schedules.yaml")
}

func defaultSchedules() Schedules {
	return Schedules{SmartScan: "Sun 03:00", BtrfsScrub: "Sun *-**-01..07 03:00"}
}

func loadSchedules(cfg config.Config) Schedules {
	p := schedulesPath(cfg)
	b, err := os.ReadFile(p)
	if err != nil {
		return defaultSchedules()
	}
	var s Schedules
	if yaml.Unmarshal(b, &s) != nil {
		return defaultSchedules()
	}
	if strings.TrimSpace(s.SmartScan) == "" {
		s.SmartScan = defaultSchedules().SmartScan
	}
	if strings.TrimSpace(s.BtrfsScrub) == "" {
		s.BtrfsScrub = defaultSchedules().BtrfsScrub
	}
	return s
}

func saveSchedules(cfg config.Config, s Schedules) error {
	p := schedulesPath(cfg)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	// use fsatomic flow
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return fsatomic.FsyncDir(filepath.Dir(p))
}

// GET /api/v1/schedules
func handleSchedulesGet(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := loadSchedules(cfg)
		if b, err := os.ReadFile(filepath.Join("/var/lib/nos", "last_fstrim")); err == nil {
			// embed as additional field via map to avoid schema churn, but we can marshal struct + extra
			m := map[string]any{"smartScan": out.SmartScan, "btrfsScrub": out.BtrfsScrub, "lastFstrim": strings.TrimSpace(string(b))}
			writeJSON(w, m)
			return
		}
		writeJSON(w, out)
	}
}

// POST /api/v1/schedules
func handleSchedulesPost(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s Schedules
		_ = json.NewDecoder(r.Body).Decode(&s)
		if !validOnCalendar(s.SmartScan) || !validOnCalendar(s.BtrfsScrub) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid schedule format")
			return
		}
		if err := saveSchedules(cfg, s); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Write the timer drop-ins via the agent. A failure used to be ignored,
		// so the UI said "saved" while the timers kept their old schedule.
		client := agentclient.New(cfg.AgentSocket())
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		dropIns := []struct{ path, calendar string }{
			{"/etc/systemd/system/nos-smart-scan.timer.d/override.conf", s.SmartScan},
			{"/etc/systemd/system/nos-btrfs-scrub@.timer.d/override.conf", s.BtrfsScrub},
		}
		for _, d := range dropIns {
			err := client.PostJSON(ctx, "/v1/fs/write", map[string]any{
				"path":    d.path,
				"content": "[Timer]\nOnCalendar=" + d.calendar + "\n",
				"mode":    "0644", "owner": "root", "group": "root", "atomic": true,
			}, nil)
			if err != nil {
				httpx.WriteTypedError(w, http.StatusBadGateway, "schedules.apply_failed", "Schedules were saved but could not be applied: "+err.Error(), 0)
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func validOnCalendar(v string) bool {
	// minimal: non-empty and no dangerous characters
	t := strings.TrimSpace(v)
	if t == "" {
		return false
	}
	if strings.ContainsAny(t, "\x00\n\r") {
		return false
	}
	return true
}
