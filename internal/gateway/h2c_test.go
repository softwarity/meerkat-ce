package gateway

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// unencrypted serves h with cleartext HTTP/2, the way the standard library
// does it since Go 1.24 - no x/net/http2 anywhere, in the tests either.
func unencrypted(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.Config.Protocols = &http.Protocols{}
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// An upstream that speaks HTTP/2 over cleartext and nothing else - which is
// what a gRPC service inside a cluster is. No gRPC library: what is being
// tested is the GATEWAY, and the wire facts that matter here (the protocol,
// the path, the headers, the trailer) are HTTP/2's, not gRPC's. A dependency
// would add its own opinion about framing and prove less.
//
// It answers like a gRPC service does: 200 with the content type, the outcome
// in a TRAILER, never in the status.
func grpcish(t *testing.T, status string) *httptest.Server {
	t.Helper()
	return unencrypted(t, grpcishHandler(status))
}

// The same answers from an ORDINARY server, which speaks HTTP/1.1. A server
// configured for unencrypted HTTP/2 accepts nothing else, so the contrast this
// suite needs cannot come from the same one.
func plainUpstream(t *testing.T, status string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(grpcishHandler(status))
	t.Cleanup(srv.Close)
	return srv
}

func grpcishHandler(status string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc")
		// Announced, so an HTTP/1.1 client would see them too - the point here
		// is that they SURVIVE the proxy, not how they are framed.
		w.Header().Set("Trailer", "grpc-status, grpc-message")
		w.Header().Set("X-Upstream-Proto", r.Proto)
		w.Header().Set("X-Upstream-Path", r.URL.Path)
		w.Header().Set("X-Upstream-Meta", r.Header.Get("X-Caller"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("body"))
		w.Header().Set("grpc-status", status)
		w.Header().Set("grpc-message", "as the upstream said")
	})
}

// h2cClient speaks prior-knowledge HTTP/2 to the gateway, the way a gRPC
// client does. The gateway's own listener has to accept it for any of this to
// be reachable at all.
func h2cClient() *http.Client {
	var p http.Protocols
	p.SetUnencryptedHTTP2(true)
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
			Protocols:   &p,
		},
		Timeout: 10 * time.Second,
	}
}

// h2cGateway serves a route over a listener that accepts cleartext HTTP/2, so
// a test can drive the whole chain the way a gRPC caller would.
func h2cGateway(t *testing.T, r store.Route) *httptest.Server {
	t.Helper()
	return unencrypted(t, newRouter(t, r))
}

func h2cRoute(id, upstream string) store.Route {
	r := pathRoute(id, id, 1, "/**", upstream)
	r.Upstream = strings.Replace(upstream, "http://", SchemeH2C+"://", 1)
	return r
}

// The enabling fact, and the one ForceAttemptHTTP2 does not give: an upstream
// declared h2c is reached over HTTP/2 WITHOUT TLS. The ordinary transport
// upgrades through ALPN, so a cleartext upstream stays on HTTP/1.1 whatever it
// supports - and a gRPC service refuses HTTP/1.1 outright.
func TestAnH2CUpstreamIsReachedOverHTTP2(t *testing.T) {
	up := grpcish(t, "0")
	gw := h2cGateway(t, h2cRoute("grpc", up.URL))

	res, err := h2cClient().Get(gw.URL + "/pkg.Service/Method")
	if err != nil {
		t.Fatalf("calling through the gateway: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)

	if got := res.Header.Get("X-Upstream-Proto"); got != "HTTP/2.0" {
		t.Fatalf("the upstream was reached over %q, want HTTP/2.0", got)
	}
}

// The contrast, and the reason the scheme exists: the SAME upstream behind an
// http:// route is reached over HTTP/1.1. Without this the test above could
// pass for a reason that has nothing to do with the change.
func TestThePlainSchemeStillMeansHTTP1(t *testing.T) {
	up := plainUpstream(t, "0")
	gw := h2cGateway(t, pathRoute("plain", "plain", 1, "/**", up.URL))

	res, err := h2cClient().Get(gw.URL + "/pkg.Service/Method")
	if err != nil {
		t.Fatalf("calling through the gateway: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)

	if got := res.Header.Get("X-Upstream-Proto"); got != "HTTP/1.1" {
		t.Errorf("a plain http upstream was reached over %q, want HTTP/1.1", got)
	}
}

// A gRPC call puts its OUTCOME in a trailer and its status is always 200. If
// the proxy drops trailers, every call looks successful to the caller and the
// failures are invisible - which is worse than not supporting gRPC at all.
func TestTheTrailerCrossesTheGateway(t *testing.T) {
	for _, status := range []string{"0", "13"} {
		up := grpcish(t, status)
		gw := h2cGateway(t, h2cRoute("grpc", up.URL))

		res, err := h2cClient().Get(gw.URL + "/pkg.Service/Method")
		if err != nil {
			t.Fatalf("calling through the gateway: %v", err)
		}
		// The trailer is only readable once the body is drained: that is what
		// a trailer IS, and a test that reads it earlier tests nothing.
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("grpc-status %s answered HTTP %d: the outcome does not belong in the status", status, res.StatusCode)
		}
		if got := res.Trailer.Get("grpc-status"); got != status {
			t.Fatalf("grpc-status = %q, want %q: the outcome did not cross", got, status)
		}
		if got := res.Trailer.Get("grpc-message"); got != "as the upstream said" {
			t.Errorf("grpc-message = %q: the upstream's own words did not cross", got)
		}
	}
}

// A gRPC path is /package.Service/Method, with dots in a segment. Nothing in
// the router should care - and this says so, rather than leaving it to be
// discovered by a service whose package has a dot in it.
func TestAGRPCPathReachesTheUpstreamUntouched(t *testing.T) {
	up := grpcish(t, "0")
	gw := h2cGateway(t, h2cRoute("grpc", up.URL))

	const path = "/acme.billing.v1.Invoices/Create"
	res, err := h2cClient().Get(gw.URL + path)
	if err != nil {
		t.Fatalf("calling through the gateway: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)

	if got := res.Header.Get("X-Upstream-Path"); got != path {
		t.Errorf("the upstream was asked for %q, want %q", got, path)
	}
}

// gRPC metadata IS HTTP/2 headers, so everything the gateway already does to
// headers applies unchanged - identity forwarding included. Checked with an
// ordinary header filter, because what is being proved is that the chain runs
// at all on this transport.
func TestHeaderFiltersApplyOnH2C(t *testing.T) {
	up := grpcish(t, "0")
	r := h2cRoute("grpc", up.URL)
	r.Filters = append(r.Filters, routing.Spec{
		Type: "set-request-header",
		Args: map[string]any{"name": "X-Caller", "value": "alice"},
	})
	gw := h2cGateway(t, r)

	res, err := h2cClient().Get(gw.URL + "/pkg.Service/Method")
	if err != nil {
		t.Fatalf("calling through the gateway: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)

	if got := res.Header.Get("X-Upstream-Meta"); got != "alice" {
		t.Errorf("the upstream saw X-Caller = %q, want alice: the filter chain did not run", got)
	}
}

// A route's access rule is about who is calling, never about how. An
// unauthenticated gRPC call is refused like any other - stated because a new
// transport is exactly where a guard gets forgotten.
func TestAccessRulesApplyToAGRPCRoute(t *testing.T) {
	up := grpcish(t, "0")
	r := h2cRoute("grpc", up.URL)
	r.Access = store.Access{Level: store.AccessAuth}
	gw := h2cGateway(t, r)

	res, err := h2cClient().Get(gw.URL + "/pkg.Service/Method")
	if err != nil {
		t.Fatalf("calling through the gateway: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusOK {
		t.Fatal("an anonymous call reached a route reserved to signed-in callers")
	}
	if got := res.Header.Get("X-Upstream-Proto"); got != "" {
		t.Error("the refusal still called the upstream")
	}
}

// What is accepted, and what a refusal says. An unknown scheme has to name the
// ones that work: an operator typing grpc:// out of habit needs the answer, not
// a verdict.
func TestTheUpstreamSchemesAreNamed(t *testing.T) {
	base := pathRoute("r", "r", 1, "/**", "http://svc:3000")
	for _, ok := range []string{"http://svc:3000", "https://svc:3000", "h2c://svc:50051"} {
		r := base
		r.Upstream = ok
		if err := Validate(r); err != nil {
			t.Errorf("upstream %q was refused: %v", ok, err)
		}
	}
	r := base
	r.Upstream = "grpc://svc:50051"
	err := Validate(r)
	if err == nil {
		t.Fatal("an unknown scheme was accepted")
	}
	for _, want := range []string{"http", "https", "h2c"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// Two routes with the same bounds share one transport, h2c included: a
// connection pool per route would hold a socket against every upstream for
// every route pointing at it, and on a multiplexed protocol that is worse
// still - HTTP/2 exists to put many streams on one connection.
func TestH2CTransportsArePooled(t *testing.T) {
	a := h2cTransportFor(5*time.Second, 15*time.Second)
	b := h2cTransportFor(5*time.Second, 15*time.Second)
	if a != b {
		t.Error("two routes with the same bounds got two transports")
	}
	if c := h2cTransportFor(1*time.Second, 15*time.Second); c == a {
		t.Error("routes with different bounds shared a transport")
	}
}
