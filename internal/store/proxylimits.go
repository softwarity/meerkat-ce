package store

import (
	"context"
	"fmt"
)

// SettingProxyLimits holds what bounds the gateway's own appetite while it
// proxies (PERF-02). One setting for the installation, deliberately: these are
// not business rules, they are what keeps the process alive.
const SettingProxyLimits = "proxy_limits"

// ProxyLimits is that setting. One field today, and a struct anyway - the next
// bound (an upstream deadline, a header ceiling) belongs beside this one
// rather than in a setting of its own.
type ProxyLimits struct {
	// BodyRewriteMiB caps what a body-rewriting filter will hold in memory to
	// change it. A bigger response is forwarded UNTOUCHED - the ceiling is not
	// a failure, it is the promise that a gateway's memory does not follow the
	// size of what it proxies.
	BodyRewriteMiB int `json:"bodyRewriteMiB"`
	// Timeouts is what every route waits unless it says otherwise (ROUTE-07).
	//
	// An INSTALLATION's numbers, not the product's: what counts as a slow
	// service depends on the network it sits on and on what it does, and two
	// constants compiled into a binary cannot know either. A route overrides
	// them one at a time; empty here means the product's own, which is what
	// every installation ran with before this existed.
	Timeouts RouteTimeouts `json:"timeouts,omitzero"`
}

// The bounds of that one number.
//
// Why there is a maximum at all: the bill is per REQUEST IN FLIGHT. A ceiling
// of 256 MiB with forty concurrent rewrites is ten gigabytes, and the gateway
// dies of a setting somebody typed once. Whoever needs more than this does not
// need a bigger buffer, they need a filter that streams.
const (
	MinBodyRewriteMiB     = 1
	MaxBodyRewriteMiB     = 256
	DefaultBodyRewriteMiB = 20
)

// DefaultProxyLimits is what an installation that never touched the screen
// runs with - and what it ran with before the screen existed.
func DefaultProxyLimits() ProxyLimits {
	return ProxyLimits{BodyRewriteMiB: DefaultBodyRewriteMiB}
}

// SanitizeProxyLimits fills in what was left out and refuses what cannot work,
// naming the range rather than clamping silently: a number quietly changed on
// the way in is a screen that lies about what is running.
func SanitizeProxyLimits(p *ProxyLimits) error {
	if p.BodyRewriteMiB == 0 {
		p.BodyRewriteMiB = DefaultBodyRewriteMiB
		return SanitizeRouteTimeouts(&p.Timeouts)
	}
	if err := SanitizeRouteTimeouts(&p.Timeouts); err != nil {
		return err
	}
	if p.BodyRewriteMiB < MinBodyRewriteMiB || p.BodyRewriteMiB > MaxBodyRewriteMiB {
		return fmt.Errorf("body rewrite ceiling %d MiB: allowed are %d to %d "+
			"(the cost is per request in flight, so a large one multiplies)",
			p.BodyRewriteMiB, MinBodyRewriteMiB, MaxBodyRewriteMiB)
	}
	return nil
}

// GetProxyLimits reads the setting, answering the defaults for an installation
// that never wrote one - or wrote one this build can no longer read.
func (s *Store) GetProxyLimits(ctx context.Context) ProxyLimits {
	p := DefaultProxyLimits()
	if err := s.GetSetting(ctx, SettingProxyLimits, &p); err != nil {
		return DefaultProxyLimits()
	}
	if err := SanitizeProxyLimits(&p); err != nil {
		return DefaultProxyLimits()
	}
	return p
}
