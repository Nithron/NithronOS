package server

import (
	"context"
	"testing"

	"nos/backend/nosd/internal/disks"
)

func TestComputeDefaultMountOptsFromDisks_AllSSD(t *testing.T) {
	rotaFalse := false
	list := []disks.Disk{{Path: "/dev/nvme0n1", Rota: &rotaFalse}, {Path: "/dev/sda", Rota: &rotaFalse}}
	got := computeDefaultMountOptsFromDisks(list)
	want := "compress=zstd:3,ssd,discard=async,noatime"
	if got != want {
		t.Fatalf("want %s got %s", want, got)
	}
}

func TestComputeDefaultMountOptsFromDisks_Mixed(t *testing.T) {
	rotaTrue := true
	rotaFalse := false
	list := []disks.Disk{{Path: "/dev/sda", Rota: &rotaTrue}, {Path: "/dev/nvme0n1", Rota: &rotaFalse}}
	got := computeDefaultMountOptsFromDisks(list)
	want := "compress=zstd:3,noatime"
	if got != want {
		t.Fatalf("want %s got %s", want, got)
	}
}

func TestComputeDefaultMountOpts_Empty(t *testing.T) {
	if got := computeDefaultMountOpts(context.Background(), nil); got == "" {
		t.Fatalf("should return default not empty")
	}
}
