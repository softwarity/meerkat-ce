package admin

import (
	"net/http"
	"strings"
)

// The cross-site guard on control-plane writes (SEC-01).
//
// A browser session rides a cookie, and a cookie is attached by the browser to
// EVERY request to this origin - including one a page on attacker.example
// triggers. `SameSite=Lax` already stops the cross-site POST, which is the
// classic hole; this is the belt to that suspenders, and the thing an audit
// looks for by name: an explicit check that a state-changing request came from
// the console itself.
//
// It reads the two signals the browser sets and the page cannot forge:
//
//   - Sec-Fetch-Site, sent by every current browser. "same-origin" is the
//     console talking to itself; "none" is a typed URL or a bookmark, which
//     cannot be a forgery either. "cross-site" and "same-site" are refused.
//   - Origin, the older signal, for a browser that sent no Sec-Fetch-Site: it
//     must equal this plane's own origin.
//
// Neither present is a NON-browser client - curl, a script, a health probe -
// which is not a CSRF vector, because CSRF is a browser attaching a cookie a
// script did not choose. So it passes, and an operator's `curl --cookie` keeps
// working.
//
// A token call never reaches here: it is refused above unless it carries no
// cookie, and a Bearer token a browser does not attach cross-site cannot be a
// CSRF vector. Only cookie sessions are checked.
func crossSiteWrite(r *http.Request) bool {
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
	// No Fetch-Metadata: fall back to Origin. Absent means a non-browser
	// client (see above); present means it must be us.
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return !strings.EqualFold(origin, originOf(r))
}
