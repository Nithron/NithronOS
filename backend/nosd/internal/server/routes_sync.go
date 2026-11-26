package server

import (
	"github.com/go-chi/chi/v5"

	"errors"
	// "nos/backend/nosd/pkg/firewall"
)

// registerSync serves NithronSync (on pr, signed-in users) and WebDAV (on root; it checks sync device tokens itself).
func (d *routeDeps) registerSync(pr, root chi.Router) {
	cfg := d.cfg
	sharesStore := d.sharesStore
	// Sync endpoints (for NithronSync clients)
	var syncHandler *SyncHandler
	syncErr := errors.New("the shares store could not be loaded")
	if sharesStore != nil && d.groupsStore != nil {
		syncHandler, syncErr = NewSyncHandler(cfg, sharesStore, d.accounts, *Logger(cfg))
	}
	if syncErr != nil {
		Logger(cfg).Error().Err(syncErr).Msg("Failed to create sync handler")
	} else {
		// Mount sync API routes
		pr.Mount("/api/v1/sync", syncHandler.Routes())

		// Mount WebDAV endpoint for file access
		webdavHandler := NewWebDAVHandler(sharesStore, syncHandler.DeviceManager(), d.accounts, *Logger(cfg))
		root.Mount("/dav", webdavHandler)

		Logger(cfg).Info().Msg("NithronSync API initialized")
	}

	// Phase 4: Real-time collaboration
	realtimeHandler := NewRealtimeHandler(*Logger(cfg))
	pr.Mount("/api/v1/sync/realtime", realtimeHandler.Routes())
	Logger(cfg).Info().Msg("Real-time collaboration API initialized")

	// Phase 4: End-to-end encryption
	encryptionHandler, encErr := NewEncryptionHandler(cfg, *Logger(cfg))
	if encErr != nil {
		Logger(cfg).Error().Err(encErr).Msg("Failed to create encryption handler")
	} else {
		pr.Mount("/api/v1/sync/encryption", encryptionHandler.Routes())
		Logger(cfg).Info().Msg("Encryption API initialized")
	}

	// Phase 4: Offline-first sync
	offlineHandler, offErr := NewOfflineHandler(cfg, *Logger(cfg))
	if offErr != nil {
		Logger(cfg).Error().Err(offErr).Msg("Failed to create offline handler")
	} else {
		pr.Mount("/api/v1/sync/offline", offlineHandler.Routes())
		Logger(cfg).Info().Msg("Offline sync API initialized")
	}

	// Phase 4: Smart sync (on-demand files)
	smartsyncHandler, smartErr := NewSmartSyncHandler(cfg, *Logger(cfg))
	if smartErr != nil {
		Logger(cfg).Error().Err(smartErr).Msg("Failed to create smart sync handler")
	} else {
		pr.Mount("/api/v1/sync/smartsync", smartsyncHandler.Routes())
		Logger(cfg).Info().Msg("Smart sync API initialized")
	}

}
