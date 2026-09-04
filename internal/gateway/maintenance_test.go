package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A gateway carrying one working route, and the switch that answers for all of
// them (LIFE-05).
func maintainable(t *testing.T) (*Router, *store.Store, *httptest.Server) {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("the application"))
	}))
	t.Cleanup(up.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.SaveRoute(context.Background(), pathRoute("r1", "app", 1, "/**", up.URL)); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	return rt, st, up
}

func switchOn(t *testing.T, rt *Router, st *store.Store, m store.Maintenance) {
	t.Helper()
	if err := st.SetSetting(context.Background(), store.SettingMaintenance, m); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// The switch answers for EVERY route, without one of them being edited - which
// is the entire reason it exists rather than a maintenance filter dropped on
// each route and taken off again from memory afterwards.
func TestTheGlobalSwitchAnswersForEveryRoute(t *testing.T) {
	rt, st, _ := maintainable(t)

	res, body := get(t, rt, "/anything")
	if res.StatusCode != http.StatusOK || body != "the application" {
		t.Fatalf("before: %d %q", res.StatusCode, body)
	}

	switchOn(t, rt, st, store.Maintenance{Enabled: true, Reason: store.ReasonMaintenance})

	res, body = get(t, rt, "/anything")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("during maintenance: %d, want 503", res.StatusCode)
	}
	if !strings.Contains(body, "Under maintenance") && !strings.Contains(body, "unavailable") {
		t.Errorf("the page says nothing recognisable: %.120s", body)
	}
	// no-store, or a cache in front goes on serving the unavailable page after
	// the switch is off - a maintenance that outlives itself.
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if res.Header.Get("Retry-After") == "" {
		t.Error("no Retry-After: a well-behaved client has nothing to wait for")
	}

	// And off again, with nothing to undo.
	switchOn(t, rt, st, store.Maintenance{Enabled: false})
	if res, body = get(t, rt, "/anything"); res.StatusCode != http.StatusOK || body != "the application" {
		t.Fatalf("after: %d %q", res.StatusCode, body)
	}
}

// Nobody is spared the page, not even the person who turned it on - that is
// the truth of what everyone is getting, and an automatic bypass hid it from
// the only person able to act on it. What an administrator gets on top is a
// way through, offered and taken deliberately.
func TestAnAdministratorSeesWhatVisitorsSee(t *testing.T) {
	rt, st, _ := maintainable(t)
	ctx := context.Background()
	sm := session.NewManager(st)
	rt.sm = sm

	for _, u := range []store.User{
		{ID: "ops", Username: "ops", Enabled: true, InfraAdmin: true},
		{ID: "visitor", Username: "visitor", Enabled: true},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	switchOn(t, rt, st, store.Maintenance{Enabled: true})

	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	as := func(userID string) (int, string) {
		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), userID); err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
		for _, c := range rec.Result().Cookies() {
			req.AddCookie(c)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res.StatusCode, string(body)
	}

	// An administrator sees the SAME page a visitor sees, because that is the
	// truth of what everyone is getting - the bypass used to be automatic, and
	// the one person who could not see the maintenance was the one who had
	// just turned it on.
	code, body := as("ops")
	if code != http.StatusServiceUnavailable {
		t.Errorf("an infra admin got %d: they should see what visitors see", code)
	}
	if !strings.Contains(body, "Continue anyway") {
		t.Error("no way through offered to an administrator: nobody can verify the fix before reopening")
	}
	code, body = as("visitor")
	if code != http.StatusServiceUnavailable {
		t.Errorf("a visitor got %d during maintenance: the switch does not hold", code)
	}
	if strings.Contains(body, "Continue anyway") {
		t.Error("a visitor was shown a way through")
	}
}

// The door: an administrator asks, and from then on browses the application.
// Deliberate, and only for a session that already privileges them - the cookie
// on its own opens nothing.
func TestTheDoorOpensOnlyForSomeoneItAlreadyPrivileges(t *testing.T) {
	rt, st, _ := maintainable(t)
	ctx := context.Background()
	sm := session.NewManager(st)
	rt.sm = sm

	for _, u := range []store.User{
		{ID: "ops", Username: "ops", Enabled: true, InfraAdmin: true},
		{ID: "visitor", Username: "visitor", Enabled: true},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	switchOn(t, rt, st, store.Maintenance{Enabled: true})

	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	// A jar per identity, so the cookie the door sets is kept the way a
	// browser keeps it.
	through := func(userID string) int {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), userID); err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(srv.URL)
		jar.SetCookies(u, rec.Result().Cookies())

		// Take the door, then ask for the application.
		res, err := client.Get(srv.URL + "/anything?" + bypassParam + "=1")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		res, err = client.Get(srv.URL + "/anything")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	if code := through("ops"); code != http.StatusOK {
		t.Errorf("an administrator taking the door still got %d", code)
	}
	if code := through("visitor"); code != http.StatusServiceUnavailable {
		t.Errorf("a visitor took the door and got %d: the cookie must be an intent, not a grant", code)
	}
}

// A setting this build cannot read must not take every application down with
// it: an unreadable switch is OFF.
func TestAnUnreadableSwitchIsOff(t *testing.T) {
	rt, st, _ := maintainable(t)
	if err := st.SetSetting(context.Background(), store.SettingMaintenance, "on please"); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if res, _ := get(t, rt, "/anything"); res.StatusCode != http.StatusOK {
		t.Fatalf("an unparseable setting took the gateway down: %d", res.StatusCode)
	}
}

// The claim the whole design rests on: maintenance replaces THE ROUTES, and
// the gateway's own pages are mounted beside this router rather than inside
// it, so they are untouched. A maintenance that locks the sign-in page is a
// maintenance nobody can end - somebody has to come in and flip it back.
//
// Checked at the mux, which is where main puts them: explicit paths win over
// the router at "/".
func TestTheGatewaysOwnPagesSurviveTheMaintenance(t *testing.T) {
	rt, st, _ := maintainable(t)
	switchOn(t, rt, st, store.Maintenance{Enabled: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("sign in"))
	})
	mux.Handle("/", rt)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("the sign-in page answers %d during maintenance: nobody can come in to lift it", res.StatusCode)
	}
	res, err = http.Get(srv.URL + "/anything")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("a route answers %d during maintenance", res.StatusCode)
	}
}

// The door is not a free pass for the hour: every page an administrator sees
// while browsing through it says so. Without that, the door is the old silent
// bypass with one extra click - which is exactly what was reported.
func TestGoingThroughIsVisibleOnEveryPage(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>app</title></head><body>the application</body></html>"))
	}))
	t.Cleanup(up.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SaveRoute(ctx, pathRoute("r1", "app", 1, "/**", up.URL)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "ops", Username: "ops", Enabled: true, InfraAdmin: true}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	switchOn(t, rt, st, store.Maintenance{Enabled: true})

	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), "ops"); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	jar.SetCookies(u, rec.Result().Cookies())

	res, err := client.Get(srv.URL + "/x?" + bypassParam + "=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	res, err = client.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("the door did not open: %d", res.StatusCode)
	}
	if !strings.Contains(string(body), "the application") {
		t.Fatal("the application did not answer")
	}
	if !strings.Contains(string(body), "mk-maint") {
		t.Error("no reminder on the page: an administrator browses an application that is down for everyone else, and nothing says so")
	}

	// And leaving puts them back with the visitors, immediately.
	res, err = client.Get(srv.URL + "/x?" + bypassParam + "=0")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, err = client.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("after leaving, the door was still open: %d", res.StatusCode)
	}
}

// A page nobody is bypassing on must be untouched - the stripe rides every
// route, so it has to cost nothing when it has nothing to say.
func TestAnOrdinaryPageIsNotTouched(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head></head><body>ordinary</body></html>"))
	}))
	t.Cleanup(up.Close)
	rt := newRouter(t, pathRoute("r1", "app", 1, "/**", up.URL))

	res, body := get(t, rt, "/x")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%d", res.StatusCode)
	}
	if strings.Contains(body, "mk-maint") {
		t.Error("the stripe landed on a page with no maintenance at all")
	}
}

// The page every visitor meets during an outage is the one page that must not
// look like it belongs to some gateway they never heard of.
func TestTheUnavailablePageWearsTheInstallationsMark(t *testing.T) {
	rt, st, _ := maintainable(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingBranding, store.Branding{
		AppName: "Acme Portal",
		Logo:    "data:image/svg+xml;base64,PHN2Zy8+",
	}); err != nil {
		t.Fatal(err)
	}
	switchOn(t, rt, st, store.Maintenance{Enabled: true, Reason: store.ReasonMaintenance})

	_, body := get(t, rt, "/anything")
	for _, want := range []string{"Acme Portal", "data:image/svg+xml;base64,PHN2Zy8+"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
	// Self-contained: it has to render with the application down and the
	// network gone, so nothing on it may be fetched from anywhere.
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Error("the page reaches for something over the network")
	}
}

// The door is taken once per SESSION, not once per hour. Sign out and back in
// and the page comes back with its door - the decision is made again by
// whoever is making it now.
func TestTheDoorIsTakenAgainAtEveryLogin(t *testing.T) {
	rt, st, _ := maintainable(t)
	ctx := context.Background()
	sm := session.NewManager(st)
	rt.sm = sm
	if err := st.CreateUser(ctx, store.User{ID: "ops", Username: "ops", Enabled: true, InfraAdmin: true}); err != nil {
		t.Fatal(err)
	}
	switchOn(t, rt, st, store.Maintenance{Enabled: true})

	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)

	signIn := func() {
		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), "ops"); err != nil {
			t.Fatal(err)
		}
		jar.SetCookies(u, rec.Result().Cookies())
	}
	ask := func() int {
		res, err := client.Get(srv.URL + "/anything")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	signIn()
	res, err := client.Get(srv.URL + "/anything?" + bypassParam + "=1")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if code := ask(); code != http.StatusOK {
		t.Fatalf("the door did not open: %d", code)
	}

	// A second sign-in is a second session. The mark from the first one must
	// not carry: it was a decision, and decisions are not inherited.
	signIn()
	if code := ask(); code != http.StatusServiceUnavailable {
		t.Errorf("after signing in again the door was still open: %d", code)
	}
}
