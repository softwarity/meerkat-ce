package gateway

import (
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// REMOTE_USER is what every gateway before this one set, so applications read
// it without being told to. The username travels under it whatever else the
// route calls it - and an inbound one is purged, being the header an upstream
// trusts most.
func TestUsernameAlsoTravelsAsRemoteUser(t *testing.T) {
	d := identityData{Username: "alice", Roles: []string{"ROLE_A"}}

	cfg := store.IdentityForward{Mechanism: "headers", Attributes: []store.IdentityAttr{
		{Field: "username", As: "x-user"},
		{Field: "roles", As: "roles"},
	}}
	got := map[string]string{}
	for _, h := range identityHeaderPairs(cfg, d) {
		got[h.Name] = h.Value
	}
	if got[RemoteUserHeader] != "alice" || got[RemoteUserHeaderX] != "alice" {
		t.Errorf("the account did not travel under both names: %v", got)
	}
	if got["x-user"] != "alice" {
		t.Errorf("the route's own name for it was dropped: %v", got)
	}

	// Named REMOTE_USER by hand: once under that name, and the other still
	// added - an application that named one may well read the other.
	cfg.Attributes = []store.IdentityAttr{{Field: "username", As: "remote_user"}}
	n := 0
	for _, h := range identityHeaderPairs(cfg, d) {
		if strings.EqualFold(h.Name, RemoteUserHeader) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("REMOTE_USER emitted %d times", n)
	}

	// Not forwarding the username at all: nothing to claim on its behalf.
	cfg.Attributes = []store.IdentityAttr{{Field: "roles", As: "roles"}}
	for _, h := range identityHeaderPairs(cfg, d) {
		if strings.EqualFold(h.Name, RemoteUserHeader) || strings.EqualFold(h.Name, RemoteUserHeaderX) {
			t.Errorf("the account was claimed while the username was not selected")
		}
	}
}
