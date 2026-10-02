package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/tracing"
)

// The relay is a door this gateway holds open for anonymous callers - the
// bundle runs on public pages too - so what it does when nobody is listening,
// and what it does under a flood, are the two things that matter.

func telemetryMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.registerTelemetry(mux)
	return mux
}

func post(mux *http.ServeMux, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "http://localhost/meerkat/telemetry", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// Nothing registered means the path does not exist, rather than a door that
// takes a body and bins it.
// openToPages is the routes' half of the relay door: some route asks for its
// pages to start the journeys, which is what the router reports on reload.
func openToPages(t *testing.T) {
	t.Helper()
	before := tracing.BrowserWanted()
	t.Cleanup(func() { tracing.SetBrowserWanted(before) })
	tracing.SetBrowserWanted(true)
}

func TestTheRelayIsClosedWhenNothingIsListening(t *testing.T) {
	tracing.RegisterRelay(nil)
	mux := telemetryMux(&Handler{})

	if got := post(mux, `{"resourceSpans":[]}`).Code; got != http.StatusNotFound {
		t.Errorf("the relay answered %d with nobody listening, want 404", got)
	}
	// And so does the script, in a binary built without `make telemetry`.
	tracing.RegisterBundle(nil)
	req := httptest.NewRequest("GET", "http://localhost/meerkat/telemetry.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("the bundle answered %d when this binary carries none, want 404", rec.Code)
	}
}

// What an ANONYMOUS page posts reaches the collector as it stands: there is
// nobody to stamp, and re-encoding a batch for nothing is a second
// implementation of a wire format whose mistakes are silent.
func TestTheRelayForwardsTheBodyUntouched(t *testing.T) {
	var got atomic.Value
	tracing.RegisterRelay(func(b []byte) error { got.Store(string(b)); return nil })
	tracing.SetRelayMaxPerSecond(0)
	openToPages(t)
	t.Cleanup(func() { tracing.RegisterRelay(nil) })

	const batch = `{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"abc"}]}]}]}`
	if code := post(telemetryMux(&Handler{}), batch).Code; code != http.StatusNoContent {
		t.Fatalf("the relay answered %d, want 204", code)
	}
	if got.Load() != batch {
		t.Errorf("the body was rewritten on the way through:\n%v", got.Load())
	}
}

// A collector that refused is reported as a refusal. A page told "kept" when
// nothing was kept has no way to know its telemetry goes nowhere.
func TestTheRelayDoesNotPretend(t *testing.T) {
	tracing.RegisterRelay(func([]byte) error { return http.ErrHandlerTimeout })
	tracing.SetRelayMaxPerSecond(0)
	openToPages(t)
	t.Cleanup(func() { tracing.RegisterRelay(nil) })

	if code := post(telemetryMux(&Handler{}), `{"resourceSpans":[]}`).Code; code != http.StatusBadGateway {
		t.Errorf("a refused batch answered %d, want 502", code)
	}
}

// The budget is what stops this being an open relay to somebody's paid
// collector. There is no way to tell a page from a flood, so the leverage is
// bounded rather than the caller judged.
func TestTheRelayIsBounded(t *testing.T) {
	var forwarded atomic.Int64
	tracing.RegisterRelay(func([]byte) error { forwarded.Add(1); return nil })
	tracing.SetRelayMaxPerSecond(5)
	openToPages(t)
	t.Cleanup(func() { tracing.RegisterRelay(nil); tracing.SetRelayMaxPerSecond(0) })

	mux := telemetryMux(&Handler{})
	refused := 0
	for i := 0; i < 200; i++ {
		if post(mux, `{"resourceSpans":[]}`).Code != http.StatusNoContent {
			refused++
		}
	}
	if forwarded.Load() > 5 {
		t.Errorf("%d batches forwarded under a budget of 5", forwarded.Load())
	}
	if refused == 0 {
		t.Error("two hundred batches went through a budget of five without one refusal")
	}
}

// A body past the cap is refused rather than read into memory: a thousand tabs
// posting megabytes is the shape of the problem this avoids.
func TestTheRelayRefusesAnOversizeBatch(t *testing.T) {
	tracing.RegisterRelay(func([]byte) error { return nil })
	tracing.SetRelayMaxPerSecond(0)
	openToPages(t)
	t.Cleanup(func() { tracing.RegisterRelay(nil) })

	huge := make([]byte, telemetryBodyLimit+1024)
	for i := range huge {
		huge[i] = 'x'
	}
	if code := post(telemetryMux(&Handler{}), string(huge)).Code; code != http.StatusRequestEntityTooLarge {
		t.Errorf("an oversize batch answered %d, want 413", code)
	}
}

// The name of the request, at the foot of a served page (OBS-04).
//
// The support gesture this exists for is somebody reading it off their screen
// and pasting it into a search. The response header carries it too, and a
// header is invisible to a person looking at a page.
func TestAServedPageCarriesTheNameOfTheRequest(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)

	const journey = "4bf92f3577b34da6a3ce929d0e0e4736"
	req := httptest.NewRequest("GET", "http://localhost/login", nil)
	req = req.WithContext(tracing.With(req.Context(),
		tracing.SpanContext{TraceID: journey, SpanID: "00f067aa0ba902b7", Inbound: true}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), journey) {
		t.Errorf("a served page does not carry the name of the request")
	}

	// And a page rendered without one says nothing rather than drawing an
	// empty line: an identifier that joins nothing is worse than none.
	plain := httptest.NewRecorder()
	mux.ServeHTTP(plain, httptest.NewRequest("GET", "http://localhost/login", nil))
	if strings.Contains(plain.Body.String(), `class="trace-id"`) {
		t.Errorf("a page with no request name drew the line anyway")
	}
}

// A signed-in page's spans leave naming the person once the switch is on -
// said by the gateway, which holds the session, and not by the page.
func TestTheRelayNamesWhoIsReadingThePage(t *testing.T) {
	var got atomic.Value
	tracing.RegisterRelay(func(b []byte) error { got.Store(string(b)); return nil })
	tracing.SetRelayMaxPerSecond(0)
	openToPages(t)
	t.Cleanup(func() { tracing.RegisterRelay(nil); tracing.SetCaller(false) })

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := t.Context()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	signed := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, signed, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	mux := telemetryMux(New(st, sm))
	send := func(on bool) string {
		tracing.SetCaller(on)
		req := httptest.NewRequest("POST", "http://localhost/meerkat/telemetry",
			bytes.NewBufferString(`{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"abc"}]}]}]}`))
		req.AddCookie(signed.Result().Cookies()[0])
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("the relay answered %d", rec.Code)
		}
		return got.Load().(string)
	}
	if b := send(false); strings.Contains(b, "user.") {
		t.Errorf("off, yet the person left: %s", b)
	}
	if b := send(true); !strings.Contains(b, `"user.id"`) || !strings.Contains(b, `"stringValue":"alice"`) {
		t.Errorf("on, the person is missing: %s", b)
	}
}
