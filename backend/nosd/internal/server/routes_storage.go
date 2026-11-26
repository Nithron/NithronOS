package server

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/disks"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"

	// "nos/backend/nosd/pkg/firewall"
	"nos/backend/nosd/pkg/httpx"
	poolroots "nos/backend/nosd/pkg/pools"

	"nos/backend/nosd/internal/fsatomic"

	"strconv"
)

// registerStorage serves disks, pools, scrub and balance, SMART, jobs and device operations.
func (d *routeDeps) registerStorage(pr chi.Router) {
	cfg := d.cfg
	adminRequired := d.adminRequired
	pr.Get("/api/v1/disks", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if runtime.GOOS != "windows" && hasCommand("lsblk") {
			if list, err := disks.Collect(ctx); err == nil {
				// Enrich with SMART when possible
				for i := range list {
					if list[i].Path != "" {
						list[i].Smart = disks.SmartSummaryFor(ctx, list[i].Path)
					}
				}
				writeJSON(w, map[string]any{"disks": list})
				return
			}
		}
		// No mock fallback: say the disk list is unavailable.
		httpx.WriteTypedError(w, http.StatusServiceUnavailable, "disks.unavailable", "Cannot list disks (lsblk unavailable or failed)", 0)
	})

	pr.Get("/api/v1/pools", func(w http.ResponseWriter, r *http.Request) {
		list, _ := pools.ListPools(r.Context())
		writeJSON(w, list)
	})

	// Pools: allowed roots for shares (mounted pool paths)
	pr.Get("/api/v1/pools/roots", func(w http.ResponseWriter, r *http.Request) {
		roots, err := poolroots.AllowedRoots()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"roots": roots})
	})

	pr.With(adminRequired).Post("/api/v1/pools/plan-create", handlePlanCreateV1)

	// Health: alerts and manual SMART scan
	pr.Get("/api/v1/alerts", handleAlertsGet(cfg))

	// Services health endpoints
	pr.Get("/api/v1/health/services", handleServicesHealth(cfg))
	pr.Get("/api/v1/health/services/{service}", handleServiceHealth(cfg))
	pr.Get("/api/v1/health/services/{service}/logs", handleServiceLogs(cfg))
	pr.With(adminRequired).Post("/api/v1/health/services/{service}/restart", handleServiceRestart(cfg))

	// Monitoring endpoints
	pr.Get("/api/v1/monitoring/logs", handleMonitoringLogs(cfg))
	pr.Get("/api/v1/monitoring/events", handleMonitoringEvents(cfg))
	pr.Get("/api/v1/monitoring/alerts", handleMonitoringAlerts(cfg))
	pr.Get("/api/v1/monitoring/services", handleMonitoringServices(cfg))
	pr.Get("/api/v1/monitoring/system", handleMonitoringSystem(cfg))

	// Scrub endpoints expected by frontend
	pr.Get("/api/v1/scrub/status", func(w http.ResponseWriter, r *http.Request) {
		// Delegate to pools scrub status
		handleScrubStatus(w, r)
	})
	pr.With(adminRequired).Post("/api/v1/scrub/start", func(w http.ResponseWriter, r *http.Request) {
		// Delegate to pools scrub start
		handleScrubStart(w, r)
	})
	pr.With(adminRequired).Post("/api/v1/scrub/cancel", func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement scrub cancel
		writeJSON(w, map[string]any{"ok": true, "message": "Scrub cancelled"})
	})

	// Balance endpoints
	pr.Get("/api/v1/balance/status", handleBalanceStatus(cfg))
	pr.With(adminRequired).Post("/api/v1/balance/start", handleBalanceStart(cfg))
	pr.With(adminRequired).Post("/api/v1/balance/cancel", handleBalanceCancel(cfg))

	// SMART endpoints
	pr.Get("/api/v1/smart/summary", handleSmartSummary(cfg))
	pr.Get("/api/v1/smart/devices", handleSmartDevices(cfg))
	pr.Get("/api/v1/smart/device/{device}", handleSmartDevice(cfg))
	pr.Get("/api/v1/smart/test/{device}", handleSmartTestDevice(cfg))
	pr.With(adminRequired).Post("/api/v1/smart/scan", handleSmartScan(cfg))
	pr.With(adminRequired).Post("/api/v1/smart/test/{device}", handleSmartTestDevice(cfg))

	// Jobs endpoints
	pr.Get("/api/v1/jobs/recent", handleJobsRecent(cfg))
	pr.Get("/api/v1/jobs/{id}", handleJobGet(cfg))

	// Devices endpoint expected by frontend
	pr.Get("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		// Delegate to existing devices handler
		handleListDevices(w, r)
	})
	pr.With(adminRequired).Post("/api/v1/health/scan", handleHealthScan(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/apply-create", handleApplyCreate(cfg))
	pr.With(adminRequired).Get("/api/v1/pools/discover", handlePoolsDiscover)
	pr.With(adminRequired).Post("/api/v1/pools/import", handlePoolsImport(cfg))
	// Device operations (plan/apply)
	pr.With(adminRequired).Post("/api/v1/pools/{id}/plan-device", handlePlanDevice(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/{id}/apply-device", handleApplyDevice(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/{id}/plan-destroy", handlePlanDestroy(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/{id}/apply-destroy", handleApplyDestroy(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/scrub/start", handleScrubStart)
	pr.With(adminRequired).Get("/api/v1/pools/scrub/status", handleScrubStatus)
	pr.Get("/api/v1/pools/{id}", handlePoolDetail)
	// Mount options (canonical + compatibility with FE path)
	pr.Get("/api/v1/pools/{id}/options", handlePoolOptionsGet(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/{id}/options", handlePoolOptionsPost(cfg))
	// FE expects mount-options nomenclature
	pr.Get("/api/v1/pools/{id}/mount-options", handlePoolOptionsGet(cfg))
	pr.With(adminRequired).Post("/api/v1/pools/{id}/mount-options", handlePoolOptionsPost(cfg))

	pr.Get("/api/v1/schedules", handleSchedulesGet(cfg))
	pr.With(adminRequired).Post("/api/v1/schedules", handleSchedulesPost(cfg))
	pr.Get("/api/v1/pools/tx/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var tx pools.Tx
		if ok, _ := fsatomic.LoadJSON(txPath(id), &tx); !ok {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, tx)
	})
	pr.Get("/api/v1/pools/tx/{id}/log", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		cursorStr := r.URL.Query().Get("cursor")
		maxStr := r.URL.Query().Get("max")
		cursor, max := 0, 1000
		if i, err := strconv.Atoi(cursorStr); err == nil && i >= 0 {
			cursor = i
		}
		if i, err := strconv.Atoi(maxStr); err == nil && i > 0 && i <= 5000 {
			max = i
		}
		lines, next := readLogTail(id, cursor, max)
		writeJSON(w, map[string]any{"lines": lines, "nextCursor": next})
	})
	pr.Get("/api/v1/pools/tx/{id}/stream", handleTxStream)

	pr.With(adminRequired).Post("/api/v1/pools/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Confirm") != "yes" {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm header required")
			return
		}
		var req pools.PlanRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if err := pools.EnsureDevicesFree(r.Context(), req.Devices); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		client := agentclient.New("/run/nos-agent.sock")
		var resp map[string]any
		err := client.PostJSON(r.Context(), "/v1/btrfs/create", map[string]any{
			"devices": req.Devices,
			"raid":    req.Raid,
			"label":   req.Label,
			"dry_run": false,
		}, &resp)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, resp)
	})

	// Pools: candidates for import
	pr.With(adminRequired).Get("/api/v1/pools/candidates", func(w http.ResponseWriter, r *http.Request) {
		list, err := pools.ListPools(r.Context())
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, list)
	})

	// Pools: import handled by handlePoolsImport(cfg)

}
