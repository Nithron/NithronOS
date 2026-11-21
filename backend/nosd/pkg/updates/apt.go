package updates

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	aptSourcesPath     = "/etc/apt/sources.list.d/nithronos.list"
	aptPreferencesPath = "/etc/apt/preferences.d/nithronos"
	aptKeyringPath     = "/usr/share/keyrings/nithronos-archive-keyring.gpg"
	channelConfigPath  = "/etc/nithronos/update/channel"
)

// APTManager manages APT repository configuration and operations
type APTManager struct {
	repoURL string
	channel Channel
	keyID   string
}

// NewAPTManager creates a new APT manager
func NewAPTManager(repoURL string, keyID string) *APTManager {
	return &APTManager{
		repoURL: repoURL,
		channel: ChannelStable,
		keyID:   keyID,
	}
}

// GetChannel returns the current update channel
func (am *APTManager) GetChannel() (Channel, error) {
	data, err := os.ReadFile(channelConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Default to stable if file doesn't exist
			return ChannelStable, nil
		}
		return "", fmt.Errorf("failed to read channel config: %w", err)
	}

	channel := strings.TrimSpace(string(data))
	switch channel {
	case "stable", "beta":
		return Channel(channel), nil
	default:
		return ChannelStable, nil
	}
}

// SetChannel sets the update channel
func (am *APTManager) SetChannel(channel Channel) error {
	// Ensure directory exists
	dir := filepath.Dir(channelConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write channel to file
	if err := os.WriteFile(channelConfigPath, []byte(string(channel)), 0644); err != nil {
		return fmt.Errorf("failed to write channel config: %w", err)
	}

	am.channel = channel

	// Update APT sources
	if err := am.UpdateSources(); err != nil {
		return fmt.Errorf("failed to update APT sources: %w", err)
	}

	// Update APT preferences for pinning
	if err := am.UpdatePreferences(); err != nil {
		return fmt.Errorf("failed to update APT preferences: %w", err)
	}

	return nil
}

// UpdateSources updates the APT sources list for the current channel
func (am *APTManager) UpdateSources() error {
	// Generate sources.list content
	content := fmt.Sprintf(`# NithronOS APT Repository
# Channel: %s
# Managed by nos-updater - DO NOT EDIT MANUALLY

deb [signed-by=%s] %s %s main
`, am.channel, aptKeyringPath, am.repoURL, am.channel)

	// Write to sources.list.d
	if err := os.WriteFile(aptSourcesPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write APT sources: %w", err)
	}

	return nil
}

// UpdatePreferences updates APT pinning preferences
func (am *APTManager) UpdatePreferences() error {
	// Generate preferences content to prevent cross-channel drift
	content := fmt.Sprintf(`# NithronOS APT Pinning
# Prevents packages from other channels being installed
# Managed by nos-updater - DO NOT EDIT MANUALLY

Package: *
Pin: origin "%s"
Pin-Priority: 1001

Package: *
Pin: release n=%s
Pin-Priority: 900
`, am.repoURL, am.channel)

	// Write to preferences.d
	if err := os.WriteFile(aptPreferencesPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write APT preferences: %w", err)
	}

	return nil
}

// ImportGPGKey imports the repository GPG key
func (am *APTManager) ImportGPGKey(keyData []byte) error {
	// Write key to keyring location
	if err := os.WriteFile(aptKeyringPath, keyData, 0644); err != nil {
		return fmt.Errorf("failed to write GPG key: %w", err)
	}

	return nil
}

// Update runs apt-get update
func (am *APTManager) Update() error {
	cmd := exec.Command("apt-get", "update")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get update failed: %s: %w", output, err)
	}

	return nil
}

// CheckForUpdates checks for available package updates
func (am *APTManager) CheckForUpdates() ([]Package, error) {
	// Run apt-get update first
	if err := am.Update(); err != nil {
		return nil, fmt.Errorf("failed to update package lists: %w", err)
	}

	// Check for upgradable packages
	cmd := exec.Command("apt", "list", "--upgradable")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list upgradable packages: %w", err)
	}

	// Parse output
	packages := []Package{}
	scanner := bufio.NewScanner(bytes.NewReader(output))

	// Regex to parse apt list output
	// Example: nosd/stable 1.2.0-1 amd64 [upgradable from: 1.1.0-1]
	re := regexp.MustCompile(`^([^/]+)/\S+\s+(\S+)\s+\S+\s+\[upgradable from:\s+(\S+)\]`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			packages = append(packages, Package{
				Name:           matches[1],
				NewVersion:     matches[2],
				CurrentVersion: matches[3],
			})
		}
	}

	return packages, nil
}

// GetPackageVersion gets the installed version of a package
func (am *APTManager) GetPackageVersion(packageName string) (string, error) {
	cmd := exec.Command("dpkg-query", "-W", "-f=${Version}", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package might not be installed
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}

// DistUpgrade performs a distribution upgrade
func (am *APTManager) DistUpgrade() error {
	// Set non-interactive environment
	env := append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
	)

	// Run dist-upgrade with automatic yes and new package installation
	cmd := exec.Command("apt-get",
		"--yes",
		"--allow-downgrades",
		"--allow-remove-essential",
		"--allow-change-held-packages",
		"--with-new-pkgs",
		"dist-upgrade",
	)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dist-upgrade failed: %s: %w", output, err)
	}

	return nil
}

// VerifySignatures verifies package signatures
func (am *APTManager) VerifySignatures() error {
	// Check if the keyring exists
	if _, err := os.Stat(aptKeyringPath); os.IsNotExist(err) {
		return fmt.Errorf("gpg keyring not found at %s", aptKeyringPath)
	}

	// Verify apt-secure is working
	cmd := exec.Command("apt-get", "check")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt security check failed: %s: %w", output, err)
	}

	// Check Release file signature
	cmd = exec.Command("apt-get", "update", "-o", "Debug::Acquire::gpgv=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if signature verification failed
		if strings.Contains(string(output), "NO_PUBKEY") {
			return fmt.Errorf("gpg signature verification failed: missing public key")
		}
		if strings.Contains(string(output), "BADSIG") {
			return fmt.Errorf("gpg signature verification failed: bad signature")
		}
		// Other error
		return fmt.Errorf("failed to verify signatures: %s: %w", output, err)
	}

	return nil
}

// CleanCache cleans the APT package cache
func (am *APTManager) CleanCache() error {
	cmd := exec.Command("apt-get", "clean")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clean cache: %s: %w", output, err)
	}

	return nil
}

// GetCacheSize returns the size of the APT cache in bytes
func (am *APTManager) GetCacheSize() (int64, error) {
	var totalSize int64

	cacheDirs := []string{
		"/var/cache/apt/archives",
		"/var/cache/apt/archives-partial",
	}

	for _, dir := range cacheDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Ignore errors
			}
			if !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}

	return totalSize, nil
}

// SimulateUpgrade simulates an upgrade and returns what would be done
func (am *APTManager) SimulateUpgrade() (string, error) {
	cmd := exec.Command("apt-get", "--simulate", "dist-upgrade")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("simulation failed: %w", err)
	}

	return string(output), nil
}
