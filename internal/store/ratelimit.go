package store

import (
	"fmt"
	"strings"
	"time"
)

// RateLimit is one bound on how much a route carries (ROUTE-08, QUOTA-01/02/05).
//
// THE WORD IS "PER", NOT "FOR". Nobody writes a limit FOR alice - a list of
// names is a list nobody maintains, and it answers the wrong question. A limit
// is written PER user, and the counters make themselves: one per caller, keyed
// by whatever the rule says to key on.
//
// So a rule carries two independent halves:
//
//   - PER: what keys the counter. One for the whole route, or one per user,
//     per token, per organisation, per address.
//   - APPLIES: who the rule is about, as an Access - the SAME vocabulary the
//     access rules already use, and the same editor already renders. Empty
//     means everybody.
//
// Which is what turns a role into a pricing tier without inventing a concept:
// "1000 a minute per user, for anyone holding partner" and "60 a minute per
// user, for anyone holding trial" are two rows, and neither names a person.
//
// SEVERAL APPLY AT ONCE, and that is the difference from an access rule, where
// the first match wins. Limits are bounds, and you want all of them: five
// thousand a minute for the whole route protects the service, a hundred per
// user keeps one caller from taking it all, sixty per address covers whoever
// has no account. The first bound exceeded refuses.
type RateLimit struct {
	// Per is what the counter is keyed on. See the Per* constants.
	Per string `json:"per"`
	// Requests is how many are allowed in Window.
	Requests int `json:"requests"`
	// Window is an ISO 8601 duration, like the route timeouts: PT1M, PT1H.
	Window string `json:"window"`
	// Applies narrows the rule to some callers. Empty means everybody, which
	// is what a rule without one has always meant - the same reading as an
	// empty Access anywhere else.
	Applies Access `json:"applies,omitempty"`
}

// What a counter can be keyed on.
const (
	// PerRoute is ONE counter for everything this rule covers: the bound that
	// protects the service behind it, whoever is calling.
	PerRoute = "route"
	// PerUser is one counter per signed-in account.
	PerUser = "user"
	// PerToken is one counter per API token, so an integration is bounded
	// without bounding the person who minted it.
	PerToken = "token"
	// PerTenant is one counter per organisation - a quota sold to a customer
	// rather than to a person.
	PerTenant = "tenant"
	// PerIP is one counter per client address, which is the only key an
	// anonymous caller has.
	PerIP = "ip"
)

// RateLimitKeys are the allowed keys, in the order a form should offer them:
// the widest bound first, since it is the one every route wants.
var RateLimitKeys = []string{PerRoute, PerUser, PerToken, PerTenant, PerIP}

// NeedsIdentity reports whether a key can only be read after the caller is
// resolved. It decides WHERE the rule is enforced, and that is not a detail:
// a bound that needs no identity refuses before a session is even looked up,
// which is the whole point of a bound - see internal/gateway/limits.go.
func (l RateLimit) NeedsIdentity() bool {
	switch l.Per {
	case PerUser, PerToken, PerTenant:
		return true
	}
	// And a rule NARROWED to some callers needs one whatever it counts by:
	// "five thousand a minute for the whole route, but only for the trial
	// tier" cannot be decided without knowing who is calling. The key and the
	// selector are two independent halves, and either of them can be the one
	// that needs a session.
	return !l.Applies.Empty()
}

// Limits on a rate limit, so a typo cannot become an outage.
const (
	// MinRateWindow: below a second, a sliding window is measuring jitter.
	MinRateWindow = time.Second
	// MaxRateWindow: a day. Longer is a quota that has to be persisted to
	// survive a restart, and this counter lives in memory (QUOTA-03).
	MaxRateWindow = 24 * time.Hour
	// MaxRateRequests is a sanity bound, not a product limit: a number this
	// large is a rule somebody meant to leave off.
	MaxRateRequests = 100_000_000
)

// SanitizeRateLimits normalizes a list and refuses what cannot work, naming
// what is allowed - the same contract every other Sanitize in this package
// keeps, and the only place that decides, so the several writers of a route
// all get the same answer.
func SanitizeRateLimits(limits []RateLimit) error {
	for i := range limits {
		l := &limits[i]
		l.Per = strings.ToLower(strings.TrimSpace(l.Per))
		if l.Per == "" {
			l.Per = PerRoute
		}
		if !contains(RateLimitKeys, l.Per) {
			return fmt.Errorf("rate limit %d: %q is not something a counter can be keyed on: allowed are %s",
				i+1, l.Per, strings.Join(RateLimitKeys, ", "))
		}
		if l.Requests <= 0 || l.Requests > MaxRateRequests {
			return fmt.Errorf("rate limit %d (per %s): %d requests is outside what a limit can mean: allowed are 1 to %d",
				i+1, l.Per, l.Requests, MaxRateRequests)
		}
		d, err := ParseISODuration(l.Window)
		if err != nil {
			return fmt.Errorf("rate limit %d (per %s): window: %w", i+1, l.Per, err)
		}
		if d < MinRateWindow || d > MaxRateWindow {
			return fmt.Errorf("rate limit %d (per %s): a window of %s is outside what this counter holds: allowed are %s to %s",
				i+1, l.Per, l.Window, MinRateWindow, MaxRateWindow)
		}
		if err := SanitizeAccess(&l.Applies); err != nil {
			return fmt.Errorf("rate limit %d (per %s): applies to: %w", i+1, l.Per, err)
		}
	}
	return nil
}

// Every is the window as a duration. Only called after SanitizeRateLimits has
// accepted it, so a window that will not parse is a zero rather than an error
// nobody can act on from the request path.
func (l RateLimit) Every() time.Duration {
	d, err := ParseISODuration(l.Window)
	if err != nil {
		return 0
	}
	return d
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
