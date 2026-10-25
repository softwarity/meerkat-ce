package auth

import (
	"context"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// The profile's way back into the applications is ONE control, chosen in a
// fixed order. Each line is one case of the rule.
func TestTheProfileOffersOneWayBackIn(t *testing.T) {
	tr := map[string]string{"back": "Back", "applications": "Applications"}
	shop := publicLink{Name: "Shop", Href: "/shop"}
	wiki := publicLink{Name: "Wiki", Href: "/wiki", Group: "Tools", Head: true}
	docs := publicLink{Name: "Docs", Href: "/docs", Group: "Tools"}
	ops := publicLink{Name: "ops", Href: "/ops"}
	admin := publicLink{Name: "admin", Href: "https://admin.example:8443/"}
	button := func(label, href string) appsControl { return appsControl{Label: label, Href: href} }

	for _, c := range []struct {
		name string
		in   appsChoice
		want appsControl
	}{
		{"came from a page still open: Back to it, whatever the mode",
			appsChoice{Back: "/shop/cart?x=1", Mode: store.PortalModePortal, AppName: "Acme",
				Catalogue: []publicLink{shop}},
			button("Back", "/shop/cart?x=1")},
		{"portal: one button to the portal, under the application's name",
			appsChoice{Mode: store.PortalModePortal, AppName: "Acme", Catalogue: []publicLink{shop}},
			button("Acme", "/shop")},
		{"portal with no name in the branding: the module's own",
			appsChoice{Mode: store.PortalModePortal, AppName: " ", Catalogue: []publicLink{shop}},
			button("Shop", "/shop")},
		{"links: a list, containers as groups",
			appsChoice{Mode: store.PortalModeLinks, Catalogue: []publicLink{shop, wiki, docs}},
			appsControl{Label: "Applications", Groups: []appsGroup{
				{Links: []publicLink{{Name: "Shop", Href: "/shop"}}},
				{Label: "Tools", Links: []publicLink{{Name: "Wiki", Href: "/wiki"}, {Name: "Docs", Href: "/docs"}}},
			}}},
		{"links with a single choice: a plain button with its name",
			appsChoice{Mode: store.PortalModeLinks, Catalogue: []publicLink{shop}},
			button("Shop", "/shop")},
		{"none: a list of the UI routes, full address for another host",
			appsChoice{Mode: store.PortalModeNone, Routes: []publicLink{ops, admin}},
			appsControl{Label: "Applications", Groups: []appsGroup{
				{Links: []publicLink{{Name: "ops", Href: "/ops"}, {Name: "admin", Href: "https://admin.example:8443/"}}},
			}}},
		{"none with a single route: its button",
			appsChoice{Mode: store.PortalModeNone, Routes: []publicLink{ops}},
			button("ops", "/ops")},
		{"portal with nothing open in the catalogue: the routes",
			appsChoice{Mode: store.PortalModePortal, AppName: "Acme", Routes: []publicLink{ops}},
			button("ops", "/ops")},
		{"links with nothing open in the catalogue: the routes",
			appsChoice{Mode: store.PortalModeLinks, Routes: []publicLink{ops, admin}},
			appsControl{Label: "Applications", Groups: []appsGroup{
				{Links: []publicLink{{Name: "ops", Href: "/ops"}, {Name: "admin", Href: "https://admin.example:8443/"}}},
			}}},
		{"nothing open at all: Applications, to the page that says so",
			appsChoice{Mode: store.PortalModeLinks},
			button("Applications", noAppsHref)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := chooseApps(c.in, tr); !reflect.DeepEqual(got, c.want) {
				t.Errorf("\n got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// A route on another host is offered at its full address, on the same port
// and scheme; one only reachable under a wildcard name has no address and is
// left out.
func TestARouteOnAnotherHostIsOfferedAtItsAddress(t *testing.T) {
	route := func(preds ...routing.Spec) store.Route { return store.Route{Predicates: preds} }
	path := func(p string) routing.Spec {
		return routing.Spec{Type: "path", Args: map[string]any{"patterns": []any{p}}}
	}
	host := func(h ...string) routing.Spec {
		var hs []any
		for _, s := range h {
			hs = append(hs, s)
		}
		return routing.Spec{Type: "host", Args: map[string]any{"hosts": hs}}
	}
	r := httptest.NewRequest("GET", "http://apps.example:36080/profile", nil)
	for _, c := range []struct {
		name string
		rt   store.Route
		want string
	}{
		{"no host: the entry path", route(path("/shop/**")), "/shop"},
		{"this host: the entry path", route(host("apps.example"), path("/shop/**")), "/shop"},
		{"a wildcard taking this host: the entry path", route(host("*.example"), path("/shop/**")), "/shop"},
		{"another host: its full address", route(host("*.other.example", "admin.example"), path("/console/**")),
			"http://admin.example:36080/console"},
		{"no path predicate: the host's root", route(host("admin.example")), "http://admin.example:36080/"},
		{"only a wildcard name: no address", route(host("*.other.example"), path("/x/**")), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := routeAddress(r, c.rt); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Back is offered only while the page is still open in the tenant ACTIVE
// now: switching tenant on the profile can close it, and then the profile
// falls back to what the new tenant opens.
func TestBackIsDroppedWhenTheTenantSwitchClosesThePage(t *testing.T) {
	mux, _, st := groupSetup(t)
	ctx := context.Background()
	for _, tid := range []string{"t1", "t2"} {
		if err := st.SetGroupMode(ctx, tid, store.GroupModeMultiple); err != nil {
			t.Fatal(err)
		}
	}
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	cookie := sessionCookieOf(login)
	do(t, mux, "POST", "/select-tenant", url.Values{"tenant": {"t1"}, "next": {"/"}}, cookie)

	from := "/profile?from=" + url.QueryEscape("/opsapp/board")
	if body := bodyString(do(t, mux, "GET", from, nil, cookie)); !strings.Contains(body, `href="/opsapp/board">Back</a>`) {
		t.Fatalf("in t1 the ops role opens the page: Back expected\n%.600s", body)
	}

	// t2 grants sales only: the ops page is closed, and the one application
	// left is offered under its name instead.
	do(t, mux, "POST", "/select-tenant", url.Values{"tenant": {"t2"}, "next": {"/"}}, cookie)
	body := bodyString(do(t, mux, "GET", from, nil, cookie))
	if strings.Contains(body, ">Back</a>") {
		t.Fatalf("the page is closed in t2, Back must not be offered\n%.600s", body)
	}
	if !strings.Contains(body, `<a class="apps-btn" href="/salesapp">salesapp</a>`) {
		t.Fatalf("t2 opens salesapp alone: its button expected\n%.600s", body)
	}
}

// A page the gateway draws itself is no application to go back to, even under
// a catch-all route; and with nothing open at all, the button leads to the
// page that tells the person to ask for access.
func TestNothingOpenLeadsToTheAccessPage(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	// A catch-all UI route closed to this account: it would swallow /login
	// if paths alone decided.
	all := uiRoute("all", 1, true, "nobody")
	all.Predicates = []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}}
	if err := st.SaveRoute(ctx, all); err != nil {
		t.Fatal(err)
	}
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	body := bodyString(do(t, mux, "GET", "/profile?from=%2Flogin", nil, sc))
	if strings.Contains(body, ">Back</a>") {
		t.Fatalf("Back to the sign-in page was offered\n%.600s", body)
	}
	if !strings.Contains(body, `<a class="apps-btn" href="/refused?why=apps">Applications</a>`) {
		t.Fatalf("nothing open: the Applications button to the access page expected\n%.600s", body)
	}
	page := bodyString(do(t, mux, "GET", noAppsHref, nil, sc))
	if !strings.Contains(page, "No application is open to your account yet.") ||
		!strings.Contains(page, "ask your administrator") {
		t.Fatalf("the access page does not say it\n%.600s", page)
	}

	// Open the catch-all to everyone signed in: Back now works for its pages,
	// still never for the sign-in page.
	all.Access = store.Access{Level: store.AccessAuth}
	if err := st.SaveRoute(ctx, all); err != nil {
		t.Fatal(err)
	}
	if body := bodyString(do(t, mux, "GET", "/profile?from=%2Fdesk", nil, sc)); !strings.Contains(body, `href="/desk">Back</a>`) {
		t.Fatalf("a page of an open route: Back expected\n%.600s", body)
	}
	if body := bodyString(do(t, mux, "GET", "/profile?from=%2Flogin", nil, sc)); strings.Contains(body, ">Back</a>") {
		t.Fatalf("Back to the sign-in page was offered under a catch-all\n%.600s", body)
	}
}
