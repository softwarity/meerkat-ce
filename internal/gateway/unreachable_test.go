package gateway

import (
	"context"
	"net"
	"syscall"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A route whose service the health watch found down is said down at once, so
// a reader about to fetch its spec answers instead of sitting out a timeout.
func TestADownRouteIsSaidDownAtOnce(t *testing.T) {
	rt := newRouter(t,
		pathRoute("dead", "dead", 1, "/dead/**", "http://10.0.0.9:7000"),
		pathRoute("alive", "alive", 2, "/alive/**", "http://10.0.0.8:7000"),
	)
	fn := &fakeNet{errs: map[string]error{
		"10.0.0.9:7000": &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
	}}
	rt.CheckTargets(context.Background(), TargetCheck{Dial: fn.dial, Proxy: noProxy})

	if why, down := rt.Unreachable("dead"); !down || why == "" {
		t.Errorf("dead route: down=%v why=%q, want down with a reason", down, why)
	}
	if _, down := rt.Unreachable("alive"); down {
		t.Error("a route whose service answers is said down")
	}
	if _, down := rt.Unreachable("nobody"); down {
		t.Error("an unknown route is said down")
	}
}

// Only a spec that lives on the route's own service goes down with it.
func TestASpecGoesDownWithItsServiceOnly(t *testing.T) {
	route := func(spec string) store.Route {
		return store.Route{Upstream: "https://orders:8443",
			API: &store.RouteAPI{Spec: &store.RouteSpec{Type: store.SpecUpstream, Path: spec}}}
	}
	for spec, want := range map[string]bool{
		"/v3/api-docs":                         true,
		"https://orders:8443/v3/api-docs":      true,
		"https://docs.example.com/orders.json": false,
	} {
		if got := SpecOnUpstream(route(spec)); got != want {
			t.Errorf("%s: on upstream %v, want %v", spec, got, want)
		}
	}
	if SpecOnUpstream(store.Route{Upstream: "https://orders:8443"}) {
		t.Error("a route with no spec has one on its upstream")
	}
}
