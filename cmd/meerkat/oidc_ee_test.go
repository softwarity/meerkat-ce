//go:build ee

package main

import (
	"testing"

	"github.com/softwarity/meerkat/internal/oidcprovider"
)

// The Enterprise binary carries the OpenID provider: its link file wires it,
// and a link file is one import nobody calls.
func TestThisBinaryCarriesTheOpenIDProvider(t *testing.T) {
	if !oidcprovider.Linked() {
		t.Fatal("no OpenID provider registered: the link file no longer wires one")
	}
}
