package gateway

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
)

// What an audited call says it audits is decided at Reload: the service behind
// each route (its upstream's host), the route's own value when it has one, and
// the installation's attribute and template. The tracing package resolves; the
// router is what hands it the routes, and a route left out of the handover
// would audit under the gateway's name.
func TestReloadHandsEachRouteItsAuditValue(t *testing.T) {
	inherits := pathRoute("orders", "Orders REST", 10, "/orders/**", "http://orders-api.shop.svc.cluster.local:8080")
	own := pathRoute("bookings", "Bookings", 20, "/bookings/**", "http://bookings:9000")
	own.AuditStream = "reservations"
	rt := newRouter(t, inherits, own)
	t.Cleanup(func() { tracing.SetStreams(tracing.Streams{}) })

	if got := tracing.AuditStreamKey(); got != tracing.DefaultAuditKey {
		t.Fatalf("attribute with nothing set = %q, want %q", got, tracing.DefaultAuditKey)
	}
	for route, want := range map[string]string{
		"Orders REST": "orders-api",   // the service, not the route's name
		"Bookings":    "reservations", // the route's own value
		"":            "meerkat",      // the gateway's own events
	} {
		if got := tracing.AuditStream(route); got != want {
			t.Errorf("AuditStream(%q) = %q, want %q", route, got, want)
		}
	}

	// The installation's setting, taken at the next Reload (saving it triggers one).
	ctx := context.Background()
	if err := rt.st.SetSetting(ctx, store.SettingTelemetry, store.TelemetryConfig{
		AuditStreamKey: "trail", AuditStream: "{servicename}.prod",
	}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if got := tracing.AuditStreamKey(); got != "trail" {
		t.Errorf("attribute = %q, want trail", got)
	}
	if got := tracing.AuditStream("Orders REST"); got != "orders-api.prod" {
		t.Errorf("inherited value = %q, want orders-api.prod", got)
	}
	if got := tracing.AuditStream("Bookings"); got != "reservations" {
		t.Errorf("the route's own value = %q, want reservations", got)
	}
}
