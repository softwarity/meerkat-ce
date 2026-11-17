package tracing

import (
	"net"
	"net/url"
	"strings"
	"sync/atomic"
)

// What an audit event says it audits: one attribute of its resource, audit
// unless the installation names another, whose value is the SERVICE the
// event is about - audit=orders-api - or "meerkat" for the gateway's
// own events (its sign-ins, its console). A log backend selects on it and
// keeps by it: Loki indexes it as a label, a retention rule on {audit=~".+"}
// keeps the audit a year, and a line WITHOUT it is a log. So the logs carry
// nothing: the absence of the attribute is what makes a line a log, and
// service.name already says who wrote it.
//
// The installation sets the attribute and the value template; {servicename}
// in the template is the service behind the route the event belongs to -
// orders-api, not the route's display name, so three routes onto one
// service share it. A route may set its own value, never the attribute. The
// router hands the whole set over at every reload (SetStreams), so an event
// is resolved without a lookup in the store.

// The default attribute and value, and the token a template names the
// service with - "meerkat" standing for what belongs to no route.
const (
	DefaultAuditKey    = "audit"
	DefaultAuditStream = "{servicename}"
	StreamServiceToken = "{servicename}"
	StreamServiceNone  = "meerkat"
)

// Streams is the installation's audit attribute and template, and each
// route's own.
type Streams struct {
	AuditKey, Audit string
	// Routes is keyed by route name.
	Routes map[string]RouteStreams
}

// RouteStreams is one route's own audit template (empty follows the
// installation) and the service behind it, what {servicename} stands for.
type RouteStreams struct{ Audit, Service string }

var streams atomic.Pointer[Streams]

// SetStreams replaces the attribute and templates in force.
func SetStreams(s Streams) { streams.Store(&s) }

// AuditStreamKey is the attribute an audit event says what it audits under.
func AuditStreamKey() string {
	if s := streams.Load(); s != nil && s.AuditKey != "" {
		return s.AuditKey
	}
	return DefaultAuditKey
}

// AuditStream is what an audit event of route (its name, or "") audits.
func AuditStream(routeName string) string {
	var route, installation string
	service := routeName
	if s := streams.Load(); s != nil {
		installation = s.Audit
		if r, ok := s.Routes[routeName]; ok && routeName != "" {
			if r.Service != "" {
				service = r.Service
			}
			route = r.Audit
		}
	}
	return ResolveStream(route, installation, DefaultAuditStream, service)
}

// ServiceOf names the service behind an upstream: the first label of its
// host (orders-api.shop.svc.cluster.local is orders-api, a
// Swarm or Compose name is itself), an address whole, and fallback when the
// upstream names no host - empty, or a vault reference not yet resolved.
func ServiceOf(upstream, fallback string) string {
	u, err := url.Parse(strings.TrimSpace(upstream))
	if err != nil || strings.Contains(upstream, "$") {
		return fallback
	}
	host := u.Hostname()
	if host == "" {
		return fallback
	}
	if net.ParseIP(host) != nil {
		return host
	}
	return strings.SplitN(host, ".", 2)[0]
}

// ResolveStream is the value for service from the route's own template, else
// the installation's, else the default.
func ResolveStream(route, installation, def, service string) string {
	t := route
	if t == "" {
		t = installation
	}
	if t == "" {
		t = def
	}
	if service == "" {
		service = StreamServiceNone
	}
	return strings.ReplaceAll(t, StreamServiceToken, service)
}
