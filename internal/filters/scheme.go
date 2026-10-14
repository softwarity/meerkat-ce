package filters

import (
	"net/http"
	"net/netip"
)

// The scheme rule: did this request reach the BROWSER over TLS, and under
// what address. Secure, Scheme and Origin are the three ways to ask.
//
// Written here beside ClientIP, for the same reason and against the same
// mistake: the answer decides a session cookie's Secure flag, a callback URL
// handed to an identity provider, the origin a WebAuthn ceremony is bound to,
// and whether the plain port redirects. It was written twelve times, six of
// them as `r.TLS != nil` alone - and behind a load balancer that terminates
// TLS r.TLS is nil on every request, so those six were all wrong at once:
// session cookies lost Secure, the console advertised http:// urls, and the
// redirect to https looped forever (terminate, forward plain, redirect,
// terminate, forward plain...).
//
// X-Forwarded-Proto is believed only from a TRUSTED PROXY - the same rule as
// X-Forwarded-For, in forwarded.go. It used to be believed from anybody, on
// the argument that a caller who lies about the scheme only locks themselves
// out. That holds for a cookie and not for everything else the answer decides:
// the plain port's redirect to https is skipped on the strength of it, and an
// origin built from it is what a callback URL or a WebAuthn ceremony is bound
// to. A proxy we trust says what the browser's hop was; a stranger's header
// says what the stranger wanted it to be.
//
// Behind a load balancer whose addresses are PUBLIC (a CDN, some cloud load
// balancers), the default does not trust it, and the gateway sees plain http:
// that installation declares the proxy's addresses on the Domains & TLS
// screen, on the plane it stands in front of (SSL-14).

// Secure reports whether the browser's hop was TLS.
//
// A name a proxy in front serves over HTTPS (Domains & TLS, SSL-14) is https
// on the operator's word, with no header to believe: the proxy may not write
// one, and nobody gains by forging a Host that leads to our own name.
func Secure(r *http.Request) bool {
	return r.TLS != nil || servedOverHTTPS(r) ||
		(r.Header.Get("X-Forwarded-Proto") == "https" && FromTrustedProxy(r))
}

// SecureUnder is what Secure would answer for r once a plane's door says
// something else: proxies believed (nil is the default) and the names a proxy
// in front serves over HTTPS. The Domains & TLS screen asks it before saving
// the console's door, so an operator never switches the door they are
// standing in into one that sees their own session as plain http.
func SecureUnder(r *http.Request, proxies []netip.Prefix, httpsNames []string) bool {
	if r.TLS != nil || servedOverHTTPSAs(r, httpsNames) {
		return true
	}
	if r.Header.Get("X-Forwarded-Proto") != "https" {
		return false
	}
	if proxies == nil {
		proxies = DefaultTrustedProxies
	}
	a, ok := peerAddr(r)
	if !ok {
		return false
	}
	for _, p := range proxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Scheme is "https" or "http".
func Scheme(r *http.Request) string {
	if Secure(r) {
		return "https"
	}
	return "http"
}

// Origin is scheme://host, the address the browser used.
func Origin(r *http.Request) string {
	return Scheme(r) + "://" + r.Host
}
