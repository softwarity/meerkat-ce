package filters

import (
	"context"
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
//     there declares their addresses on the Domains & TLS screen.
//   - link-local (169.254.0.0/16, fe80::/10): no proxy is addressed there, and
//     a neighbour on the same segment is not a proxy either.
//
// The cost of the default, said where it is decided: on a LAN where people
// reach the gateway DIRECTLY from a private address, each of them is trusted
// as a proxy and can name an address. That installation says on the Domains &
// TLS screen that the gateway serves its names itself, and nobody is believed.
var DefaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
}

// Whose word is taken is decided PER PLANE, by what the operator says about
// that plane on the Domains & TLS screen (SSL-14, certs.Door.Trusted): nothing
// said keeps the default above; a proxy in front means its declared addresses,
// or the default when none is declared; the gateway serving its names itself
// means nobody - nothing stands in front, so a peer writing X-Forwarded-For is
// a caller choosing its own address. It used to be one set for the whole
// process, read from MEERKAT_TRUSTED_PROXIES: a parameter nobody finds, for a
// fact the screen already states.

// planeRule is what one plane's door says.
type planeRule struct {
	// proxies are the peers believed; nil is DefaultTrustedProxies, empty is
	// nobody.
	proxies []netip.Prefix
	// https are the names a proxy in front serves over HTTPS: a request for
	// one of them reached the browser over TLS, whatever header says or does
	// not say so.
	https []string
}

// rules holds every plane's, swapped whole so a request never reads half of
// one. Written at each TLS reload, read on every request.
var rules atomic.Pointer[map[string]planeRule]

func init() { rules.Store(&map[string]planeRule{}) }

// SetPlane installs what one plane's door says. The plane is the one OnPlane
// names its requests with; "" is a request no plane named (a test, a
// synthesised request).
func SetPlane(plane string, proxies []netip.Prefix, httpsNames []string) {
	for {
		old := rules.Load()
		next := make(map[string]planeRule, len(*old)+1)
		for k, v := range *old {
			next[k] = v
		}
		r := planeRule{https: append([]string(nil), httpsNames...)}
		if proxies != nil {
			r.proxies = append([]netip.Prefix{}, proxies...)
		}
		next[plane] = r
		if rules.CompareAndSwap(old, &next) {
			return
		}
	}
}

type planeKey struct{}

// OnPlane names the plane every request of next arrives on, for the rules
// above to be read by. main wraps each plane's handler once, the plain door
// and the HTTPS one alike.
func OnPlane(plane string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), planeKey{}, plane)))
	})
}

func ruleOf(r *http.Request) planeRule {
	plane, _ := r.Context().Value(planeKey{}).(string)
	return (*rules.Load())[plane]
}

// trustedFor is the set of proxies believed on the plane r arrived on.
func trustedFor(r *http.Request) []netip.Prefix {
	if p := ruleOf(r).proxies; p != nil {
		return p
	}
	return DefaultTrustedProxies
}

// servedOverHTTPS reports whether r names a host a proxy in front of its
// plane serves over HTTPS.
func servedOverHTTPS(r *http.Request) bool {
	return servedOverHTTPSAs(r, ruleOf(r).https)
}

// servedOverHTTPSAs reports whether r names one of names.
func servedOverHTTPSAs(r *http.Request, names []string) bool {
	if len(names) == 0 {
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, n := range names {
		if n == host {
			return true
		}
	}
	return false
}

// isTrusted reports whether a is one of the proxies whose word we take on the
// plane r arrived on.
func isTrusted(r *http.Request, a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range trustedFor(r) {
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
	return ok && isTrusted(r, a)
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
	if !isTrusted(r, peer) {
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
		if !isTrusted(r, a) {
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

// TellUpstream writes, on a request leaving for a service, what the gateway
// believes about the browser's hop - the same answers it uses for itself, so
// the service and the gateway never disagree about where the caller is.
//
// It runs after httputil's SetXForwarded, which writes what IT sees: the
// gateway's own connection. Behind a proxy that terminates TLS that is plain
// http, and the service built http:// links under an https page - GeoServer's
// login posted to http:// and its own policy refused it (2026-10-14).
//
//   - X-Forwarded-Proto: Scheme, https when the gateway's door was TLS or a
//     trusted proxy said the browser's was.
//   - X-Forwarded-For: the chain a trusted proxy sent, the peer appended (as
//     SetXForwarded wrote it); from anybody else, the peer alone - a stranger's
//     chain names whatever address they liked, and a service reading its first
//     entry would believe it.
//   - Forwarded (RFC 7239), which httputil removes and never writes: one
//     element saying the same three things, for the services that read only
//     the standard header.
//
// X-Forwarded-Host stays the Host the gateway was reached by, which is what
// the gateway itself takes for its own address (Origin).
func TellUpstream(in *http.Request, out http.Header) {
	scheme := Scheme(in)
	out.Set("X-Forwarded-Proto", scheme)
	if !FromTrustedProxy(in) {
		out.Set("X-Forwarded-For", PeerIP(in))
	}
	out.Set("Forwarded", "for="+forwardedNode(ClientIP(in))+";host="+forwardedValue(in.Host)+";proto="+scheme)
}

// forwardedNode writes an address as RFC 7239 wants it: an IPv6 one in
// brackets and quotes, since its colons are not allowed in a bare token.
func forwardedNode(ip string) string {
	if a, err := netip.ParseAddr(ip); err == nil && a.Is6() {
		return `"[` + a.String() + `]"`
	}
	return forwardedValue(ip)
}

// forwardedValue quotes a value when it is not a plain token (a host with its
// port, for one: the colon is not a token character).
func forwardedValue(v string) string {
	for _, c := range v {
		if !isTokenChar(c) {
			return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
		}
	}
	return v
}

// isTokenChar is RFC 7230's tchar.
func isTokenChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", c)
}
