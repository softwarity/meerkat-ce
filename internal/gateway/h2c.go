package gateway

import (
	"net"
	"net/http"
	"net/url"
	"time"
)

// SchemeH2C is HTTP/2 over cleartext, which is what gRPC speaks inside a
// cluster (ROUTE-20).
//
// The ordinary transport cannot reach it, and ForceAttemptHTTP2 does not
// change that: it upgrades over TLS, through ALPN, so a plain-text upstream
// stays on HTTP/1.1 whatever it supports. gRPC needs HTTP/2 from the first
// byte - "prior knowledge" - which is a different transport, not a setting.
//
// The route DECLARES it in the one place an upstream's protocol belongs: the
// scheme. `h2c://checkout:50051` reads as what it is, sits in the field the
// console already has, and needs no second field to disagree with the first.
//
// Nothing else changes, and that is the point: predicates match a gRPC path
// (`/pkg.Service/Method`) like any other, identity forwarding writes headers
// and gRPC metadata IS HTTP/2 headers, and a route's access rules never looked
// at the protocol. The tests in h2c_test.go say each of those out loud, because
// "nothing else changes" is a claim, not an observation.
//
// It costs no dependency. Unencrypted HTTP/2 is in the standard library since
// Go 1.24 (http.Protocols), so this is a field on the transport the gateway
// already builds - x/net/http2 is not linked into the product for it.
const SchemeH2C = "h2c"

// upstreamSchemes are what an upstream URL may say, kept beside the transport
// that answers each one so that adding a protocol is one place and not three.
var upstreamSchemes = []string{"http", "https", SchemeH2C}

// h2cTarget rewrites an h2c URL into the http one the request carries. The
// scheme chose the transport; on the wire the request is a plain one.
func h2cTarget(u *url.URL) *url.URL {
	if u.Scheme != SchemeH2C {
		return u
	}
	c := *u
	c.Scheme = "http"
	return &c
}

// h2cTransportFor is transportFor's cleartext-HTTP/2 twin, pooled the same way
// and for the same reason: one transport per pair of bounds, not one per route.
// On a multiplexed protocol that matters more, not less - HTTP/2 exists to put
// many streams on one connection, and a pool per route would undo it.
func h2cTransportFor(connect, response time.Duration) http.RoundTripper {
	key := [2]time.Duration{connect, response}
	transportsMu.Lock()
	defer transportsMu.Unlock()
	if t, ok := h2cTransports[key]; ok {
		return t
	}
	// Unencrypted HTTP/2 and NOTHING else: a transport that could fall back to
	// HTTP/1.1 would reach a gRPC service and be refused by it, which reads as
	// a broken service rather than as a wrong scheme.
	var p http.Protocols
	p.SetUnencryptedHTTP2(true)
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: connect}).DialContext,
		ResponseHeaderTimeout: response,
		Protocols:             &p,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       55 * time.Second,
	}
	h2cTransports[key] = t
	return t
}

var h2cTransports = map[[2]time.Duration]http.RoundTripper{}
