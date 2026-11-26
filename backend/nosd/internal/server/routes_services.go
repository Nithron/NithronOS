package server

import (
	"github.com/go-chi/chi/v5"

	"nithronos/backend/nosd/internal/api"
	"nithronos/backend/nosd/pkg/agentclient"
	// "nithronos/backend/nosd/pkg/firewall"
)

// registerServices serves dashboard status, health, storage, Btrfs, schedules, shares and backup.
func (d *routeDeps) registerServices(pr chi.Router) {
	cfg := d.cfg
	sharesHandler := d.sharesHandler
	backupHandler := d.backupHandler
	// System status for the dashboard and health pages
	pr.Get("/api/v1/health/system", handleSystemHealth(cfg))
	pr.Get("/api/v1/health/disks", handleDiskHealth(cfg))
	pr.Get("/api/v1/health/disks/summary", api.HandleDisksSummary)
	pr.Get("/api/v1/health/smart", handleSmartProxy)
	pr.Get("/api/v1/dashboard", api.HandleDashboard)
	pr.Get("/api/v1/storage/summary", api.HandleStorageSummary)
	pr.Get("/api/v1/storage/devices", handleListDevices)
	pr.Get("/api/v1/events/recent", api.HandleRecentEvents)
	pr.Get("/api/v1/maintenance/status", api.HandleMaintenanceStatus)

	// Health endpoints
	healthHandler := NewHealthHandler(agentclient.New(cfg.AgentSocket()))
	pr.Mount("/api/v1/health", healthHandler.Routes())

	// Storage endpoints
	storageHandler := NewStorageHandler(agentclient.New(cfg.AgentSocket()))
	pr.Mount("/api/v1/storage", storageHandler.Routes())

	// Btrfs endpoints
	btrfsHandler := NewBtrfsHandler(agentclient.New(cfg.AgentSocket()))
	pr.Mount("/api/v1/btrfs", btrfsHandler.Routes())

	// Schedule endpoints
	schedulesHandler := NewSchedulesHandler()
	pr.Mount("/api/v1/schedules", schedulesHandler.Routes())

	// Share endpoints (v1 API) - use real implementation
	if sharesHandler != nil {
		pr.Mount("/api/v1/shares", sharesHandler.Routes())
	} else {
		// No mock fallback: report the failure instead of showing fake shares.
		pr.Mount("/api/v1/shares", unavailable("shares.unavailable", "Shares are unavailable: the shares store could not be opened (see the nosd log)"))
	}

	// Jobs endpoints are already defined above

	// Backup endpoints
	if backupHandler != nil {
		pr.Mount("/api/v1/backup", backupHandler.Routes())
	}

}
