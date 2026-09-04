package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// What these measure, and what they do NOT.
//
// The absolute number is a property of the machine, not of this product: the
// same commit reads three times faster on a workstation than on a shared CI
// runner, and quoting one in a datasheet is quoting the box. So nothing here
// is meant to be read alone.
//
// Two things ARE the product's own, and both are read by comparison inside one
// run:
//
//   - The OVERHEAD. BenchmarkBareProxy is the standard library's reverse proxy
//     against the same upstream on the same machine in the same run. Every
//     other benchmark divided by that one says what Meerkat adds - matching,
//     filtering, signing - in a figure the hardware cancels out of.
//   - The ALLOCATIONS. allocs/op is a count, not a duration: it does not move
//     with the clock speed, the load on the runner or the Go version's
//     scheduler. It is the same number on a laptop and in CI, which makes it
//     the one thing here that can gate a merge - see TestPerRequestAllocations.
//
// Run them:  go test ./internal/gateway/ -bench . -benchmem -run '^$'
func benchUpstream(b *testing.B) *httptest.Server {
	b.Helper()
	body := []byte(`{"ok":true}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	b.Cleanup(srv.Close)
	return srv
}

func drive(b *testing.B, h http.Handler, path string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			b.Fatalf("status %d", rec.Code)
		}
	}
}

// The floor: what the standard library costs, with none of this product in the
// way. Everything else is read against it.
func BenchmarkBareProxy(b *testing.B) {
	up := benchUpstream(b)
	target, _ := url.Parse(up.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	drive(b, proxy, "/x")
}

// One route, one path predicate, nothing else: the cheapest thing this gateway
// can be asked to do.
func BenchmarkRouteMinimal(b *testing.B) {
	up := benchUpstream(b)
	rt := newRouter(b, pathRoute("r1", "bench", 1, "/**", up.URL))
	drive(b, rt, "/x")
}

// The same request through a table of fifty, matching the LAST one. Route
// selection is linear and runs on every request, so this is where a table that
// grows with an installation would show up.
func BenchmarkRouteAmong50(b *testing.B) {
	up := benchUpstream(b)
	routes := make([]store.Route, 0, 50)
	for i := range 49 {
		routes = append(routes, pathRoute(
			fmt.Sprintf("r%d", i), fmt.Sprintf("miss%d", i), i,
			fmt.Sprintf("/never-%d/**", i), up.URL))
	}
	routes = append(routes, pathRoute("last", "bench", 99, "/**", up.URL))
	rt := newRouter(b, routes...)
	drive(b, rt, "/x")
}

// A route carrying the filters an ordinary one carries: a prefix stripped, two
// headers added on the way out, one on the way in.
func BenchmarkRouteWithFilters(b *testing.B) {
	up := benchUpstream(b)
	rt := newRouter(b, pathRoute("r1", "bench", 1, "/api/**", up.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
		routing.Spec{Type: "add-request-header", Args: map[string]any{"name": "X-From", "value": "meerkat"}},
		routing.Spec{Type: "add-response-header", Args: map[string]any{"name": "X-Served", "value": "meerkat"}},
		routing.Spec{Type: "security-headers"},
	))
	drive(b, rt, "/api/x")
}

// Route SELECTION, with the socket taken out of the picture.
//
// The proxying benchmarks above are honest about the whole path and useless
// for this one question: a loopback round trip costs ~60us and buries a
// decision that costs a fraction of it. So these route to a terminal filter -
// no upstream, no socket - and the only thing that varies is how many routes
// the request had to be offered to before one took it. Matching is linear and
// runs on EVERY request, so this is the number that follows an installation as
// its table grows. A real one runs 156 routes.
func benchSelection(b *testing.B, n int) {
	routes := make([]store.Route, 0, n)
	for i := range n - 1 {
		routes = append(routes, pathRoute(
			fmt.Sprintf("r%d", i), fmt.Sprintf("miss%d", i), i,
			fmt.Sprintf("/never-%d/**", i), "",
			routing.Spec{Type: "respond", Args: map[string]any{"body": "{}", "status": 200}}))
	}
	routes = append(routes, pathRoute("last", "bench", n, "/**", "",
		routing.Spec{Type: "respond", Args: map[string]any{"body": "{}", "status": 200}}))
	drive(b, newRouter(b, routes...), "/x")
}

func BenchmarkSelectionAmong1(b *testing.B)   { benchSelection(b, 1) }
func BenchmarkSelectionAmong50(b *testing.B)  { benchSelection(b, 50) }
func BenchmarkSelectionAmong200(b *testing.B) { benchSelection(b, 200) }

// A route that answers by itself: no upstream, no socket. It bounds how cheap
// the product can possibly be, which is the other end of the same ruler.
func BenchmarkTerminalRespond(b *testing.B) {
	rt := newRouter(b, pathRoute("r1", "bench", 1, "/**", "",
		routing.Spec{Type: "respond", Args: map[string]any{
			"body":        `{"ok":true}`,
			"contentType": "application/json",
			"status":      200,
		}},
	))
	drive(b, rt, "/x")
}

// The one number here that can gate a merge.
//
// A duration cannot: it moves with the machine, the load on the runner and the
// Go release. An allocation COUNT does not - it is the same on a laptop and on
// a shared runner - so the invariant is stated as a count, and as a shape
// rather than a ceiling: matching a request against two hundred routes must
// allocate exactly what matching it against one allocates.
//
// It did not. pathPattern.match cut the request path into a slice, once per
// route, and a profile of selection put 82% of everything it allocated in
// strings.Split - one wasted slice per route on every request, on a real
// installation running a hundred and fifty of them. Read as a ceiling this
// would have passed for years; read as a shape it fails the moment somebody
// puts a per-route allocation back.
func TestSelectionAllocationsDoNotGrowWithTheTable(t *testing.T) {
	if testing.Short() {
		t.Skip("benchmarks")
	}
	one := testing.Benchmark(func(b *testing.B) { benchSelection(b, 1) })
	many := testing.Benchmark(func(b *testing.B) { benchSelection(b, 200) })
	if one.AllocsPerOp() == 0 {
		t.Fatal("measured nothing")
	}
	if many.AllocsPerOp() != one.AllocsPerOp() {
		t.Errorf("selection allocates %d per request among 200 routes and %d among 1: "+
			"something in the matching path allocates once per route, and an installation "+
			"pays for it on every request",
			many.AllocsPerOp(), one.AllocsPerOp())
	}
}
