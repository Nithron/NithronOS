package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// Device tokens used to fail every check: the cache was keyed by the bcrypt
// hash but searched by sha256, the fallback scanned only the user "", and the
// hashes weren't saved, so a restart also lost every device.
func TestDeviceTokensValidateAndSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	dm := NewDeviceManager(store, zerolog.Nop(), DefaultDeviceManagerConfig())
	reg, err := dm.RegisterDevice("u-1", DeviceRegisterRequest{DeviceName: "laptop", DeviceType: DeviceTypeLinux})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := dm.ValidateDeviceToken(reg.DeviceToken, "10.0.0.2", "test"); err != nil || d.UserID != "u-1" {
		t.Fatalf("fresh token: %v", err)
	}

	// The saved file has the hashes, not the tokens.
	raw, _ := os.ReadFile(filepath.Join(dir, "devices.json"))
	if !strings.Contains(string(raw), "token_hash") || strings.Contains(string(raw), reg.DeviceToken) || strings.Contains(string(raw), reg.RefreshToken) {
		t.Fatalf("devices.json: %s", raw)
	}

	// nosd restarts: a new store and manager over the same files.
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	dm2 := NewDeviceManager(store2, zerolog.Nop(), DefaultDeviceManagerConfig())
	if _, err := dm2.ValidateDeviceToken(reg.DeviceToken, "10.0.0.2", "test"); err != nil {
		t.Fatalf("token after restart: %v", err)
	}

	ref, err := dm2.RefreshDeviceToken(reg.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := dm2.ValidateDeviceToken(ref.DeviceToken, "10.0.0.2", "test"); err != nil {
		t.Fatalf("refreshed token: %v", err)
	}
	if _, err := dm2.ValidateDeviceToken(reg.DeviceToken, "10.0.0.2", "test"); err == nil {
		t.Fatal("the old token must stop working after a refresh")
	}
	if _, err := dm2.ValidateDeviceToken("nos_dt_wrong", "10.0.0.2", "test"); err == nil {
		t.Fatal("wrong token accepted")
	}
}
