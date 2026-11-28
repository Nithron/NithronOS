//go:build windows

package updates

// freeBytes is not measured off Linux; nOS updates only run on Debian.
var freeBytes = func(string) (uint64, error) { return ^uint64(0), nil }
