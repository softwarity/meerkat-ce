package filters

import "net/http"

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
// X-Forwarded-Proto is trusted, which is a decision and not an oversight. A
// caller can send it, and what they buy is a cookie their own browser will
// then refuse to send back over http - they lock themselves out and nobody
// else. The alternative, a list of trusted proxies, is a setting to get wrong
// in exchange for stopping an attack on oneself. The rule is the one the
// product already applied in five places: a proxy that forwards a header a
// client sent is a proxy that has to be fixed.
//
// Note the asymmetry with ClientIP, which does NOT believe the leftmost
// X-Forwarded-For: there, believing the caller lets them invent an address per
// request and walk through a rate limit. The scheme has no such leverage.

// Secure reports whether the browser's hop was TLS.
func Secure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
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
