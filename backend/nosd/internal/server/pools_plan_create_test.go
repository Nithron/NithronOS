package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"
)

func TestPlanCreateSingleDisk(t *testing.T) {
	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"name":       "pool1",
		"mountpoint": "/mnt/pool1",
		"devices":    []string{"/dev/sda"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var out struct {
		Plan struct{ Steps []struct{ Command string } }
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	if len(out.Plan.Steps) == 0 {
		t.Fatalf("no steps")
	}
	found := false
	for _, s := range out.Plan.Steps {
		if s.Command != "" && bytes.Contains([]byte(s.Command), []byte("mkfs.btrfs")) {
			if !containsAll(s.Command, []string{"-d", "single", "-m", "single"}) {
				t.Fatalf("expected single profile in mkfs, got: %s", s.Command)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("mkfs step not found")
	}
}

func TestPlanCreateTwoDisks(t *testing.T) {
	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"name":       "pool2",
		"mountpoint": "/mnt/pool2",
		"devices":    []string{"/dev/sda", "/dev/sdb"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var out struct {
		Plan struct{ Steps []struct{ Command string } }
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	found := false
	for _, s := range out.Plan.Steps {
		if s.Command != "" && bytes.Contains([]byte(s.Command), []byte("mkfs.btrfs")) {
			if !containsAll(s.Command, []string{"-d", "raid1", "-m", "raid1"}) {
				t.Fatalf("expected raid1 profile in mkfs, got: %s", s.Command)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("mkfs step not found")
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !bytes.Contains([]byte(s), []byte(p)) {
			return false
		}
	}
	return true
}

func TestPlanCreateForbiddenRaid(t *testing.T) {
	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"name":       "pool3",
		"mountpoint": "/mnt/pool3",
		"devices":    []string{"/dev/sda", "/dev/sdb"},
		"raidData":   "raid5",
		"raidMeta":   "raid1",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for forbidden raid, got %d: %s", res.Code, res.Body.String())
	}
}

func TestPlanCreateDefaultMountOptionsByDeviceMix(t *testing.T) {
	// Override default opts to simulate SSD vs mixed using device paths present in request
	old := getDefaultMountOpts
	getDefaultMountOpts = func(ctx context.Context, devs []string) string {
		// If any device name contains "nvme" => SSD path
		for _, d := range devs {
			if strings.Contains(d, "nvme") {
				return "compress=zstd:3,ssd,discard=async,noatime"
			}
		}
		return "compress=zstd:3,noatime"
	}
	defer func() { getDefaultMountOpts = old }()

	r := NewRouter(config.FromEnv())
	// Case 1: SSD-only
	body1 := map[string]any{
		"name": "p1", "mountpoint": "/mnt/p1", "devices": []string{"/dev/nvme0n1"}, "mountOptions": "",
	}
	b1, _ := json.Marshal(body1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b1))
	res1 := httptest.NewRecorder()
	r.ServeHTTP(res1, req1)
	if res1.Code != http.StatusOK {
		t.Fatalf("code: %d %s", res1.Code, res1.Body.String())
	}
	if !bytes.Contains(res1.Body.Bytes(), []byte("ssd,discard=async")) {
		t.Fatalf("expected ssd defaults in response")
	}

	// Case 2: mixed
	body2 := map[string]any{
		"name": "p2", "mountpoint": "/mnt/p2", "devices": []string{"/dev/sda"}, "mountOptions": "",
	}
	b2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b2))
	res2 := httptest.NewRecorder()
	r.ServeHTTP(res2, req2)
	if res2.Code != http.StatusOK {
		t.Fatalf("code: %d %s", res2.Code, res2.Body.String())
	}
	if bytes.Contains(res2.Body.Bytes(), []byte("ssd,discard=async")) {
		t.Fatalf("should not include ssd/discard async for mixed")
	}
}

func TestPlanCreateWarnsWithoutForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// Provide fake wipefs that prints a signature to stdout so handler finds warnings
	dir := t.TempDir()
	fake := filepath.Join(dir, "wipefs")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho signature: ext4\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"name":       "pool4",
		"mountpoint": "/mnt/pool4",
		"devices":    []string{"/dev/sda"},
		"force":      false,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 when signatures present without force, got %d: %s", res.Code, res.Body.String())
	}
}

func TestPlanCreateForceAllowsWhenSignatures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "wipefs")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho signature: ext4\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"name":       "pool5",
		"mountpoint": "/mnt/pool5",
		"devices":    []string{"/dev/sda"},
		"force":      true,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/plan-create", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 with force=true, got %d: %s", res.Code, res.Body.String())
	}
}
