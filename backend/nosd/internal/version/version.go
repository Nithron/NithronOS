// Package version holds the NithronOS release version of this nosd build.
package version

// Version is set at build time from the repository's VERSION file:
//
//	go build -ldflags "-X nithronos/backend/nosd/internal/version.Version=$(cat VERSION)"
//
// Builds without the flag (go run, tests) report "dev".
var Version = "dev"
