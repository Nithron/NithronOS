package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"nos/backend/nosd/internal/apps"
	pkgapps "nos/backend/nosd/pkg/apps"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// handleGetCatalog returns the merged app catalog
func handleGetCatalog(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalog, err := appManager.GetCatalog()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "Failed to get catalog")
			return
		}

		writeJSON(w, catalog)
	}
}

// handleGetInstalledApps returns all installed apps
func handleGetInstalledApps(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apps := appManager.GetInstalledApps()

		response := map[string]interface{}{
			"items": apps,
		}

		writeJSON(w, response)
	}
}

// handleGetApp returns a specific installed app
func handleGetApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")

		app, err := appManager.GetApp(appID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to get app")
			}
			return
		}

		writeJSON(w, app)
	}
}

// handleInstallApp installs a new app
func handleInstallApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pkgapps.InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate request
		if req.ID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "App ID is required")
			return
		}

		// Get user ID from context
		userID := getUserIDFromContext(r)

		// Install app
		if err := appManager.InstallApp(r.Context(), req, userID); err != nil {
			if strings.Contains(err.Error(), "already installed") {
				httpx.WriteError(w, http.StatusConflict, "App already installed")
			} else if strings.Contains(err.Error(), "not found in catalog") {
				httpx.WriteError(w, http.StatusNotFound, "App not found in catalog")
			} else if strings.Contains(err.Error(), "validation failed") {
				httpx.WriteError(w, http.StatusBadRequest, err.Error())
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to install app")
			}
			return
		}

		// Get installed app details
		app, _ := appManager.GetApp(req.ID)

		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{
			"message": "App installed successfully",
			"app":     app,
		})
	}
}

// handleUpgradeApp upgrades an existing app
func handleUpgradeApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")

		var req pkgapps.UpgradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Get user ID from context
		userID := getUserIDFromContext(r)

		// Upgrade app
		if err := appManager.UpgradeApp(r.Context(), appID, req, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else if strings.Contains(err.Error(), "validation failed") {
				httpx.WriteError(w, http.StatusBadRequest, err.Error())
			} else if strings.Contains(err.Error(), "rolled back") {
				httpx.WriteError(w, http.StatusInternalServerError, "Upgrade failed and was rolled back")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to upgrade app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "App upgraded successfully",
			"version": req.Version,
		})
	}
}

// handleStartApp starts an app
func handleStartApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		userID := getUserIDFromContext(r)

		if err := appManager.StartApp(r.Context(), appID, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to start app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "App started successfully",
		})
	}
}

// handleStopApp stops an app
func handleStopApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		userID := getUserIDFromContext(r)

		if err := appManager.StopApp(r.Context(), appID, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to stop app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "App stopped successfully",
		})
	}
}

// handleRestartApp restarts an app
func handleRestartApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		userID := getUserIDFromContext(r)

		if err := appManager.RestartApp(r.Context(), appID, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to restart app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "App restarted successfully",
		})
	}
}

// handleDeleteApp deletes an app
func handleDeleteApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		userID := getUserIDFromContext(r)

		// Parse query parameters
		keepData := r.URL.Query().Get("keep_data") == "true"

		// Or parse from body if POST
		if r.Method == "DELETE" && r.ContentLength > 0 {
			var req pkgapps.DeleteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				keepData = req.KeepData
			}
		}

		if err := appManager.DeleteApp(r.Context(), appID, keepData, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to delete app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": fmt.Sprintf("App deleted successfully (data kept: %v)", keepData),
		})
	}
}

// handleSnapshotApp takes a snapshot of an app's data on request.
func handleSnapshotApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		uid, _ := signedInUID(r)
		snap, err := appManager.SnapshotApp(r.Context(), appID, uid)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteTypedError(w, http.StatusNotFound, "apps.not_found", "App not found", 0)
				return
			}
			httpx.WriteTypedError(w, http.StatusBadGateway, "apps.snapshot_failed", err.Error(), 0)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, snap)
	}
}

// handleRollbackApp rolls back an app to a snapshot
func handleRollbackApp(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")
		userID := getUserIDFromContext(r)

		var req pkgapps.RollbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.SnapshotTimestamp == "" {
			httpx.WriteError(w, http.StatusBadRequest, "Snapshot timestamp is required")
			return
		}

		if err := appManager.RollbackApp(r.Context(), appID, req.SnapshotTimestamp, userID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App or snapshot not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to rollback app")
			}
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "App rolled back successfully",
		})
	}
}

// handleGetAppLogs streams app logs
func handleGetAppLogs(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")

		// Parse options
		options := pkgapps.LogStreamOptions{
			Follow:     r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1",
			Tail:       100,
			Timestamps: r.URL.Query().Get("timestamps") == "true",
			Container:  r.URL.Query().Get("container"),
		}

		// Get tail parameter
		if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
			var tail int
			if _, err := fmt.Sscanf(tailStr, "%d", &tail); err == nil {
				options.Tail = tail
			}
		}

		// Following: server-sent events, one log line per event.
		if options.Follow {
			flusher, ok := w.(http.Flusher)
			if !ok {
				httpx.WriteError(w, http.StatusInternalServerError, "Streaming is not supported")
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			err := appManager.StreamAppLogs(r.Context(), appID, options, func(line string) error {
				if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
					return err
				}
				flusher.Flush()
				return nil
			})
			if err != nil && r.Context().Err() == nil {
				_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
			}
			return
		}

		logs, err := appManager.GetAppLogs(r.Context(), appID, options)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else if strings.Contains(err.Error(), "invalid container") {
				httpx.WriteError(w, http.StatusBadRequest, err.Error())
			} else {
				httpx.WriteTypedError(w, http.StatusBadGateway, "apps.logs_failed", err.Error(), 0)
			}
			return
		}

		// Return logs as plain text
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write(logs); err != nil {
			fmt.Printf("Failed to write logs: %v\n", err)
		}
	}
}

// handleGetAppSchema returns a catalog app's install form schema.
func handleGetAppSchema(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema, err := appManager.GetSchema(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteTypedError(w, http.StatusNotFound, "apps.schema_not_found", "No schema for this app", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(schema)
	}
}

// handleGetAppEvents returns app events
func handleGetAppEvents(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")

		// Get limit parameter
		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			var l int
			if _, err := fmt.Sscanf(limitStr, "%d", &l); err == nil && l > 0 && l <= 1000 {
				limit = l
			}
		}

		events := appManager.GetEvents(appID, limit)

		writeJSON(w, map[string]interface{}{
			"events": events,
		})
	}
}

// handleForceHealthCheck forces a health check for an app
func handleForceHealthCheck(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appID := chi.URLParam(r, "id")

		if err := appManager.ForceHealthCheck(r.Context(), appID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "Failed to check health")
			}
			return
		}

		// Get updated app with health
		app, _ := appManager.GetApp(appID)

		writeJSON(w, map[string]interface{}{
			"message": "Health check completed",
			"health":  app.Health,
		})
	}
}

// handleSyncCatalogs manually triggers catalog sync (admin only)
func handleSyncCatalogs(appManager *apps.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := appManager.SyncCatalogs(); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "Failed to sync catalogs")
			return
		}

		writeJSON(w, map[string]interface{}{
			"message": "Catalogs synced successfully",
		})
	}
}

// Helper to get user ID from request context
func getUserIDFromContext(r *http.Request) string {
	// Get from X-UID header set by session middleware
	if uid := r.Header.Get("X-UID"); uid != "" {
		return uid
	}
	return "system"
}
