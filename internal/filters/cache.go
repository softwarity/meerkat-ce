package filters

import "net/http"

// Personal marks a response the gateway has written somebody's IDENTITY into.
//
// The gateway stamps a signed-in person's name, roles and fields straight into
// the HTML it passes through (UIF-03). From that moment the bytes belong to one
// person, and the upstream's own caching headers describe a document that no
// longer exists: an application serving its index.html as
// `public, max-age=300` - the default of every static file server - would have
// a CDN, a company proxy or a shared browser hand Alice's page to Bob. The
// stamp is invisible to all of them; they see HTML that said it was public.
//
// no-store rather than private: `private` only asks SHARED caches to abstain,
// and the store that hurts most here is the one on a machine two people use.
// The cost is a round trip per navigation on a document that is, by
// construction, different for every visitor.
//
// Expires and Pragma go because they can say the opposite: an old intermediary
// that ignores Cache-Control still honours a date in the future.
func Personal(h http.Header) {
	h.Set("Cache-Control", "no-store, private")
	h.Del("Expires")
	h.Del("Pragma")
	// The upstream's validators describe the upstream's bytes, and these are
	// not them. RewriteBody already drops the ETag for the same reason; a date
	// left behind is what heuristic freshness is computed from.
	h.Del("Last-Modified")
	h.Del("ETag")
}
