//go:build !windows
// +build !windows

package server

import "syscall"

func getDiskUsage(path string) (int, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	if total == 0 {
		return 0, nil
	}
	return int((used * 100) / total), nil
}
