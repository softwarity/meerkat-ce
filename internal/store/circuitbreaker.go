package store

import (
	"fmt"
	"time"
)

// CircuitBreaker says when a route stops calling an upstream that has stopped
// answering (ROUTE-09).
//
// Off by default, and that is a decision rather than caution: a breaker turns
// one kind of failure into another - a service that is merely slow to recover
// gets refused for the whole cool-down - and an installation should meet that
// because somebody chose it. Where it is on, it stops a dead service from
// eating the gateway's own capacity, and stops it being met by a stampede the
// moment it comes back.
type CircuitBreaker struct {
	Enabled bool `json:"enabled,omitempty"`
	// Trip is how many CONSECUTIVE failures open the circuit. Consecutive on
	// purpose: what this looks for is a service that stopped answering, not
	// one that fails now and then under load.
	Trip int `json:"trip,omitempty"`
	// Cool is how long it stays open before one request is let through to find
	// out, as an ISO-8601 duration.
	Cool string `json:"cool,omitempty"`
}

// The defaults, and the range around them. Five failures is enough that a
// single blip does not open anything, few enough that a dead service is
// noticed within a second or two of traffic.
const (
	DefaultBreakerTrip = 5
	DefaultBreakerCool = 15 * time.Second
	MinBreakerTrip     = 2
	MaxBreakerTrip     = 100
	MinBreakerCool     = time.Second
	MaxBreakerCool     = 5 * time.Minute
)

// Settings resolves the pair, defaults included.
func (c CircuitBreaker) Settings() (trip int, cool time.Duration) {
	trip, cool = DefaultBreakerTrip, DefaultBreakerCool
	if c.Trip > 0 {
		trip = c.Trip
	}
	if c.Cool != "" {
		if d, err := ParseISODuration(c.Cool); err == nil && d > 0 {
			cool = d
		}
	}
	if !c.Enabled {
		// Never trips. Expressed as a number rather than as a branch at every
		// call site, so "off" and "on" travel the same path.
		trip = 1 << 30
	}
	return trip, cool
}

// SanitizeCircuitBreaker refuses what cannot work, naming the range.
func SanitizeCircuitBreaker(c *CircuitBreaker) error {
	if c == nil {
		return nil
	}
	if c.Trip != 0 && (c.Trip < MinBreakerTrip || c.Trip > MaxBreakerTrip) {
		return fmt.Errorf("circuit breaker: %d failures is outside what is useful: allowed are %d to %d",
			c.Trip, MinBreakerTrip, MaxBreakerTrip)
	}
	if c.Cool != "" {
		d, err := ParseISODuration(c.Cool)
		if err != nil {
			return fmt.Errorf("circuit breaker cool-down: %w", err)
		}
		if d < MinBreakerCool || d > MaxBreakerCool {
			return fmt.Errorf("circuit breaker cool-down %s is outside what is useful: allowed are %s to %s",
				c.Cool, MinBreakerCool, MaxBreakerCool)
		}
	}
	return nil
}
