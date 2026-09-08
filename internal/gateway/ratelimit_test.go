package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func limitedRoute(upstream string, limits ...store.RateLimit) store.Route {
	r := pathRoute("r1", "demo", 1, "/**", upstream)
	r.Limits = limits
	return r
}

// The bound refuses, and says the three things a client needs to back off on
// its own: what the limit was, what is left, and when to come back.
func TestARouteWideBoundRefusesWithItsHeaders(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL,
		store.RateLimit{Per: store.PerRoute, Requests: 3, Window: "PT1M"}))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	for i := 1; i <= 3; i++ {
		res, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("request %d answered %d, want 200", i, res.StatusCode)
		}
	}
	res, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the fourth request answered %d, want 429", res.StatusCode)
	}
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "Retry-After"} {
		if res.Header.Get(h) == "" {
			t.Errorf("no %s on the refusal: a door with no sign on it", h)
		}
	}
	if got := res.Header.Get("Retry-After"); got == "0" {
		t.Error("Retry-After: 0 is an invitation to hammer")
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Error("a refusal that can be cached answers the next caller too")
	}
	// The message names WHICH bound fired: a route with three rules and one
	// answer of "too many requests" leaves an operator guessing.
	if !contains(string(body), "3 per route") {
		t.Errorf("the refusal does not name the bound it hit: %q", body)
	}
}

// Nothing is bounded until somebody writes a bound, and a route without one
// must not pay for the machinery.
func TestARouteWithNoBoundIsNotLimited(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	for i := 0; i < 50; i++ {
		res, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("request %d answered %d on an unbounded route", i, res.StatusCode)
		}
	}
}

// Per address, each caller gets their own budget - and the address is read the
// way the whole product reads it: the RIGHTMOST forwarded entry, the only one
// the hop we are talking to wrote. Trusting the leftmost would let a caller
// name their own address and mint a fresh budget per request.
func TestPerAddressCountsCallersApart(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL,
		store.RateLimit{Per: store.PerIP, Requests: 2, Window: "PT1M"}))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	get := func(forwarded string) int {
		req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
		req.Header.Set("X-Forwarded-For", forwarded)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	for i := 0; i < 2; i++ {
		if got := get("10.0.0.1"); got != http.StatusOK {
			t.Fatalf("10.0.0.1 request %d answered %d", i, got)
		}
	}
	if got := get("10.0.0.1"); got != http.StatusTooManyRequests {
		t.Fatalf("10.0.0.1 went over budget and answered %d", got)
	}
	// A different address has its own.
	if got := get("10.0.0.2"); got != http.StatusOK {
		t.Fatalf("10.0.0.2 answered %d, want its own budget", got)
	}
	// And a caller PREPENDING a forged address does not get a fresh one: what
	// counts is the entry our own hop appended, which is the last.
	if got := get("1.2.3.4, 10.0.0.1"); got != http.StatusTooManyRequests {
		t.Errorf("a prepended address minted a fresh budget: answered %d", got)
	}
}

// A bound keyed on a user does not cover somebody who has none. It is not a
// bound of zero and not one of infinity - covering the anonymous is what a
// bound per address is for.
func TestPerUserDoesNotCoverTheAnonymous(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL,
		store.RateLimit{Per: store.PerUser, Requests: 1, Window: "PT1M"}))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	for i := 0; i < 5; i++ {
		res, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("an anonymous caller was refused by a per-user bound: %d", res.StatusCode)
		}
	}
}

// The one that makes the feature what it is: a limit written PER user gives
// each signed-in caller their own budget, without naming a single one of them.
func TestPerUserGivesEachCallerTheirOwnBudget(t *testing.T) {
	upstream := echoUpstream(t)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, u := range []string{"alice", "bob"} {
		if err := st.CreateUser(ctx, store.User{ID: "u-" + u, Username: u, PasswordHash: "x", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	route := limitedRoute(upstream.URL, store.RateLimit{Per: store.PerUser, Requests: 2, Window: "PT1M"})
	route.Access = store.Access{Level: store.AccessAuth}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	cookieFor := func(id string) *http.Cookie {
		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), id); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	get := func(c *http.Cookie) int {
		req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
		req.AddCookie(c)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	alice, bob := cookieFor("u-alice"), cookieFor("u-bob")
	for i := 0; i < 2; i++ {
		if got := get(alice); got != http.StatusOK {
			t.Fatalf("alice request %d answered %d", i, got)
		}
	}
	if got := get(alice); got != http.StatusTooManyRequests {
		t.Fatalf("alice went over her budget and answered %d", got)
	}
	// Bob's budget is his own, and alice hitting hers says nothing about it.
	if got := get(bob); got != http.StatusOK {
		t.Errorf("bob was refused because alice was busy: %d", got)
	}
}

// Two bounds, both true at once, and the narrower one answers. This is where a
// rate limit differs from an access rule, where the first match wins.
func TestEveryBoundApplies(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL,
		store.RateLimit{Per: store.PerRoute, Requests: 100, Window: "PT1M"},
		store.RateLimit{Per: store.PerIP, Requests: 2, Window: "PT1M"}))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	codes := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		res, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		codes = append(codes, res.StatusCode)
	}
	want := []int{200, 200, 429, 429}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("answers %v, want %v: the narrower bound is not being applied", codes, want)
		}
	}
}

// A refusal is a request the route answered, so it lands in the counters like
// any other - the metrics screen has to show a route being throttled, not a
// route that went quiet.
func TestRefusalsAreCounted(t *testing.T) {
	rt := newRouter(t, limitedRoute(echoUpstream(t).URL,
		store.RateLimit{Per: store.PerRoute, Requests: 1, Window: "PT1M"}))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	for i := 0; i < 3; i++ {
		res, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
	}
	for _, s := range rt.Metrics().Snapshot().Routes {
		if s.ID == "r1" {
			if s.ByClass[4] != 2 {
				t.Errorf("%d refusals counted as 4xx, want 2", s.ByClass[4])
			}
			return
		}
	}
	t.Fatal("the route has no counters at all")
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	}()
}

func BenchmarkRateGate(b *testing.B) {
	rt := &Router{}
	free, _ := compileLimits([]store.RateLimit{
		{Per: store.PerRoute, Requests: 1_000_000_000, Window: "PT1M"},
		{Per: store.PerIP, Requests: 1_000_000_000, Window: "PT1M"},
	})
	h := rt.rateGate(free, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "10.0.0.7:44444"
	w := httptest.NewRecorder()
	b.ReportAllocs()
	for b.Loop() {
		h.ServeHTTP(w, req)
	}
	_ = time.Now
}

// A bound on one operation leaves the others alone. That is the point of
// QUOTA-05: the expensive endpoint of a route gets its own ceiling without
// throttling the cheap ones beside it.
func TestAnEndpointCarriesItsOwnBound(t *testing.T) {
	route := pathRoute("r1", "demo", 1, "/demo/**", echoUpstream(t).URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}})
	route.API = &store.RouteAPI{Security: &store.EndpointSecurity{
		Endpoints: []store.EndpointPolicy{
			{Method: "GET", Path: "/report", Limits: []store.RateLimit{
				{Per: store.PerRoute, Requests: 2, Window: "PT1M"},
			}},
			{Method: "GET", Path: "/ping"},
		},
	}}
	rt := newRouter(t, route)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	code := func(path string) int {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	for i := 0; i < 2; i++ {
		if got := code("/demo/report"); got != http.StatusOK {
			t.Fatalf("/report request %d answered %d", i, got)
		}
	}
	if got := code("/demo/report"); got != http.StatusTooManyRequests {
		t.Fatalf("/report went over its bound and answered %d", got)
	}
	// The operation next to it has none, and did not inherit one.
	for i := 0; i < 5; i++ {
		if got := code("/demo/ping"); got != http.StatusOK {
			t.Fatalf("/ping was throttled by /report's bound: %d", got)
		}
	}
}

// What "only for" means for everybody else: NOTHING. A rule bounds the callers
// it describes, and somebody outside it is bounded by the other rules that
// cover them - by none, when there are none.
//
// This is the question the screen could not answer on its own, and the answer
// it now warns about: three narrow rules make a route look bounded while it is
// wide open to everyone the three do not describe.
func TestANarrowBoundLeavesTheRestAlone(t *testing.T) {
	upstream := echoUpstream(t)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.CreateUser(ctx, store.User{ID: "u-trial", Username: "trial", PasswordHash: "x", Enabled: true}))
	must(st.CreateUser(ctx, store.User{ID: "u-partner", Username: "partner", PasswordHash: "x", Enabled: true}))
	must(st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "acme", Enabled: true}))
	must(st.SaveRole(ctx, store.Role{ID: "r-trial", Name: "trial"}))
	must(st.SaveGroup(ctx, store.Group{ID: "g-trial", TenantID: "t1", Name: "Trial", RoleIDs: []string{"r-trial"}}))
	for _, u := range []string{"u-trial", "u-partner"} {
		must(st.SaveMembership(ctx, store.Membership{UserID: u, TenantID: "t1", Type: store.MemberUser,
			Enabled: true, BusinessAccess: store.BusinessAccess{Inherited: true}}))
	}
	must(st.SetMemberGroups(ctx, "t1", "u-trial", []string{"g-trial"}))

	route := limitedRoute(upstream.URL, store.RateLimit{
		Per: store.PerUser, Requests: 2, Window: "PT1M",
		Applies: store.Access{Level: store.AccessAuth, Roles: []string{"trial"}},
	})
	route.Access = store.Access{Level: store.AccessAuth}
	must(st.SaveRoute(ctx, route))

	sm := session.NewManager(st)
	rt := New(st, sm)
	must(rt.Reload(ctx))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	cookie := func(user, group string) *http.Cookie {
		rec := httptest.NewRecorder()
		if _, err := sm.IssueWith(ctx, rec, httptest.NewRequest("POST", "/login", nil),
			user, "t1", group, time.Hour, "", "", ""); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	code := func(c *http.Cookie) int {
		req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
		req.AddCookie(c)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	trial := cookie("u-trial", "g-trial")
	for i := 0; i < 2; i++ {
		if got := code(trial); got != http.StatusOK {
			t.Fatalf("trial request %d answered %d", i, got)
		}
	}
	if got := code(trial); got != http.StatusTooManyRequests {
		t.Fatalf("the rule did not bound the callers it describes: %d", got)
	}
	// And whoever the rule does not describe is bounded by nothing here.
	partner := cookie("u-partner", "")
	for i := 0; i < 10; i++ {
		if got := code(partner); got != http.StatusOK {
			t.Fatalf("a caller outside the rule was bounded by it: %d on request %d", got, i)
		}
	}
}
