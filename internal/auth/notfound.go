package auth

import (
	"context"
	"html/template"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// The page a browser gets when no route serves the address it asked for
// (ROUTE-10).
//
// A fresh gateway has no route, and removing a catch-all leaves every path it
// used to take unserved: the answer used to be Go's own line of text, "404
// page not found", on a blank page - the one answer a visitor can do nothing
// with, on the installation's own address. So it goes through the chrome of
// every other page this gateway serves, which is also what gives it the
// theme, the language and the scheme without a line of its own.
//
// Two shapes, decided by the installation and not by this page:
//
//   - behind the portal, it stands where an application would, with the bar
//     a proxied page carries for this visitor (BehindPortal): the bar is the
//     way on, the page only says so;
//   - without it, the sign-in pages' frame, and the way on drawn here: the
//     applications this visitor may open, the profile when signed in, the
//     sign-in when not.
//
// The status stays 404 either way: a page that answered 200 would tell a
// crawler, a link checker and a monitoring probe that the address exists.
// Only a browser gets it - the gateway hands everything else the plain 404
// it always had.

type notFoundData struct {
	flowChrome
	// Path is the address asked for, the path alone: a query string is where
	// a token or a code travels, and this page prints what it is given.
	Path string
	// Way is the sentence that says where to go from here, already chosen
	// and translated: the menu behind the portal, the list below without it,
	// or that nothing is open to this account yet. Empty for a visitor who is
	// not signed in and has no list: the sign-in below says it all.
	Way string
	// Apps are the applications this visitor may open, without the portal.
	// Empty behind it - the bar lists them - and empty when nothing is open.
	Apps []publicLink
	// SignIn offers the sign-in: not signed in, and no bar to carry it.
	SignIn bool
	// Profile offers the profile: signed in, and no bar to carry it.
	Profile bool
	// Back is the page of this site the visitor came from, when there is one
	// (backFromReferer): a dead link followed from a page leads back to it.
	Back template.URL
}

const notFoundBody = `    <style>
      .nf { display: grid; gap: 12px; }
      body.mk-app .nf { width: min(520px, calc(100vw - 32px)); margin: 12vh auto 0; }
      .nf-path {
        margin: 0; padding: 8px 12px; border-radius: 8px;
        font-family: var(--mk-mono); font-size: .8rem; overflow-wrap: anywhere;
        background: var(--mk-surface-container-high, var(--mk-surface-container));
        color: var(--mk-on-surface); border: 1px solid color-mix(in srgb, var(--mk-outline) 45%, transparent);
      }
      .nf .hint { margin: 0; font-size: .86rem; line-height: 1.5; white-space: pre-line; color: var(--mk-on-surface-variant); }
      .nf .public { margin-top: 4px; }
      .nf .public-links { justify-content: flex-start; }
    </style>
    <div class="panel nf">
      <p class="lead">{{.T.notFoundLead}}</p>
      <p class="nf-path">{{.Path}}</p>
      <p class="hint">{{.T.notFoundHint}}</p>
      {{if .Way}}<p class="hint">{{.Way}}</p>{{end}}
      {{if .Apps}}<div class="public">
        <nav class="public-links">
          {{range .Apps}}{{if .Head}}<span class="group">{{.Group}}</span>{{end}}<a href="{{.Href}}">{{.Name}}</a>{{end}}
        </nav>
      </div>{{end}}
      {{if .SignIn}}<p class="hint">{{.T.notFoundSignIn}}</p>
      <a class="choice" href="/login">{{.T.signIn}}</a>{{end}}
    </div>
    {{if .Back}}<p class="back"><a href="{{.Back}}">{{.T.back}}</a></p>{{end}}
    {{if .Profile}}<p class="back"><a href="/profile">{{.T.profile}}</a></p>{{end}}
`

var notFoundPage = flowPage("not-found", notFoundBody)

// ServeNotFound writes the page, with status 404. The gateway calls it for a
// browser asking for a page (GET or HEAD, text/html accepted) that no route
// serves; it decides who gets it, this draws it.
//
// The links are absolute paths ("/login", "/profile"): the page answers for
// any address, and a relative link under /some/deep/page would resolve under
// /some/deep/.
func (h *Handler) ServeNotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, err := h.sm.Resolve(ctx, r)
	signedIn := err == nil && sess.Pending == ""

	grants := h.visitorGrants(ctx, sess, signedIn)
	chrome := h.flowData(r, "titleNotFound")
	// Behind the portal only when the bar has something to show this
	// visitor: a bar with no entry draws nothing at all, not even its
	// account button, and "choose from the menu" over no menu is a dead end.
	// The page then offers its own way on, as it does without a portal.
	portal := false
	if behind := h.BehindPortal(r, chrome); behind.BodyStart != "" &&
		len(h.catalogueAt(ctx, grants, routeEntryPath)) > 0 {
		// An application's place, under the bar: the bar carries the brand,
		// and the page is a screen rather than a step of the sign-in.
		chrome, portal = AppPage(behind), true
	}
	data := notFoundData{
		flowChrome: chrome,
		Path:       r.URL.Path,
		Back:       template.URL(backFromReferer(r)), //nolint:gosec // same-host path only, see backFromReferer
	}
	t := chrome.T
	if portal {
		data.Way = t["notFoundMenu"]
	} else {
		data.Apps = h.openApps(r, grants)
		data.SignIn, data.Profile = !signedIn, signedIn
		switch {
		case len(data.Apps) > 0:
			data.Way = t["notFoundPick"]
		case signedIn:
			// Nothing open to them at all: the refusal page's own words.
			data.Way = t["refusedApps"] + " " + t["refusedHint"]
		}
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	writeFlow(w, notFoundPage, data, http.StatusNotFound)
}

// visitorGrants is the test a route's access puts to this visitor. One who is
// not signed in is offered what the sign-in page offers - the routes with no
// gate at all, since a link that leads to the sign-in page is not an
// application.
func (h *Handler) visitorGrants(ctx context.Context, sess store.Session, signedIn bool) func(store.Route) bool {
	if !signedIn {
		return func(rt store.Route) bool { return rt.Access.Empty() }
	}
	caller := h.sessionCaller(ctx, sess)
	return func(rt store.Route) bool { return rt.Access.Grants(caller) }
}

// openApps are the applications a visitor may open, as the profile's
// Applications list offers them: the catalogue when the installation has one,
// the UI routes otherwise.
func (h *Handler) openApps(r *http.Request, grants func(store.Route) bool) []publicLink {
	ctx := r.Context()
	addr := func(rt store.Route) string { return routeAddress(r, rt) }
	if links := h.catalogueAt(ctx, grants, addr); len(links) > 0 {
		return links
	}
	routes, err := h.st.ListRoutes(ctx)
	if err != nil {
		return nil
	}
	return openRoutes(routes, grants, addr)
}
