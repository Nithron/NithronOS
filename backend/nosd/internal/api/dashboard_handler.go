package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// DashboardResponse aggregates all dashboard data
type DashboardResponse struct {
	System      SystemSummary     `json:"system"`
	Storage     StorageSummary    `json:"storage"`
	Disks       DisksSummary      `json:"disks"`
	Shares      []ShareInfo       `json:"shares"`
	Apps        []AppInfo         `json:"apps"`
	Maintenance MaintenanceStatus `json:"maintenance"`
	Events      []EventInfo       `json:"events"`
}

// SystemSummary for dashboard widget
type SystemSummary struct {
	Status    string  `json:"status"` // ok, degraded, critical
	CPUPct    float64 `json:"cpuPct"`
	Load1     float64 `json:"load1"`
	Memory    MemInfo `json:"mem"`
	Swap      MemInfo `json:"swap"`
	UptimeSec int64   `json:"uptimeSec"`
}

// MemInfo for memory stats
type MemInfo struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

// StorageSummary for dashboard widget
type StorageSummary struct {
	TotalBytes  uint64 `json:"totalBytes"`
	UsedBytes   uint64 `json:"usedBytes"`
	PoolsOnline int    `json:"poolsOnline"`
	PoolsTotal  int    `json:"poolsTotal"`
}

// DisksSummary for dashboard widget
type DisksSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	// Unknown: SMART had no answer for the disk.
	Unknown int `json:"unknown"`
	// LastScanISO is when SMART was last read; empty before the first check.
	LastScanISO string `json:"lastScanISO,omitempty"`
}

// ShareInfo for network shares
type ShareInfo struct {
	Name  string `json:"name"`
	Proto string `json:"proto"` // SMB, NFS, AFP
	Path  string `json:"path"`
	State string `json:"state"` // active, disabled
}

// AppInfo for installed apps
type AppInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
}

// MaintenanceStatus for scrub/balance operations
type MaintenanceStatus struct {
	Scrub   MaintenanceOp `json:"scrub"`
	Balance MaintenanceOp `json:"balance"`
}

// MaintenanceOp status
type MaintenanceOp struct {
	State   string `json:"state"` // idle, running, scheduled
	NextISO string `json:"nextISO"`
}

// EventInfo for recent activity
type EventInfo struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Severity  string `json:"severity"` // info, warning, error
}

// DashboardSources supplies the dashboard parts that nosd's stores own. A
// nil source shows an empty list.
type DashboardSources struct {
	Shares func() []ShareInfo
	Apps   func() []AppInfo
	// Disks is the disks' SMART health (the server caches it).
	Disks func() DisksSummary
}

func (s DashboardSources) disks() DisksSummary {
	if s.Disks == nil {
		return DisksSummary{}
	}
	return s.Disks()
}

func (s DashboardSources) shares() []ShareInfo {
	if s.Shares == nil {
		return []ShareInfo{}
	}
	return s.Shares()
}

func (s DashboardSources) apps() []AppInfo {
	if s.Apps == nil {
		return []AppInfo{}
	}
	return s.Apps()
}

// Dashboard returns aggregated dashboard data. (The shares and apps widgets
// used to show made-up Plex and Nextcloud entries.)
func Dashboard(src DashboardSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleDashboard(w, r, src)
	}
}

func handleDashboard(w http.ResponseWriter, r *http.Request, src DashboardSources) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use 300ms timeout to allow for proper CPU measurement
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	response := DashboardResponse{
		System:      getSystemSummary(ctx),
		Storage:     getStorageSummary(ctx),
		Disks:       src.disks(),
		Shares:      src.shares(),
		Apps:        src.apps(),
		Maintenance: getMaintenanceStatus(ctx),
		Events:      getRecentEvents(ctx),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getSystemSummary(ctx context.Context) SystemSummary {
	summary := SystemSummary{
		Status: "ok",
		Memory: MemInfo{},
		Swap:   MemInfo{},
	}

	// CPU usage (with proper interval for accurate measurement)
	cpuChan := make(chan float64, 1)
	go func() {
		// Use 200ms interval for more accurate CPU measurement
		if percents, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(percents) > 0 {
			cpuChan <- percents[0]
		} else {
			cpuChan <- 0
		}
	}()

	select {
	case summary.CPUPct = <-cpuChan:
	case <-ctx.Done():
		// If context times out, try to get instant reading
		if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
			summary.CPUPct = percents[0]
		} else {
			summary.CPUPct = 0
		}
	}

	// Load average
	if l, err := load.Avg(); err == nil {
		summary.Load1 = l.Load1
	}

	// Memory
	if m, err := mem.VirtualMemory(); err == nil {
		summary.Memory.Used = m.Used
		summary.Memory.Total = m.Total
	}

	// Swap
	if s, err := mem.SwapMemory(); err == nil {
		summary.Swap.Used = s.Used
		summary.Swap.Total = s.Total
	}

	// Uptime
	if uptime, err := host.Uptime(); err == nil {
		summary.UptimeSec = int64(uptime)
	}

	// Determine health status
	if summary.CPUPct > 90 || (summary.Memory.Total > 0 && float64(summary.Memory.Used)/float64(summary.Memory.Total) > 0.9) {
		summary.Status = "critical"
	} else if summary.CPUPct > 70 || (summary.Memory.Total > 0 && float64(summary.Memory.Used)/float64(summary.Memory.Total) > 0.7) {
		summary.Status = "degraded"
	}

	return summary
}

func getStorageSummary(ctx context.Context) StorageSummary {
	summary := StorageSummary{
		PoolsOnline: 1, // Default to 1 for now
		PoolsTotal:  1,
	}

	// Get disk usage for all mount points
	partitions, err := disk.Partitions(false)
	if err != nil {
		return summary
	}

	for _, partition := range partitions {
		// Skip special filesystems
		if partition.Fstype == "tmpfs" || partition.Fstype == "devtmpfs" {
			continue
		}

		if usage, err := disk.Usage(partition.Mountpoint); err == nil {
			summary.TotalBytes += usage.Total
			summary.UsedBytes += usage.Used
		}
	}

	return summary
}

func getMaintenanceStatus(ctx context.Context) MaintenanceStatus {
	// Live scrub and balance progress is at /api/v1/{scrub,balance}/status;
	// no next run is shown (it used to be a made-up date a week ahead).
	return MaintenanceStatus{
		Scrub:   MaintenanceOp{State: "idle"},
		Balance: MaintenanceOp{State: "idle"},
	}
}

func getRecentEvents(ctx context.Context) []EventInfo {
	events := []EventInfo{}

	// Read events from event log file
	eventFile := "/var/lib/nos/events.jsonl"
	if runtime.GOOS == "windows" {
		eventFile = `C:\ProgramData\nOS\events.jsonl`
	}

	// Try to read actual events
	if file, err := os.Open(eventFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)

		// Read all events into memory to get the most recent ones
		allEvents := []EventInfo{}
		for scanner.Scan() {
			var event map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
				evt := EventInfo{
					ID:        getStringField(event, "id"),
					Timestamp: getStringField(event, "timestamp"),
					Type:      getStringField(event, "category"),
					Message:   getStringField(event, "message"),
					Severity:  getStringField(event, "level"),
				}
				if evt.ID == "" {
					evt.ID = fmt.Sprintf("evt_%d", len(allEvents))
				}
				// Map level to severity
				switch evt.Severity {
				case "critical", "error":
					evt.Severity = "error"
				case "warning", "warn":
					evt.Severity = "warning"
				default:
					evt.Severity = "info"
				}
				allEvents = append(allEvents, evt)
			}
		}

		// Return the last 10 events
		start := 0
		if len(allEvents) > 10 {
			start = len(allEvents) - 10
		}
		events = allEvents[start:]
	}

	// No events is an empty list (it used to be a made-up "System
	// started" at the current time).

	// Reverse to show newest first
	for i := len(events)/2 - 1; i >= 0; i-- {
		opp := len(events) - 1 - i
		events[i], events[opp] = events[opp], events[i]
	}

	return events
}

func getStringField(m map[string]any, field string) string {
	if v, ok := m[field]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// Individual endpoint handlers for granular access

// HandleStorageSummary returns storage summary
func HandleStorageSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	defer cancel()

	summary := getStorageSummary(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// DisksSummaryHandler serves the disks' SMART health. (It used to call
// partitions over 80% full "warning" and everything else "healthy", with a
// "last scan" of now.)
func DisksSummaryHandler(src DashboardSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(src.disks())
	}
}

// HandleRecentEvents returns recent events
func HandleRecentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	defer cancel()

	events := getRecentEvents(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleMaintenanceStatus returns maintenance status
func HandleMaintenanceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	defer cancel()

	status := getMaintenanceStatus(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
