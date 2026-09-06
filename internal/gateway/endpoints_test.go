package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// find is the counters of one operation, as the console would rank them.
func find(t *testing.T, rt *Router, method, path string) metrics.EndpointSnapshot {
	t.Helper()
	for _, e := range rt.Metrics().Operations(time.Time{}) {
		if e.Method == method && e.Path == path {
			return e
		}
	}
	t.Fatalf("no counters for %s %s", method, path)
	return metrics.EndpointSnapshot{}
}

// A request is attributed to the OPERATION it was, named from what the route
// declares - here the per-endpoint rules, which the router reads while it
// compiles. The route strips one segment, so /demo/delay/3 is the spec's
// /delay/{delay}: the same coordinates the endpoint guard reads paths in.
func TestRequestsAreCountedPerOperation(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status/500" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(upstream.Close)

	route := pathRoute("r1", "demo", 1, "/demo/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.API = &store.RouteAPI{Security: &store.EndpointSecurity{
		Endpoints: []store.EndpointPolicy{
			{Method: "GET", Path: "/delay/{delay}"},
			{Method: "GET", Path: "/status/{code}"},
		},
	}}
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	get := func(path string) {
		t.Helper()
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	get("/demo/delay/3")
	get("/demo/delay/7") // the SAME operation: the template is the unit, not the path
	get("/demo/status/500")

	// Two calls on one template, and not one series per delay - which is the
	// whole reason the unit is the template.
	if got := find(t, rt, "GET", "/delay/{delay}").Requests; got != 2 {
		t.Fatalf("GET /delay/{delay}: got %d requests, want 2", got)
	}
	if got := find(t, rt, "GET", "/status/{code}").Errors; got != 1 {
		t.Fatalf("GET /status/{code}: got %d failures, want 1", got)
	}
}

// A path no template describes is counted on the ROUTE and named nowhere.
// Inventing an operation from the traffic would invent one per order.
func TestUnnamedPathsStayOnTheRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(upstream.Close)

	route := pathRoute("r1", "demo", 1, "/demo/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.API = &store.RouteAPI{Security: &store.EndpointSecurity{
		Endpoints: []store.EndpointPolicy{{Method: "GET", Path: "/get"}},
	}}
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/demo/nothing/declares/this")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	if got := find(t, rt, "GET", "/get").Requests; got != 0 {
		t.Fatalf("an undeclared path landed on /get: %d requests", got)
	}
	var onRoute uint64
	for _, r := range rt.Metrics().Snapshot().Routes {
		if r.ID == "r1" {
			onRoute = r.Total()
		}
	}
	if onRoute != 1 {
		t.Fatalf("route r1: got %d requests, want 1", onRoute)
	}
}

// What the control plane pushed must survive a reload.
//
// This is the reason the index lives on the router rather than on the compiled
// route: routes are recompiled on every configuration change, and the spec is
// resolved on a schedule of its own. Blanking the endpoints every time
// somebody saves a route would leave the screen empty for up to ten minutes,
// with no way to tell that from a route nobody calls.
func TestPushedOperationsSurviveAReload(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(upstream.Close)

	route := pathRoute("r1", "demo", 1, "/demo/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	rt.SetOperations("r1", []Operation{{Method: "GET", Path: "/anything/{name}"}})
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	res, err := http.Get(srv.URL + "/demo/anything/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	if got := find(t, rt, "GET", "/anything/{name}").Requests; got != 1 {
		t.Fatalf("GET /anything/{name}: got %d requests after a reload, want 1", got)
	}
}

// A route that is gone takes its slot with it: the map is keyed by route id
// and would otherwise grow for the life of the process.
func TestDeletedRoutesDropTheirOperations(t *testing.T) {
	route := pathRoute("r1", "demo", 1, "/demo/**", "http://127.0.0.1:1")
	rt := newRouter(t, route)
	rt.SetOperations("r1", []Operation{{Method: "GET", Path: "/get"}})
	if _, err := rt.st.DeleteRoute(context.Background(), "r1"); err != nil {
		t.Fatalf("DeleteRoute: %v", err)
	}
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	rt.opsMu.Lock()
	n := len(rt.opsSlots)
	rt.opsMu.Unlock()
	if n != 0 {
		t.Fatalf("got %d operation slots after the route was deleted, want 0", n)
	}
}
