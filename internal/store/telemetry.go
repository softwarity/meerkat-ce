package store

import (
	"fmt"
	"strings"
)

// Where this gateway's traces go, and how much of them (OBS-04).
//
// The same shape as the Prometheus switch: delivered OFF, decided by a person
// rather than inherited from an upgrade, and pointing at something the client
// already runs. We speak OTLP; whether that reaches an OpenTelemetry
// Collector, Tempo, Jaeger or a vendor is their business and not a setting
// here.
//
// THE CREDENTIAL IS A VAULT REFERENCE, never a literal. A collector on the
// public internet wants an api key, and an api key in a settings row is an api
// key in an export, in a backup, in the audit trail and on this screen. The
// reference travels instead and is resolved at the moment of the call - the
// rule the route headers already follow (VAULT-05).

// TelemetryConfig is the whole setting.
type TelemetryConfig struct {
	// Enabled is the decision that anything leaves this gateway for a
	// collector at all. WHAT leaves is the two signals below.
	Enabled bool `json:"enabled"`
	// Traces sends this gateway's spans (OBS-04). Which routes produce any is
	// still each route's own answer.
	Traces bool `json:"traces"`
	// Metrics pushes the same counters /metrics exposes, over OTLP, to the
	// same collector (OBS-05): the other way out for an installation whose
	// stack receives rather than scrapes. Cumulative, one resource per node.
	Metrics bool `json:"metrics,omitempty"`
	// Endpoint is the collector's BASE address. The /v1/traces path is added
	// by the exporter: a caller writing it themselves writes it wrong once.
	Endpoint string `json:"endpoint"`
	// Headers travel with every export. Values may be vault references
	// (`$name`), which is how a SaaS key reaches a collector without ever
	// being stored here.
	Headers map[string]string `json:"headers,omitempty"`
	// Sample is the share of journeys OPENED HERE that get recorded, wherever
	// "here" is: the front door for a call that arrives undecided, and the
	// injected bundle for a page that starts its own. One rate, because a
	// journey is opened once and the two places are never both the opener.
	//
	// A journey a caller already decided about is not ours to decide again, so
	// this does not apply to those - see MaxPerSecond for what does. That is
	// also why a SECOND rate for the browser was a lie: the gateway honours a
	// page's decision, so a browser rate silently overruled the gateway one
	// for every request coming from an instrumented page.
	Sample float64 `json:"sample"`
	// MaxPerSecond is the ceiling on journeys recorded per second, whoever
	// decided. It is the budget, and the only thing that bounds a caller who
	// sets the sampling flag on every request. Zero means no ceiling.
	MaxPerSecond int64 `json:"maxPerSecond"`

	// GatewayDetail adds the gateway's OWN steps to the traces it records: the
	// identity handed to an upstream, each query to the store. Off by default,
	// because most installations trace their own services and do not need the
	// inside of the gateway in every trace; on for as long as somebody is
	// looking at where the gateway's time goes. It never adds a trace - it only
	// deepens the ones already sampled - but each of those weighs more in the
	// collector.
	GatewayDetail bool `json:"gatewayDetail,omitempty"`
}

// WHERE THE BROWSER HALF WENT. It used to be a switch here - "start traces in
// the browser" - and it said the same thing as a route's own answer from a
// second place. Injecting the bundle is a property of ONE application: the
// route says whether its pages open the journey, and this screen keeps the
// questions that are the installation's (where traces go, how many, at what
// cost). The relay follows the routes, so adding the first such route opens
// the path and removing the last one closes it.

// DefaultTelemetry is what an installation that never touched this has.
func DefaultTelemetry() TelemetryConfig {
	return TelemetryConfig{Traces: true, Sample: 0.1, MaxPerSecond: 200}
}

// maxTelemetryHeaders bounds what travels with an export. A collector wants
// one header, sometimes two; a dozen is somebody filling the row.
const maxTelemetryHeaders = 8

// reservedTelemetryHeaders are the ones the exporter writes itself. Refused by
// name rather than silently overwritten: a header that is ignored without a
// word is an afternoon somebody does not get back.
var reservedTelemetryHeaders = []string{"content-type", "content-length", "host"}

// ExportsTraces says whether spans leave: the export is on AND carries them.
func (c TelemetryConfig) ExportsTraces() bool { return c.Enabled && c.Traces }

// ExportsMetrics says whether the counters are pushed.
func (c TelemetryConfig) ExportsMetrics() bool { return c.Enabled && c.Metrics }

// SanitizeTelemetry refuses what cannot work, and names what is allowed.
func SanitizeTelemetry(c *TelemetryConfig) error {
	c.Endpoint = strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	if c.Enabled && !c.Traces && !c.Metrics {
		return fmt.Errorf("the export is on and would send nothing: choose traces, metrics, or both")
	}
	if c.Enabled && c.Endpoint == "" {
		return fmt.Errorf("the export needs a collector to send to, such as http://otel-collector:4318")
	}
	if c.Endpoint != "" &&
		!strings.HasPrefix(c.Endpoint, "http://") && !strings.HasPrefix(c.Endpoint, "https://") {
		return fmt.Errorf("the collector address %q must start with http:// or https://", c.Endpoint)
	}
	// The path is ours to add. Somebody who pastes the full endpoint from a
	// vendor's documentation gets it taken off rather than a 404 they have to
	// work out for themselves.
	c.Endpoint = strings.TrimSuffix(strings.TrimSuffix(c.Endpoint, "/v1/traces"), "/v1/metrics")

	if len(c.Headers) > maxTelemetryHeaders {
		return fmt.Errorf("an export carries %d headers, %d at most: it authenticates to a collector, it does not carry a configuration",
			len(c.Headers), maxTelemetryHeaders)
	}
	for name := range c.Headers {
		clean := strings.TrimSpace(name)
		if clean == "" {
			return fmt.Errorf("a header needs a name")
		}
		for _, reserved := range reservedTelemetryHeaders {
			if strings.EqualFold(clean, reserved) {
				return fmt.Errorf("the header %q is written by the exporter itself and cannot be set here (reserved: %s)",
					name, strings.Join(reservedTelemetryHeaders, ", "))
			}
		}
	}
	if err := shareInRange("sample", c.Sample); err != nil {
		return err
	}
	if c.MaxPerSecond < 0 {
		return fmt.Errorf("maxPerSecond is a number of journeys per second and cannot be negative; 0 means no ceiling")
	}
	return nil
}

func shareInRange(field string, v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("%s is a share between 0 and 1 (0.1 is a tenth of the journeys), got %v", field, v)
	}
	return nil
}
