package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// CatalogManager manages app catalogs (built-in and remote)
type CatalogManager struct {
	builtinPath string
	cachePath   string
	sourcesPath string
	httpClient  *http.Client
	mu          sync.RWMutex
	cache       *Catalog
	sources     []CatalogSource
	lastSync    time.Time
}

// NewCatalogManager creates a new catalog manager
func NewCatalogManager(builtinPath, cachePath, sourcesPath string) *CatalogManager {
	return &CatalogManager{
		builtinPath: builtinPath,
		cachePath:   cachePath,
		sourcesPath: sourcesPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoadBuiltinCatalog loads the built-in catalog from disk
func (cm *CatalogManager) LoadBuiltinCatalog() (*Catalog, error) {
	catalogPath := filepath.Join(cm.builtinPath, "catalog.yaml")

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty catalog if file doesn't exist
			return &Catalog{
				Version:   "1.0",
				Entries:   []CatalogEntry{},
				Source:    "builtin",
				UpdatedAt: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to read builtin catalog: %w", err)
	}

	var catalog Catalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse builtin catalog: %w", err)
	}

	catalog.Source = "builtin"
	catalog.UpdatedAt = time.Now()

	return &catalog, nil
}

// LoadSources loads catalog sources from configuration
func (cm *CatalogManager) LoadSources() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.sources = []CatalogSource{}

	// Check if sources directory exists
	if _, err := os.Stat(cm.sourcesPath); os.IsNotExist(err) {
		return nil // No sources configured
	}

	// Read all .yaml files in sources directory
	entries, err := os.ReadDir(cm.sourcesPath)
	if err != nil {
		return fmt.Errorf("failed to read sources directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !hasYAMLExtension(entry.Name()) {
			continue
		}

		path := filepath.Join(cm.sourcesPath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", entry.Name(), err)
		}

		var source CatalogSource
		if err := yaml.Unmarshal(data, &source); err != nil {
			return fmt.Errorf("failed to parse source file %s: %w", entry.Name(), err)
		}

		if source.Enabled {
			cm.sources = append(cm.sources, source)
		}
	}

	return nil
}

// SyncRemoteCatalogs fetches and verifies remote catalogs
func (cm *CatalogManager) SyncRemoteCatalogs() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	mergedCatalog := &Catalog{
		Version:   "1.0",
		Entries:   []CatalogEntry{},
		UpdatedAt: time.Now(),
	}

	// Start with built-in catalog
	builtin, err := cm.LoadBuiltinCatalog()
	if err != nil {
		return fmt.Errorf("failed to load builtin catalog: %w", err)
	}
	mergedCatalog.Entries = append(mergedCatalog.Entries, builtin.Entries...)

	// Fetch each remote source
	for _, source := range cm.sources {
		if !source.Enabled {
			continue
		}

		catalog, err := cm.fetchRemoteCatalog(source)
		if err != nil {
			// Log error but continue with other sources
			fmt.Fprintf(os.Stderr, "Failed to fetch catalog from %s: %v\n", source.Name, err)
			continue
		}

		// Verify if SHA256 is provided
		if source.SHA256 != "" {
			if err := cm.verifyCatalogHash(catalog, source.SHA256); err != nil {
				fmt.Fprintf(os.Stderr, "Catalog verification failed for %s: %v\n", source.Name, err)
				continue
			}
		}

		// Merge entries (later sources can override earlier ones)
		mergedCatalog.Entries = mergeCatalogEntries(mergedCatalog.Entries, catalog.Entries)
	}

	// Save to cache
	if err := cm.saveCache(mergedCatalog); err != nil {
		return fmt.Errorf("failed to save catalog cache: %w", err)
	}

	cm.cache = mergedCatalog
	cm.lastSync = time.Now()

	return nil
}

// fetchRemoteCatalog fetches a catalog from a remote source
func (cm *CatalogManager) fetchRemoteCatalog(source CatalogSource) (*Catalog, error) {
	switch source.Type {
	case "http", "https":
		return cm.fetchHTTPCatalog(source.URL)
	case "git":
		// Not supported: a remote catalog only adds entries, whose templates
		// must already be installed, so a git checkout would add nothing.
		return nil, fmt.Errorf("git catalog sources are not supported; use an http(s) URL")
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

// fetchHTTPCatalog fetches a catalog via HTTP
func (cm *CatalogManager) fetchHTTPCatalog(url string) (*Catalog, error) {
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var catalog Catalog

	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &catalog); err != nil {
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			return nil, fmt.Errorf("failed to parse catalog: %w", err)
		}
	}

	catalog.UpdatedAt = time.Now()
	return &catalog, nil
}

// verifyCatalogHash verifies the SHA256 hash of a catalog
func (cm *CatalogManager) verifyCatalogHash(catalog *Catalog, expectedHash string) error {
	data, err := json.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %w", err)
	}

	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// saveCache saves the merged catalog to cache file
func (cm *CatalogManager) saveCache(catalog *Catalog) error {
	// Ensure cache directory exists
	cacheDir := filepath.Dir(cm.cachePath)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %w", err)
	}

	// Write atomically
	tmpPath := cm.cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Sync and rename
	if err := os.Rename(tmpPath, cm.cachePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	return nil
}

// LoadCache loads the cached merged catalog
func (cm *CatalogManager) LoadCache() (*Catalog, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.cache != nil && time.Since(cm.lastSync) < 5*time.Minute {
		return cm.cache, nil
	}

	data, err := os.ReadFile(cm.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No cache, load builtin only
			return cm.LoadBuiltinCatalog()
		}
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse cache: %w", err)
	}

	return &catalog, nil
}

// GetCatalog returns the current merged catalog
func (cm *CatalogManager) GetCatalog() (*Catalog, error) {
	// Try cache first
	catalog, err := cm.LoadCache()
	if err == nil {
		return catalog, nil
	}

	// Fall back to builtin
	return cm.LoadBuiltinCatalog()
}

// GetEntry returns a specific catalog entry by ID
func (cm *CatalogManager) GetEntry(id string) (*CatalogEntry, error) {
	catalog, err := cm.GetCatalog()
	if err != nil {
		return nil, err
	}

	for _, entry := range catalog.Entries {
		if entry.ID == id {
			return &entry, nil
		}
	}

	return nil, fmt.Errorf("app not found in catalog: %s", id)
}

// mergeCatalogEntries merges two lists of catalog entries
// Later entries override earlier ones with the same ID
func mergeCatalogEntries(base, additions []CatalogEntry) []CatalogEntry {
	entryMap := make(map[string]CatalogEntry)

	// Add base entries
	for _, entry := range base {
		entryMap[entry.ID] = entry
	}

	// Override/add new entries
	for _, entry := range additions {
		entryMap[entry.ID] = entry
	}

	// Convert back to slice
	result := make([]CatalogEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		result = append(result, entry)
	}

	return result
}

// hasYAMLExtension checks if a file has a YAML extension
func hasYAMLExtension(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}
