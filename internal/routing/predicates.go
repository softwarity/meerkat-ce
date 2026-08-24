package routing

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Predicate decides whether a request matches. All predicates of a route
// must match (AND).
type Predicate func(*http.Request) bool

// predicateDef describes one predicate type: its schema and its compiler.
type predicateDef struct {
	Type    string
	Doc     string
	Details string
	Ref     *Reference
	Params  []Param
	compile func(a decoded) (Predicate, error)
}

// compiledPredicate keeps the SPEC next to the executable form: a route probe
// has to say WHICH predicate refused a request, not just that one did.
type compiledPredicate struct {
	spec Spec
	fn   Predicate
}

// CompiledPredicates is the executable form of a route's predicate list.
type CompiledPredicates struct {
	preds   []compiledPredicate
	weights []*pendingWeight
}

// Match reports whether every predicate accepts the request.
func (c CompiledPredicates) Match(r *http.Request) bool {
	for _, p := range c.preds {
		if !p.fn(r) {
			return false
		}
	}
	return true
}

// Verdict is one predicate's answer about one request (route probe).
type Verdict struct {
	Type    string         `json:"type"`
	Args    map[string]any `json:"args,omitempty"`
	Matched bool           `json:"matched"`
}

// Explain evaluates EVERY predicate and reports each verdict in order. It does
// not stop at the first refusal: an admin chasing a routing surprise wants the
// whole picture, and evaluating a predicate costs nothing.
func (c CompiledPredicates) Explain(r *http.Request) []Verdict {
	out := make([]Verdict, 0, len(c.preds))
	for _, p := range c.preds {
		out = append(out, Verdict{Type: p.spec.Type, Args: p.spec.Args, Matched: p.fn(r)})
	}
	return out
}

// HasWeight reports whether this route participates in a weight group.
func (c CompiledPredicates) HasWeight() bool { return len(c.weights) > 0 }

// CompilePredicates turns specs into an executable matcher. Weight
// predicates need a second pass across ALL routes: call ResolveWeights once
// every route of a snapshot is compiled.
func CompilePredicates(specs []Spec) (CompiledPredicates, error) {
	var out CompiledPredicates
	for _, s := range specs {
		def, ok := predicateRegistry[s.Type]
		if !ok {
			return out, fmt.Errorf("unknown predicate type %q (available: %s)", s.Type, knownPredicates())
		}
		args, err := decodeArgs("predicate "+s.Type, def.Params, s.Args)
		if err != nil {
			return out, err
		}
		if s.Type == "weight" {
			pw := &pendingWeight{group: args.str("group"), weight: args.num("weight")}
			out.weights = append(out.weights, pw)
			out.preds = append(out.preds, compiledPredicate{spec: s, fn: pw.match})
			continue
		}
		p, err := def.compile(args)
		if err != nil {
			return out, err
		}
		out.preds = append(out.preds, compiledPredicate{spec: s, fn: p})
	}
	return out, nil
}

// ---- weight groups ---------------------------------------------------------
//
// Weight predicates split traffic between routes of a group (canary). Each
// request draws one lottery value in [0,1) - injected by the router - and a
// route matches when the value falls in its cumulative share of the group.

type pendingWeight struct {
	group  string
	weight int
	lo, hi float64 // resolved by ResolveWeights
}

func (w *pendingWeight) match(r *http.Request) bool {
	v, ok := lotteryFrom(r.Context())
	if !ok {
		return false // router did not inject a lottery - misconfiguration
	}
	return v >= w.lo && v < w.hi
}

// ResolveWeights computes each route's share of its group, in the order the
// routes appear in the snapshot.
func ResolveWeights(all []*CompiledPredicates) error {
	totals := map[string]int{}
	for _, cp := range all {
		for _, w := range cp.weights {
			if w.weight <= 0 {
				return fmt.Errorf("weight group %q: weight must be > 0", w.group)
			}
			totals[w.group] += w.weight
		}
	}
	cursor := map[string]float64{}
	for _, cp := range all {
		for _, w := range cp.weights {
			share := float64(w.weight) / float64(totals[w.group])
			w.lo = cursor[w.group]
			w.hi = w.lo + share
			cursor[w.group] = w.hi
		}
	}
	return nil
}

type clockKey struct{}

// WithClock pins the instant the time predicates compare against. Unset means
// time.Now(); the route probe sets it so an admin can ask "would this match at
// 3am next Sunday?" without waiting for Sunday.
func WithClock(ctx context.Context, at time.Time) context.Context {
	return context.WithValue(ctx, clockKey{}, at)
}

func nowFrom(r *http.Request) time.Time {
	if t, ok := r.Context().Value(clockKey{}).(time.Time); ok {
		return t
	}
	return time.Now()
}

type lotteryKey struct{}

// WithLottery attaches the request's lottery draw for weight predicates.
func WithLottery(ctx context.Context, v float64) context.Context {
	return context.WithValue(ctx, lotteryKey{}, v)
}

func lotteryFrom(ctx context.Context) (float64, bool) {
	v, ok := ctx.Value(lotteryKey{}).(float64)
	return v, ok
}

// ---- predicate catalog -----------------------------------------------------

var predicateRegistry = map[string]predicateDef{}

func registerPredicate(def predicateDef) { predicateRegistry[def.Type] = def }

func knownPredicates() string {
	names := make([]string, 0, len(predicateRegistry))
	for n := range predicateRegistry {
		names = append(names, n)
	}
	return joinSorted(names)
}

func init() {
	registerPredicate(predicateDef{
		Type:    "path",
		Doc:     "Matches the request path against one or more patterns.",
		Details: "Patterns match path segments: /orders/* is one segment, /orders/** is everything below, {id} captures one segment. Several patterns act as OR.",
		Params: []Param{
			{Name: "patterns", Kind: KindStringList, Required: true, Doc: "e.g. /api/users/{id}, /static/**"},
		},
		compile: func(a decoded) (Predicate, error) {
			raw := a.strs("patterns")
			pats := make([]pathPattern, len(raw))
			for i, r := range raw {
				p, err := compilePathPattern(r)
				if err != nil {
					return nil, err
				}
				pats[i] = p
			}
			return func(r *http.Request) bool {
				for _, p := range pats {
					if p.match(r.URL.Path) {
						return true
					}
				}
				return false
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "host",
		Doc:     "Matches the request Host against exact names or *.suffix wildcards.",
		Details: "Matches the Host sent by the caller, port ignored. A wildcard covers subdomains only: *.example.com matches app.example.com but not example.com.",
		Params: []Param{
			{Name: "hosts", Kind: KindStringList, Required: true, Doc: "e.g. app.example.com, *.example.com"},
		},
		compile: func(a decoded) (Predicate, error) {
			hosts := a.strs("hosts")
			return func(r *http.Request) bool {
				h := hostname(r.Host)
				for _, want := range hosts {
					if suffix, ok := strings.CutPrefix(want, "*."); ok {
						if strings.HasSuffix(h, "."+suffix) {
							return true
						}
					} else if h == want {
						return true
					}
				}
				return false
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "method",
		Doc:     "Matches the HTTP method.",
		Details: "Matches the HTTP verb. Several verbs act as OR.",
		Params: []Param{
			{Name: "methods", Kind: KindStringList, Required: true, Doc: "e.g. GET, POST"},
		},
		compile: func(a decoded) (Predicate, error) {
			set := map[string]bool{}
			for _, m := range a.strs("methods") {
				set[strings.ToUpper(m)] = true
			}
			return func(r *http.Request) bool { return set[r.Method] }, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "header",
		Doc:     "Matches when a header is present, against a list of values or a regexp.",
		Details: "Matches when the header is present. Give values to accept a short list, or a regexp when the value has a shape rather than a list. Header names are case-insensitive, values are not.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "values", Kind: KindStringList, Doc: "e.g. prod, staging - the value must be one of them"},
			{Name: "regexp", Kind: KindString, Doc: "full-match regexp on the value; use it for a shape, not for a list"},
		},
		compile: compileNamedValue(func(r *http.Request, name string) (string, bool) {
			v := r.Header.Get(name)
			return v, v != ""
		}),
	})

	registerPredicate(predicateDef{
		Type:    "cookie",
		Doc:     "Matches when a cookie is present, against a list of values or a regexp.",
		Details: "Matches when the cookie is present. Give values to accept a short list, or a regexp for a shape. Useful to pin a caller to one side of a canary, since a cookie lasts the whole session.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "values", Kind: KindStringList, Doc: "the cookie value must be one of them"},
			{Name: "regexp", Kind: KindString, Doc: "full-match regexp on the value; use it for a shape, not for a list"},
		},
		compile: compileNamedValue(func(r *http.Request, name string) (string, bool) {
			c, err := r.Cookie(name)
			if err != nil {
				return "", false
			}
			return c.Value, true
		}),
	})

	registerPredicate(predicateDef{
		Type:    "query",
		Doc:     "Matches when a query parameter is present, against a list of values or a regexp.",
		Details: "Matches when the query parameter is present, even with an empty value. Give values to accept a short list, or a regexp for a shape.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "values", Kind: KindStringList, Doc: "the parameter value must be one of them"},
			{Name: "regexp", Kind: KindString, Doc: "full-match regexp on the value; use it for a shape, not for a list"},
		},
		compile: compileNamedValue(func(r *http.Request, name string) (string, bool) {
			if !r.URL.Query().Has(name) {
				return "", false
			}
			return r.URL.Query().Get(name), true
		}),
	})

	registerPredicate(predicateDef{
		Type:    "remote-addr",
		Doc:     "Matches the client address against CIDR ranges.",
		Details: "Matches the client IP against CIDR ranges, for example 10.0.0.0/8 or 192.168.1.10/32. Behind another proxy this is the proxy's IP: use x-forwarded-remote-addr instead.",
		Params: []Param{
			{Name: "cidrs", Kind: KindStringList, Required: true, Doc: "e.g. 10.0.0.0/8, 192.168.1.10/32"},
			{Name: "useForwarded", Kind: KindBool, Default: false, Doc: "trust the first X-Forwarded-For entry (only behind a trusted proxy)"},
		},
		compile: func(a decoded) (Predicate, error) {
			prefixes := make([]netip.Prefix, 0, len(a.strs("cidrs")))
			for _, c := range a.strs("cidrs") {
				p, err := netip.ParsePrefix(c)
				if err != nil {
					return nil, fmt.Errorf("remote-addr: bad cidr %q: %w", c, err)
				}
				prefixes = append(prefixes, p)
			}
			useForwarded := a.boolean("useForwarded")
			return func(r *http.Request) bool {
				addr, ok := clientAddr(r, useForwarded)
				if !ok {
					return false
				}
				for _, p := range prefixes {
					if p.Contains(addr) {
						return true
					}
				}
				return false
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "x-forwarded-remote-addr",
		Doc:     "Matches the rightmost X-Forwarded-For address against CIDR ranges - the address the LAST proxy reports; use behind a trusted front proxy.",
		Details: "Matches the last address of X-Forwarded-For against CIDR ranges. Use it only when a trusted proxy writes that header, since a client can send any value.",
		Params: []Param{
			{Name: "cidrs", Kind: KindStringList, Required: true, Doc: "e.g. 10.0.0.0/8, 192.168.1.10/32"},
		},
		compile: func(a decoded) (Predicate, error) {
			prefixes := make([]netip.Prefix, 0, len(a.strs("cidrs")))
			for _, c := range a.strs("cidrs") {
				p, err := netip.ParsePrefix(c)
				if err != nil {
					return nil, fmt.Errorf("x-forwarded-remote-addr: bad cidr %q: %w", c, err)
				}
				prefixes = append(prefixes, p)
			}
			return func(r *http.Request) bool {
				xff := r.Header.Get("X-Forwarded-For")
				if xff == "" {
					return false
				}
				parts := strings.Split(xff, ",")
				addr, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1]))
				if err != nil {
					return false
				}
				for _, p := range prefixes {
					if p.Contains(addr) {
						return true
					}
				}
				return false
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "time-window",
		Doc:     "Matches requests made inside a time window.",
		Details: "Opens or closes a route at a date: a migration that starts Monday, an offer that ends at midnight. Outside the window the route does not match, so the next matching route answers - or nobody, and that is a 404.",
		Ref:     &Reference{Label: "RFC 3339", URL: "https://www.rfc-editor.org/rfc/rfc3339"},
		Params: []Param{
			{Name: "from", Kind: KindString, Doc: "RFC 3339, e.g. 2026-01-20T17:42:47+01:00 - the offset travels with it"},
			{Name: "to", Kind: KindString, Doc: "RFC 3339; must be after from"},
		},
		compile: func(a decoded) (Predicate, error) {
			from, to := a.str("from"), a.str("to")
			if from == "" && to == "" {
				return nil, fmt.Errorf("time-window: give from, to, or both - a window open at both ends matches every moment, which is what having no time predicate already does")
			}
			var start, end time.Time
			var err error
			if from != "" {
				if start, err = parseDatetime(from); err != nil {
					return nil, fmt.Errorf("time-window: from: %w", err)
				}
			}
			if to != "" {
				if end, err = parseDatetime(to); err != nil {
					return nil, fmt.Errorf("time-window: to: %w", err)
				}
			}
			if from != "" && to != "" && !end.After(start) {
				return nil, fmt.Errorf("time-window: to %s is not after from %s, so the window accepts nothing", to, from)
			}
			return func(r *http.Request) bool {
				now := nowFrom(r)
				if from != "" && !now.After(start) {
					return false
				}
				if to != "" && !now.Before(end) {
					return false
				}
				return true
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "version",
		Doc:     "Matches an API version range read from a header, a query parameter or the path.",
		Details: "Two routes on the same path, one per API version: v2 to the new service, the rest to the old one. Versions compare as numbers, so 1.10 comes after 1.9 - a regexp reads it as before. A request with no version does not match, so put the unversioned route underneath.",
		Params: []Param{
			{Name: "source", Kind: KindString, Default: "header", Options: []string{"header", "query", "path"},
				Doc: "where the version is read from"},
			{Name: "name", Kind: KindString, Default: "X-API-Version", Doc: "the header or parameter carrying the version; unused on path"},
			{Name: "pattern", Kind: KindString, Required: true, Literal: true, Initial: `v?(\d+(?:\.\d+)*)`,
				Doc: `extracts the version, e.g. /v?(\\d+(?:\\.\\d+)*)`},
			{Name: "from", Kind: KindString, Doc: "lowest version accepted, inclusive - e.g. 2.0"},
			{Name: "to", Kind: KindString, Doc: "first version REFUSED, exclusive - e.g. 3.0"},
		},
		compile: func(a decoded) (Predicate, error) {
			m, err := compileVersionMatcher(a)
			if err != nil {
				return nil, err
			}
			return func(r *http.Request) bool {
				_, ok := m.read(m.rawFrom(r))
				return ok
			}, nil
		},
	})

	registerPredicate(predicateDef{
		Type:    "weight",
		Doc:     "Splits traffic between the routes of a group (canary): a route takes weight/total of the requests.",
		Details: "Sends part of the traffic to a new version: 8 and 2 on two routes gives one request in five to the second. The draw happens per request, so one person sees both versions; a cookie predicate keeps them on one side.",
		Params: []Param{
			{Name: "group", Kind: KindString, Required: true},
			{Name: "weight", Kind: KindInt, Required: true},
		},
		compile: nil, // handled structurally in CompilePredicates
	})
}

// compileNamedValue factors the header/cookie/query predicates: extract a
// named value, then presence or full-regexp match.
func compileNamedValue(get func(*http.Request, string) (string, bool)) func(decoded) (Predicate, error) {
	return func(a decoded) (Predicate, error) {
		name := a.str("name")
		expr := a.str("regexp")
		values := a.strs("values")
		// One way or the other, not both: a list and a pattern on the same
		// value are two answers to the same question, and nobody would know
		// which one decides.
		if expr != "" && len(values) > 0 {
			return nil, fmt.Errorf("give values or regexp, not both: a list matches whole values, a regexp matches a shape")
		}
		var re *regexp.Regexp
		if expr != "" {
			var err error
			if re, err = regexp.Compile("^(?:" + expr + ")$"); err != nil {
				return nil, fmt.Errorf("bad regexp %q: %w", expr, err)
			}
		}
		return func(r *http.Request) bool {
			v, ok := get(r, name)
			if !ok {
				return false
			}
			if len(values) > 0 {
				return slices.Contains(values, v)
			}
			return re == nil || re.MatchString(v)
		}, nil
	}
}

// parseDatetime reads the after/before/between bounds: RFC 3339 only, so a
// bad value fails at validation, never silently at match time.
func parseDatetime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad datetime %q: use RFC 3339, like 2026-01-20T17:42:47+01:00", s)
	}
	return t, nil
}

func hostname(hostport string) string {
	if i := strings.LastIndexByte(hostport, ':'); i >= 0 && !strings.Contains(hostport[i:], "]") {
		return hostport[:i]
	}
	return hostport
}

func clientAddr(r *http.Request, useForwarded bool) (netip.Addr, bool) {
	if useForwarded {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if a, err := netip.ParseAddr(strings.TrimSpace(first)); err == nil {
				return a, true
			}
		}
	}
	a, err := netip.ParseAddr(hostname(r.RemoteAddr))
	return a, err == nil
}

// version is a dotted numeric version, compared segment by segment as NUMBERS.
// Three segments are enough for what this matches on, and a shorter one is
// padded: 2 is 2.0.0, so "from 2" takes 2.1 as well.
type version [3]int

// parseVersion reads 2, 2.1, v2.1.3. Anything else is refused HERE, at save
// time, rather than silently never matching at request time - except for what
// a caller sends, which cannot be validated in advance and simply does not
// match.
func parseVersion(s string) (version, error) {
	raw := strings.TrimSpace(s)
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "v"), "V")
	if raw == "" {
		return version{}, fmt.Errorf("%q is not a version", s)
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 3 {
		return version{}, fmt.Errorf("%q has more than three segments", s)
	}
	var out version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, fmt.Errorf("%q is not a version: segment %q is not a number", s, p)
		}
		out[i] = n
	}
	return out, nil
}

func compareVersions(a, b version) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

// versionMatcher is the version predicate's whole behaviour, in one place: the
// preview endpoint the console calls while someone types runs THIS, so what an
// editor shows and what the gateway will do cannot diverge. A preview computed
// in the browser would be a second implementation, and JavaScript regexps have
// lookarounds that Go's RE2 refuses - it would report a match on a pattern the
// gateway rejects at save.
type versionMatcher struct {
	source, name string
	re           *regexp.Regexp
	group        int // capture group holding the version, 0 when the whole match is it
	from, to     string
	lo, hi       version
}

func compileVersionMatcher(a decoded) (*versionMatcher, error) {
	m := &versionMatcher{source: a.str("source"), name: a.str("name"), from: a.str("from"), to: a.str("to")}
	if m.from == "" && m.to == "" {
		return nil, fmt.Errorf("version: give from, to, or both - a range open on both sides matches every version, which is what having no version predicate already does")
	}
	var err error
	if m.from != "" {
		if m.lo, err = parseVersion(m.from); err != nil {
			return nil, fmt.Errorf("version: from: %w", err)
		}
	}
	if m.to != "" {
		if m.hi, err = parseVersion(m.to); err != nil {
			return nil, fmt.Errorf("version: to: %w", err)
		}
	}
	if m.from != "" && m.to != "" && compareVersions(m.lo, m.hi) >= 0 {
		return nil, fmt.Errorf("version: from %s is not below to %s, so the range accepts nothing (to is exclusive)", m.from, m.to)
	}
	if pattern := a.str("pattern"); pattern != "" {
		if m.re, err = regexp.Compile(pattern); err != nil {
			return nil, fmt.Errorf("version: pattern: %w", err)
		}
		// The named group wins, the first capturing group does otherwise, and
		// with no group at all the whole match IS the version.
		for i, n := range m.re.SubexpNames() {
			if n == "version" {
				m.group = i
			}
		}
		if m.group == 0 && m.re.NumSubexp() > 0 {
			m.group = 1
		}
	}
	return m, nil
}

// rawFrom is the text the version is looked for in.
func (m *versionMatcher) rawFrom(r *http.Request) string {
	switch m.source {
	case "query":
		return r.URL.Query().Get(m.name)
	case "path":
		return r.URL.Path
	default:
		return r.Header.Get(m.name)
	}
}

// read extracts the version from raw and says whether it falls in the range.
// It returns what it extracted either way, because that is what a preview has
// to show: "extracted 1.2, outside [2.0 3.0[" is a different problem from
// "extracted nothing".
func (m *versionMatcher) read(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if m.re != nil {
		hit := m.re.FindStringSubmatch(raw)
		if hit == nil || m.group >= len(hit) {
			return "", false
		}
		raw = hit[m.group]
	}
	got, err := parseVersion(raw)
	if err != nil {
		return raw, false // a caller sending nonsense is not "any version"
	}
	if m.from != "" && compareVersions(got, m.lo) < 0 {
		return raw, false
	}
	if m.to != "" && compareVersions(got, m.hi) >= 0 {
		return raw, false
	}
	return raw, true
}

// PreviewVersion runs a version predicate's args against a sample, for the
// editor: what was extracted, whether it matches, and the engine's own error
// when the args do not compile.
func PreviewVersion(args map[string]any, sample string) (extracted string, matches bool, err error) {
	decodedArgs, err := decodeArgs("predicate version", predicateRegistry["version"].Params, args)
	if err != nil {
		return "", false, err
	}
	m, err := compileVersionMatcher(decodedArgs)
	if err != nil {
		return "", false, err
	}
	extracted, matches = m.read(sample)
	return extracted, matches, nil
}
