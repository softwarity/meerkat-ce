package routing

import (
	"fmt"
	"strings"
)

// pathPattern matches request paths segment by segment. Supported syntax:
//
//	/api/users          exact segments
//	/api/users/{id}     {name} matches exactly one segment
//	/static/**          ** (last segment only) matches zero or more segments
//
// Matching is on segment boundaries - /demo/** matches /demo and /demo/x,
// never /demolition.
type pathPattern struct {
	raw      string
	segments []string
	tail     bool // trailing **
}

func compilePathPattern(raw string) (pathPattern, error) {
	if !strings.HasPrefix(raw, "/") {
		return pathPattern{}, fmt.Errorf("path pattern %q must start with /", raw)
	}
	segs := splitPath(raw)
	p := pathPattern{raw: raw, segments: segs}
	for i, s := range segs {
		switch {
		case s == "**":
			if i != len(segs)-1 {
				return pathPattern{}, fmt.Errorf("path pattern %q: ** is only allowed as the last segment", raw)
			}
			p.tail = true
			p.segments = segs[:i]
		case s == "":
			return pathPattern{}, fmt.Errorf("path pattern %q has an empty segment", raw)
		case strings.HasPrefix(s, "{") != strings.HasSuffix(s, "}"):
			return pathPattern{}, fmt.Errorf("path pattern %q: malformed variable segment %q", raw, s)
		}
	}
	return p, nil
}

// match walks the path segment by segment WITHOUT cutting it into a slice.
//
// This runs once per route on every request, and splitPath allocates. On a
// table of two hundred routes that was two hundred slices built from the same
// string to answer the same question, and a profile of route selection put
// 82% of everything it allocated in strings.Split. The walk below is the same
// comparison with an index instead of a slice: same segment boundaries, same
// treatment of an empty segment, nothing on the heap.
func (p pathPattern) match(path string) bool {
	rest := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/")
	if rest == "" {
		// No segments at all: only a pattern that wants none can take it,
		// tail or no tail - "/api/**" does not match "/".
		return len(p.segments) == 0
	}
	// `more` and not `rest != ""`: a rest of "/" still holds ONE segment, an
	// empty one, and "///" against "/{x}" is where the difference shows. The
	// slice this replaces counted that segment, so this has to as well.
	more := true
	for _, want := range p.segments {
		if !more {
			return false // the path ran out before the pattern did
		}
		var seg string
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			seg, rest = rest[:i], rest[i+1:]
		} else {
			seg, rest, more = rest, "", false
		}
		// A variable segment takes whatever is there, including an empty one:
		// splitPath yields one for "//", and the two must agree.
		if !strings.HasPrefix(want, "{") && seg != want {
			return false
		}
	}
	// The pattern is satisfied. Without a tail the path has to end here too.
	return p.tail || !more
}

// splitPath cuts a path into segments, ignoring a single trailing slash
// ("/demo/" and "/demo" are the same route from a user's point of view).
func splitPath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// CompiledPath is an exported, precompiled path matcher for callers outside the
// package (endpoint-level security precompiles each OpenAPI operation path at
// reload time). It wraps the internal pathPattern so the matching semantics
// stay identical to route predicates: {name} matches one segment, ** a tail.
type CompiledPath struct{ p pathPattern }

// CompilePath compiles an OpenAPI operation path template (e.g. /users/{id})
// into a reusable matcher. Operation paths never carry a ** tail, but the same
// engine handles them.
func CompilePath(raw string) (CompiledPath, error) {
	p, err := compilePathPattern(raw)
	return CompiledPath{p}, err
}

// Match reports whether a concrete request path satisfies the template.
func (c CompiledPath) Match(path string) bool { return c.p.match(path) }

// StripSegments removes the first n segments of a path - the strip-prefix
// filter. Stripping more segments than the path has yields "/".
func StripSegments(path string, n int) string {
	segs := splitPath(path)
	if n >= len(segs) {
		return "/"
	}
	return "/" + strings.Join(segs[n:], "/")
}

// PathPrefixes returns the LITERAL head of each path pattern the specs carry:
// the segments before the first variable or **. "/app/**" gives "/app",
// "/a/{id}/b" gives "/a", "/**" gives "" (nothing is literal).
//
// It exists for the locale path mechanism, which inserts the language AFTER
// what the route matched on: a request the predicate accepted as /app/route
// must reach its upstream as /app/fr/route, not /fr/app/route - the upstream
// serves the application, and the application lives under /app.
func PathPrefixes(specs []Spec) []string {
	var out []string
	for _, spec := range specs {
		if spec.Type != "path" {
			continue
		}
		for _, raw := range allStrings(spec.Args, "patterns") {
			var head []string
			for _, seg := range splitPath(raw) {
				if seg == "**" || strings.HasPrefix(seg, "{") {
					break
				}
				head = append(head, seg)
			}
			if len(head) > 0 {
				out = append(out, "/"+strings.Join(head, "/"))
			}
		}
	}
	return out
}

// allStrings reads a string-list argument in the three shapes a spec may carry
// it (one string, []any from JSON, []string from Go).
func allStrings(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		var out []string
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return nil
}

// MatchPrefix is the static head a client's path must carry to enter a route
// ("/demo/**" -> "/demo"): the first path predicate's first pattern, cut at the
// first segment that matches more than itself. It is where a route's own
// documents live publicly - the spec it serves, and the base its operations
// are reachable from - so the gateway and the console must compute it the same
// way, from here.
func MatchPrefix(specs []Spec) string {
	for _, spec := range specs {
		if spec.Type != "path" {
			continue
		}
		patterns := allStrings(spec.Args, "patterns")
		if len(patterns) == 0 {
			break
		}
		var kept []string
		for _, seg := range splitPath(patterns[0]) {
			if strings.ContainsAny(seg, "*{") {
				break
			}
			kept = append(kept, seg)
		}
		if len(kept) == 0 {
			return ""
		}
		return "/" + strings.Join(kept, "/")
	}
	return ""
}
