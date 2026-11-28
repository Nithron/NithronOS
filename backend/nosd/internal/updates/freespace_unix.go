//go:build !windows

package updates

import "syscall"

// freeBytes is the space available to root on the filesystem holding path.
var freeBytes = func(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bfree * uint64(st.Bsize), nil
}
