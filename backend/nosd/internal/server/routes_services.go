package server

import (
	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/api"
	"nos/backend/nosd/pkg/agentclient"
	// "nos/backend/nosd/pkg/firewall"
)

// registerServices serves dashboard status, health, storage, schedules, shares and backup.
func (d *routeDeps) registerServices(pr chi.Router) {
	cfg := d.cfg
	sharesHandler := d.sharesHandler
	backupHandler := d.backupHandler
	// System status for the dashboard and health pages
	pr.Get("/api/v1/health/system", handleSystemHealth(cfg))
	pr.Get("/api/v1/metrics/stream", handleMetricsStream)
	pr.Get("/api/v1/health/disks", handleDiskHealth(cfg))
	pr.Get("/api/v1/health/disks/summary", api.DisksSummaryHandler(d.dashboardSources()))
	pr.Get("/api/v1/health/smart", handleSmartProxy)
	pr.Get("/api/v1/dashboard", api.Dashboard(d.dashboardSources()))
	pr.Get("/api/v1/storage/summary", api.HandleStorageSummary)
	pr.Get("/api/v1/storage/devices", handleListDevices)
	pr.Get("/api/v1/events/recent", api.HandleRecentEvents)
	pr.Get("/api/v1/maintenance/status", api.HandleMaintenanceStatus)

	// Health endpoints
	healthHandler := NewHealthHandler(agentclient.New(cfg.AgentSocket()))
	healthHandler.Register(pr)

	// Storage endpoints
	storageHandler := NewStorageHandler(agentclient.New(cfg.AgentSocket()))
	pr.Mount("/api/v1/storage", storageHandler.Routes())

	// Schedule endpoints
	schedulesHandler := NewSchedulesHandler()
	pr.Mount("/api/v1/schedules", schedulesHandler.Routes())

	// Shares, groups and the policy. No mock fallback: when a store can't be
	// read, the API says so instead of showing fake data.
	if sharesHandler != nil {
		pr.Mount("/api/v1/shares", sharesHandler.Routes(d.adminRequired))
		pr.Mount("/api/v1/groups", d.groupsHandler.Routes(d.adminRequired))
	} else {
		pr.Mount("/api/v1/shares", unavailable("shares.unavailable", "Shares are unavailable: the shares, groups or policy store could not be opened (see the nosd log)"))
		pr.Mount("/api/v1/groups", unavailable("groups.unavailable", "Groups are unavailable: the shares, groups or policy store could not be opened (see the nosd log)"))
	}
	if d.policyHandler != nil {
		pr.Mount("/api/v1/policy", d.policyHandler.Routes(d.adminRequired))
	} else {
		pr.Mount("/api/v1/policy", unavailable("policy.unavailable", "The policy could not be read (see the nosd log)"))
	}

	// Jobs endpoints are already defined above

	// Backup endpoints
	pr.With(d.adminRequired).Mount("/api/v1/backup", backupHandler.Routes())

}
