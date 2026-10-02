// Package version exposes build metadata injected at link time.
package version

import "os"

// Set via -ldflags at build time; see Makefile.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// An image is built, tested and published BEFORE its version is chosen, and
// it is never rebuilt for it: a release stamps the tested image with
// MEERKAT_VERSION (docker/Dockerfile.stamp) rather than linking a new binary.
// So a binary built as `dev` takes its version from the environment, and a
// binary the linker versioned keeps what it was given.
func init() {
	if Version != "dev" {
		return
	}
	if v := os.Getenv("MEERKAT_VERSION"); v != "" {
		Version = v
	}
}
