package store

import (
	"fmt"
	"time"
)

// How long a route's upstream may take (ROUTE-07).
//
// The gateway is on the path of every request, so a slow upstream is not that
// service's problem: it is everybody's. A connection held open for three
// minutes is a connection not serving anyone else, and the failure surfaces
// far from where it started - which is why it is diagnosed badly and late.
//
// The bounds are per ROUTE because that is the grain a service has here (the
// Service entity was dropped; a route carries everything). Until now they were
// one global pair, so a report endpoint that legitimately thinks for a minute
// forced everybody else to wait a minute too.
//
// What is NOT bounded, deliberately, is the body. A download and a websocket
// must be allowed to last: what is bounded is how long an upstream may take to
// ACCEPT a connection and to START answering.

// Default bounds - what every route ran with when they were global.
const (
	DefaultConnectTimeout  = 5 * time.Second
	DefaultResponseTimeout = 15 * time.Second
)

// The range an operator may choose within. A maximum exists because these are
// the guard rails themselves: a response bound of an hour is a route with no
// bound at all, written in a way that looks like one.
const (
	MinRouteTimeout    = time.Second
	MaxConnectTimeout  = 30 * time.Second
	MaxResponseTimeout = 10 * time.Minute
)

// RouteTimeouts is the pair, as ISO-8601 durations - the spelling this product
// already uses for a session TTL, a rate-limit window and an outage estimate.
// Empty means the default, so a route that never asked keeps the behaviour it
// always had.
type RouteTimeouts struct {
	// Connect bounds accepting the connection (and the TLS handshake with it).
	Connect string `json:"connect,omitempty"`
	// Response bounds the wait for the answer's HEADERS. The body that follows
	// is not bounded at all.
	Response string `json:"response,omitempty"`
}

// SanitizeRouteTimeouts refuses what cannot work, naming the range - and it is
// the only place that decides, so the six writers of a route all get it.
func SanitizeRouteTimeouts(t *RouteTimeouts) error {
	if t == nil {
		return nil
	}
	if _, err := boundedDuration("connect", t.Connect, MaxConnectTimeout); err != nil {
		return err
	}
	if _, err := boundedDuration("response", t.Response, MaxResponseTimeout); err != nil {
		return err
	}
	return nil
}

// Durations resolves the pair to what the transport needs.
//
// Three layers, narrowest first: what this route asked for, then what the
// installation set (Infra > Global), then the product's own. Each bound falls
// back on its own - naming one must not quietly move the other.
func (t *RouteTimeouts) Durations(defaults RouteTimeouts) (connect, response time.Duration) {
	connect, response = defaults.own()
	if t == nil {
		return connect, response
	}
	if d, err := boundedDuration("connect", t.Connect, MaxConnectTimeout); err == nil && d > 0 {
		connect = d
	}
	if d, err := boundedDuration("response", t.Response, MaxResponseTimeout); err == nil && d > 0 {
		response = d
	}
	return connect, response
}

// own is the installation's pair, falling back to the product's.
func (t RouteTimeouts) own() (connect, response time.Duration) {
	connect, response = DefaultConnectTimeout, DefaultResponseTimeout
	if d, err := boundedDuration("connect", t.Connect, MaxConnectTimeout); err == nil && d > 0 {
		connect = d
	}
	if d, err := boundedDuration("response", t.Response, MaxResponseTimeout); err == nil && d > 0 {
		response = d
	}
	return connect, response
}

func boundedDuration(what, iso string, longest time.Duration) (time.Duration, error) {
	if iso == "" {
		return 0, nil
	}
	d, err := ParseISODuration(iso)
	if err != nil {
		return 0, fmt.Errorf("%s timeout: %w", what, err)
	}
	if d < MinRouteTimeout || d > longest {
		return 0, fmt.Errorf("%s timeout %s is outside what a gateway can promise: allowed are %s to %s",
			what, iso, MinRouteTimeout, longest)
	}
	return d, nil
}
