package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestDeviceTokenTypes(t *testing.T) {
	tests := []struct {
		name     string
		dt       DeviceType
		expected bool
	}{
		{"windows", DeviceTypeWindows, true},
		{"linux", DeviceTypeLinux, true},
		{"macos", DeviceTypeMacOS, true},
		{"android", DeviceTypeAndroid, true},
		{"ios", DeviceTypeIOS, true},
		{"unknown", DeviceTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidDeviceType(tt.dt)
			if result != tt.expected {
				t.Errorf("isValidDeviceType(%v) = %v, want %v", tt.dt, result, tt.expected)
			}
		})
	}
}

func TestDeviceTokenToPublic(t *testing.T) {
	now := time.Now()
	device := &DeviceToken{
		ID:            "test-id",
		UserID:        "user-123",
		DeviceName:    "My Laptop",
		DeviceType:    DeviceTypeWindows,
		OSVersion:     "Windows 11",
		ClientVersion: "1.0.0",
		TokenHash:     "secret-hash-should-not-be-exposed",
		RefreshHash:   "refresh-hash-should-not-be-exposed",
		CreatedAt:     now,
		LastSyncAt:    &now,
		LastSeenAt:    &now,
		LastIP:        "192.168.1.100",
		Scopes:        []string{"sync.read", "sync.write"},
		SyncCount:     42,
		BytesSynced:   1024 * 1024 * 100, // 100 MB
	}

	pub := device.ToPublic()

	if pub.ID != device.ID {
		t.Errorf("ID mismatch: got %v, want %v", pub.ID, device.ID)
	}
	if pub.DeviceName != device.DeviceName {
		t.Errorf("DeviceName mismatch: got %v, want %v", pub.DeviceName, device.DeviceName)
	}
	if pub.DeviceType != device.DeviceType {
		t.Errorf("DeviceType mismatch: got %v, want %v", pub.DeviceType, device.DeviceType)
	}
	if pub.SyncCount != device.SyncCount {
		t.Errorf("SyncCount mismatch: got %v, want %v", pub.SyncCount, device.SyncCount)
	}
	if pub.IsRevoked != false {
		t.Errorf("IsRevoked should be false for non-revoked device")
	}
}

func TestDefaultDeviceScopes(t *testing.T) {
	scopes := DefaultDeviceScopes()
	if len(scopes) != 2 {
		t.Errorf("Expected 2 default scopes, got %d", len(scopes))
	}
	
	hasRead := false
	hasWrite := false
	for _, s := range scopes {
		if s == string(ScopeSyncRead) {
			hasRead = true
		}
		if s == string(ScopeSyncWrite) {
			hasWrite = true
		}
	}
	
	if !hasRead {
		t.Error("Default scopes should include sync.read")
	}
	if !hasWrite {
		t.Error("Default scopes should include sync.write")
	}
}

func TestCursorEncodeDecode(t *testing.T) {
	original := &Cursor{
		Timestamp:   time.Now(),
		FileHashes:  map[string]string{"file1.txt": "abc123", "file2.txt": "def456"},
		LastScanned: time.Now(),
	}

	encoded := EncodeCursor(original)
	if encoded == "" {
		t.Fatal("EncodeCursor returned empty string")
	}

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if len(decoded.FileHashes) != len(original.FileHashes) {
		t.Errorf("FileHashes length mismatch: got %d, want %d", len(decoded.FileHashes), len(original.FileHashes))
	}

	for path, hash := range original.FileHashes {
		if decoded.FileHashes[path] != hash {
			t.Errorf("FileHash mismatch for %s: got %v, want %v", path, decoded.FileHashes[path], hash)
		}
	}
}

func TestCursorDecodeEmpty(t *testing.T) {
	cursor, err := DecodeCursor("")
	if err != nil {
		t.Errorf("DecodeCursor('') should not return error: %v", err)
	}
	if cursor != nil {
		t.Error("DecodeCursor('') should return nil cursor")
	}
}

func TestCursorDecodeInvalid(t *testing.T) {
	_, err := DecodeCursor("invalid-base64!")
	if err == nil {
		t.Error("DecodeCursor should return error for invalid input")
	}
}

func TestDeltaSyncBlockSize(t *testing.T) {
	// Test default block size
	ds := NewDeltaSync(0)
	if ds.blockSize != DefaultBlockSize {
		t.Errorf("Expected default block size %d, got %d", DefaultBlockSize, ds.blockSize)
	}

	// Test minimum block size enforcement
	ds = NewDeltaSync(1024) // Less than MinBlockSize
	if ds.blockSize != MinBlockSize {
		t.Errorf("Expected minimum block size %d, got %d", MinBlockSize, ds.blockSize)
	}

	// Test maximum block size enforcement
	ds = NewDeltaSync(100 * 1024 * 1024) // More than MaxBlockSize
	if ds.blockSize != MaxBlockSize {
		t.Errorf("Expected maximum block size %d, got %d", MaxBlockSize, ds.blockSize)
	}

	// Test valid custom block size
	ds = NewDeltaSync(1024 * 1024) // 1MB
	if ds.blockSize != 1024*1024 {
		t.Errorf("Expected block size 1048576, got %d", ds.blockSize)
	}
}

func TestChangeTrackerExcludePatterns(t *testing.T) {
	logger := zerolog.Nop()
	ct := NewChangeTracker(logger, DefaultChangeTrackerConfig())

	tests := []struct {
		name     string
		filename string
		relPath  string
		excluded bool
	}{
		{"tmp file", "file.tmp", "file.tmp", true},
		{"temp file", "file.temp", "file.temp", true},
		{"DS_Store", ".DS_Store", ".DS_Store", true},
		{"Thumbs.db", "Thumbs.db", "Thumbs.db", true},
		{"git dir", ".git", ".git", true},
		{"swap file", "file.swp", "file.swp", true},
		{"normal file", "document.pdf", "document.pdf", false},
		{"nested normal", "report.docx", "folder/report.docx", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ct.shouldExclude(tt.filename, tt.relPath)
			if result != tt.excluded {
				t.Errorf("shouldExclude(%s, %s) = %v, want %v", tt.filename, tt.relPath, result, tt.excluded)
			}
		})
	}
}

func TestStoreCreation(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Verify directories were created
	if _, err := os.Stat(filepath.Join(tmpDir, "state")); os.IsNotExist(err) {
		t.Error("State directory was not created")
	}

	// Test initial state
	if store.DeviceCount() != 0 {
		t.Errorf("Initial device count should be 0, got %d", store.DeviceCount())
	}
}

func TestStoreDeviceCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create device
	device := &DeviceToken{
		ID:         "device-1",
		UserID:     "user-1",
		DeviceName: "Test Device",
		DeviceType: DeviceTypeWindows,
		TokenHash:  "hash123",
		CreatedAt:  time.Now(),
		Scopes:     DefaultDeviceScopes(),
	}

	err = store.SaveDevice(device)
	if err != nil {
		t.Fatalf("Failed to save device: %v", err)
	}

	// Read device
	retrieved, ok := store.GetDevice("device-1")
	if !ok {
		t.Fatal("Failed to retrieve saved device")
	}
	if retrieved.DeviceName != "Test Device" {
		t.Errorf("Device name mismatch: got %s, want Test Device", retrieved.DeviceName)
	}

	// List by user
	devices := store.ListDevicesByUser("user-1")
	if len(devices) != 1 {
		t.Errorf("Expected 1 device for user-1, got %d", len(devices))
	}

	// Count by user
	count := store.DeviceCountByUser("user-1")
	if count != 1 {
		t.Errorf("Expected device count 1, got %d", count)
	}

	// Delete device
	err = store.DeleteDevice("device-1")
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}

	// Verify soft delete
	retrieved, ok = store.GetDevice("device-1")
	if !ok {
		t.Fatal("Device should still exist after soft delete")
	}
	if retrieved.RevokedAt == nil {
		t.Error("RevokedAt should be set after delete")
	}

	// List should not include revoked
	devices = store.ListDevicesByUser("user-1")
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices after revocation, got %d", len(devices))
	}
}

func TestStoreSyncState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	state := &SyncState{
		DeviceID: "device-1",
		ShareID:  "share-1",
		Cursor:   "cursor123",
		LastSync: time.Now(),
		Files: map[string]FileState{
			"file1.txt": {Path: "file1.txt", Hash: "abc123"},
		},
		TotalFiles: 1,
		TotalBytes: 1024,
	}

	err = store.SaveState(state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Retrieve state
	retrieved, ok := store.GetState("device-1", "share-1")
	if !ok {
		t.Fatal("Failed to retrieve saved state")
	}
	if retrieved.Cursor != "cursor123" {
		t.Errorf("Cursor mismatch: got %s, want cursor123", retrieved.Cursor)
	}
	if len(retrieved.Files) != 1 {
		t.Errorf("Files length mismatch: got %d, want 1", len(retrieved.Files))
	}
}

func TestRollingChecksum(t *testing.T) {
	data := []byte("Hello, World!")
	
	checksum := ComputeRollingChecksum(data)
	if checksum == 0 {
		t.Error("Rolling checksum should not be 0")
	}

	// Same data should produce same checksum
	checksum2 := ComputeRollingChecksum(data)
	if checksum != checksum2 {
		t.Error("Rolling checksum should be deterministic")
	}

	// Different data should produce different checksum
	data2 := []byte("Different data")
	checksum3 := ComputeRollingChecksum(data2)
	if checksum == checksum3 {
		t.Error("Different data should produce different checksum")
	}
}

func TestBlockTransferPlan(t *testing.T) {
	ds := NewDeltaSync(1024) // 1KB blocks

	// Create mock block hashes
	localHashes := &BlockHashResponse{
		Path:      "test.txt",
		Size:      3072,
		Hash:      "fullhash123",
		BlockSize: 1024,
		Blocks: []BlockHash{
			{Offset: 0, Size: 1024, Hash: "block0"},
			{Offset: 1024, Size: 1024, Hash: "block1"},
			{Offset: 2048, Size: 1024, Hash: "block2"},
		},
	}

	remoteHashes := &BlockHashResponse{
		Path:      "test.txt",
		Size:      3072,
		Hash:      "fullhash456", // Different full hash
		BlockSize: 1024,
		Blocks: []BlockHash{
			{Offset: 0, Size: 1024, Hash: "block0"},     // Same
			{Offset: 1024, Size: 1024, Hash: "blockX"},  // Different
			{Offset: 2048, Size: 1024, Hash: "block2"},  // Same
		},
	}

	plan := ds.CreateTransferPlan(localHashes, remoteHashes)

	if plan.TotalBlocks != 3 {
		t.Errorf("Expected 3 total blocks, got %d", plan.TotalBlocks)
	}

	// Only block1 should need to be sent (it's in local but not in remote)
	if len(plan.BlocksToSend) != 1 {
		t.Errorf("Expected 1 block to send, got %d", len(plan.BlocksToSend))
	}

	if plan.BlocksToSend[0].Hash != "block1" {
		t.Errorf("Expected block1 to be sent, got %s", plan.BlocksToSend[0].Hash)
	}

	// Savings should be > 0 (we're saving 2/3 blocks)
	if plan.Savings <= 0 {
		t.Errorf("Expected positive savings, got %f", plan.Savings)
	}
}

func TestBlockTransferPlanIdenticalFiles(t *testing.T) {
	ds := NewDeltaSync(1024)

	hashes := &BlockHashResponse{
		Path:      "test.txt",
		Size:      2048,
		Hash:      "samehash",
		BlockSize: 1024,
		Blocks: []BlockHash{
			{Offset: 0, Size: 1024, Hash: "block0"},
			{Offset: 1024, Size: 1024, Hash: "block1"},
		},
	}

	plan := ds.CreateTransferPlan(hashes, hashes)

	if len(plan.BlocksToSend) != 0 {
		t.Errorf("Identical files should not need any blocks transferred, got %d", len(plan.BlocksToSend))
	}

	if plan.BytesToTransfer != 0 {
		t.Errorf("Bytes to transfer should be 0 for identical files, got %d", plan.BytesToTransfer)
	}

	if plan.Savings != 100.0 {
		t.Errorf("Savings should be 100%% for identical files, got %f", plan.Savings)
	}
}

