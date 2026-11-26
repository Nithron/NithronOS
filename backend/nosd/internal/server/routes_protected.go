package server

import (
	"os"

	"github.com/go-chi/chi/v5"
	// "nos/backend/nosd/pkg/firewall"
)

// registerProtected serves everything that needs a signed-in user.
func (d *routeDeps) registerProtected(r chi.Router) {
	cfg := d.cfg
	users := d.users
	// Protected routes
	r.Group(func(pr chi.Router) {
		// Require auth via new opaque cookies or legacy session cookie (skip in tests when NOS_TEST_SKIP_AUTH=1)
		if os.Getenv("NOS_TEST_SKIP_AUTH") != "1" {
			pr.Use(requireAuth)
		}
		if os.Getenv("NOS_TEST_SKIP_AUTH") != "1" {
			pr.Use(requireCSRF)
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
