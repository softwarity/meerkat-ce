package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

func TestDeduceTemplate(t *testing.T) {
	cases := []struct{ path, want string }{
		// One marker whatever the shape: /orders/1042 and /orders/<uuid>
		// are the same endpoint served two ways, and two lines would split
		// the answer.
		{"/orders/1042", "/orders/{id}"},
		{"/orders/1042/items/7", "/orders/{id}/items/{id}"},
		{"/get", "/get"},
		{"/", "/"},
		{"", ""},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "/users/{id}"},
		{"/users/550E8400-E29B-41D4-A716-446655440000", "/users/{id}"},
		{"/blobs/5f2b8c1d9e4a7b3c6d8e0f1a2b3c4d5e", "/blobs/{id}"},
		// Not the 8-4-4-4-12 dialect, and it must fold all the same: a strict
		// UUID test let these through, and each one that gets through is a
		// series of its own here AND in whatever scrapes this gateway.
		{"/anything/000001832bb2-4486-305d-7e5e-00007ffe02cb", "/anything/{id}"},
		{"/t/9f86d081-884c-7d65-9a2f-eaa0c55ad015", "/t/{id}"},
		// A dash-only segment names nothing: there has to be a hex digit.
		{"/a/----------------", "/a/----------------"},
		// NOT folded, and knowingly: a ULID is Crockford base32, base64 is its
		// own alphabet, and each rule added catches more identifiers and takes
		// more words for identifiers. These cost budget, and the budget is
		// what they are fenced by - see maxDeduced.
		{"/t/01H8XGJWBWBAQ4RRSKNP2F6A0V", "/t/01H8XGJWBWBAQ4RRSKNP2F6A0V"},
		// Under sixteen, hex is as likely to be a word as a digest.
		{"/status/deadbeef", "/status/deadbeef"},
		{"/facade", "/facade"},
		// The known lies, kept as cases so nobody "fixes" them by accident:
		// they are exactly why these lines are marked deduced.
		{"/files/2024/report", "/files/{id}/report"},
		{"/api/v2/users", "/api/v2/users"},
		// Shapes that must survive untouched.
		{"/a//b", "/a//b"},
		{"/delay/2/", "/delay/{id}/"},
		{"/50x.html", "/50x.html"},
	}
	for _, c := range cases {
		if got := deduceTemplate(c.path); got != c.want {
			t.Errorf("deduceTemplate(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// A path that folds to itself is returned as itself, so the common case of a
// service with no identifiers in its paths costs nothing.
func TestDeduceDoesNotAllocateWhenNothingFolds(t *testing.T) {
	if n := testing.AllocsPerRun(200, func() {
		_ = deduceTemplate("/anything/whatever")
	}); n != 0 {
		t.Errorf("deduceTemplate allocated %v times per run, want 0", n)
	}
}

func deduceRouter(t *testing.T, upstreamURL string) (*Router, *httptest.Server) {
	t.Helper()
	route := pathRoute("r1", "demo", 1, "/demo/**", upstreamURL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	return rt, srv
}

func echoUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(up.Close)
	return up
}

func call(t *testing.T, srv *httptest.Server, path string) {
	t.Helper()
	res, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
}

// A route with no spec and no rules still names its endpoints - by shape. This
// is the whole point: two orders are one line, not two.
func TestARouteWithNothingDeclaredIsNamedByShape(t *testing.T) {
	rt, srv := deduceRouter(t, echoUpstream(t).URL)
	call(t, srv, "/demo/orders/1042")
	call(t, srv, "/demo/orders/1043")
	call(t, srv, "/demo/orders/1044/items")

	ops := rt.Metrics().Operations(time.Time{})
	if len(ops) != 2 {
		t.Fatalf("got %d templates, want 2: %v", len(ops), ops)
	}
	byPath := map[string]uint64{}
	for _, e := range ops {
		if !e.Deduced {
			t.Errorf("%s %s is not marked deduced", e.Method, e.Path)
		}
		byPath[e.Path] = e.Requests
	}
	if byPath["/orders/{id}"] != 2 {
		t.Errorf("/orders/{id}: got %d requests, want 2 (the two orders are one line)", byPath["/orders/{id}"])
	}
	if byPath["/orders/{id}/items"] != 1 {
		t.Errorf("/orders/{id}/items: got %d, want 1", byPath["/orders/{id}/items"])
	}
}

// What somebody DECLARED wins, and never grows a deduced twin beside it. Only
// what the spec leaves out is guessed at.
func TestDeclaredBeatsDeduced(t *testing.T) {
	route := pathRoute("r1", "demo", 1, "/demo/**", echoUpstream(t).URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.API = &store.RouteAPI{Security: &store.EndpointSecurity{
		Endpoints: []store.EndpointPolicy{{Method: "GET", Path: "/orders/{orderId}"}},
	}}
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	call(t, srv, "/demo/orders/1042")
	call(t, srv, "/demo/carts/9")

	declared := find(t, rt, "GET", "/orders/{orderId}")
	if declared.Requests != 1 {
		t.Fatalf("the declared template got %d requests, want 1", declared.Requests)
	}
	if declared.Deduced {
		t.Error("a declared template is marked deduced")
	}
	// And the one the rules do not describe is still named, by shape.
	guessed := find(t, rt, "GET", "/carts/{id}")
	if !guessed.Deduced || guessed.Requests != 1 {
		t.Fatalf("/carts/{id}: deduced=%v, %d requests", guessed.Deduced, guessed.Requests)
	}
	// The declared name is /orders/{orderId}; deduction would have called the
	// same requests /orders/{id}. Finding both means the same traffic is
	// counted on two lines.
	for _, e := range rt.Metrics().Operations(time.Time{}) {
		if e.Path == "/orders/{id}" {
			t.Error("the declared /orders/{orderId} grew a deduced twin /orders/{id}")
		}
	}
}

// A template deduced first and declared later keeps its counters: the spec
// arriving ten minutes after the traffic must not restart the series.
func TestDeducedIsPromotedWithoutLosingItsCount(t *testing.T) {
	rt, srv := deduceRouter(t, echoUpstream(t).URL)
	call(t, srv, "/demo/carts/9")
	if e := find(t, rt, "GET", "/carts/{id}"); !e.Deduced || e.Requests != 1 {
		t.Fatalf("before: deduced=%v, %d requests", e.Deduced, e.Requests)
	}
	rt.SetOperations("r1", []Operation{{Method: "GET", Path: "/carts/{id}"}})
	e := find(t, rt, "GET", "/carts/{id}")
	if e.Deduced {
		t.Error("still marked deduced after the spec declared it")
	}
	if e.Requests != 1 {
		t.Errorf("lost its count on promotion: %d requests, want 1", e.Requests)
	}
}

// The fence. A hostile path space must cost the budget and never a byte more,
// because the alternative is a gateway an attacker can grow without limit.
func TestDeductionIsCapped(t *testing.T) {
	rt, srv := deduceRouter(t, echoUpstream(t).URL)
	for i := 0; i < maxDeduced+50; i++ {
		// Each one folds to a DIFFERENT template - a number would collapse.
		call(t, srv, fmt.Sprintf("/demo/w%d", i))
	}
	ops := rt.Metrics().Operations(time.Time{})
	if len(ops) > maxDeduced+1 { // +1 for the overflow bucket
		t.Fatalf("got %d templates, want at most %d", len(ops), maxDeduced+1)
	}
	var other uint64
	for _, e := range ops {
		if e.Path == otherTemplate {
			other = e.Requests
		}
	}
	if other != 50 {
		t.Errorf("the overflow bucket holds %d requests, want 50", other)
	}
}
