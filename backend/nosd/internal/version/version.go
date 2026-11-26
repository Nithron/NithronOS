// Package version holds the nOS release version of this nosd build.
package version

// Version is set at build time from the repository's VERSION file:
//
//	go build -ldflags "-X nos/backend/nosd/internal/version.Version=$(cat VERSION)"
//
// Builds without the flag (go run, tests) report "dev".
var Version = "dev"
