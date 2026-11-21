package appsdk

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest defines the structure of a NithronOS app manifest (nosapp.yaml)
type Manifest struct {
	// Meta contains app metadata
	Meta AppMeta `yaml:"meta" json:"meta"`

	// Runtime defines runtime configuration
	Runtime RuntimeConfig `yaml:"runtime" json:"runtime"`

	// Permissions defines required permissions
	Permissions PermissionsConfig `yaml:"permissions" json:"permissions"`

	// Backup defines backup configuration
	Backup BackupConfig `yaml:"backup" json:"backup"`

	// WebUI defines web UI configuration
	WebUI *WebUIConfig `yaml:"webui,omitempty" json:"webui,omitempty"`
}

// AppMeta contains app metadata
type AppMeta struct {
	Name        string   `yaml:"name" json:"name"`
	ID          string   `yaml:"id" json:"id"`
	Version     string   `yaml:"version" json:"version"`
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description string   `yaml:"description" json:"description"`
	Upstream    string   `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Author      string   `yaml:"author,omitempty" json:"author,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage    string   `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Categories  []string `yaml:"categories,omitempty" json:"categories,omitempty"`
}

// RuntimeConfig defines runtime configuration
type RuntimeConfig struct {
	// DockerComposePath is the path to docker-compose.yaml
	DockerComposePath string `yaml:"docker_compose_path" json:"docker_compose_path"`

	// EnvSchema is the JSON Schema for environment variables
	EnvSchema map[string]interface{} `yaml:"env_schema,omitempty" json:"env_schema,omitempty"`

	// Mounts defines volume mounts
	Mounts []Mount `yaml:"mounts,omitempty" json:"mounts,omitempty"`

	// Ports defines exposed ports
	Ports []Port `yaml:"ports,omitempty" json:"ports,omitempty"`

	// HealthChecks defines health checks
	HealthChecks []HealthCheck `yaml:"health_checks,omitempty" json:"health_checks,omitempty"`

	// UpdateStrategy defines update strategy
	UpdateStrategy UpdateStrategy `yaml:"update_strategy,omitempty" json:"update_strategy,omitempty"`

	// Resources defines resource limits
	Resources ResourceLimits `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// Mount defines a volume mount
type Mount struct {
	Name        string `yaml:"name" json:"name"`
	Path        string `yaml:"path" json:"path"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Persistent  bool   `yaml:"persistent,omitempty" json:"persistent,omitempty"`
}

// Port defines an exposed port
type Port struct {
	Port        int    `yaml:"port" json:"port"`
	Protocol    string `yaml:"protocol,omitempty" json:"protocol,omitempty"` // tcp/udp
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	WebUI       bool   `yaml:"webui,omitempty" json:"webui,omitempty"`
}

// HealthCheck defines a health check
type HealthCheck struct {
	Type      string `yaml:"type" json:"type"` // container, http, tcp
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
	URL       string `yaml:"url,omitempty" json:"url,omitempty"`
	Port      int    `yaml:"port,omitempty" json:"port,omitempty"`
	Interval  int    `yaml:"interval,omitempty" json:"interval,omitempty"` // seconds
	Timeout   int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // seconds
	Retries   int    `yaml:"retries,omitempty" json:"retries,omitempty"`
}

// UpdateStrategy defines update strategy
type UpdateStrategy struct {
	Type              string `yaml:"type,omitempty" json:"type,omitempty"` // rolling, recreate
	MaxUnavailable    int    `yaml:"max_unavailable,omitempty" json:"max_unavailable,omitempty"`
	MaxSurge          int    `yaml:"max_surge,omitempty" json:"max_surge,omitempty"`
	HealthCheckDelay  int    `yaml:"health_check_delay,omitempty" json:"health_check_delay,omitempty"`
	PreUpdateSnapshot bool   `yaml:"pre_update_snapshot,omitempty" json:"pre_update_snapshot,omitempty"`
}

// ResourceLimits defines resource limits
type ResourceLimits struct {
	CPU    CPULimits    `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory MemoryLimits `yaml:"memory,omitempty" json:"memory,omitempty"`
}

// CPULimits defines CPU limits
type CPULimits struct {
	Request string `yaml:"request,omitempty" json:"request,omitempty"` // e.g., "0.5"
	Limit   string `yaml:"limit,omitempty" json:"limit,omitempty"`     // e.g., "2"
}

// MemoryLimits defines memory limits
type MemoryLimits struct {
	Request string `yaml:"request,omitempty" json:"request,omitempty"` // e.g., "512Mi"
	Limit   string `yaml:"limit,omitempty" json:"limit,omitempty"`     // e.g., "2Gi"
}

// PermissionsConfig defines required permissions
type PermissionsConfig struct {
	// APIScopes defines required API scopes
	APIScopes []string `yaml:"api_scopes,omitempty" json:"api_scopes,omitempty"`

	// AgentOps defines required agent operations
	AgentOps []string `yaml:"agent_ops,omitempty" json:"agent_ops,omitempty"`

	// Privileged indicates if privileged mode is required
	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty"`

	// HostNetwork indicates if host network is required
	HostNetwork bool `yaml:"host_network,omitempty" json:"host_network,omitempty"`

	// HostPID indicates if host PID namespace is required
	HostPID bool `yaml:"host_pid,omitempty" json:"host_pid,omitempty"`

	// Capabilities defines required Linux capabilities
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// BackupConfig defines backup configuration
type BackupConfig struct {
	// IncludePaths defines paths to include in backups
	IncludePaths []string `yaml:"include_paths,omitempty" json:"include_paths,omitempty"`

	// ExcludePaths defines paths to exclude from backups
	ExcludePaths []string `yaml:"exclude_paths,omitempty" json:"exclude_paths,omitempty"`

	// QuiesceHooks defines quiesce hooks for consistent backups
	QuiesceHooks *QuiesceHooks `yaml:"quiesce_hooks,omitempty" json:"quiesce_hooks,omitempty"`

	// BackupSize provides size hints for backup planning
	BackupSize string `yaml:"backup_size,omitempty" json:"backup_size,omitempty"` // e.g., "100MB", "5GB"
}

// QuiesceHooks defines hooks for quiescing the app
type QuiesceHooks struct {
	PreBackup  string `yaml:"pre_backup,omitempty" json:"pre_backup,omitempty"`   // Command to run before backup
	PostBackup string `yaml:"post_backup,omitempty" json:"post_backup,omitempty"` // Command to run after backup
	Timeout    int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`         // Timeout in seconds
}

// WebUIConfig defines web UI configuration
type WebUIConfig struct {
	// Path defines the reverse proxy path
	Path string `yaml:"path" json:"path"` // e.g., "/apps/myapp"

	// Port defines the internal port to proxy to
	Port int `yaml:"port" json:"port"`

	// AuthMode defines authentication mode
	AuthMode string `yaml:"auth_mode,omitempty" json:"auth_mode,omitempty"` // none, inherit, custom

	// StripPath indicates if the path should be stripped
	StripPath bool `yaml:"strip_path,omitempty" json:"strip_path,omitempty"`

	// Headers defines additional headers to set
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// Hooks defines lifecycle hooks
type Hooks struct {
	PreInstall  *Hook `yaml:"pre_install,omitempty" json:"pre_install,omitempty"`
	PostInstall *Hook `yaml:"post_install,omitempty" json:"post_install,omitempty"`
	PreUpdate   *Hook `yaml:"pre_update,omitempty" json:"pre_update,omitempty"`
	PostUpdate  *Hook `yaml:"post_update,omitempty" json:"post_update,omitempty"`
	PreRemove   *Hook `yaml:"pre_remove,omitempty" json:"pre_remove,omitempty"`
	PostRemove  *Hook `yaml:"post_remove,omitempty" json:"post_remove,omitempty"`
}

// Hook defines a lifecycle hook
type Hook struct {
	Script  string `yaml:"script" json:"script"`                         // Script path or inline script
	Timeout int    `yaml:"timeout,omitempty" json:"timeout,omitempty"`   // Timeout in seconds
	OnError string `yaml:"on_error,omitempty" json:"on_error,omitempty"` // continue, fail
}

// LoadManifest loads a manifest from a file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Validate manifest
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// SaveManifest saves a manifest to a file
func (m *Manifest) SaveManifest(path string) error {
	// Validate before saving
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// Validate validates a manifest
func (m *Manifest) Validate() error {
	// Validate meta
	if m.Meta.Name == "" {
		return fmt.Errorf("meta.name is required")
	}
	if m.Meta.ID == "" {
		return fmt.Errorf("meta.id is required")
	}
	if m.Meta.Version == "" {
		return fmt.Errorf("meta.version is required")
	}

	// Validate runtime
	if m.Runtime.DockerComposePath == "" {
		return fmt.Errorf("runtime.docker_compose_path is required")
	}

	// Validate ports
	for _, port := range m.Runtime.Ports {
		if port.Port <= 0 || port.Port > 65535 {
			return fmt.Errorf("invalid port: %d", port.Port)
		}
		if port.Protocol != "" && port.Protocol != "tcp" && port.Protocol != "udp" {
			return fmt.Errorf("invalid protocol: %s", port.Protocol)
		}
	}

	// Validate health checks
	for _, hc := range m.Runtime.HealthChecks {
		switch hc.Type {
		case "container":
			if hc.Container == "" {
				return fmt.Errorf("container is required for container health check")
			}
		case "http":
			if hc.URL == "" {
				return fmt.Errorf("url is required for http health check")
			}
		case "tcp":
			if hc.Port <= 0 || hc.Port > 65535 {
				return fmt.Errorf("invalid port for tcp health check: %d", hc.Port)
			}
		default:
			return fmt.Errorf("invalid health check type: %s", hc.Type)
		}
	}

	// Validate WebUI if present
	if m.WebUI != nil {
		if m.WebUI.Path == "" {
			return fmt.Errorf("webui.path is required")
		}
		if m.WebUI.Port <= 0 || m.WebUI.Port > 65535 {
			return fmt.Errorf("invalid webui.port: %d", m.WebUI.Port)
		}
		if m.WebUI.AuthMode != "" &&
			m.WebUI.AuthMode != "none" &&
			m.WebUI.AuthMode != "inherit" &&
			m.WebUI.AuthMode != "custom" {
			return fmt.Errorf("invalid webui.auth_mode: %s", m.WebUI.AuthMode)
		}
	}

	return nil
}

// GetRequiredAPIScopes returns all required API scopes
func (m *Manifest) GetRequiredAPIScopes() []string {
	return m.Permissions.APIScopes
}

// GetRequiredAgentOps returns all required agent operations
func (m *Manifest) GetRequiredAgentOps() []string {
	return m.Permissions.AgentOps
}

// IsPrivileged returns true if the app requires privileged mode
func (m *Manifest) IsPrivileged() bool {
	return m.Permissions.Privileged
}

// GetDataPaths returns all data paths that should be backed up
func (m *Manifest) GetDataPaths() []string {
	paths := []string{}

	// Add mounted paths
	for _, mount := range m.Runtime.Mounts {
		if mount.Persistent {
			paths = append(paths, mount.Path)
		}
	}

	// Add explicit backup paths
	paths = append(paths, m.Backup.IncludePaths...)

	return paths
}
