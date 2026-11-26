package server

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	// "nos/backend/nosd/pkg/firewall"
)

// registerApps serves the app catalog and installed apps.
func (d *routeDeps) registerApps(pr chi.Router) {
	appsManager := d.appsManager
	adminRequired := d.adminRequired
	// App management routes
	if appsManager != nil {
		// Start app manager
		go func() {
			if err := appsManager.Start(context.Background()); err != nil {
				log.Error().Err(err).Msg("Failed to start the app manager")
			}
		}()

		// Catalog and installed apps
		pr.Get("/api/v1/apps/catalog", handleGetCatalog(appsManager))
		pr.Get("/api/v1/apps/catalog/{id}/schema", handleGetAppSchema(appsManager))
		pr.Get("/api/v1/apps/installed", handleGetInstalledApps(appsManager))

		// Individual app operations
		pr.Get("/api/v1/apps/{id}", handleGetApp(appsManager))
		pr.Get("/api/v1/apps/{id}/logs", handleGetAppLogs(appsManager))
		pr.Get("/api/v1/apps/{id}/events", handleGetAppEvents(appsManager))

		// App lifecycle operations (admin only)
		pr.With(adminRequired).Post("/api/v1/apps/install", handleInstallApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/upgrade", handleUpgradeApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/start", handleStartApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/stop", handleStopApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/restart", handleRestartApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/rollback", handleRollbackApp(appsManager))
		pr.With(adminRequired).Delete("/api/v1/apps/{id}", handleDeleteApp(appsManager))
		pr.With(adminRequired).Post("/api/v1/apps/{id}/health", handleForceHealthCheck(appsManager))

		// Admin operations
		pr.With(adminRequired).Post("/api/v1/apps/catalog/sync", handleSyncCatalogs(appsManager))
	} else {
		// No stubs: say the app runtime is unavailable instead of showing an
		// empty catalog.
		pr.Mount("/api/v1/apps", unavailable("apps.unavailable", "Apps are unavailable: the app manager could not start (see the nosd log)"))
	}

}
