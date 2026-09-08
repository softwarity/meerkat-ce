//go:build ee

package main

import (
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// This binary can open an external database, which is what its link file is
// for. Asserted on the binary because a link file is one import nobody calls,
// and its loss would show up as an installation that cannot reach its
// database.
func TestThisBinaryCanOpenAnExternalDatabase(t *testing.T) {
	if !store.ExternalAvailable() {
		t.Fatal("no driver registered: the link file no longer wires one")
	}
}
