//go:build windows
// +build windows

package server

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func getDiskUsage(path string) (int, error) {
	h := windows.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes int64

	pathPtr, _ := windows.UTF16PtrFromString(path)
	_, _, err := c.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if err != nil && err.Error() != "The operation completed successfully." {
		return 0, err
	}

	if totalNumberOfBytes == 0 {
		return 0, nil
	}

	used := totalNumberOfBytes - totalNumberOfFreeBytes
	return int((used * 100) / totalNumberOfBytes), nil
}
