package updates

import (
	"os"
	"testing"
)

func TestAPTManager(t *testing.T) {
	am := NewAPTManager("https://apt.nithronos.com", "test-key")

	t.Run("GetChannel", func(t *testing.T) {
		// Default channel should be stable
		channel, err := am.GetChannel()
		if err != nil && !os.IsNotExist(err) {
			t.Errorf("Failed to get channel: %v", err)
		}

		// If file doesn't exist, default to stable
		if channel != ChannelStable && channel != ChannelBeta {
			t.Errorf("Expected stable or beta channel, got %s", channel)
		}
	})

	t.Run("SetChannel", func(t *testing.T) {
		// Skip if not running as root
		if os.Geteuid() != 0 {
			t.Skip("Skipping test that requires root privileges")
		}

		err := am.SetChannel(ChannelBeta)
		if err != nil {
			t.Errorf("Failed to set channel: %v", err)
		}

		channel, _ := am.GetChannel()
		if channel != ChannelBeta {
			t.Errorf("Expected beta channel after setting, got %s", channel)
		}

		// Reset to stable
		_ = am.SetChannel(ChannelStable)
	})

	t.Run("GetPackageVersion", func(t *testing.T) {
		// Test with a package that should exist
		version, err := am.GetPackageVersion("bash")
		if err != nil {
			t.Logf("Could not get bash version: %v", err)
		} else if version == "" {
			t.Log("Bash appears to not be installed")
		} else {
			t.Logf("Bash version: %s", version)
		}

		// Test with non-existent package
		version, err = am.GetPackageVersion("non-existent-package-xyz")
		if err != nil {
			// Expected
			t.Logf("Non-existent package returned error as expected: %v", err)
		}
		if version != "" {
			t.Errorf("Expected empty version for non-existent package, got %s", version)
		}
	})

	t.Run("GetCacheSize", func(t *testing.T) {
		size, err := am.GetCacheSize()
		if err != nil {
			t.Errorf("Failed to get cache size: %v", err)
		}

		// Size should be >= 0
		if size < 0 {
			t.Errorf("Expected non-negative cache size, got %d", size)
		}

		t.Logf("APT cache size: %d bytes", size)
	})
}

func TestAPTSources(t *testing.T) {
	// Skip if not running as root
	if os.Geteuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	am := NewAPTManager("https://apt.nithronos.com", "test-key")

	t.Run("UpdateSources", func(t *testing.T) {
		err := am.UpdateSources()
		if err != nil {
			t.Errorf("Failed to update sources: %v", err)
		}

		// Check if file was created
		if _, err := os.Stat(aptSourcesPath); os.IsNotExist(err) {
			t.Error("Sources file was not created")
		}
	})

	t.Run("UpdatePreferences", func(t *testing.T) {
		err := am.UpdatePreferences()
		if err != nil {
			t.Errorf("Failed to update preferences: %v", err)
		}

		// Check if file was created
		if _, err := os.Stat(aptPreferencesPath); os.IsNotExist(err) {
			t.Error("Preferences file was not created")
		}
	})

	t.Run("ImportGPGKey", func(t *testing.T) {
		// Create test key data
		testKey := []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\nTest key data\n-----END PGP PUBLIC KEY BLOCK-----")

		err := am.ImportGPGKey(testKey)
		if err != nil {
			t.Errorf("Failed to import GPG key: %v", err)
		}

		// Check if file was created
		if _, err := os.Stat(aptKeyringPath); os.IsNotExist(err) {
			t.Error("Keyring file was not created")
		}

		// Clean up
		os.Remove(aptKeyringPath)
	})
}

func TestAPTOperations(t *testing.T) {
	// These tests require a working APT system
	// Skip if not on a Debian-based system
	if _, err := os.Stat("/usr/bin/apt-get"); os.IsNotExist(err) {
		t.Skip("Skipping APT tests on non-Debian system")
	}

	am := NewAPTManager("https://apt.nithronos.com", "test-key")

	t.Run("SimulateUpgrade", func(t *testing.T) {
		output, err := am.SimulateUpgrade()
		if err != nil {
			// May fail if APT is locked or sources are misconfigured
			t.Logf("Simulation failed (may be normal): %v", err)
		} else {
			t.Logf("Simulation output length: %d bytes", len(output))
		}
	})

	t.Run("CheckForUpdates", func(t *testing.T) {
		// Skip if not running as root (apt update requires root)
		if os.Geteuid() != 0 {
			t.Skip("Skipping test that requires root privileges")
		}

		packages, err := am.CheckForUpdates()
		if err != nil {
			t.Logf("Check for updates failed: %v", err)
		} else {
			t.Logf("Found %d upgradable packages", len(packages))
			for _, pkg := range packages {
				t.Logf("  %s: %s -> %s", pkg.Name, pkg.CurrentVersion, pkg.NewVersion)
			}
		}
	})
}
