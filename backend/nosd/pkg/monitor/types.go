package monitor

import (
	"time"
)

// MetricType defines the type of metric
type MetricType string

const (
	MetricTypeCPU           MetricType = "cpu"
	MetricTypeMemory        MetricType = "memory"
	MetricTypeSwap          MetricType = "swap"
	MetricTypeLoad          MetricType = "load"
	MetricTypeDiskIO        MetricType = "disk_io"
	MetricTypeDiskSpace     MetricType = "disk_space"
	MetricTypeDiskTemp      MetricType = "disk_temp"
	MetricTypeDiskSMART     MetricType = "disk_smart"
	MetricTypeNetworkRX     MetricType = "net_rx"
	MetricTypeNetworkTX     MetricType = "net_tx"
	MetricTypeServiceHealth MetricType = "service_health"
	MetricTypeBtrfsScrub    MetricType = "btrfs_scrub"
	MetricTypeBtrfsErrors   MetricType = "btrfs_errors"
	MetricTypeBackupJobs    MetricType = "backup_jobs"
)

// DataPoint represents a single metric measurement
type DataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// TimeSeries represents a series of data points
type TimeSeries struct {
	Metric     MetricType        `json:"metric"`
	Labels     map[string]string `json:"labels,omitempty"`
	DataPoints []DataPoint       `json:"data_points"`
}

// SystemMetrics represents current system metrics
type SystemMetrics struct {
	Timestamp time.Time     `json:"timestamp"`
	CPU       CPUMetrics    `json:"cpu"`
	Memory    MemoryMetrics `json:"memory"`
	Load      LoadMetrics   `json:"load"`
	Uptime    int64         `json:"uptime_seconds"`
}

// CPUMetrics represents CPU usage
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	CoreUsage    []float64 `json:"core_usage,omitempty"`
	Temperature  float64   `json:"temperature,omitempty"`
}

// MemoryMetrics represents memory usage
type MemoryMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

// LoadMetrics represents system load
type LoadMetrics struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// DiskMetrics represents disk metrics
type DiskMetrics struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mount_point,omitempty"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	ReadBytes   uint64  `json:"read_bytes"`
	WriteBytes  uint64  `json:"write_bytes"`
	ReadOps     uint64  `json:"read_ops"`
	WriteOps    uint64  `json:"write_ops"`
	Temperature float64 `json:"temperature,omitempty"`
	SMARTStatus string  `json:"smart_status,omitempty"`
}

// NetworkMetrics represents network interface metrics
type NetworkMetrics struct {
	Interface string `json:"interface"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxDropped uint64 `json:"tx_dropped"`
	LinkState string `json:"link_state"`
	Speed     int    `json:"speed_mbps,omitempty"`
}

// ServiceMetrics represents service health
type ServiceMetrics struct {
	Name         string    `json:"name"`
	State        string    `json:"state"`
	SubState     string    `json:"sub_state"`
	Active       bool      `json:"active"`
	Running      bool      `json:"running"`
	Since        time.Time `json:"since,omitempty"`
	MainPID      int       `json:"main_pid,omitempty"`
	MemoryBytes  uint64    `json:"memory_bytes,omitempty"`
	CPUPercent   float64   `json:"cpu_percent,omitempty"`
	RestartCount int       `json:"restart_count,omitempty"`
}

// BtrfsMetrics represents Btrfs-specific metrics
type BtrfsMetrics struct {
	Device        string    `json:"device"`
	MountPoint    string    `json:"mount_point"`
	ScrubState    string    `json:"scrub_state,omitempty"`
	ScrubProgress float64   `json:"scrub_progress,omitempty"`
	LastScrub     time.Time `json:"last_scrub,omitempty"`
	ErrorsWrite   uint64    `json:"errors_write"`
	ErrorsRead    uint64    `json:"errors_read"`
	ErrorsFlush   uint64    `json:"errors_flush"`
	ErrorsCorrupt uint64    `json:"errors_corrupt"`
	ErrorsGen     uint64    `json:"errors_generation"`
}

// MonitorOverview provides a quick system snapshot
type MonitorOverview struct {
	Timestamp    time.Time        `json:"timestamp"`
	System       SystemMetrics    `json:"system"`
	Disks        []DiskMetrics    `json:"disks"`
	Network      []NetworkMetrics `json:"network"`
	Services     []ServiceMetrics `json:"services"`
	Btrfs        []BtrfsMetrics   `json:"btrfs,omitempty"`
	AlertsActive int              `json:"alerts_active"`
}

// TimeSeriesQuery defines parameters for querying time series data
type TimeSeriesQuery struct {
	Metric    MetricType        `json:"metric"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Step      time.Duration     `json:"step"`
	Filters   map[string]string `json:"filters,omitempty"`
	Aggregate string            `json:"aggregate,omitempty"` // avg, min, max, sum
}

// SMARTData represents SMART health data
type SMARTData struct {
	Device               string           `json:"device"`
	HealthStatus         string           `json:"health_status"`
	Temperature          int              `json:"temperature_celsius,omitempty"`
	PowerOnHours         uint64           `json:"power_on_hours"`
	PowerCycles          uint64           `json:"power_cycles"`
	ReallocatedSectors   uint64           `json:"reallocated_sectors"`
	PendingSectors       uint64           `json:"pending_sectors"`
	UncorrectableSectors uint64           `json:"uncorrectable_sectors"`
	LastChecked          time.Time        `json:"last_checked"`
	Attributes           []SMARTAttribute `json:"attributes,omitempty"`
}

// SMARTAttribute represents a single SMART attribute
type SMARTAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  string `json:"raw_value"`
	Failed    bool   `json:"failed"`
}
