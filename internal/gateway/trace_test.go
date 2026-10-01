package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/tracing"
)

// The journey's name on the real router, which is the only place it counts
// (OBS-04).
func TestTraceparentReachesTheUpstream(t *testing.T) {
	// The upstream repeats what it was told about the journey.
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
	if err := st.SaveRoute(ctx, pathRoute("r-trace", "trace", 1, "/t/**", upstream.URL)); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	call := func(sent string) string {
		req := httptest.NewRequest("GET", "http://localhost/t/x", nil)
		if sent != "" {
			req.Header.Set(tracing.Header, sent)
		}
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("the call answered %d", rec.Code)
		}
		return rec.Body.String()
	}

	// What the caller brought keeps its journey: that identifier is what
	// stitches their spans to the service's, so changing it would break the
	// one thing it exists for.
	const sent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	got, ok := tracing.Parse(call(sent))
	if !ok {
		t.Fatalf("the upstream was sent something unreadable: %q", call(sent))
	}
	if got.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("the journey was renamed on the way through: %q", got.TraceID)
	}
	if !got.Sampled {
		t.Error("a caller's sampling decision was dropped")
	}

	// Nobody brought one, so the upstream still gets a name to file its own
	// audit under - and the flag says nobody is expected to report it.
	opened, ok := tracing.Parse(call(""))
	if !ok {
		t.Fatalf("no traceparent reached the upstream: %q", call(""))
	}
	if opened.Sampled {
		t.Error("a journey we opened asks the upstream to export it")
	}
	if len(opened.TraceID) != 32 {
		t.Errorf("trace id = %q", opened.TraceID)
	}

	// And two calls are two journeys, not one identifier reused.
	if a, b := call(""), call(""); a == b {
		t.Errorf("two requests shared a journey: %q", a)
	}
}

// The name of the request goes back to whoever made it - in a header on every
// answer, and in the text of the one answer a person actually reads.
//
// The support gesture this exists for: somebody pastes the identifier from
// their screen into a search and lands on the line that explains it. A header
// alone would not do, because a header is invisible on a page.
func TestTheCallerIsToldTheNameOfTheirRequest(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	// Port 1 on the loopback: nothing listens there, and nothing will.
	if err := st.SaveRoute(ctx, pathRoute("r-dead", "dead", 1, "/dead/**", "http://127.0.0.1:1")); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	req := httptest.NewRequest("GET", "http://localhost/dead/x", nil)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("a dead upstream answered %d, want 502", rec.Code)
	}
	id := rec.Header().Get(tracing.HeaderOut)
	if len(id) != 32 {
		t.Fatalf("%s = %q, want a 32-character identifier", tracing.HeaderOut, id)
	}
	if !strings.Contains(rec.Body.String(), id) {
		t.Errorf("the 502 a person reads does not carry the identifier %q:\n%s", id, rec.Body.String())
	}
}

// And the answer carries the caller's OWN identifier when they brought one:
// the name they already know is the name we answer with.
func TestTheAnswerCarriesTheCallersOwnName(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	req := httptest.NewRequest("GET", "http://localhost/nothing-matches", nil)
	req.Header.Set(tracing.Header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)

	if got := rec.Header().Get(tracing.HeaderOut); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("%s = %q, want the caller's own journey", tracing.HeaderOut, got)
	}
}
