package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
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
func benchUpstream(b testing.TB) *httptest.Server {
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

// An authenticated API call, the way one arrives in production: a personal
// token in the Authorization header, a route that requires a signed-in caller,
// and the caller forwarded to the upstream as an ES256-signed JWT carrying the
// username and the roles.
//
// It is the path the others leave out, and the one that reads the database:
// the token (once per cache window, since the session layer remembers it) and
// the identity the JWT is built from (on every request, for now). On the
// embedded engine a read is a WAL read lock, a few syscalls, which is why this
// benchmark is the one to watch when identity work is touched.
func BenchmarkAuthenticatedToken(b *testing.B) {
	st, err := store.OpenAt(b.TempDir(), dbtest.URL(b))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Enabled: true, PasswordHash: "x"}); err != nil {
		b.Fatal(err)
	}
	const secret = "mk_bench-token-0123456789abcdef"
	sum := sha256.Sum256([]byte(secret))
	if err := st.AddAPIToken(ctx, store.NewToken{ID: "t1", UserID: "u1", Name: "bench",
		TokenHash: hex.EncodeToString(sum[:]), Prefix: secret[:10],
		Plane: store.PlaneData, Scope: store.ScopeFull}); err != nil {
		b.Fatal(err)
	}
	up := benchUpstream(b)
	route := pathRoute("r1", "bench", 1, "/**", up.URL)
	route.Access = store.Access{Level: "auth"}
	route.Identity = &store.IdentityForward{Mechanism: "signed-jwt", Algorithm: "ES256",
		Attributes: []store.IdentityAttr{{Field: "username"}, {Field: "roles"}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		b.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		rt.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status %d", rec.Code)
		}
	}
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

// Counting must not be felt. The gateway is on the path of every request, so
// an observation costs an atomic add on a pointer the compiled route already
// holds - no map, no key built, no allocation. Read against
// BenchmarkSelectionAmong1 above: the two must allocate the same.
func TestCountingAllocatesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("benchmarks")
	}
	res := testing.Benchmark(func(b *testing.B) {
		r := &metrics.Route{ID: "r1", Name: "bench"}
		b.ReportAllocs()
		for b.Loop() {
			r.Observe(200, 3*time.Millisecond)
		}
	})
	if res.AllocsPerOp() != 0 {
		t.Errorf("observing a request allocates %d times; it runs on every request",
			res.AllocsPerOp())
	}
}
