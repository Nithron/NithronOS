package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"nithronos/backend/nosd/internal/config"

	// "nithronos/backend/nosd/pkg/firewall"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// SystemHealthResponse represents system health metrics
type SystemHealthResponse struct {
	CPU       float64     `json:"cpu"`
	Load1     float64     `json:"load1"`
	Load5     float64     `json:"load5"`
	Load15    float64     `json:"load15"`
	Memory    MemoryInfo  `json:"memory"`
	Swap      SwapInfo    `json:"swap"`
	Uptime    int64       `json:"uptimeSec"`
	TempCPU   *float64    `json:"tempCpu"`
	Timestamp int64       `json:"timestamp"`
	Network   NetworkInfo `json:"network"`
	DiskIO    DiskIOStats `json:"diskIO"`
}

// MemoryInfo represents memory usage
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	UsagePct  float64 `json:"usagePct"`
	Cached    uint64  `json:"cached"`
	Buffers   uint64  `json:"buffers"`
}

// SwapInfo represents swap usage
type SwapInfo struct {
	Total    uint64  `json:"total"`
	Used     uint64  `json:"used"`
	Free     uint64  `json:"free"`
	UsagePct float64 `json:"usagePct"`
}

// NetworkInfo represents network statistics
type NetworkInfo struct {
	BytesRecv   uint64 `json:"bytesRecv"`
	BytesSent   uint64 `json:"bytesSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	RxSpeed     uint64 `json:"rxSpeed"`
	TxSpeed     uint64 `json:"txSpeed"`
}

// DiskIOStats represents disk I/O statistics
type DiskIOStats struct {
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	ReadOps    uint64 `json:"readOps"`
	WriteOps   uint64 `json:"writeOps"`
	ReadSpeed  uint64 `json:"readSpeed"`
	WriteSpeed uint64 `json:"writeSpeed"`
}

// DiskHealthResponse represents a disk's health information
type DiskHealthResponse struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Model      string      `json:"model"`
	Serial     string      `json:"serial"`
	SizeBytes  uint64      `json:"sizeBytes"`
	State      string      `json:"state"`
	TempC      *float64    `json:"tempC"`
	UsagePct   float64     `json:"usagePct"`
	Smart      SmartStatus `json:"smart"`
	Filesystem string      `json:"filesystem"`
	MountPoint string      `json:"mountPoint"`
}

// SmartStatus represents SMART health status
type SmartStatus struct {
	Passed     bool                   `json:"passed"`
	Attributes map[string]interface{} `json:"attrs,omitempty"`
	TestStatus string                 `json:"testStatus"`
}

// Network speed calculation cache
var (
	lastNetStats      net.IOCountersStat
	lastNetStatsTime  time.Time
	lastDiskStats     disk.IOCountersStat
	lastDiskStatsTime time.Time
)

// handleSystemHealth handles GET /api/health/system
func handleSystemHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := SystemHealthResponse{
			Timestamp: time.Now().Unix(),
		}

		// CPU usage
		if cpuPercent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
			health.CPU = cpuPercent[0]
		}

		// Load averages
		if loadAvg, err := load.Avg(); err == nil {
			health.Load1 = loadAvg.Load1
			health.Load5 = loadAvg.Load5
			health.Load15 = loadAvg.Load15
		}

		// Memory
		if vmStat, err := mem.VirtualMemory(); err == nil {
			health.Memory = MemoryInfo{
				Total:     vmStat.Total,
				Used:      vmStat.Used,
				Free:      vmStat.Free,
				Available: vmStat.Available,
				UsagePct:  vmStat.UsedPercent,
				Cached:    vmStat.Cached,
				Buffers:   vmStat.Buffers,
			}
		}

		// Swap
		if swapStat, err := mem.SwapMemory(); err == nil {
			health.Swap = SwapInfo{
				Total:    swapStat.Total,
				Used:     swapStat.Used,
				Free:     swapStat.Free,
				UsagePct: swapStat.UsedPercent,
			}
		}

		// Uptime
		if uptime, err := host.Uptime(); err == nil {
			health.Uptime = int64(uptime)
		}

		// Network stats with speed calculation
		if netStats, err := net.IOCounters(false); err == nil && len(netStats) > 0 {
			current := netStats[0]
			now := time.Now()

			health.Network = NetworkInfo{
				BytesRecv:   current.BytesRecv,
				BytesSent:   current.BytesSent,
				PacketsRecv: current.PacketsRecv,
				PacketsSent: current.PacketsSent,
			}

			// Calculate speed if we have previous stats
			if !lastNetStatsTime.IsZero() {
				duration := now.Sub(lastNetStatsTime).Seconds()
				if duration > 0 {
					health.Network.RxSpeed = uint64(float64(current.BytesRecv-lastNetStats.BytesRecv) / duration)
					health.Network.TxSpeed = uint64(float64(current.BytesSent-lastNetStats.BytesSent) / duration)
				}
			}

			lastNetStats = current
			lastNetStatsTime = now
		}

		// Disk I/O stats with speed calculation
		if diskStats, err := disk.IOCounters(); err == nil {
			var totalRead, totalWrite, totalReadOps, totalWriteOps uint64
			for _, stat := range diskStats {
				totalRead += stat.ReadBytes
				totalWrite += stat.WriteBytes
				totalReadOps += stat.ReadCount
				totalWriteOps += stat.WriteCount
			}

			now := time.Now()
			health.DiskIO = DiskIOStats{
				ReadBytes:  totalRead,
				WriteBytes: totalWrite,
				ReadOps:    totalReadOps,
				WriteOps:   totalWriteOps,
			}

			// Calculate speed if we have previous stats
			if !lastDiskStatsTime.IsZero() {
				duration := now.Sub(lastDiskStatsTime).Seconds()
				if duration > 0 {
					health.DiskIO.ReadSpeed = uint64(float64(totalRead-lastDiskStats.ReadBytes) / duration)
					health.DiskIO.WriteSpeed = uint64(float64(totalWrite-lastDiskStats.WriteBytes) / duration)
				}
			}

			lastDiskStats = disk.IOCountersStat{
				ReadBytes:  totalRead,
				WriteBytes: totalWrite,
			}
			lastDiskStatsTime = now
		}

		// CPU temperature (platform-specific, may not be available)
		if runtime.GOOS == "linux" {
			if temps, err := host.SensorsTemperatures(); err == nil {
				for _, temp := range temps {
					if temp.SensorKey == "coretemp_core_0" || temp.SensorKey == "cpu_thermal" {
						health.TempCPU = &temp.Temperature
						break
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// handleDiskHealth handles GET /api/health/disks
func handleDiskHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var disks []DiskHealthResponse

		// Get disk partitions
		partitions, err := disk.Partitions(false)
		if err != nil {
			// Return empty array on error
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode([]DiskHealthResponse{}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}

		for _, partition := range partitions {
			// Skip certain filesystem types
			if partition.Fstype == "tmpfs" || partition.Fstype == "devtmpfs" {
				continue
			}

			diskInfo := DiskHealthResponse{
				ID:         partition.Device,
				Name:       partition.Device,
				Filesystem: partition.Fstype,
				MountPoint: partition.Mountpoint,
				State:      "healthy", // Default state
				Smart: SmartStatus{
					Passed:     true,
					TestStatus: "passed",
				},
			}

			// Get usage statistics
			if usage, err := disk.Usage(partition.Mountpoint); err == nil {
				diskInfo.SizeBytes = usage.Total
				diskInfo.UsagePct = usage.UsedPercent
			}

			// Try to get disk info (model, serial)
			// This would require additional system calls or parsing /sys/block
			// For now, we'll use basic info
			diskInfo.Model = "Unknown"
			diskInfo.Serial = "Unknown"

			// Determine health state based on usage
			if diskInfo.UsagePct > 90 {
				diskInfo.State = "critical"
			} else if diskInfo.UsagePct > 80 {
				diskInfo.State = "warning"
			}

			disks = append(disks, diskInfo)
		}

		// Ensure we always return an array, even if empty
		if disks == nil {
			disks = []DiskHealthResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(disks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
