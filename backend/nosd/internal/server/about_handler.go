package server

import (
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"nos/backend/nosd/internal/config"

	"github.com/go-chi/chi/v5"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// AboutInfo represents system information for the About page
type AboutInfo struct {
	System    SystemInfo   `json:"system"`
	Hardware  HardwareInfo `json:"hardware"`
	Software  SoftwareInfo `json:"software"`
	License   LicenseInfo  `json:"license"`
	Support   SupportInfo  `json:"support"`
	BuildInfo BuildInfo    `json:"build_info"`
}

// SystemInfo represents basic system information
type SystemInfo struct {
	Hostname     string    `json:"hostname"`
	Platform     string    `json:"platform"`
	Architecture string    `json:"architecture"`
	Kernel       string    `json:"kernel"`
	Uptime       int64     `json:"uptime"`
	BootTime     time.Time `json:"boot_time"`
	Timezone     string    `json:"timezone"`
	LocalTime    time.Time `json:"local_time"`
}

// HardwareInfo represents hardware information
type HardwareInfo struct {
	CPU         CPUInfo    `json:"cpu"`
	Memory      MemoryInfo `json:"memory"`
	Storage     []DiskInfo `json:"storage"`
	Network     []NICInfo  `json:"network"`
	Motherboard string     `json:"motherboard"`
	BIOS        BIOSInfo   `json:"bios"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Model    string  `json:"model"`
	Cores    int     `json:"cores"`
	Threads  int     `json:"threads"`
	Speed    float64 `json:"speed_mhz"`
	Cache    int     `json:"cache_kb"`
	VendorID string  `json:"vendor_id"`
	Family   string  `json:"family"`
}

// DiskInfo represents disk information
type DiskInfo struct {
	Device       string  `json:"device"`
	Model        string  `json:"model"`
	Size         uint64  `json:"size"`
	Type         string  `json:"type"` // SSD, HDD
	Filesystem   string  `json:"filesystem"`
	MountPoint   string  `json:"mount_point"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// NICInfo represents network interface information
type NICInfo struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	Speed     int      `json:"speed_mbps"`
	Addresses []string `json:"addresses"`
	Type      string   `json:"type"` // ethernet, wifi, virtual
}

// BIOSInfo represents BIOS/UEFI information
type BIOSInfo struct {
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
	Date    string `json:"date"`
}

// SoftwareInfo represents software version information
type SoftwareInfo struct {
	NOSVersion    string            `json:"nos_version"`
	APIVersion    string            `json:"api_version"`
	WebUIVersion  string            `json:"webui_version"`
	AgentVersion  string            `json:"agent_version"`
	GoVersion     string            `json:"go_version"`
	NodeVersion   string            `json:"node_version"`
	DockerVersion string            `json:"docker_version"`
	Components    map[string]string `json:"components"`
}

// LicenseInfo represents license information
type LicenseInfo struct {
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	ExpiryDate time.Time `json:"expiry_date,omitempty"`
	Features   []string  `json:"features"`
	MaxUsers   int       `json:"max_users"`
	MaxStorage int64     `json:"max_storage_gb"`
}

// SupportInfo represents support information
type SupportInfo struct {
	Email         string `json:"email,omitempty"`
	Website       string `json:"website"`
	Documentation string `json:"documentation"`
	Forum         string `json:"forum"`
	Discord       string `json:"discord"`
	GitHub        string `json:"github"`
}

// BuildInfo represents build information
type BuildInfo struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	Branch    string    `json:"branch"`
	BuildDate time.Time `json:"build_date"`
	BuildHost string    `json:"build_host"`
	Compiler  string    `json:"compiler"`
}

// AboutHandler handles about/system information endpoints
type AboutHandler struct {
	config config.Config
}

// NewAboutHandler creates a new about handler
func NewAboutHandler(cfg config.Config) *AboutHandler {
	return &AboutHandler{
		config: cfg,
	}
}

// Routes returns the routes for the about handler
func (h *AboutHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetAboutInfo)
	return r
}

// GetAboutInfo returns comprehensive system information
func (h *AboutHandler) GetAboutInfo(w http.ResponseWriter, r *http.Request) {
	info := AboutInfo{
		System:    h.getSystemInfo(),
		Hardware:  h.getHardwareInfo(),
		Software:  h.getSoftwareInfo(),
		License:   h.getLicenseInfo(),
		Support:   h.getSupportInfo(),
		BuildInfo: h.getBuildInfo(),
	}

	writeJSON(w, info)
}

// Helper methods

func (h *AboutHandler) getSystemInfo() SystemInfo {
	info := SystemInfo{
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// Get host info
	if hostInfo, err := host.Info(); err == nil {
		info.Kernel = hostInfo.KernelVersion
		info.Uptime = int64(hostInfo.Uptime)
		info.BootTime = time.Now().Add(-time.Duration(hostInfo.Uptime) * time.Second)
	}

	// Get timezone
	info.Timezone = time.Local.String()
	info.LocalTime = time.Now()

	return info
}

func (h *AboutHandler) getHardwareInfo() HardwareInfo {
	info := HardwareInfo{
		Storage: []DiskInfo{},
		Network: []NICInfo{},
	}

	// Get CPU info
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		info.CPU = CPUInfo{
			Model:    cpuInfo[0].ModelName,
			Cores:    int(cpuInfo[0].Cores),
			Threads:  len(cpuInfo),
			Speed:    cpuInfo[0].Mhz,
			VendorID: cpuInfo[0].VendorID,
			Family:   cpuInfo[0].Family,
		}
		if cpuInfo[0].CacheSize > 0 {
			info.CPU.Cache = int(cpuInfo[0].CacheSize)
		}
	}

	// Get memory info
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.Memory = MemoryInfo{
			Total:     memInfo.Total,
			Used:      memInfo.Used,
			Free:      memInfo.Free,
			Available: memInfo.Available,
			UsagePct:  memInfo.UsedPercent,
			Cached:    memInfo.Cached,
			Buffers:   memInfo.Buffers,
		}
	}

	// Get disk info
	if partitions, err := disk.Partitions(false); err == nil {
		for _, partition := range partitions {
			// Skip special filesystems
			if strings.HasPrefix(partition.Fstype, "tmp") || partition.Fstype == "devtmpfs" {
				continue
			}

			diskInfo := DiskInfo{
				Device:     partition.Device,
				Filesystem: partition.Fstype,
				MountPoint: partition.Mountpoint,
			}

			// Get usage
			if usage, err := disk.Usage(partition.Mountpoint); err == nil {
				diskInfo.Size = usage.Total
				diskInfo.Used = usage.Used
				diskInfo.Free = usage.Free
				diskInfo.UsagePercent = usage.UsedPercent
			}

			// Determine disk type (simplified)
			if strings.Contains(strings.ToLower(partition.Device), "nvme") {
				diskInfo.Type = "NVMe SSD"
			} else if strings.Contains(strings.ToLower(partition.Device), "sd") {
				diskInfo.Type = "SATA"
			} else {
				diskInfo.Type = "Unknown"
			}

			info.Storage = append(info.Storage, diskInfo)
		}
	}

	// Get BIOS info (Linux-specific)
	if runtime.GOOS == "linux" {
		info.BIOS = h.getBIOSInfo()
		info.Motherboard = h.getMotherboardInfo()
	}

	return info
}

func (h *AboutHandler) getBIOSInfo() BIOSInfo {
	info := BIOSInfo{}

	if runtime.GOOS != "linux" {
		return info
	}

	// Try to read DMI information
	if data, err := os.ReadFile("/sys/class/dmi/id/bios_vendor"); err == nil {
		info.Vendor = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile("/sys/class/dmi/id/bios_version"); err == nil {
		info.Version = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile("/sys/class/dmi/id/bios_date"); err == nil {
		info.Date = strings.TrimSpace(string(data))
	}

	return info
}

func (h *AboutHandler) getMotherboardInfo() string {
	if runtime.GOOS != "linux" {
		return "Unknown"
	}

	board := ""
	if data, err := os.ReadFile("/sys/class/dmi/id/board_vendor"); err == nil {
		board = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile("/sys/class/dmi/id/board_name"); err == nil {
		if board != "" {
			board += " "
		}
		board += strings.TrimSpace(string(data))
	}

	if board == "" {
		board = "Unknown"
	}

	return board
}

func (h *AboutHandler) getSoftwareInfo() SoftwareInfo {
	info := SoftwareInfo{
		NOSVersion:   "0.9.5-pre-alpha",
		APIVersion:   "1.0.0",
		WebUIVersion: "1.0.0",
		AgentVersion: "1.0.0",
		GoVersion:    runtime.Version(),
		Components:   make(map[string]string),
	}

	// Get Node.js version
	if output, err := exec.Command("node", "--version").Output(); err == nil {
		info.NodeVersion = strings.TrimSpace(string(output))
	}

	// Get Docker version
	if output, err := exec.Command("docker", "--version").Output(); err == nil {
		info.DockerVersion = strings.TrimSpace(string(output))
	}

	// Get component versions
	info.Components["caddy"] = h.getComponentVersion("caddy")
	info.Components["samba"] = h.getComponentVersion("smbd")
	info.Components["nfs"] = h.getComponentVersion("nfs-server")
	info.Components["systemd"] = h.getComponentVersion("systemd")

	return info
}

func (h *AboutHandler) getComponentVersion(component string) string {
	switch component {
	case "caddy":
		if output, err := exec.Command("caddy", "version").Output(); err == nil {
			parts := strings.Fields(string(output))
			if len(parts) > 0 {
				return parts[0]
			}
		}
	case "smbd":
		if output, err := exec.Command("smbd", "--version").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				return strings.TrimSpace(lines[0])
			}
		}
	case "systemd":
		if output, err := exec.Command("systemctl", "--version").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				parts := strings.Fields(lines[0])
				if len(parts) > 1 {
					return parts[1]
				}
			}
		}
	}
	return "Unknown"
}

func (h *AboutHandler) getLicenseInfo() LicenseInfo {
	return LicenseInfo{
		Type:   "Open Source",
		Status: "Active",
		Features: []string{
			"Unlimited Users",
			"Unlimited Storage",
			"All Features Enabled",
			"Community Support",
		},
		MaxUsers:   -1, // Unlimited
		MaxStorage: -1, // Unlimited
	}
}

func (h *AboutHandler) getSupportInfo() SupportInfo {
	return SupportInfo{
		Website:       "https://github.com/Nithron/nOS",
		Documentation: "https://github.com/Nithron/nOS/tree/main/docs",
		Forum:         "https://github.com/Nithron/nOS/discussions",
		Discord:       "https://discord.gg/qzB37WS5AT",
		GitHub:        "https://github.com/Nithron/nOS",
	}
}

func (h *AboutHandler) getBuildInfo() BuildInfo {
	info := BuildInfo{
		Version:   "0.9.5-pre-alpha",
		Branch:    "main",
		BuildDate: time.Now(), // Would be set at build time
		Compiler:  runtime.Version(),
	}

	// Try to get git commit
	if output, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		info.Commit = strings.TrimSpace(string(output))[:8] // First 8 chars
	}

	// Try to get git branch
	if output, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		info.Branch = strings.TrimSpace(string(output))
	}

	// Get build host
	if hostname, err := os.Hostname(); err == nil {
		info.BuildHost = hostname
	}

	return info
}
