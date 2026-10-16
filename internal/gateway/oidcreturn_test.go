package gateway

import (
	"slices"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

func oidcRoute(t *testing.T, name string, preds ...routing.Spec) OIDCRoute {
	t.Helper()
	r := oidcRouteOf(store.Route{Name: name, Predicates: preds,
		Identity: &store.IdentityForward{Mechanism: store.IdentityOIDC, Client: "app"}})
	if r == nil {
		t.Fatal("an oidc route was not read as one")
	}
	return *r
}

func paths(p ...string) routing.Spec {
	return routing.Spec{Type: "path", Args: map[string]any{"patterns": p}}
}

func hosts(h ...string) routing.Spec {
	return routing.Spec{Type: "host", Args: map[string]any{"hosts": h}}
}

func places(returns []OIDCReturn) []string {
	var out []string
	for _, r := range returns {
		out = append(out, r.String())
	}
	return out
}

// The routes say where the application lives; the provider reads it there.
func TestOIDCReturnsAreWhereTheRoutesServe(t *testing.T) {
	const issuer = "https://gw.example.com"
	got := OIDCReturns(issuer, []OIDCRoute{
		oidcRoute(t, "rabbitmq", paths("/rabbitmq/**")),
		oidcRoute(t, "orders", paths("/api/{tenant}/orders/**", "/shop/**")),
		oidcRoute(t, "app", hosts("app.example.com", "*.corp.example.com"), paths("/**")),
		oidcRoute(t, "nothing", hosts("*.elsewhere.io")),
	})
	want := []string{
		"https://gw.example.com/rabbitmq/",
		"https://gw.example.com/api/",
		"https://gw.example.com/shop/",
		"https://app.example.com/",
	}
	if !slices.Equal(places(got), want) {
		t.Fatalf("got %q, want %q", places(got), want)
	}
	if got[0].WholeHost() || !got[3].WholeHost() {
		t.Fatal("only /** covers the whole host")
	}
}

// Every case the provider decides on, from the routes alone.
func TestOIDCReturnAccepts(t *testing.T) {
	returns := OIDCReturns("https://gw.example.com", []OIDCRoute{
		oidcRoute(t, "rabbitmq", paths("/rabbitmq/**")),
		oidcRoute(t, "app", hosts("app.example.com"), paths("/ui/**")),
		oidcRoute(t, "both", paths("/a/**"), paths("/a/b/**", "/c/**")),
	})
	for uri, want := range map[string]bool{
		"https://gw.example.com/rabbitmq/callback":        true,
		"https://gw.example.com/rabbitmq":                 true,
		"https://gw.example.com/rabbitmq/cb?next=%2Fq":    true,
		"https://GW.example.com:443/rabbitmq/cb":          true,
		"https://app.example.com/ui/callback":             true,
		"https://app.example.com/ui/callback?x=1":         true,
		"https://gw.example.com/a/b/cb":                   true,
		"https://gw.example.com/a/cb":                     false, // under one predicate, not both
		"https://gw.example.com/c/cb":                     false,
		"https://gw.example.com/rabbitmqx/cb":             false, // a lookalike prefix
		"https://gw.example.com/grafana/cb":               false, // another route's path, or none
		"https://gw.example.com/ui/callback":              false, // app's path, not on app's host
		"https://app.example.com/rabbitmq/callback":       false, // rabbitmq's path, not on its host
		"https://evil.example.com/rabbitmq/cb":            false,
		"http://gw.example.com/rabbitmq/cb":               false,
		"https://gw.example.com:8443/rabbitmq/cb":         false,
		"https://gw.example.com/rabbitmq/cb#x":            false,
		"https://gw.example.com/rabbitmq/../admin/cb":     false,
		"https://gw.example.com/rabbitmq/%2e%2e/admin/cb": false,
		"https://gw.example.com/rabbitmq/..%2fadmin":      false,
		"https://gw.example.com/rabbitmq\\..\\admin":      false,
		"https://someone@gw.example.com/rabbitmq/cb":      false,
		"/rabbitmq/cb": false,
		"javascript://gw.example.com/rabbitmq/%0aalert(1)": false,
	} {
		if got := OIDCReturnAccepts(returns, uri); got != want {
			t.Errorf("%s: got %v, want %v", uri, got, want)
		}
	}
}

// Without the issuer as an address, a route that names no host has no origin
// to accept; one that names its host still has.
func TestOIDCReturnsWithoutAPublicAddress(t *testing.T) {
	got := OIDCReturns("", []OIDCRoute{
		oidcRoute(t, "rabbitmq", paths("/rabbitmq/**")),
		oidcRoute(t, "app", hosts("app.example.com"), paths("/x/**")),
	})
	if want := []string{"https://app.example.com/x/"}; !slices.Equal(places(got), want) {
		t.Fatalf("got %q, want %q", places(got), want)
	}
	if OIDCReturnAccepts(nil, "https://gw.example.com/") {
		t.Fatal("no place accepts nothing")
	}
}

// A developer's gateway on a loopback address: its scheme and port carry to
// the names the routes give.
func TestOIDCReturnsOnALoopbackIssuer(t *testing.T) {
	returns := OIDCReturns("http://localhost:18080", []OIDCRoute{
		oidcRoute(t, "dev", paths("/dev/**")),
		oidcRoute(t, "named", hosts("127.0.0.1"), paths("/named/**")),
	})
	if want := []string{"http://localhost:18080/dev/", "http://127.0.0.1:18080/named/"}; !slices.Equal(places(returns), want) {
		t.Fatalf("got %q, want %q", places(returns), want)
	}
	for uri, want := range map[string]bool{
		"http://localhost:18080/dev/cb":   true,
		"http://127.0.0.1:18080/named/cb": true,
		"https://127.0.0.1/named/cb":      true,
		"http://localhost:18081/dev/cb":   false,
		"http://127.0.0.1:18080/dev/cb":   false,
		"http://localhost:18080/named/cb": false,
	} {
		if got := OIDCReturnAccepts(returns, uri); got != want {
			t.Errorf("%s: got %v, want %v", uri, got, want)
		}
	}
	if got := OIDCReturnOrigins(returns); !slices.Contains(got, "http://localhost:18080") || !slices.Contains(got, "http://127.0.0.1:18080") {
		t.Fatalf("origins: %q", got)
	}
}
