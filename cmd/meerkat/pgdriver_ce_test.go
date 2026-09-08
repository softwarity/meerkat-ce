//go:build !ee

package main

import (
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// This image runs on the embedded database. Asserted on the BINARY because
// that is where the linking is decided, and a link file is exactly the kind of
// line that changes by accident.
func TestThisBinaryRunsOnTheEmbeddedDatabase(t *testing.T) {
	if store.ExternalAvailable() {
		t.Fatal("a driver for an external database was linked in")
	}
}
