package auth

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// TestNotifyPlaceLeadsWhereTheReaderMayGo: the portal's notifications page
// (NOTIF-05) is where a reader who may open its route reads; anybody else, and
// everybody while the route is off, reads on the built-in page. The community
// image has no bar, so nothing is ever in it.
func TestNotifyPlaceLeadsWhereTheReaderMayGo(t *testing.T) {
	_, sm, st := mfaSetup(t)
	ctx := context.Background()
	h := New(st, sm)
	if err := st.SaveRoute(ctx, uiRoute("app", 1, true, "readers")); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingPortal, store.PortalConfig{Mode: store.PortalModePortal,
		Entries:    []store.PortalEntry{{Special: store.PortalSpecialNotifications}},
		NotifyPage: store.NotifyPage{RouteID: "app", Path: "/#/inbox"}}); err != nil {
		t.Fatal(err)
	}
	reader := store.Caller{Authenticated: true, Roles: []string{"readers"}}
	if p := h.NotifyPlace(ctx, reader); p.Page != "/app/#/inbox" || p.Route.ID != "app" || p.InBar {
		t.Fatalf("a reader of the route should read on its page, with no bar here: %+v", p)
	}
	if p := h.NotifyPlace(ctx, store.Caller{Authenticated: true}); p.Page != NotificationsPagePath || p.Route.ID != "" {
		t.Fatalf("a reader the route refuses should read on the built-in page: %+v", p)
	}
	off := uiRoute("app", 1, true, "readers")
	off.Enabled = false
	if err := st.SaveRoute(ctx, off); err != nil {
		t.Fatal(err)
	}
	if p := h.NotifyPlace(ctx, reader); p.Page != NotificationsPagePath {
		t.Fatalf("with the route off, the built-in page: %+v", p)
	}
	for _, c := range [][3]string{{"/app", "/inbox", "/app/inbox"}, {"/", "/inbox", "/inbox"}, {"/app", "", "/app/"}, {"/app", "/#/x", "/app/#/x"}} {
		if got := NotifyHref(c[0], c[1]); got != c[2] {
			t.Errorf("NotifyHref(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
