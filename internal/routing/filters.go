package routing

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"strings"

	"github.com/softwarity/meerkat/internal/filters"
)

// ForwardedPrefixHeader tells a service the public prefix it is published
// under - the segments strip-prefix took off. A service that only ever answers
// does not need it; one that BUILDS a url does, and it has no other way to
// know: it sees /orders and writes /orders, which lands outside the route.
//
// Convention rather than standard, like REMOTE_USER (ROUTE-19): nothing at the
// IANA, and RFC 7239 normalises Forwarded without a notion of prefix. Spring
// reads it through ForwardedHeaderFilter, nginx poses it, which is what makes
// it the right answer here.
//
// It is PURGED on the way in (see the proxy rewrite): a caller who poses their
// own would make the service write its links wherever they asked.
const ForwardedPrefixHeader = "X-Forwarded-Prefix"

// RequestFilter mutates the outgoing (proxied) request.
type RequestFilter func(*httputil.ProxyRequest)

// Gate decides whether a request goes on at all. It returns false when it has
// ANSWERED the caller itself - the refusal is written, nothing further runs.
//
// This is what a modifier cannot be. A modifier takes a request and transforms
// it; its signature carries neither an error nor a response, and it runs inside
// the proxy's Rewrite, where there is no ResponseWriter to answer with. A gate
// runs before all of that, in the handler chain, so it can turn a caller away
// with a status they can act on.
//
// A gate that refuses never passes the hand on, and that is what tells it from
// the route's other two deciders: a predicate that does not match, and an
// access rule that does not grant, both let the NEXT route try. Too large is
// not "not for this route", it is no.
type Gate func(http.ResponseWriter, *http.Request) bool

// ResponseFilter mutates the upstream response before it reaches the client.
type ResponseFilter func(*http.Response) error

// CompiledFilters is the executable form of a route's filter list. When
// Terminal is set the route never proxies (e.g. redirect).
type CompiledFilters struct {
	// Gates run FIRST and in declared order, around everything else: what a
	// route refuses to carry is decided before a modifier touches anything.
	Gates    []Gate
	Request  []RequestFilter
	Response []ResponseFilter
	Terminal http.Handler
	// IgnoredFilters counts the INCOMING filters dropped because a terminal
	// answers the route and nothing is proxied (see CompileFilters).
	IgnoredFilters int
	// TerminalNeedsIdentity says the terminal answers FROM the caller, so the
	// engine must resolve the session and carry it in (routing.WithIdentity).
	// Declared rather than assumed: resolving a session costs a read, and
	// redirect and maintenance have no use for one.
	TerminalNeedsIdentity bool
}

type filterPhase string

const (
	// phaseGate: bricks that accept or refuse. They sit beside predicates and
	// the access rule in the console, under Filters - the three that decide a
	// request's fate before anything transforms it.
	phaseGate     filterPhase = "gate"
	phaseRequest  filterPhase = "request"
	phaseResponse filterPhase = "response"
	phaseTerminal filterPhase = "terminal"
)

type filterDef struct {
	Type    string
	Doc     string
	Details string
	Ref     *Reference
	Phase   filterPhase
	Params  []Param

	compileGate     func(a decoded) (Gate, error)
	compileRequest  func(a decoded) (RequestFilter, error)
	compileResponse func(a decoded) (ResponseFilter, error)
	compileTerminal func(a decoded) (http.Handler, error)
	// needsIdentity: the terminal reads the caller (see CompiledFilters).
	needsIdentity bool
}

// CompileFilters turns specs into executable chains, applied in declared
// order within each phase.
func CompileFilters(specs []Spec) (CompiledFilters, error) {
	var out CompiledFilters
	terminalType := ""
	for _, s := range specs {
		def, ok := filterRegistry[s.Type]
		if !ok {
			return out, fmt.Errorf("unknown filter type %q (available: %s)", s.Type, knownFilters())
		}
		args, err := decodeArgs("filter "+s.Type, def.Params, s.Args)
		if err != nil {
			return out, err
		}
		switch def.Phase {
		case phaseGate:
			g, err := def.compileGate(args)
			if err != nil {
				return out, err
			}
			out.Gates = append(out.Gates, g)
		case phaseRequest:
			f, err := def.compileRequest(args)
			if err != nil {
				return out, err
			}
			out.Request = append(out.Request, f)
		case phaseResponse:
			f, err := def.compileResponse(args)
			if err != nil {
				return out, err
			}
			out.Response = append(out.Response, f)
		case phaseTerminal:
			if out.Terminal != nil {
				return out, fmt.Errorf("filter %s: a route can have only one terminal filter", s.Type)
			}
			h, err := def.compileTerminal(args)
			if err != nil {
				return out, err
			}
			out.Terminal = h
			out.TerminalNeedsIdentity = def.needsIdentity
			terminalType = s.Type
		}
	}
	// A terminal answers the route itself. INCOMING filters then have nothing
	// to modify - the request is never proxied - so they are dropped. OUTGOING
	// ones still apply: the answer is a response like any other, and putting a
	// CORS or a Cache-Control header on it is exactly as legitimate as on a
	// proxied one.
	//
	// Both used to be an error, which meant an existing route could not be
	// switched to an answering mode without first hunting down its modifiers in
	// two other sections - and the refusal named "redirect" whatever the
	// terminal actually was.
	// Gates are NOT dropped here, deliberately: they transform nothing, so a
	// route that answers by itself has exactly as much reason to refuse an
	// oversized request as one that proxies.
	if out.Terminal != nil && len(out.Request) > 0 {
		out.IgnoredFilters = len(out.Request)
		out.Request = nil
		slog.Warn("incoming filters ignored: this route answers by itself, nothing is proxied",
			"terminal", terminalType, "ignored", out.IgnoredFilters)
	}
	return out, nil
}

// LiteralArgs lists the arguments of a filter that must reach the brick
// untouched (see Param.Literal). Unknown filter: nothing to protect.
func LiteralArgs(filterType string) []string {
	def, ok := filterRegistry[filterType]
	if !ok {
		return nil
	}
	var out []string
	for _, p := range def.Params {
		if p.Literal {
			out = append(out, p.Name)
		}
	}
	return out
}

var filterRegistry = map[string]filterDef{}

func registerFilter(def filterDef) { filterRegistry[def.Type] = def }

func knownFilters() string {
	names := make([]string, 0, len(filterRegistry))
	for n := range filterRegistry {
		names = append(names, n)
	}
	return joinSorted(names)
}

func init() {
	// ---- gates: what the route refuses to carry (ROUTE-04) ------------------
	//
	// Two bricks rather than one with two fields: one brick, one refusal, one
	// status. An admin who wants a body cap and not a header cap adds one and
	// not the other, instead of learning that an empty field means no limit.

	registerFilter(filterDef{
		Type: "max-request-body", Phase: phaseGate,
		Doc:     "Refuses a request body over this size, with 413.",
		Details: "Caps what a caller may upload. A declared length is refused without reading anything; a chunked body is cut off as it arrives. Answers 413. WebSocket connections are never capped.",
		Params: []Param{
			{Name: "size", Kind: KindString, Required: true, Doc: "e.g. 2MB, 512KB, or a number of bytes"},
		},
		compileGate: func(a decoded) (Gate, error) {
			limit, err := filters.ParseSize(a.str("size"))
			if err != nil {
				return nil, fmt.Errorf("max-request-body: %w", err)
			}
			return func(w http.ResponseWriter, r *http.Request) bool {
				if !filters.HasBody(r) {
					return true
				}
				// A declared length is answered without reading anything: the
				// caller said how much was coming, and refusing at that word
				// costs one comparison instead of a megabyte of transfer.
				if r.ContentLength > limit {
					filters.RefuseSize(w, r, http.StatusRequestEntityTooLarge,
						"request body too large", r.ContentLength, limit)
					return false
				}
				// Chunked, or a length the client did not declare: the cap has
				// to be enforced on the way through. MaxBytesReader stops the
				// read at the limit and tells the server to close rather than
				// read the rest of a body it refused; the proxy's error path
				// turns that into the same 413 (see buildProxy).
				r.Body = http.MaxBytesReader(w, r.Body, limit)
				return true
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "max-request-headers", Phase: phaseGate,
		Doc:     "Refuses a request whose headers weigh more than this, with 431.",
		Details: "Caps the header block, for a caller sending hundreds of cookies or roles. Answers 431, which tells them which half to shrink. What the gateway adds afterwards is not counted.",
		Params: []Param{
			{Name: "size", Kind: KindString, Required: true, Doc: "e.g. 16KB, or a number of bytes"},
		},
		compileGate: func(a decoded) (Gate, error) {
			limit, err := filters.ParseSize(a.str("size"))
			if err != nil {
				return nil, fmt.Errorf("max-request-headers: %w", err)
			}
			return func(w http.ResponseWriter, r *http.Request) bool {
				if n := filters.HeaderBytes(r); n > limit {
					filters.RefuseSize(w, r, http.StatusRequestHeaderFieldsTooLarge,
						"request headers too large", n, limit)
					return false
				}
				return true
			}, nil
		},
	})

	// ---- request: path -----------------------------------------------------

	registerFilter(filterDef{
		Type: "strip-prefix", Phase: phaseRequest,
		Doc:     "Removes the first N segments of the path before proxying.",
		Details: "The gateway publishes /demo/orders, the service only knows /orders. With 1, the first segment is removed. Tell the service where it lives with X-Forwarded-Prefix, or the links it builds will point outside the route.",
		Params: []Param{
			{Name: "parts", Kind: KindInt, Default: 1, Doc: "number of leading segments to remove"},
			{Name: "announcePrefix", Kind: KindBool, Default: true,
				Doc: "tell the service where it is published, as X-Forwarded-Prefix"},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			n := a.num("parts")
			if n < 1 {
				return nil, fmt.Errorf("strip-prefix: parts must be >= 1")
			}
			announce := a.boolean("announcePrefix")
			return func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Path = StripSegments(pr.Out.URL.Path, n)
				pr.Out.URL.RawPath = ""
				if !announce {
					return
				}
				// What was consumed, taken as the DIFFERENCE between what the
				// caller asked for and what is left - not as "the first n
				// segments of the inbound path". The difference is right when
				// two strip-prefix filters follow one another (each one widens
				// the announced prefix instead of overwriting it with its own
				// share), and it costs one comparison.
				if prefix := strings.TrimSuffix(pr.In.URL.Path, pr.Out.URL.Path); prefix != "" {
					pr.Out.Header.Set(ForwardedPrefixHeader, prefix)
				}
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "prefix-path", Phase: phaseRequest,
		Doc:     "Prepends a prefix to the path before proxying.",
		Details: "The reverse: the caller asks /orders and the service receives /api/orders, for an application mounted under a path the gateway does not show.",
		Params: []Param{
			{Name: "prefix", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			prefix := a.str("prefix")
			return func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Path = prefix + pr.Out.URL.Path
				pr.Out.URL.RawPath = ""
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "rewrite-path", Phase: phaseRequest,
		Doc:     "Rewrites the path with a regexp replacement.",
		Details: "Reshapes the path with a regexp when removing or adding a prefix is not enough. Pattern /red/(.*) with replacement /$1 turns /red/blue/orders into /blue/orders.",
		Params: []Param{
			{Name: "pattern", Kind: KindString, Required: true},
			{Name: "replacement", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			re, err := regexp.Compile(a.str("pattern"))
			if err != nil {
				return nil, fmt.Errorf("rewrite-path: bad pattern: %w", err)
			}
			repl := a.str("replacement")
			return func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Path = re.ReplaceAllString(pr.Out.URL.Path, repl)
				pr.Out.URL.RawPath = ""
			}, nil
		},
	})

	// ---- request: headers & query ------------------------------------------

	registerFilter(filterDef{
		Type: "set-request-header", Phase: phaseRequest,
		Doc:     "Sets a request header (replacing any client value).",
		Details: "Replaces every value the caller sent under that name. The value is fixed: nothing is taken from the path or the query. Identity forwarding runs after the modifiers and overwrites the headers it writes itself.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name, value := a.str("name"), a.str("value")
			return func(pr *httputil.ProxyRequest) { pr.Out.Header.Set(name, value) }, nil
		},
	})

	registerFilter(filterDef{
		Type: "add-request-header", Phase: phaseRequest,
		Doc:     "Adds a request header value; ifNotPresent skips when the client already sent one.",
		Details: "Adds a value next to the existing ones. With ifNotPresent, nothing is added when the caller already sent that header.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
			{Name: "ifNotPresent", Kind: KindBool, Default: false},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name, value, skip := a.str("name"), a.str("value"), a.boolean("ifNotPresent")
			return func(pr *httputil.ProxyRequest) {
				if skip && pr.Out.Header.Get(name) != "" {
					return
				}
				pr.Out.Header.Add(name, value)
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "remove-request-header", Phase: phaseRequest,
		Doc:     "Removes a request header before proxying.",
		Details: "Removes every value under that name. Header names are case-insensitive. To remove a cookie use remove-request-cookie: cookies share one header.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name := a.str("name")
			return func(pr *httputil.ProxyRequest) { pr.Out.Header.Del(name) }, nil
		},
	})

	registerFilter(filterDef{
		Type: "set-query-param", Phase: phaseRequest,
		Doc:     "Sets a query parameter on the proxied request.",
		Details: "Sets the parameter, replacing any value the caller sent, and adds it when absent. The value is encoded on the way out.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name, value := a.str("name"), a.str("value")
			return func(pr *httputil.ProxyRequest) {
				q := pr.Out.URL.Query()
				q.Set(name, value)
				pr.Out.URL.RawQuery = q.Encode()
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "remove-query-param", Phase: phaseRequest,
		Doc:     "Removes a query parameter from the proxied request.",
		Details: "Removes every value of that parameter. The rest of the query string is unchanged.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name := a.str("name")
			return func(pr *httputil.ProxyRequest) {
				q := pr.Out.URL.Query()
				q.Del(name)
				pr.Out.URL.RawQuery = q.Encode()
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "set-path", Phase: phaseRequest,
		Doc:     "Replaces the whole path sent upstream.",
		Details: "Sends everything the route matches to one fixed path, typically a health endpoint.",
		Params: []Param{
			{Name: "path", Kind: KindString, Required: true, Doc: "absolute, e.g. /health"},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			path := a.str("path")
			if !strings.HasPrefix(path, "/") {
				return nil, fmt.Errorf("set-path: %q must start with /", path)
			}
			return func(pr *httputil.ProxyRequest) {
				// RawPath as well: it is what the proxy sends when the path
				// holds anything that had to be escaped, and leaving a stale
				// one behind sends the OLD path to the upstream.
				pr.Out.URL.Path, pr.Out.URL.RawPath = path, ""
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "set-host", Phase: phaseRequest,
		Doc:     "Sets the Host sent upstream.",
		Details: "One machine serves several sites and picks by name. This is the name it gets, whatever the upstream address is.",
		Params: []Param{
			{Name: "host", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			host := a.str("host")
			return func(pr *httputil.ProxyRequest) {
				// Both, and this is the point of the filter: Go sends
				// Request.Host on the wire and ignores the header, while
				// anything reading the request after us reads the header. Set
				// one and the two disagree - which is the vhost bug that takes
				// an afternoon.
				pr.Out.Host = host
				pr.Out.Header.Set("Host", host)
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "rename-request-header", Phase: phaseRequest,
		Doc:     "Moves a request header to another name, values and all.",
		Details: "The caller sends X-User and the service expects REMOTE_USER. Moves the value; the old name is gone. Nothing happens when the header is absent.",
		Params: []Param{
			{Name: "from", Kind: KindString, Required: true},
			{Name: "to", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			from, to := a.str("from"), a.str("to")
			return func(pr *httputil.ProxyRequest) {
				values := pr.Out.Header.Values(from)
				if len(values) == 0 {
					return
				}
				pr.Out.Header.Del(from)
				pr.Out.Header.Del(to)
				for _, v := range values {
					pr.Out.Header.Add(to, v)
				}
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "add-query-param", Phase: phaseRequest,
		Doc:     "Adds a query parameter.",
		Details: "Adds a value and keeps what the caller already sent under that name. Use set-query-param to replace it.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name, value := a.str("name"), a.str("value")
			return func(pr *httputil.ProxyRequest) {
				q := pr.Out.URL.Query()
				q.Add(name, value)
				pr.Out.URL.RawQuery = q.Encode()
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "remove-request-cookie", Phase: phaseRequest,
		Doc:     "Removes one cookie from the request.",
		Details: "Removes one cookie on the way to the service and keeps the rest - a session cookie the application would mistake for its own, typically.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			name := a.str("name")
			return func(pr *httputil.ProxyRequest) {
				kept := pr.Out.Cookies()[:0]
				for _, c := range pr.Out.Cookies() {
					if c.Name != name {
						kept = append(kept, c)
					}
				}
				// Rebuilt rather than edited: the Cookie header is one string
				// carrying every cookie, so removing one means writing the
				// header again - and deleting it outright would take the
				// session with it.
				pr.Out.Header.Del("Cookie")
				for _, c := range kept {
					pr.Out.AddCookie(c)
				}
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "copy-request-header", Phase: phaseRequest,
		Doc:     "Copies a request header under a second name, leaving the original in place.",
		Details: "During a migration, one service reads the new header name and another still needs the old one. Copies the value to the second name and keeps the first.",
		Params: []Param{
			{Name: "from", Kind: KindString, Required: true},
			{Name: "to", Kind: KindString, Required: true},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			from, to := a.str("from"), a.str("to")
			return func(pr *httputil.ProxyRequest) {
				values := pr.Out.Header.Values(from)
				if len(values) == 0 {
					return
				}
				// The target is replaced, not appended to: copying twice would
				// otherwise pile the same value up on a retried request.
				pr.Out.Header.Del(to)
				for _, v := range values {
					pr.Out.Header.Add(to, v)
				}
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "preserve-host", Phase: phaseRequest,
		Doc:     "Sends the caller's own Host upstream instead of the upstream's.",
		Details: "The service builds its links from the name it was called by. Without this it sees the internal name, and every link it writes leads there.",
		Params:  []Param{},
		compileRequest: func(_ decoded) (RequestFilter, error) {
			return func(pr *httputil.ProxyRequest) {
				// Both, like set-host: Go puts Request.Host on the wire and
				// ignores the header, and the header is what tells the proxy
				// rewrite that a filter asked for this Host at all - without
				// it, SetURL hands the upstream's name over and this filter
				// would be a no-op nobody could explain.
				pr.Out.Host = pr.In.Host
				pr.Out.Header.Set("Host", pr.In.Host)
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "rewrite-query-param", Phase: phaseRequest,
		Doc:     "Rewrites a query parameter's value with a regexp replacement.",
		Details: "Cleans a parameter the service should not receive as sent: pattern ^https?://[^/]+ with an empty replacement turns ?redirect=https://evil.test/back into ?redirect=/back.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "pattern", Kind: KindString, Required: true},
			{Name: "replacement", Kind: KindString},
		},
		compileRequest: func(a decoded) (RequestFilter, error) {
			re, err := regexp.Compile(a.str("pattern"))
			if err != nil {
				return nil, fmt.Errorf("rewrite-query-param: bad pattern: %w", err)
			}
			name, repl := a.str("name"), a.str("replacement")
			return func(pr *httputil.ProxyRequest) {
				q := pr.Out.URL.Query()
				values, ok := q[name]
				if !ok {
					return
				}
				for i, v := range values {
					values[i] = re.ReplaceAllString(v, repl)
				}
				q[name] = values
				pr.Out.URL.RawQuery = q.Encode()
			}, nil
		},
	})

	// ---- response ----------------------------------------------------------

	registerFilter(filterDef{
		Type: "set-response-header", Phase: phaseResponse,
		Doc:     "Sets a response header (replacing any upstream value).",
		Details: "Replaces every value the service sent under that name. Headers that describe the body, such as Content-Type or Content-Encoding, are not enforced: setting one does not change the bytes.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			name, value := a.str("name"), a.str("value")
			return func(res *http.Response) error { res.Header.Set(name, value); return nil }, nil
		},
	})

	registerFilter(filterDef{
		Type: "add-response-header", Phase: phaseResponse,
		Doc:     "Adds a response header value.",
		Details: "Adds a value next to the ones the service sent. Use it on multi-valued headers such as Vary or Set-Cookie; two values on a single-valued header are ambiguous.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "value", Kind: KindString, Required: true},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			name, value := a.str("name"), a.str("value")
			return func(res *http.Response) error { res.Header.Add(name, value); return nil }, nil
		},
	})

	registerFilter(filterDef{
		Type: "remove-response-header", Phase: phaseResponse,
		Doc:     "Removes a response header before it reaches the client.",
		Details: "Removes every value under that name before the answer reaches the client. Use it on what a service says about itself, such as its framework or version.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			name := a.str("name")
			return func(res *http.Response) error { res.Header.Del(name); return nil }, nil
		},
	})

	registerFilter(filterDef{
		Type: "rename-response-header", Phase: phaseResponse,
		Doc:     "Moves a response header to another name, values and all.",
		Details: "Moves the header to another name with all its values; the old name is gone.",
		Params: []Param{
			{Name: "from", Kind: KindString, Required: true},
			{Name: "to", Kind: KindString, Required: true},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			from, to := a.str("from"), a.str("to")
			return func(res *http.Response) error {
				values := res.Header.Values(from)
				if len(values) == 0 {
					return nil
				}
				res.Header.Del(from)
				res.Header.Del(to)
				for _, v := range values {
					res.Header.Add(to, v)
				}
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "cache-control", Phase: phaseResponse,
		Doc:     "Sets Cache-Control on the response.",
		Details: "Decides what may be cached, for a service that says nothing about it. Also removes Expires and Pragma, which could say the opposite. no-store for personal data, \"public, max-age=3600\" for shared content.",
		Params: []Param{
			{Name: "value", Kind: KindString, Required: true, Doc: `e.g. "public, max-age=3600" or "no-store"`},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			value := a.str("value")
			return func(res *http.Response) error {
				res.Header.Set("Cache-Control", value)
				// An Expires or a Pragma from the upstream would argue with
				// what was just decided, and the older header wins in some
				// caches: saying one thing twice is how a page gets cached
				// that should not have been.
				res.Header.Del("Expires")
				res.Header.Del("Pragma")
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "security-headers", Phase: phaseResponse,
		Doc:     "Adds the response headers a browser hardens on.",
		Details: "The headers a browser hardens on, for an application that sets none. nosniff always; referrer, frame policy and CSP when set. HSTS only over TLS, because a browser remembers it for months and would then refuse plain HTTP. X-XSS-Protection and the two IE-era headers are not sent: browsers ignore them.",
		Params: []Param{
			{Name: "referrerPolicy", Kind: KindString, Default: "strict-origin-when-cross-origin"},
			{Name: "frameOptions", Kind: KindString, Default: "SAMEORIGIN", Options: []string{"", "DENY", "SAMEORIGIN"},
				Doc: `empty leaves what the application set`},
			{Name: "hstsMaxAge", Kind: KindInt, Default: 0, Doc: "seconds; 0 leaves Strict-Transport-Security alone"},
			{Name: "contentSecurityPolicy", Kind: KindString, Doc: "sent as-is when set; a CSP is written per application, never guessed"},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			referrer, frame := a.str("referrerPolicy"), a.str("frameOptions")
			hsts, csp := a.num("hstsMaxAge"), a.str("contentSecurityPolicy")
			return func(res *http.Response) error {
				// set, not add: these are decisions, and two of them is none.
				res.Header.Set("X-Content-Type-Options", "nosniff")
				if referrer != "" {
					res.Header.Set("Referrer-Policy", referrer)
				}
				if frame != "" {
					res.Header.Set("X-Frame-Options", frame)
				}
				// HSTS only over TLS: sent on a plain request it is either
				// ignored or, worse, remembered by a browser that then cannot
				// reach a development gateway at all.
				if hsts > 0 && res.Request != nil && res.Request.TLS != nil {
					res.Header.Set("Strict-Transport-Security",
						"max-age="+strconv.Itoa(hsts)+"; includeSubDomains")
				}
				if csp != "" {
					res.Header.Set("Content-Security-Policy", csp)
				}
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "cookie-attributes", Phase: phaseResponse,
		Doc:     "Forces attributes on the cookies an upstream sets.",
		Details: "The application runs on http behind the gateway and sets its cookies accordingly. This adds what is missing: Set-Cookie: id=abc; Path=/ becomes Set-Cookie: id=abc; Path=/; Secure. SameSite=None forces Secure, or browsers drop the cookie.",
		Params: []Param{
			{Name: "secure", Kind: KindBool, Default: true},
			{Name: "httpOnly", Kind: KindBool, Default: false},
			{Name: "sameSite", Kind: KindString, Options: []string{"", "Lax", "Strict", "None"},
				Doc: `empty leaves what the application set`},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			secure, httpOnly, sameSite := a.boolean("secure"), a.boolean("httpOnly"), a.str("sameSite")
			switch strings.ToLower(sameSite) {
			case "", "lax", "strict", "none":
			default:
				return nil, fmt.Errorf("cookie-attributes: sameSite %q is not allowed: use Lax, Strict or None", sameSite)
			}
			return func(res *http.Response) error {
				cookies := res.Cookies()
				if len(cookies) == 0 {
					return nil
				}
				res.Header.Del("Set-Cookie")
				for _, c := range cookies {
					if secure {
						c.Secure = true
					}
					if httpOnly {
						c.HttpOnly = true
					}
					switch strings.ToLower(sameSite) {
					case "lax":
						c.SameSite = http.SameSiteLaxMode
					case "strict":
						c.SameSite = http.SameSiteStrictMode
					case "none":
						// SameSite=None without Secure is dropped by every
						// current browser: asking for one is asking for both.
						c.SameSite, c.Secure = http.SameSiteNoneMode, true
					}
					res.Header.Add("Set-Cookie", c.String())
				}
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "remove-json-fields", Phase: phaseResponse,
		Doc:     "Removes fields from a JSON answer, by name or by dotted path.",
		Details: "The service answers with more than its callers should see. Removes fields by name (password) or by path (user.email), inside arrays too. JSON responses only; streams and oversized bodies pass through untouched.",
		Params: []Param{
			{Name: "fields", Kind: KindStringList, Required: true,
				Doc: `field names or dotted paths, e.g. "password" or "user.email"`},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			fields := a.strs("fields")
			if len(fields) == 0 {
				return nil, fmt.Errorf("remove-json-fields: name at least one field")
			}
			paths := make([][]string, 0, len(fields))
			for _, f := range fields {
				parts := strings.Split(f, ".")
				for _, p := range parts {
					if strings.TrimSpace(p) == "" {
						return nil, fmt.Errorf("remove-json-fields: %q is not a field path", f)
					}
				}
				paths = append(paths, parts)
			}
			return func(res *http.Response) error {
				if !isJSON(res) {
					return nil
				}
				return filters.RewriteBody(res, func(body []byte) []byte {
					var tree any
					if err := json.Unmarshal(body, &tree); err != nil {
						// Not the JSON it claimed to be: hand it over as it is.
						// A gateway that mangles what it cannot parse is worse
						// than one that forwards it.
						return nil
					}
					removed := false
					for _, path := range paths {
						removed = dropPath(tree, path) || removed
					}
					if !removed {
						return nil
					}
					out, err := json.Marshal(tree)
					if err != nil {
						return nil
					}
					return out
				})
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "dedupe-response-header", Phase: phaseResponse,
		Doc:     "Drops repeated values of a response header.",
		Details: "The gateway and the service both set the same header, and the browser refuses the pair - CORS, most often. Keeps one value: the service's, the last one, or each distinct value once.",
		Params: []Param{
			{Name: "names", Kind: KindStringList, Required: true, Doc: "header names, e.g. Access-Control-Allow-Origin"},
			{Name: "keep", Kind: KindString, Default: "first", Options: []string{"first", "last", "unique"},
				Doc: "which value survives"},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			names, keep := a.strs("names"), a.str("keep")
			return func(res *http.Response) error {
				for _, name := range names {
					values := res.Header.Values(name)
					if len(values) < 2 {
						continue
					}
					switch keep {
					case "last":
						res.Header.Set(name, values[len(values)-1])
					case "unique":
						// Keeps the ORDER of first appearance: a browser reads
						// these lists positionally, and a set would shuffle
						// them differently on every response.
						seen := map[string]bool{}
						res.Header.Del(name)
						for _, v := range values {
							if !seen[v] {
								seen[v] = true
								res.Header.Add(name, v)
							}
						}
					default:
						res.Header.Set(name, values[0])
					}
				}
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "rewrite-response-header", Phase: phaseResponse,
		Doc:     "Rewrites a response header's value with a regexp replacement.",
		Details: "Takes an internal name out of what the service says about itself: ^[^.]+\\\\.internal replaced by service turns billing-3.internal:8080 into service:8080.",
		Params: []Param{
			{Name: "name", Kind: KindString, Required: true},
			{Name: "pattern", Kind: KindString, Required: true},
			{Name: "replacement", Kind: KindString},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			re, err := regexp.Compile(a.str("pattern"))
			if err != nil {
				return nil, fmt.Errorf("rewrite-response-header: bad pattern: %w", err)
			}
			name, repl := a.str("name"), a.str("replacement")
			return func(res *http.Response) error {
				values := res.Header.Values(name)
				if len(values) == 0 {
					return nil
				}
				rewritten := make([]string, len(values))
				for i, v := range values {
					rewritten[i] = re.ReplaceAllString(v, repl)
				}
				res.Header.Del(name)
				for _, v := range rewritten {
					res.Header.Add(name, v)
				}
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "rewrite-location", Phase: phaseResponse,
		Doc:     "Brings the Location of an upstream redirect back into the public space.",
		Details: "The service redirects to its own address and the browser leaves the gateway. This rewrites it: http://billing.internal:8080/login becomes https://shop.example/login. Set from and to to rewrite a path prefix as well.",
		Params: []Param{
			{Name: "from", Kind: KindString, Doc: "prefix to replace; empty means the upstream's own origin"},
			{Name: "to", Kind: KindString, Doc: "what replaces it; empty means the origin the caller used"},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			from, to := a.str("from"), a.str("to")
			return func(res *http.Response) error {
				loc := res.Header.Get("Location")
				if loc == "" {
					return nil
				}
				f, t := from, to
				if f == "" {
					// The upstream's origin, as this response's request was
					// actually sent - not as the route was configured, so a
					// redirect chain lands on the host that answered.
					if res.Request == nil || res.Request.URL == nil {
						return nil
					}
					f = res.Request.URL.Scheme + "://" + res.Request.URL.Host
				}
				if t == "" {
					t = publicOrigin(res.Request)
					if t == "" {
						return nil // nothing reliable to point at: leave it alone
					}
				}
				if !strings.HasPrefix(loc, f) {
					return nil
				}
				res.Header.Set("Location", t+strings.TrimPrefix(loc, f))
				return nil
			}, nil
		},
	})

	registerFilter(filterDef{
		Type: "set-status", Phase: phaseResponse,
		Doc:     "Overrides the upstream response status code.",
		Details: "Replaces the status code. The body is untouched, so a 200 set over an error page hides the error from anything that only reads the code.",
		Params: []Param{
			{Name: "status", Kind: KindInt, Required: true},
		},
		compileResponse: func(a decoded) (ResponseFilter, error) {
			status := a.num("status")
			if status < 100 || status > 599 {
				return nil, fmt.Errorf("set-status: %d is not a valid HTTP status", status)
			}
			return func(res *http.Response) error {
				res.StatusCode = status
				res.Status = http.StatusText(status)
				return nil
			}, nil
		},
	})

	// NOTE: the old generic "inject-head" filter is gone - page injections are
	// a UI-route affair (Injections section: custom CSS/JS, user button, page
	// stamps), all riding filters.InjectAfterHead internally.

	// ---- terminal ----------------------------------------------------------

	registerFilter(filterDef{
		Type: "maintenance", Phase: phaseTerminal,
		Doc:     "Answers 503 with a gateway maintenance page instead of proxying.",
		Details: "Answers the unavailable page without calling the service. The route keeps matching, so no other route takes its traffic while the service is down.",
		Params: []Param{
			{Name: "message", Kind: KindString, Doc: "optional text shown on the page"},
		},
		compileTerminal: func(a decoded) (http.Handler, error) {
			page := maintenancePage(a.str("message"))
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Retry-After", "300")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write(page)
			}), nil
		},
	})

	registerFilter(filterDef{
		Type: "redirect", Phase: phaseTerminal,
		Doc:     "Answers with a redirect instead of proxying.",
		Details: "Answers a redirect without calling anything. 301 is permanent and browsers remember it for a long time; 307 is temporary and keeps the method, so a POST stays a POST.",
		Params: []Param{
			{Name: "location", Kind: KindString, Required: true},
			{Name: "status", Kind: KindInt, Default: 302},
		},
		compileTerminal: func(a decoded) (http.Handler, error) {
			status := a.num("status")
			if status < 300 || status > 399 {
				return nil, fmt.Errorf("redirect: status must be 3xx, got %d", status)
			}
			location := a.str("location")
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, status)
			}), nil
		},
	})
}

// isJSON reports whether this response says it carries JSON. The media type
// publicOrigin rebuilds the origin the CALLER used, from the forwarding
// headers the proxy set on the way out (SetXForwarded). The outgoing request
// is all a response filter can see - it points at the upstream - so without
// these the gateway has no idea under which name it was reached, which is
// exactly what a redirect has to be written in.
//
// Empty when the headers are missing: a Location rewritten towards a host
// nobody asked for is worse than one left alone.
func publicOrigin(req *http.Request) string {
	if req == nil {
		return ""
	}
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		return ""
	}
	scheme := req.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + host
}

// is the only thing to go on, and guessing from the bytes would mean rewriting
// documents nobody declared as data.
func isJSON(res *http.Response) bool {
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	return strings.HasPrefix(ct, "application/json") ||
		strings.HasSuffix(strings.Split(ct, ";")[0], "+json")
}

// dropPath removes one field, wherever the path leads - and INSIDE ARRAYS: a
// service answering a list of users would otherwise keep every password it was
// asked to drop, which is the case this filter exists for.
func dropPath(node any, path []string) bool {
	switch v := node.(type) {
	case map[string]any:
		if len(path) == 1 {
			if _, ok := v[path[0]]; !ok {
				return false
			}
			delete(v, path[0])
			return true
		}
		child, ok := v[path[0]]
		if !ok {
			return false
		}
		return dropPath(child, path[1:])
	case []any:
		removed := false
		for _, item := range v {
			removed = dropPath(item, path) || removed
		}
		return removed
	}
	return false
}

// maintenancePage renders the built-in "under maintenance" answer: a sober
// dark page, self-contained (the gateway must run offline). The message is
// the admin's own text, escaped.
func maintenancePage(message string) []byte {
	extra := ""
	if message != "" {
		extra = `<p class="msg">` + html.EscapeString(message) + `</p>`
	}
	return []byte(`<!doctype html><html><head><meta charset="utf-8"><title>Maintenance</title>
<meta name="viewport" content="width=device-width, initial-scale=1"><style>
body { margin:0; min-height:100vh; display:grid; place-items:center; background:#1b2f40; color:#e6edf3; font-family:system-ui,sans-serif; }
main { text-align:center; padding:24px; }
h1 { font-size:1.3rem; letter-spacing:.1em; text-transform:uppercase; }
.msg { color:#9fb3c8; max-width:52ch; }
.dot { width:10px; height:10px; margin:20px auto 0; border-radius:50%; background:#25c2e0; box-shadow:0 0 12px #25c2e0; }
</style></head><body><main><h1>Under maintenance</h1>` + extra + `<div class="dot"></div></main></body></html>`)
}
