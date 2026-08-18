package gateway

import (
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A framed page is a portal, and a portal carries its own user menu: signing
// out from inside the frame would end a session the portal still shows as
// open. The component decides that itself (window.self !== window.top), so the
// markup is the same for everyone and no cache can hand one context the
// other's page - the route only says whether to override it.
func TestUserButtonStaysOutOfFramesUnlessTold(t *testing.T) {
	route := func(inFrame bool) store.Route {
		return store.Route{
			ID:   "r1",
			IsUI: true,
			UI:   &store.RouteUI{UserButton: store.UserButton{Enabled: true, InFrame: inFrame}},
		}
	}
	if got := userButtonFragment(route(false), nil); strings.Contains(got, "in-frame") {
		t.Errorf("a route that said nothing asked for the button in a frame:\n%s", got)
	}
	got := userButtonFragment(route(true), nil)
	if !strings.Contains(got, "<meerkat-user-button") || !strings.Contains(got, " in-frame") {
		t.Errorf("the override did not reach the markup:\n%s", got)
	}
}
