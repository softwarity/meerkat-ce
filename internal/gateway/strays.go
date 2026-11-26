package gateway

import (
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// Requests that went astray: a page served by a UI route published under a
// prefix asks for an address outside that prefix, and the request lands on
// another route's catch-all, or on nothing. That is what an application
// written for the root looks like from the gateway (routing/rootpaths.go) -
// and from the operator's side it looks like nothing at all: the page shows,
// half of it is missing, or a request carries what ANOTHER route forwards.
// So the gateway notes them, by route, and the route's screen says so.
//
// Known by the Referer: the page that asked. A request whose Referer is under
// /app and that itself is not, answered by no route, by a 404, or by a route
// that takes every path, is the app's. A call from that page to another
// application's own route is not: it landed where somebody meant it to.
//
// In memory, on the node that answered, and per REVISION of the route: what
// was noted before the route changed is about a configuration that no longer
// exists, and a switch just turned on must not keep showing the strays it was
// turned on to fix.

// strayWatch is what a UI route publishing under a stripped prefix is watched
// with, built at compile time.
type strayWatch struct {
	routeID string
	rev     int64
	// prefixes are the literal heads of its path patterns: "/app" for /app/**.
	prefixes []string
}

// Stray is one address, folded to its first segment, the route's pages asked
// for outside the route.
type Stray struct {
	// Path is the first segment ("/static"), or the whole path when it has
	// one segment only ("/spec.json"): what an operator recognises, without
	// one line per file.
	Path string `json:"path"`
	// Example is the last full path seen under it.
	Example string `json:"example"`
	Count   int64  `json:"count"`
	// LandedOn is the route that answered instead, by name; empty when no
	// route did.
	LandedOn string    `json:"landedOn,omitempty"`
	Status   int       `json:"status"`
	Last     time.Time `json:"last"`
}

// maxStrays bounds what one route keeps: the folding already makes a page's
// files one line, and past twenty lines the message is made.
const maxStrays = 20

type strayBook struct {
	mu      sync.Mutex
	byRoute map[string]*routeStrays
}

type routeStrays struct {
	rev   int64
	paths map[string]*Stray
}

// watchOf is the watch for a route, or nil when it publishes nothing under a
// stripped prefix: an application that keeps its prefix knows its addresses.
func watchOf(r store.Route) *strayWatch {
	if !r.IsUI || stripPrefixCount(r.Filters) == 0 {
		return nil
	}
	prefixes := routing.PathPrefixes(r.Predicates)
	if len(prefixes) == 0 {
		return nil
	}
	return &strayWatch{routeID: r.ID, rev: r.Rev, prefixes: prefixes}
}

// takesEverything reports whether a route matches any path: no path
// predicate, or a pattern with nothing literal ("/**"). A request landing
// there was not addressed to it.
func takesEverything(r store.Route) bool {
	heads := routing.PathHeads(r.Predicates)
	if len(heads) == 0 {
		return true
	}
	for _, list := range heads {
		if slices.Contains(list, "/") {
			return true
		}
	}
	return false
}

// noteStray looks at a request once it is answered. served is the route that
// answered, nil when none did. Costs a header read for a request without a
// Referer, and a loop over the watched routes otherwise.
func (rt *Router) noteStray(req *http.Request, served *compiledRoute, status int) {
	ref := req.Header.Get("Referer")
	if ref == "" {
		return
	}
	rt.mu.RLock()
	watches := rt.watches
	rt.mu.RUnlock()
	if len(watches) == 0 {
		return
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host != req.Host {
		return
	}
	if served != nil && !served.catchAll && status != http.StatusNotFound {
		return
	}
	for _, w := range watches {
		if served != nil && served.id == w.routeID {
			continue
		}
		from := ""
		for _, p := range w.prefixes {
			if pathUnder(u.Path, p) {
				from = p
				break
			}
		}
		if from == "" || pathUnder(req.URL.Path, from) {
			continue
		}
		landed := ""
		if served != nil {
			landed = served.name
		}
		rt.strays.note(w, req.URL.Path, landed, status)
		return
	}
}

func (b *strayBook) note(w *strayWatch, path, landed string, status int) {
	key := path
	if i := strings.IndexByte(path[1:], '/'); i >= 0 {
		key = path[:i+1]
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.byRoute == nil {
		b.byRoute = map[string]*routeStrays{}
	}
	rs := b.byRoute[w.routeID]
	if rs == nil || rs.rev != w.rev {
		rs = &routeStrays{rev: w.rev, paths: map[string]*Stray{}}
		b.byRoute[w.routeID] = rs
	}
	s := rs.paths[key]
	if s == nil {
		if len(rs.paths) >= maxStrays {
			return
		}
		s = &Stray{Path: key}
		rs.paths[key] = s
	}
	s.Example, s.LandedOn, s.Status, s.Last = path, landed, status, time.Now().UTC()
	s.Count++
}

// Strays are what the pages of a route asked for outside it, since its
// current revision, most frequent first. Empty for a route nobody watches.
func (rt *Router) Strays(routeID string) []Stray {
	rt.mu.RLock()
	var w *strayWatch
	for _, x := range rt.watches {
		if x.routeID == routeID {
			w = x
		}
	}
	rt.mu.RUnlock()
	if w == nil {
		return []Stray{}
	}
	rt.strays.mu.Lock()
	defer rt.strays.mu.Unlock()
	rs := rt.strays.byRoute[routeID]
	if rs == nil || rs.rev != w.rev {
		return []Stray{}
	}
	out := make([]Stray, 0, len(rs.paths))
	for _, s := range rs.paths {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Path < out[j].Path
	})
	return out
}
