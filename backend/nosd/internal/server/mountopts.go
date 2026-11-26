package server

import (
	"context"
	"strings"

	"nos/backend/nosd/internal/disks"
)

// computeDefaultMountOptsFromDisks decides defaults based on rotational flag.
// If all devices are non-rotational (ssd), returns "compress=zstd:3,ssd,discard=async,noatime".
// Otherwise returns "compress=zstd:3,noatime".
func computeDefaultMountOptsFromDisks(list []disks.Disk) string {
	allSSD := true
	for _, d := range list {
		if d.Rota != nil && *d.Rota {
			allSSD = false
			break
		}
	}
	if allSSD {
		return "compress=zstd:3,ssd,discard=async,noatime"
	}
	return "compress=zstd:3,noatime"
}

// computeDefaultMountOpts resolves lsblk and filters by device paths.
func computeDefaultMountOpts(ctx context.Context, devicePaths []string) string {
	list, err := disks.Collect(ctx)
	if err != nil || len(devicePaths) == 0 {
		return "compress=zstd:3,noatime"
	}
	sel := []disks.Disk{}
	for _, want := range devicePaths {
		w := strings.TrimSpace(want)
		for _, d := range list {
			if d.Path == w || ("/dev/"+d.KName) == w || ("/dev/"+d.Name) == w {
				sel = append(sel, d)
				break
			}
		}
	}
	if len(sel) == 0 {
		return "compress=zstd:3,noatime"
	}
	return computeDefaultMountOptsFromDisks(sel)
}

// test seam: allow overriding default computation in handlers
var getDefaultMountOpts = computeDefaultMountOpts
