package gateway

import (
	"net/url"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/routing"
)

// Where an application may receive its sign-in without declaring it.
//
// An application signing its people in through this gateway is, nearly
// always, served by the very routes that name its client: RabbitMQ under
// /rabbitmq, a database console under /dbgate. Asking the administrator to
// type, once more, an address the routes already describe was a step where
// the only thing that could happen was a typo. So the provider accepts a
// redirect_uri the routes themselves serve, and a declared one stays for an
// application served elsewhere.
//
// The two halves stay bound: the origins of ONE route with the paths of THAT
// route. A route on app.example.com covering its whole host says nothing
// about the gateway's own name, and taking the hosts of one route with the
// paths of another would accept an address no route serves.
//
// Computed here, in the community tree, because the console shows it: what
// the administrator reads under the client is what the provider accepts,
// from the same function.

// OIDCReturn is one place a route bound to a client serves its application.
type OIDCReturn struct {
	// Route is the route's name, for the console and the log.
	Route string
	// Origins are the scheme://host[:port] the route answers on, the
	// likeliest first: the gateway's public origin for a route without a
	// host predicate, the predicate's names otherwise.
	Origins []string
	// Prefix is the literal head of a path pattern, "/" when the route covers
	// the whole host.
	Prefix string
}

// WholeHost says whether the route covers every path of its host: any
// address there may then receive a code for the client.
func (r OIDCReturn) WholeHost() bool { return r.Prefix == "/" }

// String is the place as an administrator reads it: https://host/prefix/.
func (r OIDCReturn) String() string {
	if len(r.Origins) == 0 {
		return ""
	}
	if r.WholeHost() {
		return r.Origins[0] + "/"
	}
	return r.Origins[0] + r.Prefix + "/"
}

// OIDCReturns are the places the routes serve, in routing order. issuerOrigin
// is the gateway's public origin (oidcprovider.IssuerOrigin), "" while the
// issuer is not an address: a route without a host predicate then names no
// origin, and accepts nothing.
func OIDCReturns(issuerOrigin string, routes []OIDCRoute) []OIDCReturn {
	scheme, host, port := "https", "", ""
	if u, err := url.Parse(issuerOrigin); err == nil && u.Host != "" {
		scheme, host, port = u.Scheme, strings.ToLower(u.Hostname()), u.Port()
	}
	var out []OIDCReturn
	for _, r := range routes {
		origins := routeOrigins(r.hosts, issuerOrigin, scheme, host, port)
		if len(origins) == 0 {
			continue
		}
		for _, prefix := range intersectHeads(r.paths) {
			out = append(out, OIDCReturn{Route: r.Name, Origins: origins, Prefix: prefix})
		}
	}
	return out
}

// routeOrigins are the origins a route answers on. Without a host predicate,
// the gateway's public one: the route answers every name, and that is the
// name people reach it by. With one, each exact name every predicate accepts,
// on https or the issuer's scheme, without a port or on the issuer's - the
// listener the public address is served by. A wildcard names no host an
// application could be at, so it adds only the gateway's own name, when it
// covers it.
func routeOrigins(hosts [][]string, issuerOrigin, scheme, issuerHost, port string) []string {
	if len(hosts) == 0 {
		if issuerOrigin == "" {
			return nil
		}
		return []string{normOrigin(issuerOrigin)}
	}
	var names []string
	all := func(h string) bool {
		for _, list := range hosts {
			if !routing.HostMatches(list, h) {
				return false
			}
		}
		return true
	}
	if issuerHost != "" && all(issuerHost) {
		names = append(names, issuerHost)
	}
	for _, h := range hosts[0] {
		if !strings.HasPrefix(h, "*.") && all(h) && !slices.Contains(names, h) {
			names = append(names, h)
		}
	}
	var out []string
	add := func(o string) {
		if o = normOrigin(o); !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	for _, n := range names {
		if n == issuerHost && issuerOrigin != "" {
			add(issuerOrigin)
		}
		for _, s := range []string{scheme, "https"} {
			if port != "" {
				add(s + "://" + n + ":" + port)
			}
			add(s + "://" + n)
		}
	}
	return out
}

// intersectHeads is where every path predicate agrees: for each list, the
// heads of the ones before it narrowed to those under one of its own. No path
// predicate is the whole host.
func intersectHeads(paths [][]string) []string {
	heads := []string{"/"}
	for _, list := range paths {
		var next []string
		for _, h := range heads {
			for _, p := range list {
				var keep string
				switch {
				case pathUnder(p, h):
					keep = p
				case pathUnder(h, p):
					keep = h
				default:
					continue
				}
				if !slices.Contains(next, keep) {
					next = append(next, keep)
				}
			}
		}
		heads = next
	}
	return heads
}

// pathUnder says whether path is prefix or below it, segment by segment:
// /rabbitmq covers /rabbitmq/callback, never /rabbitmqx.
func pathUnder(path, prefix string) bool {
	if prefix == "/" {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// normOrigin lower-cases an origin and drops the scheme's default port, so
// one written two ways compares as one.
func normOrigin(o string) string {
	o = strings.ToLower(o)
	if rest, ok := strings.CutSuffix(o, ":443"); ok && strings.HasPrefix(o, "https://") {
		return rest
	}
	if rest, ok := strings.CutSuffix(o, ":80"); ok && strings.HasPrefix(o, "http://") {
		return rest
	}
	return o
}

// OIDCReturnAccepts says whether a code may be sent to uri because a route
// serves it. The address must be one a browser lands on as written: absolute,
// no credentials, no fragment the code would be lost behind, and no dot
// segment or encoded separator - /rabbitmq/../admin is /admin once the
// browser has read it, and a prefix check on the text would have let it by.
// A query is the application's and is kept.
func OIDCReturnAccepts(returns []OIDCReturn, uri string) bool {
	if len(returns) == 0 || strings.ContainsAny(uri, "#\\") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" ||
		(u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	raw := strings.ToLower(u.EscapedPath())
	if strings.Contains(raw, "%2e") || strings.Contains(raw, "%2f") || strings.Contains(raw, "%5c") {
		return false
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	origin := normOrigin(u.Scheme + "://" + u.Host)
	for _, r := range returns {
		if slices.Contains(r.Origins, origin) && pathUnder(path, r.Prefix) {
			return true
		}
	}
	return false
}

// OIDCReturnOrigins are the origins of the places, once each: where an
// application's browser code runs, and may read the provider's answers from.
func OIDCReturnOrigins(returns []OIDCReturn) []string {
	var out []string
	for _, r := range returns {
		for _, o := range r.Origins {
			if !slices.Contains(out, o) {
				out = append(out, o)
			}
		}
	}
	return out
}
