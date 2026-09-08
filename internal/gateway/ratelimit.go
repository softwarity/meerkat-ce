package gateway

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/limits"
	"github.com/softwarity/meerkat/internal/store"
)

// Enforcing a route's bounds (ROUTE-08, QUOTA-01/02).
//
// The counters and the reason they are shaped the way they are live in
// internal/limits. This file is only about WHERE a bound is checked and WHAT
// the caller is told, and both of those are decisions.
//
// WHERE. Bounds that need no identity - one for the whole route, one per
// address - are checked OUTERMOST, before a session is looked up, before a
// body is read, before an upstream is dialled. That is the same rule the size
// gates already follow, and for the same reason: refusing early is the whole
// point of a bound. Bounds keyed on a user, a token or an organisation cannot
// be, so they cost a session resolve - paid only by the routes that ask for
// one, which is why the two kinds are separated at compile time rather than
// sorted out per request.
//
// WHAT. A 429 with RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset and
// Retry-After. A refusal without them is a door with no sign on it: a client
// that cannot read when to come back either gives up or hammers, and both are
// worse for the service the bound was installed to protect.

// compiledLimit is one rule with the counter it feeds.
type compiledLimit struct {
	rule    store.RateLimit
	counter *limits.Counter
	// needsIdentity is read from the rule at compile time. Recomputing it per
	// request would be free; keeping it here is what lets the two lists be
	// built once and wrapped at two different depths.
	needsIdentity bool
}

// compileLimits turns a route's rules into counters, split by what they need.
//
// The counters are made HERE, which means a reload gives a route fresh ones
// and a caller mid-window starts again. That is the honest trade for a design
// with no persistence: carrying counters across a reload would mean keying
// them by rule identity, and a rule has no identity - somebody editing "100 a
// minute" into "200 a minute" would inherit the count of a bound that no
// longer exists.
func compileLimits(rules []store.RateLimit) (free, identified []compiledLimit) {
	for _, rule := range rules {
		every := rule.Every()
		if every <= 0 || rule.Requests <= 0 {
			continue // refused by SanitizeRateLimits; nothing to enforce
		}
		c := compiledLimit{rule: rule, counter: limits.New(rule.Requests, every),
			needsIdentity: rule.NeedsIdentity()}
		if c.needsIdentity {
			identified = append(identified, c)
		} else {
			free = append(free, c)
		}
	}
	return free, identified
}

// subject is who the caller is, resolved at most once per request and only
// when a rule actually needs it.
type subject struct {
	ok       bool
	userID   string
	tokenID  string
	tenantID string
	caller   store.Caller
}

// limitSubject resolves the caller for the limits that need one. Separate from
// sessionIdentity's own callers on purpose: this runs before the access rule
// has been applied on this path, so it must answer for a caller who may end up
// refused - somebody hammering with credentials that do not work is exactly
// who a bound is for.
func (rt *Router) limitSubject(req *http.Request, wantToken bool) *subject {
	s := &subject{}
	if rt.sm == nil {
		return s
	}
	d, ok := rt.sessionIdentity(req)
	if !ok {
		return s
	}
	s.ok, s.userID, s.tenantID = true, d.UserID, d.TenantID
	s.caller = rt.caller(req, d, ok)
	if wantToken {
		// A second resolve, and only for a rule that counts by token: the id
		// is on the session and sessionIdentity does not carry it out.
		if sess, err := rt.sm.Resolve(req.Context(), req); err == nil {
			s.tokenID = sess.TokenID
		}
	}
	return s
}

// key is what this rule counts by for this request, and "" means the rule does
// not apply to this caller at all.
//
// An anonymous caller has no user, no token and no organisation, so a rule
// keyed on one of those simply does not cover them - it is not a bound of zero
// and not a bound of infinity. Covering them is what a rule per address is
// for, and writing the two together is the ordinary shape.
func (c compiledLimit) key(req *http.Request, s *subject) (string, bool) {
	switch c.rule.Per {
	case store.PerRoute:
		return "", true
	case store.PerIP:
		return filters.ClientIP(req), true
	case store.PerUser:
		return s.userID, s.userID != ""
	case store.PerToken:
		return s.tokenID, s.tokenID != ""
	case store.PerTenant:
		return s.tenantID, s.tenantID != ""
	}
	return "", false
}

// rateGate applies a set of bounds before next.
//
// ALL of them are checked, not the first that matches: a bound is a bound, and
// "five thousand a minute for the route" and "a hundred a minute per user" are
// both true at once. The first one exceeded answers.
//
// A request refused by a LATER bound has already been counted against the
// earlier ones. Left that way knowingly: undoing it would need a check pass
// and a commit pass over every counter, and the error it removes goes in the
// direction you want to be wrong in - under abuse the wider bounds trip
// sooner, not later.
func (rt *Router) rateGate(rules []compiledLimit, next http.Handler) http.Handler {
	if len(rules) == 0 {
		return next // a route with no bound pays nothing per request
	}
	wantToken := false
	needsSubject := false
	for _, c := range rules {
		needsSubject = needsSubject || c.needsIdentity
		wantToken = wantToken || c.rule.Per == store.PerToken
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s := &subject{}
		if needsSubject {
			s = rt.limitSubject(req, wantToken)
		}
		now := time.Now()
		for i := range rules {
			c := &rules[i]
			if !c.rule.Applies.Empty() && !c.rule.Applies.Grants(s.caller) {
				continue
			}
			key, covered := c.key(req, s)
			if !covered {
				continue
			}
			if v := c.counter.Allow(key, now); !v.OK {
				refuseRate(w, req, c.rule, v)
				return
			}
		}
		next.ServeHTTP(w, req)
	})
}

// refuseRate answers 429 and says what to do about it.
func refuseRate(w http.ResponseWriter, r *http.Request, rule store.RateLimit, v limits.Verdict) {
	secs := int(v.ResetIn / time.Second)
	if secs < 1 {
		secs = 1
	}
	h := w.Header()
	// The standard fields (QUOTA-02). A client that reads them backs off on
	// its own, which is the difference between a bound that protects a service
	// and one that turns every caller into a retry storm.
	h.Set("RateLimit-Limit", strconv.Itoa(v.Limit))
	h.Set("RateLimit-Remaining", strconv.Itoa(v.Remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(secs))
	h.Set("Retry-After", strconv.Itoa(secs))
	h.Set("Content-Type", "text/plain; charset=utf-8")
	// Nothing was served, so nothing may be cached: the next request, a second
	// later, must reach the gateway rather than be answered from this.
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusTooManyRequests)
	if r != nil && r.Method == http.MethodHead {
		return
	}
	// Which bound was hit, in the words it was written in: "too many requests"
	// alone leaves an operator guessing which of a route's three rules fired.
	_, _ = fmt.Fprintf(w, "too many requests: this route allows %d per %s every %s, try again in %ds\n",
		rule.Requests, rule.Per, rule.Every(), secs)
}
