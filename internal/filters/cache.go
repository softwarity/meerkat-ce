package filters

import (
	"net/http"
	"strings"
)

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

// Rewritten marks a response whose BODY the gateway changed.
//
// The upstream's caching headers describe the upstream's bytes, and these are
// not them - the same reason RewriteBody drops the ETag just above. A static
// file server sending its index.html as `max-age=600` is describing a document
// that did not carry a user button, a portal bar, a tracing bundle or a
// stylesheet of yours. Keep that header and the visitor holds ten minutes of
// the PREVIOUS configuration: tick a box in the console, reload, see nothing,
// and conclude the box does not work. Which is exactly what happened.
//
// no-cache rather than no-store: the browser may keep the copy, it simply may
// not use it without asking. no-store would also disable the back/forward
// cache in Chrome and Firefox, making every Back on every proxied page a full
// round trip - a real cost, paid on every navigation, to fix a problem
// no-cache already fixes.
//
// A response already marked Personal is left alone: no-store is stronger, it
// is there because the bytes belong to one person, and this must never quietly
// weaken it.
func Rewritten(h http.Header) {
	if strings.Contains(strings.ToLower(h.Get("Cache-Control")), "no-store") {
		return
	}
	h.Set("Cache-Control", "no-cache")
	h.Del("Expires")
	h.Del("Pragma")
	// Heuristic freshness is computed from this when nothing else says
	// otherwise, so a date left behind reintroduces the cache we just removed.
	h.Del("Last-Modified")
}
