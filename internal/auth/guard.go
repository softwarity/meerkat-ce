package auth

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/filters"
)

// routes is the mux the sign-in pages, the profile and the agent consent mount
// on, with every WRITE behind the cross-site guard (audit P0-03).
//
// The guard used to sit on the control plane's API alone. These pages carry
// the same cookie on the same planes, and nothing checked who sent their
// POSTs: a page on the data plane of the same host is same-site to the console
// - a port does not make a site, SameSite=Lax attaches the cookie - so it could
// submit the consent form and hand an agent root's powers, or turn the second
// factor off. A wrapper on the mux rather than a line in each handler: a POST
// added later is guarded by being mounted, not by somebody remembering.
//
// Every response also says who may frame it: the page's own origin, which is
// how an integrator frames the sign-in in its application and how the console
// frames its previews, and nobody else - a foreign page cannot lay a "Connect"
// button under a decoy and steal the click. The consent asks for no frame at
// all (oauthAuthorize).
type routes struct{ mux Mux }

// Mux is what the auth endpoints are mounted on: a *http.ServeMux, or a test
// that records the patterns, since a ServeMux cannot be asked what it holds
// (cmd/meerkat/apidocs_test.go holds every plane to its specification).
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// crossSiteOpen are the writes a forgery cannot abuse, and which a foreign
// origin legitimately sends:
//   - the agent's own calls to the authorisation server - registration, the
//     code exchange, revocation - made by a program with no cookie;
//   - SAML's answer, posted to us by the identity provider's page, cross-site
//     by design, and relayed once from our own page (relayExternal).
var crossSiteOpen = map[string]bool{
	"POST /oauth/register":            true,
	"POST /oauth/token":               true,
	"POST /oauth/revoke":              true,
	"POST /login/{provider}/callback": true,
}

func (m routes) HandleFunc(pattern string, fn http.HandlerFunc) {
	m.Handle(pattern, fn)
}

func (m routes) Handle(pattern string, next http.Handler) {
	open := crossSiteOpen[pattern]
	m.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !open && filters.CrossSiteWrite(r) {
			http.Error(w, "refused: this request was sent by another site's page", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	}))
}
