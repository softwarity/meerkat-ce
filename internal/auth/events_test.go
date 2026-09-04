package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/devtunnel"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"golang.org/x/crypto/bcrypt"
)

// The endpoint's whole job is the grant: which rooms THIS connection is in.
// Everything downstream publishes to a topic and never asks a permission
// question, so this is where the answer has to be right.
func TestTheLiveChannelGrantsRoomsFromTheSession(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: string(hash), Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "b", Username: "bob", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h := New(st, session.NewManager(st))
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	hub := h.Events()

	devCookie := postLogin(t, mux, url.Values{"username": {"devon"}, "password": {"s3cret"}}).Result().Cookies()[0]
	bobCookie := postLogin(t, mux, url.Values{"username": {"bob"}, "password": {"s3cret"}}).Result().Cookies()[0]

	// Three pages: nobody signed in, a plain account, a developer.
	anon, anonR := dialEvents(t, srv.URL, nil)
	bob, bobR := dialEvents(t, srv.URL, bobCookie)
	dev, devR := dialEvents(t, srv.URL, devCookie)
	waitFor(t, hub, 3)

	// What everyone is entitled to know reaches all three, signed in or not:
	// a service served from a developer's machine changes the application in
	// front of a visitor, whoever they are.
	hub.Publish(events.TopicAll, events.Message{Type: "served"})
	for _, r := range []*reader{anonR, bobR, devR} {
		if got := readText(t, r); got != `{"type":"served"}` {
			t.Fatalf("a page missed a broadcast: %s", got)
		}
	}

	// The developer room reaches the developer only. The others must be
	// SILENT, so the assertion is the next frame they see, not the absence of
	// one: a broadcast published afterwards proves they skipped nothing.
	hub.Publish(events.TopicDev, events.Message{Type: "tools"})
	hub.Publish(events.UserTopic("b"), events.Message{Type: "session"})
	hub.Publish(events.TopicAll, events.Message{Type: "after"})

	if got := readText(t, devR); got != `{"type":"tools"}` {
		t.Fatalf("the developer page missed the dev room: %s", got)
	}
	if got := readText(t, devR); got != `{"type":"after"}` {
		t.Fatalf("the developer page received another account's message: %s", got)
	}
	if got := readText(t, bobR); got != `{"type":"session"}` {
		t.Fatalf("the account's own room did not reach its page: %s", got)
	}
	if got := readText(t, bobR); got != `{"type":"after"}` {
		t.Fatalf("a plain account received the dev room: %s", got)
	}
	if got := readText(t, anonR); got != `{"type":"after"}` {
		t.Fatalf("an anonymous page received a room it was not granted: %s", got)
	}
	_, _, _ = anon, bob, dev
}

// The echo is the channel's own instrument, and its whole safety is that it
// can only reach the caller's own pages: a developer debugging a socket must
// not be able to write on everyone's screen.
func TestTheChannelEchoOnlyReachesItsOwnPages(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: string(hash), Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "b", Username: "bob", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h := New(st, session.NewManager(st))
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	devCookie := postLogin(t, mux, url.Values{"username": {"devon"}, "password": {"s3cret"}}).Result().Cookies()[0]
	bobCookie := postLogin(t, mux, url.Values{"username": {"bob"}, "password": {"s3cret"}}).Result().Cookies()[0]

	_, devR := dialEvents(t, srv.URL, devCookie)
	_, bobR := dialEvents(t, srv.URL, bobCookie)
	waitFor(t, h.Events(), 2)

	// A non-dev is refused: same gate as the rest of the developer surface.
	if rec := postJSON(t, mux, "/meerkat/events/echo", `{"type":"probe"}`, bobCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a non-dev reached the echo: %d", rec.Code)
	}
	// A message needs a type - it is what the page switches on.
	if rec := postJSON(t, mux, "/meerkat/events/echo", `{"data":{"a":1}}`, devCookie); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a typeless event was accepted: %d", rec.Code)
	}
	if rec := postJSON(t, mux, "/meerkat/events/echo", `{"type":"probe","data":{"a":1}}`, devCookie); rec.Code != http.StatusNoContent {
		t.Fatalf("echo: %d", rec.Code)
	}
	if got := readText(t, devR); got != `{"type":"probe","data":{"a":1}}` {
		t.Fatalf("the echo did not come back: %s", got)
	}
	// Bob must not have seen it. Proven by what he sees NEXT, not by an
	// absence: a broadcast published now proves he skipped nothing.
	h.Events().Publish(events.TopicAll, events.Message{Type: "after"})
	if got := readText(t, bobR); got != `{"type":"after"}` {
		t.Fatalf("another account's page received the echo: %s", got)
	}
}

// The channel is data-plane only: the console has its own live needs and none
// of them are these. Mounting it on both would have been one line and a second
// place to keep the grant right.
func TestTheLiveChannelIsNotOnTheAdminPlane(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mux := http.NewServeMux()
	NewAdmin(st, session.NewManager(st)).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/meerkat/events", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("the admin plane answered the live channel: %d", rec.Code)
	}
}

// ---- a hand-written client, matching the hand-written server ----------

type reader struct {
	c  net.Conn
	br *bufio.Reader
}

func dialEvents(t *testing.T, server string, cookie *http.Cookie) (net.Conn, *reader) {
	t.Helper()
	u, err := url.Parse(server)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	req, _ := http.NewRequest(http.MethodGet, server+"/meerkat/events", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(nonce[:]))
	req.Header.Set("Sec-WebSocket-Version", "13")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if err := req.Write(c); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake: %s", resp.Status)
	}
	return c, &reader{c: c, br: br}
}

// readText returns the next TEXT frame, stepping over the control frames the
// server may interleave.
func readText(t *testing.T, r *reader) string {
	t.Helper()
	for {
		_ = r.c.SetReadDeadline(time.Now().Add(3 * time.Second))
		var head [2]byte
		if _, err := io.ReadFull(r.br, head[:]); err != nil {
			t.Fatal(err)
		}
		n := int(head[1] & 0x7F)
		if n == 126 {
			var ext [2]byte
			if _, err := io.ReadFull(r.br, ext[:]); err != nil {
				t.Fatal(err)
			}
			n = int(ext[0])<<8 | int(ext[1])
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r.br, payload); err != nil {
			t.Fatal(err)
		}
		if head[0]&0x0F == 0x1 {
			return string(payload)
		}
	}
}

// waitFor lets the handlers get their subscriptions in: a message published
// into an empty hub is simply not sent, so publishing too early would test
// nothing at all.
func waitFor(t *testing.T, hub *events.Hub, n int) {
	t.Helper()
	for range 200 {
		if hub.Count() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("only %d of %d pages are listening", hub.Count(), n)
}

// postJSON is the echo's shape: a JSON body, not a form.
func postJSON(t *testing.T, mux *http.ServeMux, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// What a developer's machine is serving travels in the payload the page
// fetches at load - to EVERY visitor, signed in or not. The live channel only
// keeps it fresh: a page whose socket never opens still shows the truth it was
// served with, which is what makes the channel safe to be missing.
func TestWhatIsServedTravelsToEveryVisitor(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mux := http.NewServeMux()
	h := New(st, session.NewManager(st))
	h.Register(mux)

	// Nothing registered: the field is absent, which is the community image's
	// whole behaviour here - no guard, nothing to show.
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, nil)); strings.Contains(body, `"served"`) {
		t.Fatalf("an empty registry still put a served list on the wire: %s", body)
	}

	reg := devtunnel.NewRegistry(h.Events())
	h.WatchServed(reg)
	reg.Set(devtunnel.Served{Name: "checkout", Who: "alice", Parked: true})

	// Anonymous: no cookie at all, and it still learns. An application may be
	// public, and what it IS changed for whoever is looking at it.
	body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, nil))
	// The NAME and WHO, and nothing else: a visitor needs to know this service
	// is answered from somebody's machine, not the state of the deployed one
	// it stands in for. That was an operator's fact on a visitor's strip.
	for _, want := range []string{`"name":"checkout"`, `"who":"alice"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("the payload misses %s: %s", want, body)
		}
	}

	// And the labels the strip needs ride along, since the component's JS is
	// cached for five minutes and this payload is not.
	for _, want := range []string{"pluggedTitle", "pluggedBy"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the payload misses the %s label", want)
		}
	}

	reg.Drop("checkout")
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, nil)); strings.Contains(body, "checkout") {
		t.Fatalf("a dropped name is still announced: %s", body)
	}
}

// Signing in while a developer's machine is answering for something lands on a
// halt naming it, once, before starting. It INFORMS rather than gates: the
// login is already recorded when it appears, and the destination is still
// reachable by hand.
func TestSigningInLandsOnTheHaltWhenSomethingIsServed(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "u", Username: "alice", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h := New(st, session.NewManager(st))
	h.Register(mux)

	// Nothing served: the sign-in goes where it was going, untouched.
	rec := postLogin(t, mux, url.Values{"username": {"alice"}, "password": {"s3cret"}})
	if loc := rec.Header().Get("Location"); strings.HasPrefix(loc, "/plugged") {
		t.Fatalf("a halt appeared with nothing served: %s", loc)
	}

	reg := devtunnel.NewRegistry(h.Events())
	h.WatchServed(reg)
	reg.Set(devtunnel.Served{Name: "checkout", Who: "alice"})

	rec = postLogin(t, mux, url.Values{"username": {"alice"}, "password": {"s3cret"}})
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/plugged") {
		t.Fatalf("the sign-in did not stop at the halt: %s", loc)
	}
	cookie := rec.Result().Cookies()[0]

	// The login was recorded BEFORE the halt, so walking past it cannot lose
	// the record - which is what makes the halt safe to skip.
	if events, err := st.ListLoginEvents(ctx, "u"); err != nil || len(events) == 0 {
		t.Fatalf("the sign-in was not recorded before the halt: %v %v", events, err)
	}

	page := bodyString(do(t, mux, "GET", "/plugged?next=%2F", nil, cookie))
	for _, want := range []string{"checkout", "alice"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the halt does not name %s: %s", want, page)
		}
	}

	// Continuing resumes the sign-in rather than redirecting on its own: the
	// organisation still has to be resolved.
	done := postJSONForm(t, mux, "/plugged", "next=%2F", cookie)
	if done.Code != http.StatusSeeOther {
		t.Fatalf("continuing from the halt: %d", done.Code)
	}

	// And once nothing is served, the halt sends the person straight on rather
	// than showing a page announcing nothing.
	reg.Drop("checkout")
	if res := do(t, mux, "GET", "/plugged?next=%2F", nil, cookie); res.Code != http.StatusSeeOther {
		t.Fatalf("an empty halt still rendered: %d", res.Code)
	}
}

// postJSONForm posts a urlencoded body, which is what the halt's own form does.
func postJSONForm(t *testing.T, mux *http.ServeMux, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
