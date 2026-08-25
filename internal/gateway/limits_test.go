package gateway

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
)

// countingUpstream reports how many bytes of body it actually received, and
// whether it was called at all. The second fact is the one that matters for a
// refusal: a limit that lets the request reach the service has cost the very
// transfer it was posed to prevent.
// Atomic, because the two facts are written by the upstream's goroutine and
// read by the test's. A refused body is cut mid-transfer, so the handler is
// often still copying when the client already has its 413 - the reader and the
// writer genuinely overlap, and the race detector said so on a CI machine
// slow enough to lose the coin toss.
func countingUpstream(t *testing.T, called *atomic.Bool, got *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		n, _ := io.Copy(io.Discard, r.Body)
		got.Store(n)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func limitedRouter(t *testing.T, upstream string, gates ...routing.Spec) *httptest.Server {
	t.Helper()
	route := pathRoute("r-lim", "limited", 1, "/**", upstream)
	route.Filters = gates
	rt := newRouter(t, route)
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	return srv
}

// The two gates, written the way an admin writes them.
func body(size string) routing.Spec {
	return routing.Spec{Type: "max-request-body", Args: map[string]any{"size": size}}
}

func headers(size string) routing.Spec {
	return routing.Spec{Type: "max-request-headers", Args: map[string]any{"size": size}}
}

func TestBodyLimitRefusesADeclaredOversizeBody(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	up := countingUpstream(t, &called, &got)
	srv := limitedRouter(t, up.URL, body("1KB"))

	res, err := http.Post(srv.URL+"/upload", "application/octet-stream",
		strings.NewReader(strings.Repeat("x", 4<<10)))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", res.StatusCode)
	}
	if called.Load() {
		t.Fatal("the upstream was called: the refusal happened too late to save the transfer")
	}
	// The arithmetic is the message: a bare "too large" is what costs an
	// afternoon, because the cap lives where the caller cannot see it.
	for _, want := range []string{"4 KB", "1 KB"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("refusal %q does not name %s", body, want)
		}
	}
}

func TestBodyLimitRefusesAnUndeclaredOversizeBody(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	up := countingUpstream(t, &called, &got)
	srv := limitedRouter(t, up.URL, body("1KB"))

	// No declared length: the client streams it (chunked), so the cap can only
	// be found on the way through - and the answer must still be 413 and not
	// the 502 an upstream failure would produce.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/upload",
		io.NopCloser(strings.NewReader(strings.Repeat("x", 8<<10))))
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = -1
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d (%s), want 413", res.StatusCode, body)
	}
	if got.Load() > 1<<10 {
		t.Fatalf("the upstream read %d bytes, more than the %d it was capped at", got.Load(), 1<<10)
	}
	// Proves the answer came from the proxy's error path recognising the cap,
	// not from the declared-length shortcut: there was no length to report,
	// so the message names the limit alone.
	if strings.Contains(string(body), "received") {
		t.Fatalf("refusal %q reports a length the client never declared", body)
	}
	if !strings.Contains(string(body), "1 KB") {
		t.Fatalf("refusal %q does not name the limit", body)
	}
}

func TestBodyUnderTheLimitGoesThrough(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	up := countingUpstream(t, &called, &got)
	srv := limitedRouter(t, up.URL, body("1KB"))

	res, err := http.Post(srv.URL+"/upload", "text/plain", strings.NewReader(strings.Repeat("x", 900)))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d, want 204", res.StatusCode)
	}
	if got.Load() != 900 {
		t.Fatalf("the upstream received %d bytes, want 900", got.Load())
	}
}

func TestHeaderLimitRefusesWith431(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	up := countingUpstream(t, &called, &got)
	srv := limitedRouter(t, up.URL, headers("1KB"))

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Roles", strings.Repeat("ROLE_SOMETHING,", 200))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	// 431 and not 413: the two halves of a request are shrunk by different
	// people, and only this status says which half is the problem.
	if res.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status %d, want 431", res.StatusCode)
	}
	if called.Load() {
		t.Fatal("the upstream was called with headers the route refuses")
	}
}

func TestNoLimitCarriesAnythingTheServiceAccepts(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	up := countingUpstream(t, &called, &got)
	srv := limitedRouter(t, up.URL)

	res, err := http.Post(srv.URL+"/upload", "application/octet-stream",
		strings.NewReader(strings.Repeat("x", 2<<20)))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d, want 204", res.StatusCode)
	}
	if got.Load() != 2<<20 {
		t.Fatalf("the upstream received %d bytes, want %d", got.Load(), 2<<20)
	}
}

// The trap this whole guard has to avoid: a WebSocket is not a request with a
// body, it is a conversation, and its bytes flow through the same reader. A
// cap applied to it would cut the socket the moment a chat window passed the
// limit - the same mistake the response rewriter refuses to make on streams.
func TestBodyLimitLeavesAWebSocketAlone(t *testing.T) {
	up := echoUpgrader(t)
	srv := limitedRouter(t, up.URL, body("16B"))

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	host := strings.TrimPrefix(srv.URL, "http://")
	_, _ = conn.Write([]byte("GET /socket HTTP/1.1\r\nHost: " + host + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n"))

	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status %d, want 101 - the cap turned an upgrade away", res.StatusCode)
	}
	// Far more than the 16-byte cap, which is exactly the point.
	line := strings.Repeat("a", 200) + "\n"
	if _, err := conn.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	echo, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("nothing came back through the socket: %v", err)
	}
	if echo != "echo:"+line {
		t.Fatalf("the socket carried %d bytes back, want the %d that were sent", len(echo), len(line)+5)
	}
}

func TestAnUnusableSizeIsRefusedAtSave(t *testing.T) {
	r := pathRoute("r-neg", "negative", 1, "/**", "http://127.0.0.1:1")
	r.Filters = []routing.Spec{body("0")}
	err := Validate(r)
	if err == nil {
		t.Fatal("a zero limit was accepted: read as a refusal of everything, it takes a route off the air")
	}
	// The error has to say how to lift a limit, because the answer is not a
	// number - it is removing the brick.
	if !strings.Contains(err.Error(), "remove the gate") {
		t.Fatalf("error %q does not say what to do instead", err)
	}
	r.Filters = []routing.Spec{body("two megs")}
	if err := Validate(r); err == nil || !strings.Contains(err.Error(), "2MB") {
		t.Fatalf("a bad size gave %v, want an error naming what is allowed", err)
	}
}

// A gate transforms nothing, so it applies to a route that answers by itself
// exactly as it does to one that proxies - unlike incoming modifiers, which
// CompileFilters drops on a terminal route because there is nothing to modify.
func TestAGateAlsoGuardsARouteThatAnswersItself(t *testing.T) {
	route := pathRoute("r-term", "answering", 1, "/**", "")
	route.Filters = []routing.Spec{
		{Type: "maintenance", Args: map[string]any{}},
		headers("512B"),
	}
	rt := newRouter(t, route)
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Roles", strings.Repeat("ROLE_LONG,", 100))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status %d, want 431 - the gate was dropped with the incoming modifiers", res.StatusCode)
	}
}
