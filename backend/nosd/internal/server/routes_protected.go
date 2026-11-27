package server

import (
	"github.com/go-chi/chi/v5"
)

// registerProtected serves everything that needs a signed-in user.
func (d *routeDeps) registerProtected(r chi.Router) {
	cfg := d.cfg
	users := d.users
	// Protected routes
	r.Group(func(pr chi.Router) {
		if !skipAuthChecks.Load() {
			pr.Use(requireAuth, requireCSRF)
		}

		// TOTP enroll (logged-in): generate secret, encrypt with secret.key, store pending enc
		// Two-factor enrollment (setup wizard) and settings: one implementation
		// on the user store, see auth_2fa.go.
		tf := twoFactor{cfg: cfg, users: users}
		tf.registerTOTPRoutes(pr)

		d.registerStorage(pr)

		d.registerSMBUsers(pr)

		d.registerApps(pr)

		d.registerServices(pr)

		d.registerSync(pr, r)

		d.registerSettings(pr)

		d.registerMaintenance(pr)

	})

}
