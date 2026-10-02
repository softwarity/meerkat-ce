package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/tracing"
)

// collected catches what the gateway emits, in place of the Enterprise
// exporter that is not linked into this binary.
type collected struct {
	mu    sync.Mutex
	spans []tracing.Span
}

func (c *collected) add(s tracing.Span) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spans = append(c.spans, s)
}

func (c *collected) byKind(k tracing.Kind) (tracing.Span, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.spans {
		if s.Kind == k {
			return s, true
		}
	}
	return tracing.Span{}, false
}

// listen makes this binary look like an Enterprise one for the length of a
// test, and puts it back after.
func listen(t *testing.T) *collected {
	t.Helper()
	c := &collected{}
	tracing.RegisterExporter(c.add)
	before := tracing.SampleRate()
	tracing.SetSampleRate(1)
	t.Cleanup(func() {
		tracing.RegisterExporter(nil)
		tracing.SetSampleRate(before)
	})
	return c
}

// The two spans, on the real router, with the nesting that makes the gateway's
// own time readable (OBS-04).
func TestTheGatewayPutsItselfOnTheTrace(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What the service is told about the journey: its parent must be the
		// CLIENT span, not the crossing that contains it.
		_, _ = w.Write([]byte(r.Header.Get(tracing.Header)))
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SaveRoute(ctx, pathRoute("r-span", "orders-api", 1, "/o/**", upstream.URL)); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	c := listen(t)
	req := httptest.NewRequest("GET", "http://localhost/o/x", nil)
	req.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the call answered %d", rec.Code)
	}

	server, ok := c.byKind(tracing.KindServer)
	if !ok {
		t.Fatal("the gateway emitted no server span: it is absent from the trace")
	}
	client, ok := c.byKind(tracing.KindClient)
	if !ok {
		t.Fatal("the gateway emitted no client span: its own time is unreadable")
	}
	// Named as OpenTelemetry names an HTTP client span - its verb - with the
	// host as an attribute, so a backend grouping by name keeps every call out
	// together rather than one row per upstream.
	if client.Name != "GET" {
		t.Errorf("the client span is named %q, want the verb", client.Name)
	}
	hasHost := false
	for _, a := range client.Attrs {
		if a.Key == "server.address" && a.Str != "" {
			hasHost = true
		}
	}
	if !hasHost {
		t.Error("the client span lost its server.address: the host is no longer anywhere")
	}

	// One journey, the caller's, kept from end to end.
	const journey = "4bf92f3577b34da6a3ce929d0e0e4736"
	if server.TraceID != journey || client.TraceID != journey {
		t.Errorf("the journey was renamed: server=%q client=%q", server.TraceID, client.TraceID)
	}
	// The caller's span is the crossing's parent.
	if server.ParentSpanID != "00f067aa0ba902b7" {
		t.Errorf("the crossing's parent = %q, want the caller's span", server.ParentSpanID)
	}
	// And the crossing is the call's parent - which is what makes a tree
	// rather than a flat list.
	if client.ParentSpanID != server.SpanID {
		t.Errorf("the call's parent = %q, want the crossing %q", client.ParentSpanID, server.SpanID)
	}
	// The call is INSIDE the crossing: it starts after and ends before,
	// because a gateway keeps working once the upstream has answered.
	if client.Start.Before(server.Start) {
		t.Error("the call started before the crossing it belongs to")
	}
	if client.End.After(server.End) {
		t.Error("the call outlived the crossing: the server span was closed too early")
	}

	// The service was told the CALL is its parent, not the crossing. Getting
	// this wrong hides the gateway's own time rather than showing it.
	told, ok := tracing.Parse(rec.Body.String())
	if !ok {
		t.Fatalf("the service was sent an unreadable context: %q", rec.Body.String())
	}
	if told.SpanID != client.SpanID {
		t.Errorf("the service's parent = %q, want the client span %q", told.SpanID, client.SpanID)
	}
	if !told.Sampled {
		t.Error("the service was told not to record a journey we are recording")
	}

	// The steps that are not worth a span of their own.
	names := map[string]bool{}
	for _, e := range server.Events {
		names[e.Name] = true
	}
	// This route carries no rule, so there is no door to report opening - but
	// which route answered is worth naming either way.
	if !names["route_chosen"] {
		t.Errorf("the crossing does not name the route that answered: %v", names)
	}
	if got := server.Events[0].Attr; len(got) == 0 || got[0].Str != "orders-api" {
		t.Errorf("route_chosen does not say which route: %v", got)
	}
}

// A route's own switch decides whether it is traced at all - not just whether
// the browser bundle is injected into its pages (OBS-04). An installation
// proxying an admin interface it has no reason to measure turns that route off
// and gets nothing for it: no crossing, no call out, and the caller's own
// context travels on untouched.
func TestARouteThatIsNotTracedEmitsNothing(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get(tracing.Header)))
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	// The one an operator follows, and the one they left out.
	if err := st.SaveRoute(ctx, pathRoute("r-on", "orders-api", 1, "/o/**", upstream.URL)); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	off := pathRoute("r-off", "rabbitmq", 2, "/rabbitmq/**", upstream.URL)
	off.IsUI = true
	no := false
	off.Telemetry = &no
	if err := st.SaveRoute(ctx, off); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	c := listen(t)
	// The caller is recording, which is the hard case: the route's answer is
	// no even when somebody else already said yes.
	const ctxHeader = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	req := httptest.NewRequest("GET", "http://localhost/rabbitmq/queues", nil)
	req.Header.Set(tracing.Header, ctxHeader)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the call answered %d", rec.Code)
	}
	if len(c.spans) != 0 {
		t.Errorf("%d spans emitted for a route that is not traced: %v", len(c.spans), c.spans)
	}
	// What the service was told is what the caller sent: we did not name
	// ourselves as its parent, since we report nothing.
	if got := rec.Body.String(); got != ctxHeader {
		t.Errorf("the upstream was sent %q, want the caller's own context %q", got, ctxHeader)
	}

	// And the route beside it is untouched: this is a per-route decision, not
	// a switch that quietly turned tracing off for the installation.
	req2 := httptest.NewRequest("GET", "http://localhost/o/x", nil)
	req2.Header.Set(tracing.Header, ctxHeader)
	rt.ServeHTTP(httptest.NewRecorder(), req2)
	if _, ok := c.byKind(tracing.KindServer); !ok {
		t.Error("the traced route emitted no server span")
	}
	if _, ok := c.byKind(tracing.KindClient); !ok {
		t.Error("the traced route emitted no client span")
	}
}

// Nobody exporting means nothing built: no note, no clock, no span. This is
// the state of every community installation and of most Enterprise ones at any
// given moment, so it is the path that has to be free.
func TestNothingIsBuiltWhenNobodyListens(t *testing.T) {
	tracing.RegisterExporter(nil)
	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	sc := tracing.FromRequest(req)
	if got := withSpan(req, sc); got != req {
		t.Error("a span note was allocated although nobody is exporting")
	}
	if spanOf(req.Context()) != nil {
		t.Error("spanOf found a note nobody created")
	}
	// And the calls that record into it are safe on a request that has none.
	spanEvent(req.Context(), "route_chosen")
	finishSpan(req, "r", "/x", 200)
}

// A journey the sampler turned down is not recorded, even with an exporter
// listening - otherwise the rate is a decoration.
func TestAnUnsampledJourneyIsNotRecorded(t *testing.T) {
	c := listen(t)
	tracing.SetSampleRate(0)

	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	sc := tracing.FromRequest(req) // opened here, so ours to decide
	if got := withSpan(req, sc); got != req {
		t.Fatal("a span was started for a journey the sampler turned down")
	}
	if len(c.spans) != 0 {
		t.Errorf("%d spans emitted at a zero rate", len(c.spans))
	}
}

// A decision that came from the caller is RESPECTED rather than rolled again:
// a second dice throw is how a trace ends up with a missing parent.
func TestTheCallersDecisionIsRespected(t *testing.T) {
	listen(t)
	tracing.SetSampleRate(0) // ours would say no

	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	req.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	sc := tracing.FromRequest(req)
	if got := withSpan(req, sc); got == req {
		t.Error("a caller who is recording was ignored because OUR rate is zero")
	}

	// And the other way: a caller who is not recording is not overruled.
	req2 := httptest.NewRequest("GET", "http://localhost/x", nil)
	req2.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00")
	tracing.SetSampleRate(1)
	sc2 := tracing.FromRequest(req2)
	if got := withSpan(req2, sc2); got != req2 {
		t.Error("a caller who asked not to be recorded was recorded anyway")
	}
}

// The browser half, injected into a proxied UI page (OBS-04).
func TestTheBundleIsInjectedIntoUIPages(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body><h1>an application</h1></body></html>"))
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	// The one whose pages open the journey, and one that only asked to be
	// traced - the gateway reports on it, its pages carry no script.
	ui := pathRoute("r-ui", "app", 1, "/app/**", upstream.URL)
	ui.IsUI = true
	ui.TelemetryUI = true
	if err := st.SaveRoute(ctx, ui); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	own := pathRoute("r-own", "own-agent", 2, "/own/**", upstream.URL)
	own.IsUI = true
	if err := st.SaveRoute(ctx, own); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}

	rt := New(st, session.NewManager(st))
	page := func(path string) string {
		if err := rt.Reload(ctx); err != nil {
			t.Fatalf("Reload: %v", err)
		}
		req := httptest.NewRequest("GET", "http://localhost"+path, nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	// Nothing is injected while the gateway exports nothing: the bundle posts
	// its spans to a collector, and there is none.
	if body := page("/app/x"); strings.Contains(body, "telemetry.js") {
		t.Errorf("the bundle was injected although the export is off:\n%s", body)
	}

	// On.
	cfg := store.DefaultTelemetry()
	cfg.Enabled, cfg.Endpoint, cfg.Sample = true, "http://collector:4318", 0.25
	if err := st.SetSetting(ctx, store.SettingTelemetry, cfg); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	body := page("/app/x")
	if !strings.Contains(body, `src="/meerkat/telemetry.js"`) {
		t.Fatalf("no bundle in a UI page:\n%s", body)
	}
	// The page is told to post to US, never to the collector: its address and
	// its credential must not reach a browser.
	if !strings.Contains(body, `"endpoint":"/meerkat/telemetry"`) {
		t.Errorf("the page was not pointed at the relay:\n%s", body)
	}
	if strings.Contains(body, "collector:4318") {
		t.Errorf("the collector's address reached a page:\n%s", body)
	}
	if !strings.Contains(body, `"sample":0.25`) {
		t.Errorf("the browser rate did not travel:\n%s", body)
	}
	// The bundle patches fetch, so it has to be the FIRST thing we inject:
	// among deferred scripts, whatever we put before it would make its calls
	// unpatched.
	if i, j := strings.Index(body, "telemetry.js"), strings.Index(body, "meerkat/page.js"); j >= 0 && i > j {
		t.Errorf("the bundle is injected after our own scripts, so their calls are missed")
	}

	// And a route that did not ask for its pages to open the journey gets no
	// script: being traced and starting the trace are two answers.
	if body := page("/own/x"); strings.Contains(body, "telemetry.js") {
		t.Errorf("the bundle was injected into a route that never asked for it:\n%s", body)
	}

	// The relay follows the routes: one route asks, so the path is open.
	if !tracing.BrowserWanted() {
		t.Error("no route was reported as starting its journeys in the page")
	}
}

// With the gateway's detail on, a crossing that hands the caller to its
// upstream says how: the handover, who the caller is, and the queries that
// took - each under the crossing, so the gap between the server span and the
// client span is no longer a single number.
func TestTheGatewayDetailsItsOwnWorkWhenAsked(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(upstream.Close)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	r := pathRoute("r-id", "with-identity", 1, "/id/**", upstream.URL)
	r.Identity = &store.IdentityForward{Mechanism: "headers",
		Attributes: []store.IdentityAttr{{Field: "username"}}}
	if err := st.SaveRoute(ctx, r); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	crossing := func() []tracing.Span {
		c := listen(t)
		req := httptest.NewRequest("GET", "http://localhost/id/x", nil)
		req.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
		rt.ServeHTTP(httptest.NewRecorder(), req)
		c.mu.Lock()
		defer c.mu.Unlock()
		return append([]tracing.Span(nil), c.spans...)
	}

	before := tracing.Detail()
	t.Cleanup(func() { tracing.SetDetail(before) })

	tracing.SetDetail(false)
	if got := crossing(); len(got) != 2 {
		t.Errorf("with the detail off a crossing is two spans, got %d", len(got))
	}

	tracing.SetDetail(true)
	spans := crossing()
	var server tracing.Span
	names := map[string]tracing.Span{}
	for _, s := range spans {
		if s.Kind == tracing.KindServer {
			server = s
		}
		names[s.Name] = s
	}
	step, ok := names["identity forward"]
	if !ok {
		t.Fatalf("no identity step among %d spans", len(spans))
	}
	if step.ParentSpanID != server.SpanID {
		t.Errorf("the identity step is not under the crossing: parent %q, crossing %q", step.ParentSpanID, server.SpanID)
	}
	if step.Kind != tracing.KindInternal {
		t.Errorf("the identity step is of kind %d, want internal", step.Kind)
	}
}

// Who made the call, on the crossing - even through a route with no rule,
// where nothing else needed to know - once the switch is on (OBS-04).
func TestTheServerSpanNamesTheCallerWhenAsked(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(upstream.Close)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := st.SaveRoute(ctx, pathRoute("r-who", "orders-api", 1, "/o/**", upstream.URL)); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	signed := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, signed, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	t.Cleanup(func() { tracing.SetCaller(false) })

	said := func(on bool, tracestate string) map[string]string {
		t.Helper()
		tracing.SetCaller(on)
		c := listen(t)
		req := httptest.NewRequest("GET", "http://localhost/o/x", nil)
		req.AddCookie(signed.Result().Cookies()[0])
		if tracestate != "" {
			req.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			req.Header.Set(tracing.StateHeader, tracestate)
		}
		rt.ServeHTTP(httptest.NewRecorder(), req)
		server, ok := c.byKind(tracing.KindServer)
		if !ok {
			t.Fatal("no server span")
		}
		got := map[string]string{}
		for _, a := range server.Attrs {
			if strings.HasPrefix(a.Key, "user.") {
				got[a.Key] = a.Str
			}
		}
		return got
	}
	if got := said(false, ""); len(got) != 0 {
		t.Errorf("off: %v, want nothing about the person", got)
	}
	if got := said(true, ""); got["user.id"] != "u1" || got["user.name"] != "alice" {
		t.Errorf("on: %v, want the id and the username", got)
	}
	// A journey our bundle opened: the page's spans carry the person, stamped
	// at the relay, so the gateway's span does not say it a second time.
	if got := said(true, "meerkat=b"); len(got) != 0 {
		t.Errorf("a journey from the page: %v, want the gateway's span silent", got)
	}
}

// A request no route answers is not traced: tracing is each route's answer,
// and no route answered.
func TestARequestNoRouteAnswersIsNotTraced(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := listen(t)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/nowhere", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("answered %d", rec.Code)
	}
	if _, ok := c.byKind(tracing.KindServer); ok {
		t.Error("a request no route answered left a span")
	}
}
