package filters

import (
	"net/http"
	"net/url"
	"strings"
)

// CrossSiteWrite says whether a state-changing request was sent by a page of
// ANOTHER origin, which is what a cross-site request forgery is: a browser
// attaching this origin's cookie to a request a foreign page chose.
//
// Written here so the two planes share one rule. The control plane's API
// guarded its writes with it (SEC-01) while the sign-in pages, the profile and
// the agent consent - the same cookie, on the same planes - had nothing: a page
// on the data plane of the same host is SAME-SITE to the console (a port does
// not make a site, and SameSite=Lax attaches the cookie), so it could submit
// the console's consent form and hand an agent root's powers (audit P0-03).
//
// It reads the two signals the browser sets and a page cannot forge:
//
//   - Sec-Fetch-Site, sent by every current browser. "same-origin" is the
//     origin talking to itself; "none" is a typed URL or a bookmark, which
//     cannot be a forgery either. "cross-site" AND "same-site" are refused:
//     same-site is precisely the neighbouring port or subdomain.
//   - Origin, the older signal, for a browser that sent no Sec-Fetch-Site: its
//     HOST must be the one the request is for. The host and not the whole
//     origin: behind a proxy that terminates TLS the scheme this side sees is
//     not the browser's, and a scheme is not what makes a forgery.
//
// Neither present is a NON-browser client - curl, a script, an agent - which
// is not a CSRF vector, because CSRF is a browser attaching a cookie a script
// did not choose. So it passes.
func CrossSiteWrite(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "cross-site", "same-site":
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	// "null" is what a sandboxed frame or a data: page sends: an origin that
	// is nobody's, and certainly not ours.
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return true
	}
	return !strings.EqualFold(u.Host, r.Host)
}
