package certs

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// Door is what an operator says about one external plane - the console or
// the application - on the Domains & TLS screen (SSL-14): the PUBLIC names a
// browser reaches it by, and who serves them over HTTPS.
//
// Two shapes, and nothing in between:
//
//   - This gateway (Proxied false): the gateway presents the names itself,
//     with the certificates placed on the plane. Each declared name has to be
//     covered by one of them.
//   - A proxy in front (Proxied true): an Ingress, nginx, a cloud load
//     balancer serves the names over HTTPS and talks to the gateway behind it,
//     in clear or re-encrypted. A certificate placed on the plane then
//     encrypts THAT hop, so it carries the plane's names inside the runtime,
//     never the public ones.
//
// The names are full names, never a wildcard: they are what a mailed link is
// written with and what the scheme is decided on, and *.example.com would
// vouch for every name under it - including one left pointing at a service
// nobody runs any more. A certificate may carry a wildcard; a declared name
// may not.
type Door struct {
	Proxied bool `json:"proxied,omitempty"`
	// Names are the public names, in the order the operator gave them: the
	// first of the application's is the one an address is written with when
	// no request says which (a mailed link, an identity provider's callback).
	Names []string `json:"names,omitempty"`
	// ProxyAddresses is where the proxy reaches the gateway from, when it is
	// not a private network: a CDN, a load balancer with public addresses.
	// Empty means the loopback and the private ranges. Read only when Proxied.
	ProxyAddresses []string `json:"proxyAddresses,omitempty"`
}

// Said reports whether the operator said anything about this door: a door
// nobody configured keeps the behaviour of before SSL-14 (forwarded headers
// believed from a private peer, every placed certificate served).
func (d Door) Said() bool {
	return d.Proxied || len(d.Names) > 0
}

// DoorOn is one external plane's door. The internal plane has none: its
// callers are programs inside the cluster, reaching it by its runtime names.
func (s Settings) DoorOn(plane string) Door {
	switch plane {
	case PlaneConsole:
		return s.ConsoleDoor
	case PlaneApp:
		return s.AppDoor
	}
	return Door{}
}

// SetDoor replaces one external plane's door.
func (s *Settings) SetDoor(plane string, d Door) {
	switch plane {
	case PlaneConsole:
		s.ConsoleDoor = d
	case PlaneApp:
		s.AppDoor = d
	}
}

// ProxiedOn is the names a proxy in front of one door serves over HTTPS,
// none when the gateway serves them itself.
func (s Settings) ProxiedOn(plane string) []string {
	if d := s.DoorOn(plane); d.Proxied {
		return d.Names
	}
	return nil
}

// Active says whether a certificate placed on a plane is one the plane serves
// with, given the door and the plane's names inside the runtime. The others
// stay placed, shown as not used in this mode: switching back brings them back.
//
//   - A proxy in front: the certificates carrying one of the runtime names -
//     the name the proxy reaches the plane by. None known (no runtime to ask):
//     every one but those covering the public names, which are the proxy's
//     to present.
//   - This gateway, names declared: the certificates covering one of them.
//   - Nothing said: every one, as before SSL-14.
func (d Door) Active(certNames, runtimeNames []string) bool {
	switch {
	case d.Proxied:
		if len(runtimeNames) > 0 {
			return coversOne(certNames, runtimeNames)
		}
		return !coversOne(certNames, d.Names)
	case len(d.Names) > 0:
		return coversOne(certNames, d.Names)
	}
	return true
}

func coversOne(certNames, hosts []string) bool {
	for _, h := range hosts {
		if Covers(certNames, h) {
			return true
		}
	}
	return false
}

// Trusted is whose forwarded headers this door believes (SEC-11):
//
//   - nothing said: the loopback and the private ranges, as before SSL-14 -
//     returned as nil, which the filters read as their default;
//   - a proxy in front: the addresses declared, the private ranges when none;
//   - this gateway, names declared: nobody - nothing stands in front, so a
//     peer that writes X-Forwarded-For is a caller choosing its own address.
func (d Door) Trusted() ([]netip.Prefix, error) {
	switch {
	case d.Proxied:
		p, _, err := ProxyAddresses(d.ProxyAddresses)
		if err != nil || len(p) == 0 {
			return nil, err
		}
		return p, nil
	case d.Said():
		return []netip.Prefix{}, nil
	}
	return nil, nil
}

// ProxyAddresses checks the addresses a proxy reaches the gateway from: CIDR
// ranges or single addresses.
func ProxyAddresses(in []string) ([]netip.Prefix, []string, error) {
	var prefixes []netip.Prefix
	var kept []string
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		p, err := parsePrefix(s)
		if err != nil {
			return nil, nil, fmt.Errorf("%q is not an address or a CIDR range: write e.g. 130.211.0.0/22, 203.0.113.10 or 2001:db8::/32", raw)
		}
		if !slices.Contains(prefixes, p) {
			prefixes = append(prefixes, p)
			kept = append(kept, p.String())
		}
	}
	return prefixes, kept, nil
}

func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	a = a.Unmap()
	return netip.PrefixFrom(a, a.BitLen()), nil
}

// Opens says whether a plane's door of that scheme is switched on (SSL-15):
// only "http" is ever switched off - HTTPS follows the certificates.
func (s Settings) Opens(plane, scheme string) bool {
	return !slices.Contains(s.Closed[plane], scheme)
}

// SetOpen switches one door of a plane on or off.
func (s *Settings) SetOpen(plane, scheme string, open bool) {
	closed := slices.DeleteFunc(slices.Clone(s.Closed[plane]), func(c string) bool { return c == scheme })
	if !open {
		closed = append(closed, scheme)
		slices.Sort(closed)
	}
	if s.Closed == nil {
		s.Closed = map[string][]string{}
	}
	if len(closed) == 0 {
		delete(s.Closed, plane)
	} else {
		s.Closed[plane] = closed
	}
	if len(s.Closed) == 0 {
		s.Closed = nil
	}
}
