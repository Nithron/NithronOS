package apps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// TemplateRenderer handles compose template rendering and validation
type TemplateRenderer struct {
	templateDir string
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(templateDir string) *TemplateRenderer {
	return &TemplateRenderer{
		templateDir: templateDir,
	}
}

// RenderComposeFile renders a compose template with given parameters
func (tr *TemplateRenderer) RenderComposeFile(entry *CatalogEntry, params map[string]interface{}) ([]byte, error) {
	// Load compose template
	composePath := filepath.Join(tr.templateDir, entry.Compose)
	templateContent, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose template: %w", err)
	}

	// Merge params with defaults
	mergedParams := tr.mergeParams(entry.Defaults, params)

	// Create environment map from params
	env := tr.paramsToEnv(mergedParams)

	// Replace ${VAR} placeholders
	rendered := tr.replaceVariables(string(templateContent), env)

	// Validate the rendered YAML
	var composeData interface{}
	if err := yaml.Unmarshal([]byte(rendered), &composeData); err != nil {
		return nil, fmt.Errorf("rendered compose file is invalid YAML: %w", err)
	}

	// Apply security defaults
	rendered = tr.applySecurityDefaults(rendered, entry.NeedsPrivileged, entry.Defaults.Resources)

	return []byte(rendered), nil
}

// ValidateParams validates parameters against JSON schema
func (tr *TemplateRenderer) ValidateParams(entry *CatalogEntry, params map[string]interface{}) error {
	if entry.Schema == "" {
		// No schema, params are optional
		return nil
	}

	// Load schema
	schemaPath := filepath.Join(tr.templateDir, entry.Schema)
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Schema file doesn't exist, skip validation
			return nil
		}
		return fmt.Errorf("failed to read schema: %w", err)
	}

	// Parse schema
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)

	// Convert params to JSON for validation
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}
	documentLoader := gojsonschema.NewBytesLoader(paramsJSON)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		errors := []string{}
		for _, err := range result.Errors() {
			errors = append(errors, fmt.Sprintf("%s: %s", err.Field(), err.Description()))
		}
		return fmt.Errorf("parameter validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// mergeParams merges user params with defaults
func (tr *TemplateRenderer) mergeParams(defaults AppDefaults, params map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Start with defaults
	for k, v := range defaults.Env {
		merged[k] = v
	}

	// Override with user params
	for k, v := range params {
		merged[k] = v
	}

	// Add port mappings
	if len(defaults.Ports) > 0 {
		ports := []map[string]interface{}{}
		for _, p := range defaults.Ports {
			ports = append(ports, map[string]interface{}{
				"host":      p.Host,
				"container": p.Container,
				"protocol":  p.Protocol,
			})
		}
		merged["_ports"] = ports
	}

	// Add volume mappings
	if len(defaults.Volumes) > 0 {
		volumes := []map[string]interface{}{}
		for _, v := range defaults.Volumes {
			volumes = append(volumes, map[string]interface{}{
				"host":      v.Host,
				"container": v.Container,
				"read_only": v.ReadOnly,
			})
		}
		merged["_volumes"] = volumes
	}

	// Add resource limits
	if defaults.Resources.CPULimit != "" || defaults.Resources.MemoryLimit != "" {
		resources := map[string]interface{}{}
		if defaults.Resources.CPULimit != "" {
			resources["cpu_limit"] = defaults.Resources.CPULimit
		}
		if defaults.Resources.MemoryLimit != "" {
			resources["memory_limit"] = defaults.Resources.MemoryLimit
		}
		merged["_resources"] = resources
	}

	return merged
}

// paramsToEnv converts params map to environment variables
func (tr *TemplateRenderer) paramsToEnv(params map[string]interface{}) map[string]string {
	env := make(map[string]string)

	for k, v := range params {
		// Skip internal keys
		if strings.HasPrefix(k, "_") {
			continue
		}

		// Convert to string
		switch val := v.(type) {
		case string:
			env[k] = val
		case bool:
			if val {
				env[k] = "true"
			} else {
				env[k] = "false"
			}
		case int, int32, int64:
			env[k] = fmt.Sprintf("%d", val)
		case float32, float64:
			env[k] = fmt.Sprintf("%f", val)
		default:
			// Use JSON for complex types
			data, _ := json.Marshal(val)
			env[k] = string(data)
		}
	}

	return env
}

// replaceVariables replaces ${VAR} placeholders in template
func (tr *TemplateRenderer) replaceVariables(content string, env map[string]string) string {
	// Pattern for ${VAR} or ${VAR:-default}
	re := regexp.MustCompile(`\$\{([^}:]+)(:-([^}]*))?\}`)

	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Parse the match
		parts := re.FindStringSubmatch(match)
		varName := parts[1]
		defaultValue := ""
		if len(parts) > 3 {
			defaultValue = parts[3]
		}

		// Look up the value
		if value, ok := env[varName]; ok && value != "" {
			return value
		}

		// Return default value or original if no default
		if defaultValue != "" {
			return defaultValue
		}

		// Return empty string for undefined variables
		return ""
	})

	return result
}

// applySecurityDefaults adds security configurations to compose file
func (tr *TemplateRenderer) applySecurityDefaults(content string, needsPrivileged bool, res ResourceLimits) string {
	// Parse YAML
	var compose map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		// Return as-is if we can't parse
		return content
	}

	// Get services
	services, ok := compose["services"].(map[string]interface{})
	if !ok {
		return content
	}

	// Apply security defaults to each service
	for _, service := range services {
		svc, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		// Add security_opt if not present
		if _, ok := svc["security_opt"]; !ok {
			svc["security_opt"] = []string{}
		}
		secOpts, _ := svc["security_opt"].([]interface{})

		// Add no-new-privileges unless privileged is needed
		if !needsPrivileged {
			hasNoNewPriv := false
			for _, opt := range secOpts {
				if str, ok := opt.(string); ok && strings.Contains(str, "no-new-privileges") {
					hasNoNewPriv = true
					break
				}
			}
			if !hasNoNewPriv {
				secOpts = append(secOpts, "no-new-privileges:true")
			}
		}

		svc["security_opt"] = secOpts

		// Set read_only if not specified and not privileged
		if !needsPrivileged {
			if _, ok := svc["read_only"]; !ok {
				// Check if the service likely needs write access
				volumes, hasVolumes := svc["volumes"]
				if !hasVolumes || len(volumes.([]interface{})) == 0 {
					svc["read_only"] = true
				}
			}
		}

		// Add restart policy if not present
		if _, ok := svc["restart"]; !ok {
			svc["restart"] = "unless-stopped"
		}

		// Resource limits for templates that set none: the catalog entry's
		// (defaults.resources), else 2 CPUs and 1 GiB. (The catalog's
		// values used to be ignored, capping e.g. Nextcloud at 1 GiB.)
		if _, ok := svc["deploy"]; !ok && !needsPrivileged {
			cpus, mem := "2.0", "1024M"
			if res.CPULimit != "" {
				cpus = res.CPULimit
			}
			if res.MemoryLimit != "" {
				mem = res.MemoryLimit
			}
			svc["deploy"] = map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{"cpus": cpus, "memory": mem},
				},
			}
		}
	}

	// Marshal back to YAML
	result, err := yaml.Marshal(compose)
	if err != nil {
		return content
	}

	return string(result)
}

// RenderEnvFile creates an environment file for the app
func (tr *TemplateRenderer) RenderEnvFile(params map[string]interface{}) ([]byte, error) {
	env := tr.paramsToEnv(params)

	var buffer bytes.Buffer
	for k, v := range env {
		// Escape values with special characters
		if strings.ContainsAny(v, " \t\n\"'\\$") {
			v = fmt.Sprintf("%q", v)
		}
		fmt.Fprintf(&buffer, "%s=%s\n", k, v)
	}

	return buffer.Bytes(), nil
}

// RenderCaddySnippet creates a Caddy configuration snippet for the app
func (tr *TemplateRenderer) RenderCaddySnippet(appID string, ports []PortMapping) ([]byte, error) {
	if len(ports) == 0 {
		return nil, nil // No ports to proxy
	}

	// Use the first HTTP port
	var proxyPort int
	for _, p := range ports {
		if p.Container == 80 || p.Container == 8080 || p.Container == 3000 {
			proxyPort = p.Container
			break
		}
	}

	// If no standard HTTP port, use the first one
	if proxyPort == 0 && len(ports) > 0 {
		proxyPort = ports[0].Container
	}

	if proxyPort == 0 {
		return nil, nil
	}

	// Create Caddy snippet
	tmpl := `# App: {{ .AppID }}
handle_path /apps/{{ .AppID }}/* {
	reverse_proxy nos-app-{{ .AppID }}-app-1:{{ .Port }} {
		flush_interval -1
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}
}
`

	t, err := template.New("caddy").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	err = t.Execute(&buffer, map[string]interface{}{
		"AppID": appID,
		"Port":  proxyPort,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return buffer.Bytes(), nil
}

// Schema returns the install form schema (JSON Schema) of a catalog entry.
// The path comes from the catalog, which may be a remote one, so it must
// stay inside the templates folder.
func (tr *TemplateRenderer) Schema(entry *CatalogEntry) ([]byte, error) {
	if entry.Schema == "" {
		return []byte(`{"type":"object","properties":{}}`), nil
	}
	base := filepath.Clean(tr.templateDir)
	p := filepath.Clean(filepath.Join(base, entry.Schema))
	if !strings.HasPrefix(p, base+string(filepath.Separator)) {
		return nil, fmt.Errorf("schema path outside the templates folder: %s", entry.Schema)
	}
	return os.ReadFile(p)
}
