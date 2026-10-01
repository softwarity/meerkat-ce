package filters

import (
	"net/http"
	"testing"
)

// A page the gateway rewrote must not be kept under the upstream's rules: the
// bytes are not the upstream's any more, and the configuration they carry has
// a lifetime the upstream knows nothing about.
func TestARewrittenPageIsNotCachedUnderTheUpstreamsRules(t *testing.T) {
	h := http.Header{}
	h.Set("Cache-Control", "public, max-age=600")
	h.Set("Expires", "Wed, 21 Oct 2026 07:28:00 GMT")
	h.Set("Pragma", "cache")
	h.Set("Last-Modified", "Tue, 20 Oct 2026 07:28:00 GMT")

	Rewritten(h)

	if got := h.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	// Each of these can say the opposite on its own, and an old intermediary
	// that ignores Cache-Control still honours a date in the future.
	for _, k := range []string{"Expires", "Pragma", "Last-Modified"} {
		if v := h.Get(k); v != "" {
			t.Errorf("%s survived: %q", k, v)
		}
	}
}

// no-cache, not no-store: no-store also disables the back/forward cache in
// Chrome and Firefox, which would make every Back on every proxied page a full
// round trip - a cost paid on every navigation for nothing extra.
func TestARewrittenPageMayStillBeStored(t *testing.T) {
	h := http.Header{}
	Rewritten(h)
	if got := h.Get("Cache-Control"); got == "no-store" {
		t.Errorf("Cache-Control = %q: no-store costs the bfcache and buys nothing here", got)
	}
}

// A page carrying somebody's identity is marked no-store, and rewriting it
// must never quietly weaken that: the two run on the same response, and this
// one runs second.
func TestRewritingNeverWeakensAPersonalPage(t *testing.T) {
	h := http.Header{}
	Personal(h)
	Rewritten(h)
	if got := h.Get("Cache-Control"); got != "no-store, private" {
		t.Errorf("Cache-Control = %q, want the personal marking untouched", got)
	}
}

// Personal is the stronger of the two and keeps saying so.
func TestPersonalDropsTheValidatorsToo(t *testing.T) {
	h := http.Header{}
	h.Set("ETag", `"abc"`)
	h.Set("Last-Modified", "Tue, 20 Oct 2026 07:28:00 GMT")
	Personal(h)
	if h.Get("ETag") != "" || h.Get("Last-Modified") != "" {
		t.Errorf("a validator survived: %v", h)
	}
}
