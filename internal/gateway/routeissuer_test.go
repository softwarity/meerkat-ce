package gateway

import (
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// TestARoutesJWTCarriesTheInternalIssuerUnlessTheRouteWritesOne: the route's
// own issuer first; else the internal plane's address, once the runtime named
// it; else the public issuer - what a gateway that cannot tell its internal
// address signed with before.
func TestARoutesJWTCarriesTheInternalIssuerUnlessTheRouteWritesOne(t *testing.T) {
	rt := &Router{}
	rt.issuer.Store("https://gw.example.com")
	id := store.IdentityForward{Mechanism: "signed-jwt"}
	if got := rt.RouteIssuer(id); got != "https://gw.example.com" {
		t.Fatalf("no internal address: %q", got)
	}
	rt.SetInternalIssuer("https://meerkat-internal.apps.svc.cluster.local:7443/")
	if got := rt.RouteIssuer(id); got != "https://meerkat-internal.apps.svc.cluster.local:7443" {
		t.Fatalf("internal: %q", got)
	}
	if got := rt.Issuer(); got != "https://gw.example.com" {
		t.Fatalf("the public issuer moved: %q", got)
	}
	id.Issuer = "https://legacy.example.com"
	if got := rt.RouteIssuer(id); got != "https://legacy.example.com" {
		t.Fatalf("route's own: %q", got)
	}
	if c := identityClaims(id, "orders", rt.RouteIssuer(id), sampleIdentity); c["iss"] != "https://legacy.example.com" {
		t.Fatalf("claims: %v", c["iss"])
	}
}

// TestARouteIssuerIsCheckedWhereItMeansSomething: refused on the headers
// mechanism, which carries no token, and with a space in it.
func TestARouteIssuerIsCheckedWhereItMeansSomething(t *testing.T) {
	route := func(id store.IdentityForward) store.Route {
		return store.Route{ID: "r", Name: "r", Enabled: true, Upstream: "http://app:8080",
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/r/**"}}}},
			Identity:   &id}
	}
	for _, c := range []struct {
		id   store.IdentityForward
		says string
	}{
		{store.IdentityForward{Mechanism: "headers", Issuer: "https://x"}, "takes no issuer"},
		{store.IdentityForward{Mechanism: "signed-jwt", Issuer: "https://x y"}, "no space"},
	} {
		if err := Validate(route(c.id)); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Fatalf("%+v: %v", c.id, err)
		}
	}
	if err := Validate(route(store.IdentityForward{Mechanism: "signed-jwt", Issuer: "https://gw.example.com"})); err != nil {
		t.Fatalf("a valid issuer: %v", err)
	}
}
