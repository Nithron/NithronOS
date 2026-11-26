package server

import (
	"os"
	"sync"

	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/setup/firstboot"
)

var (
	firstBootMu       sync.Mutex
	firstBootManagers = map[string]*firstboot.Manager{}
)

// FirstBootManager returns the setup-code manager for cfg. There is one per
// state file, shared by the HTTP handlers and the rotation loop that main
// runs, so they never hand out different codes.
func FirstBootManager(cfg config.Config) *firstboot.Manager {
	key := cfg.FirstBootPath + "\x00" + cfg.UsersPath + "\x00" + cfg.FirstBootIssuePath
	firstBootMu.Lock()
	defer firstBootMu.Unlock()
	if m, ok := firstBootManagers[key]; ok {
		return m
	}
	opts := firstboot.Options{
		HasAdmin: func() bool {
			us, err := userstore.New(cfg.UsersPath)
			return err == nil && us.HasAdmin()
		},
	}
	if cfg.FirstBootIssuePath != "" {
		opts.Announcer = &firstboot.IssueAnnouncer{Path: cfg.FirstBootIssuePath, Log: os.Stdout}
	}
	m := firstboot.NewManager(cfg.FirstBootPath, opts)
	firstBootManagers[key] = m
	return m
}
