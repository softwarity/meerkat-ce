package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A spec deposited in YAML on a route (SVC-06) answers on the route's own
// prefix, as JSON, and the upstream never sees the request. Everything else on
// the route still goes upstream: the file decorates the route, it does not
// become a second target.
//
// The YAML matters more than it looks: response codes are written unquoted, so
// YAML reads them as integers, and a conversion that hands those keys to a JSON
// encoder as they are fails. The spec requires strings there.
func TestADepositedSpecAnswersOnTheRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "up:"+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	const yamlSpec = `openapi: 3.0.0
info:
  title: Orders
  version: "1"
paths:
  /orders:
    get:
      responses:
        200:
          description: ok
`
	route := pathRoute("r1", "orders", 1, "/api/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.API = &store.RouteAPI{Spec: &store.RouteSpec{
		Type: store.SpecFile, Path: "openapi.json", Filename: "orders.yaml",
	}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRouteSpec(ctx, "r1", []byte(yamlSpec)); err != nil {
		t.Fatal(err)
	}

	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("spec: %d %s", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want json: what is served is JSON whatever was deposited", ct)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("the deposited YAML did not come back as JSON: %v\n%s", err, body)
	}
	if !strings.Contains(string(body), `"200"`) {
		t.Errorf("the response code lost its string form:\n%s", body)
	}
	// Rewritten to the route's own prefix, so what the document describes is
	// reachable where the document sits.
	servers, _ := doc["servers"].([]any)
	if len(servers) == 0 {
		t.Fatalf("no servers in the served spec:\n%s", body)
	}
	if first, _ := servers[0].(map[string]any); first["url"] != "/api" {
		t.Errorf("servers[0].url = %v, want /api", first["url"])
	}

	// The upstream is untouched on that path, and reached on every other.
	res, err = http.Get(srv.URL + "/api/orders")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if got := string(body); got != "up:/orders" {
		t.Errorf("other paths must still be proxied, got %q", got)
	}
}

// The file inherits the ROUTE's access rule. A route nobody may call does not
// hand out its contract either - and the rule is applied even when
// per-operation policies exist, since those move the route's own rule inside
// the endpoint guard, which the spec deliberately sits outside of.
func TestADepositedSpecObeysTheRouteRule(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	const spec = `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{"/orders":{"get":{}}}}`
	route := pathRoute("r1", "orders", 1, "/api/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.Access = store.Access{Level: store.AccessAuth}
	route.API = &store.RouteAPI{
		Spec: &store.RouteSpec{Type: store.SpecFile, Path: "openapi.json", Filename: "o.json"},
		// Per-operation overrides: they take the route's rule inside the
		// guard, and the spec must not fall through that hole.
		Security: &store.EndpointSecurity{Endpoints: []store.EndpointPolicy{
			{Method: "GET", Path: "/orders", Access: store.Access{}},
		}},
	}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRouteSpec(ctx, "r1", []byte(spec)); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatalf("the spec answered %d to an anonymous caller on a route that requires a session", res.StatusCode)
	}
	// The reopened operation still works: the guard is untouched.
	res, err = http.Get(srv.URL + "/api/orders")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("reopened operation = %d, want 200", res.StatusCode)
	}
}
