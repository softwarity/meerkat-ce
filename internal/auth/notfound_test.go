package auth

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The page a browser gets where no route answers: themed, a 404, the address
// asked for, and the way on from where this visitor stands.
func TestThePageWhereNothingAnswers(t *testing.T) {
	mux, sm, st := mfaSetup(t)
	ctx := context.Background()
	for _, rt := range []store.Route{
		uiRoute("shop", 1, false, ""),
		uiRoute("intranet", 2, true, ""),
		uiRoute("ops", 3, true, "operations"),
	} {
		if err := st.SaveRoute(ctx, rt); err != nil {
			t.Fatalf("SaveRoute %s: %v", rt.ID, err)
		}
	}
	seedCatalogue(t, st, store.PortalModeLinks, "shop", "intranet", "ops")
	h := New(st, sm)
	en := messagesFor("en")

	serve := func(t *testing.T, cookie *http.Cookie) (int, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/old/page?token=s3cret-value", nil)
		req.Header.Set("Accept", "text/html")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeNotFound(rec, req)
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("answered %q, not a page", ct)
		}
		return rec.Code, rec.Body.String()
	}
	has := func(t *testing.T, body, what, want string) {
		t.Helper()
		if !strings.Contains(body, want) {
			t.Errorf("%s is not on the page (%q)", what, want)
		}
	}
	hasNot := func(t *testing.T, body, what, want string) {
		t.Helper()
		if strings.Contains(body, want) {
			t.Errorf("%s is on the page (%q)", what, want)
		}
	}

	t.Run("a visitor who is not signed in", func(t *testing.T) {
		code, body := serve(t, nil)
		if code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", code)
		}
		has(t, body, "the theme", "--mk-primary")
		has(t, body, "the title", html.EscapeString(en["titleNotFound"]))
		has(t, body, "the statement", html.EscapeString(en["notFoundLead"]))
		has(t, body, "the path asked for", `<p class="nf-path">/old/page</p>`)
		hasNot(t, body, "the query string", "s3cret-value")
		has(t, body, "the public application", `href="/shop"`)
		hasNot(t, body, "an application behind the sign-in", `href="/intranet"`)
		has(t, body, "the sign-in", `<a class="choice" href="/login">`)
		hasNot(t, body, "the profile", `href="/profile"`)
		hasNot(t, body, "the portal bar", "meerkat-portal-nav")
	})

	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	sc := sessionCookieOf(login)

	t.Run("a signed-in visitor", func(t *testing.T) {
		code, body := serve(t, sc)
		if code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", code)
		}
		has(t, body, "the sentence over the list", html.EscapeString(en["notFoundPick"]))
		has(t, body, "the public application", `href="/shop"`)
		has(t, body, "the application open to them", `href="/intranet"`)
		hasNot(t, body, "an application asking for a role they lack", `href="/ops"`)
		has(t, body, "the profile", `href="/profile"`)
		hasNot(t, body, "the sign-in", `href="/login"`)
	})

	t.Run("a signed-in visitor with nothing open", func(t *testing.T) {
		for _, id := range []string{"shop", "intranet", "ops"} {
			if _, err := st.DeleteRoute(ctx, id); err != nil {
				t.Fatalf("DeleteRoute %s: %v", id, err)
			}
		}
		_, body := serve(t, sc)
		has(t, body, "what to do about it", html.EscapeString(en["refusedApps"]))
		hasNot(t, body, "a list", `class="public-links"`)
	})

	t.Run("behind the portal", func(t *testing.T) {
		RegisterPortalBar(&PortalBar{Fragment: func(r store.Route, _ []string) string {
			return `<meerkat-portal-nav route="` + r.ID + `"></meerkat-portal-nav>`
		}})
		t.Cleanup(func() { RegisterPortalBar(nil) })
		if err := st.SaveRoute(ctx, uiRoute("shop", 1, false, "")); err != nil {
			t.Fatal(err)
		}
		seedCatalogue(t, st, store.PortalModePortal, "shop")
		for _, c := range []struct {
			who    string
			cookie *http.Cookie
		}{{"not signed in", nil}, {"signed in", sc}} {
			code, body := serve(t, c.cookie)
			if code != http.StatusNotFound {
				t.Fatalf("%s: status %d, want 404", c.who, code)
			}
			has(t, body, c.who+": the bar", "<meerkat-portal-nav")
			has(t, body, c.who+": the page agent", "/meerkat/page.js")
			has(t, body, c.who+": the way on", html.EscapeString(en["notFoundMenu"]))
			has(t, body, c.who+": the path asked for", `<p class="nf-path">/old/page</p>`)
			// The bar lists the applications and mounts the account button:
			// the page draws neither a list nor a sign-in of its own.
			hasNot(t, body, c.who+": a list", `class="public-links"`)
			hasNot(t, body, c.who+": a sign-in", `<a class="choice" href="/login">`)
			has(t, body, c.who+": an application screen", "mk-app")
		}

		// A bar with nothing to show this visitor draws nothing, its account
		// button included: the page offers its own way on instead.
		if err := st.SaveRoute(ctx, uiRoute("shop", 1, true, "")); err != nil {
			t.Fatal(err)
		}
		_, body := serve(t, nil)
		hasNot(t, body, "an empty bar", "<meerkat-portal-nav")
		hasNot(t, body, "the menu it does not have", html.EscapeString(en["notFoundMenu"]))
		has(t, body, "the sign-in", `<a class="choice" href="/login">`)
		_, body = serve(t, sc)
		has(t, body, "the bar, for whoever may open its entry", "<meerkat-portal-nav")
	})
}
