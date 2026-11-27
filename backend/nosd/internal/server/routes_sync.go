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
		// The sync API authenticates itself (device token or session), so
		// it is mounted outside the session-only group.
		root.Mount("/api/v1/sync", syncHandler.Routes())

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

	// (Offline mode and smart sync's placeholder files are client work: the
	// sync client keeps its own queue. The server-side versions, at
	// /api/v1/sync/offline and /sync/smartsync, kept bookkeeping nothing
	// used, and "processing" the offline queue marked uploads done without
	// uploading anything; they are gone.)

}
