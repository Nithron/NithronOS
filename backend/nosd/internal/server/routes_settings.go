package server

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	// "nos/backend/nosd/pkg/firewall"
)

// registerSettings serves network, updates, users, appearance and about.
func (d *routeDeps) registerSettings(pr chi.Router) {
	cfg := d.cfg
	users := d.users
	mgr := d.mgr
	adminRequired := d.adminRequired
	// (/api/v1/notifications, a second notification system with its own
	// channels, is gone: nothing sent to it or read it, and any signed-in
	// user could add a webhook and have nosd call any URL. Alerts and their
	// channels are /api/v1/monitor/alerts.)

	// Users: admins manage them; anyone signed in changes their own password.
	usersHandler := NewUsersHandler(users, mgr, cfg)
	usersHandler.onDeleted = d.forgetUser
	if d.agentClient != nil {
		usersHandler.system = agentAccounts{agent: d.agentClient}
	}
	authRoutes := twoFactor{cfg: cfg, users: users}.settingsRoutes()
	authRoutes.Post("/password", usersHandler.ChangeOwnPassword)
	pr.Mount("/api/v1/auth", authRoutes)

	// Network endpoints (M4)
	netLogger := Logger(cfg)
	netHandler, err := NewNetHandler(*netLogger, d.agentClient, filepath.Dir(cfg.SessionsPath))
	if err != nil {
		netLogger.Error().Err(err).Msg("Failed to create network handler")
		// Continue without networking features
	} else {
		pr.Mount("/api/v1/net", netHandler.Routes())
	}

	// Updates endpoints (M5)
	d.registerUpdates(pr)

	pr.With(adminRequired).Mount("/api/v1/users", usersHandler.Routes())

	// Appearance settings endpoints
	appearanceHandler := NewAppearanceHandler(cfg)
	pr.Mount("/api/v1/settings/appearance", appearanceHandler.Routes(adminRequired))

	// About/System info endpoints
	aboutHandler := NewAboutHandler(cfg)
	pr.Mount("/api/v1/about", aboutHandler.Routes())

	// (A second, older apps API used to be registered here: GET
	// /api/v1/apps, /apps/{id}/status, /apps/uninstall and POST
	// /apps/install. The last replaced the app manager's install, so
	// catalog installs ran a hard-coded template list and ignored every
	// error.)

	pr.Get("/api/v1/remote/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"mode": "lan-only", "https": true})
	})

}
