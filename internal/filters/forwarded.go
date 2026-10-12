package filters

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
)

// Who is calling, as this gateway is entitled to believe it.
//
// The TCP peer is the one address nobody can choose: it is the end of the
// connection the gateway accepted. X-Forwarded-For is a header, and a header is
// whatever the caller typed. It used to be read whoever sent it, rightmost
// entry first, on the theory that the last hop wrote the last entry - true
// behind a proxy, and false the moment somebody reaches the gateway directly:
// then the "last hop" is the caller themselves, and a new address per request
// walked through every counter keyed on it. Fifty wrong passwords against a
// budget of three gave fifty 401 and not one 429.
//
// So the header is believed only from a peer that is a TRUSTED PROXY, and the
// chain is walked from the right, through the proxies we trust, to the first
// address that is not one of them. That is the client: the first hop nobody we
// trust vouches for beyond. Every entry to its left was written by somebody we
// have no reason to believe.
//
// Written HERE so every caller shares one rule. It decides who gets counted
// (the route rate limits, the sign-in and reset throttles), what a sign-in
// history and the security trail show, and - for X-Forwarded-Proto - whether
// a cookie is Secure. A security rule written twice is a rule with a copy that
// drifts.

// DefaultTrustedProxies is what is trusted when nobody said: the loopback and
// the private ranges, where the proxy in front of a gateway lives in every
// shape this product ships for - the Docker bridge (172.17/16), a compose
// network (172.16/12 or 192.168/16), a Kubernetes pod or node network
// (10/8), an Ingress controller beside the pod, nginx on the same host.
//
// Deduced from the peer and not configured, because a setting that has to be
// right before the gateway counts anyone correctly is a setting that is wrong
// on the first day. A peer with a private address is, in all those shapes, the
// proxy; a peer with a public one is a stranger, and its header is ignored.
//
// NOT in the list:
//
//   - 100.64.0.0/10, the carrier-grade NAT range. It is the address space of
//     SUBSCRIBERS - a mobile operator's customers, and every device on a
//     Tailscale network - not of infrastructure. A gateway that sees a peer
//     there is talking to an end user's device, and trusting it would let that
//     device name any address it likes. A platform that does put its proxies
//     there sets MEERKAT_TRUSTED_PROXIES.
//   - link-local (169.254.0.0/16, fe80::/10): no proxy is addressed there, and
//     a neighbour on the same segment is not a proxy either.
//
// The cost of the default, said where it is decided: on a LAN where people
// reach the gateway DIRECTLY from a private address, each of them is trusted
// as a proxy and can name an address. That installation sets the variable to
// its proxy's address, or to "none".
var DefaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
}

// NoTrustedProxies is the value that trusts nobody: every header is ignored and
// the peer is the client, always. For a gateway reached directly from the
// internet, where any peer that sends X-Forwarded-For is lying.
const NoTrustedProxies = "none"

// trusted holds the set in force. A pointer swapped whole, so a request never
// reads half a list: it is written once at startup and read on every request.
var trusted atomic.Pointer[[]netip.Prefix]

func init() { SetTrustedProxies(DefaultTrustedProxies) }

// SetTrustedProxies replaces the set. An empty, non-nil slice trusts nobody;
// nil goes back to the default.
func SetTrustedProxies(p []netip.Prefix) {
	if p == nil {
		p = DefaultTrustedProxies
	}
	cp := append([]netip.Prefix(nil), p...)
	trusted.Store(&cp)
}

// TrustedProxies is the set in force, for the startup line that says it.
func TrustedProxies() []netip.Prefix { return *trusted.Load() }

// ParseTrustedProxies reads MEERKAT_TRUSTED_PROXIES: CIDR ranges or single
// addresses, separated by commas. Empty means the default, "none" means nobody.
// A value REPLACES the default rather than adding to it: someone who names
// their proxies is saying which ones they are, and a default still standing
// beside the list would keep trusting every private address they meant to
// exclude.
func ParseTrustedProxies(raw string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.EqualFold(raw, NoTrustedProxies) {
		return []netip.Prefix{}, nil
	}
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(part, NoTrustedProxies) {
			return nil, fmt.Errorf("trusted proxies %q: %q trusts nobody and cannot be combined with addresses", raw, NoTrustedProxies)
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("trusted proxies: %q is not a CIDR range: write e.g. 10.0.0.0/8, 192.168.1.10, 2001:db8::/32 or %s", part, NoTrustedProxies)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("trusted proxies: %q is not an address: write e.g. 10.0.0.0/8, 192.168.1.10, 2001:db8::/32 or %s", part, NoTrustedProxies)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("trusted proxies %q: no address in it; leave it empty for the default or write %s", raw, NoTrustedProxies)
	}
	return out, nil
}

// isTrusted reports whether a is one of the proxies whose word we take.
func isTrusted(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range *trusted.Load() {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// peerAddr is the TCP peer, parsed. ok is false for a RemoteAddr that is not
// an address at all (a unix socket, a synthesised request): nothing about it
// can be trusted, so nothing it carries is believed.
func peerAddr(r *http.Request) (netip.Addr, bool) {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

// PeerIP is the TCP peer's address: the hop the gateway is actually talking
// to, which is the client only when nothing stands in front.
func PeerIP(r *http.Request) string {
	if a, ok := peerAddr(r); ok {
		return a.String()
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// FromTrustedProxy says whether the peer is one whose forwarded headers are
// believed.
func FromTrustedProxy(r *http.Request) bool {
	a, ok := peerAddr(r)
	return ok && isTrusted(a)
}

// forwardedEntry reads one X-Forwarded-For entry. Proxies write a bare address,
// but some add the port, and IPv6 then comes in brackets.
func forwardedEntry(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap(), true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	if a, err := netip.ParseAddr(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")); err == nil {
		return a.Unmap(), true
	}
	return netip.Addr{}, false
}

// ClientIP is the caller's address as this gateway is entitled to believe it:
// the peer, unless the peer is a trusted proxy - then the rightmost entry of
// X-Forwarded-For that is not itself a trusted proxy, or the leftmost when the
// whole chain is trusted.
//
// An entry that is not an address stops the walk at the last one that was: it
// was written by a proxy we trust, and we cannot read what it meant. The
// nearest address we can vouch for is a more honest answer than a word like
// "unknown" becoming the key every caller shares.
func ClientIP(r *http.Request) string {
	peer, ok := peerAddr(r)
	if !ok {
		return PeerIP(r)
	}
	if !isTrusted(peer) {
		return peer.String()
	}
	// Every X-Forwarded-For line, in order: a proxy may add a line rather than
	// extend the last one, and the chain is all of them read as one list.
	var entries []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		for _, e := range strings.Split(v, ",") {
			if e = strings.TrimSpace(e); e != "" {
				entries = append(entries, e)
			}
		}
	}
	believed := peer
	for i := len(entries) - 1; i >= 0; i-- {
		a, ok := forwardedEntry(entries[i])
		if !ok {
			return believed.String()
		}
		if !isTrusted(a) {
			return a.String()
		}
		believed = a
	}
	return believed.String()
}

// ClientAndPeer is ClientIP with the peer beside it when the two differ - what
// a record of who signed in keeps, so it never shows only an address a header
// claimed: "203.0.113.5 via 10.42.0.15" says who was believed AND on whose
// word. peer is empty when the client is the peer.
func ClientAndPeer(r *http.Request) (client, peer string) {
	client, peer = ClientIP(r), PeerIP(r)
	if client == peer {
		peer = ""
	}
	return client, peer
}
