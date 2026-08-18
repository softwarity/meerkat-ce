package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// Route probing (ROUTE-15): take a request, run it through the REAL matcher of
// the live snapshot, and report what every route answered, in order. Order is
// significant (first match wins), so "my request lands on the wrong route" is
// almost always a route ABOVE the intended one accepting it first - the probe
// names that route and the predicate that let it through.
//
// Nothing is proxied: this is matching only, no upstream is touched.

// ProbeStep is one route's answer, in snapshot order.
type ProbeStep struct {
	RouteID string            `json:"routeId"`
	Name    string            `json:"name"`
	Matched bool              `json:"matched"`
	Preds   []routing.Verdict `json:"predicates"`
	// Rule is the route's SECURITY, which takes part in choosing it: a caller
	// it turns away falls through to the next route matching the same paths.
	// A probe that stopped at the predicates would name a winner the engine
	// does not pick.
	Rule       bool   `json:"rule"`
	RuleReason string `json:"ruleReason,omitempty"`
}

// ProbeResult is the whole cascade.
type ProbeResult struct {
	// Request is what actually went through the matcher.
	Request routing.Synthesis `json:"request"`
	Steps   []ProbeStep       `json:"steps"`
	// WinnerID is the route that took the request ("" = none, the request
	// would have got a 404).
	WinnerID   string `json:"winnerId,omitempty"`
	WinnerName string `json:"winnerName,omitempty"`
	// TargetID is the route the probe was aiming at, when there was one.
	TargetID string `json:"targetId,omitempty"`
	// Outcome: "match" (the target won), "intercepted" (another route took it
	// first), "missed" (the target refused it too), "none" (nobody matched).
	Outcome string `json:"outcome"`
}

// ProbeAs describes who the probe pretends to be. The zero value is an
// anonymous caller, which is a case worth probing on its own.
type ProbeAs struct {
	Username    string   `json:"username,omitempty"`
	TenantID    string   `json:"tenantId,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Memberships []string `json:"memberships,omitempty"`
	// SignedIn says there is a session at all. Set on its own it probes
	// "somebody, in no organisation" - the state a fresh account is in.
	SignedIn bool `json:"signedIn,omitempty"`
}

// caller turns the description into what a rule is evaluated against.
func (p ProbeAs) caller() store.Caller {
	authed := p.SignedIn || p.Username != "" || p.TenantID != "" || len(p.Roles) > 0
	ms := p.Memberships
	if ms == nil && p.TenantID != "" {
		ms = []string{p.TenantID}
	}
	return store.Caller{
		Authenticated: authed, Username: p.Username, TenantID: p.TenantID,
		Roles: p.Roles, Memberships: ms,
	}
}

// Probe runs one synthesized request through the live snapshot, as the given
// caller, and reports every route's verdict - predicates AND rule, because
// both take part in choosing. targetID may be empty: the caller then just
// wants to know where a request lands.
func (rt *Router) Probe(targetID string, s routing.Synthesis, as ProbeAs) ProbeResult {
	rt.mu.RLock()
	routes := rt.routes
	rt.mu.RUnlock()

	req := buildProbeRequest(s)
	out := ProbeResult{Request: s, TargetID: targetID, Steps: make([]ProbeStep, 0, len(routes))}
	for i := range routes {
		verdicts := routes[i].preds.Explain(req)
		matched := true
		for _, v := range verdicts {
			if !v.Matched {
				matched = false
			}
		}
		// The rule is asked only of a route whose predicates matched: telling
		// someone they lack a role on a route that was never about their path
		// is noise.
		rule, reason := true, ""
		if matched && !routes[i].access.Empty() {
			who := as.caller()
			if rule = routes[i].access.Grants(who); !rule {
				reason = refusalReason(routes[i].access, who)
			}
		}
		out.Steps = append(out.Steps, ProbeStep{
			RouteID: routes[i].id, Name: routes[i].name, Matched: matched, Preds: verdicts,
			Rule: rule, RuleReason: reason,
		})
		if matched && rule && out.WinnerID == "" {
			out.WinnerID, out.WinnerName = routes[i].id, routes[i].name
		}
	}
	switch {
	case out.WinnerID == "":
		out.Outcome = "none"
	case targetID == "" || out.WinnerID == targetID:
		out.Outcome = "match"
	default:
		out.Outcome = "intercepted"
		// A target that refuses its OWN synthesized request is a different
		// story from one that was merely beaten to it: say which.
		for _, st := range out.Steps {
			if st.RouteID == targetID && (!st.Matched || !st.Rule) {
				out.Outcome = "missed"
			}
		}
	}
	return out
}

// buildProbeRequest turns the description into the *http.Request the matcher
// will see. The lottery and the clock ride in the context, exactly as they do
// on a served request, so the time and canary predicates answer for the
// instant and the draw the admin asked about.
func buildProbeRequest(s routing.Synthesis) *http.Request {
	path := s.Path
	if path == "" {
		path = "/"
	}
	u := &url.URL{Path: path}
	if len(s.Query) > 0 {
		q := url.Values{}
		for k, v := range s.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	method := strings.ToUpper(strings.TrimSpace(s.Method))
	if method == "" {
		method = http.MethodGet
	}
	req := &http.Request{
		Method: method,
		URL:    u,
		Host:   s.Host,
		Header: http.Header{},
		Proto:  "HTTP/1.1",
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range s.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	req.RemoteAddr = s.RemoteAddr
	if req.RemoteAddr == "" {
		req.RemoteAddr = "203.0.113.7:54321"
	}
	ctx := context.Background()
	ctx = routing.WithLottery(ctx, s.Lottery)
	if !s.At.IsZero() {
		ctx = routing.WithClock(ctx, s.At)
	}
	return req.WithContext(ctx)
}
