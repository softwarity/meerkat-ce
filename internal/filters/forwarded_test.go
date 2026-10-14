package filters

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// withTrusted runs a test under a given set and puts the default back: the set
// is process-wide, as it is in the gateway.
func withTrusted(t *testing.T, raw string) {
	t.Helper()
	p, err := ParseTrustedProxies(raw)
	if err != nil {
		t.Fatal(err)
	}
	SetTrustedProxies(p)
	t.Cleanup(func() { SetTrustedProxies(nil) })
}

func reqFrom(peer string, xff ...string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	r.RemoteAddr = peer
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

// The bypass the audit reproduced: a caller reaching the gateway directly
// wrote X-Forwarded-For, the rightmost entry was believed, and a new address
// per request was a new counter per request. From a PUBLIC peer the header is
// ignored, so whatever is written there the key stays the peer.
func TestPublicPeerCannotNameItsAddress(t *testing.T) {
	withTrusted(t, "")
	seen := map[string]bool{}
	for _, forged := range []string{"1.1.1.1", "2.2.2.2", "10.0.0.1, 3.3.3.3", "::1"} {
		seen[ClientIP(reqFrom("203.0.113.9:4000", forged))] = true
	}
	if len(seen) != 1 || !seen["203.0.113.9"] {
		t.Fatalf("a public peer chose its address: %v", seen)
	}
	if c, p := ClientAndPeer(reqFrom("203.0.113.9:4000", "1.1.1.1")); c != "203.0.113.9" || p != "" {
		t.Fatalf("client/peer = %q/%q, want the peer alone", c, p)
	}
}

// Behind the proxies a gateway ships with - an Ingress in the pod network, the
// Docker bridge - the peer is private and its header is believed: the walk
// goes right to left through the trusted hops, to the first one that is not.
func TestPrivatePeerIsBelieved(t *testing.T) {
	withTrusted(t, "")
	cases := []struct {
		name string
		req  *http.Request
		want string
	}{
		{"one proxy", reqFrom("10.42.0.15:5000", "203.0.113.5"), "203.0.113.5"},
		// The client prepended a lie; the proxy appended the truth.
		{"forged prefix", reqFrom("10.42.0.15:5000", "6.6.6.6, 203.0.113.5"), "203.0.113.5"},
		// Two proxies of ours (a CDN edge in a private range, then the
		// Ingress): both skipped.
		{"chain of ours", reqFrom("10.42.0.15:5000", "203.0.113.5, 192.168.1.2"), "203.0.113.5"},
		{"two header lines", reqFrom("172.17.0.1:5000", "6.6.6.6", "203.0.113.5"), "203.0.113.5"},
		{"with a port", reqFrom("10.42.0.15:5000", "[2001:db8::7]:443"), "2001:db8::7"},
		// A whole chain we trust: the leftmost is the furthest we can see.
		{"all trusted", reqFrom("127.0.0.1:5000", "192.168.1.9, 10.0.0.3"), "192.168.1.9"},
		// Garbage written by a trusted hop: the nearest address we can vouch
		// for, not the word.
		{"unreadable", reqFrom("10.42.0.15:5000", "unknown"), "10.42.0.15"},
		{"no header", reqFrom("10.42.0.15:5000"), "10.42.0.15"},
		{"mapped peer", reqFrom("[::ffff:10.42.0.15]:5000", "203.0.113.5"), "203.0.113.5"},
	}
	for _, c := range cases {
		if got := ClientIP(c.req); got != c.want {
			t.Errorf("%s: ClientIP = %q, want %q", c.name, got, c.want)
		}
	}
	if c, p := ClientAndPeer(reqFrom("10.42.0.15:5000", "203.0.113.5")); c != "203.0.113.5" || p != "10.42.0.15" {
		t.Fatalf("client/peer = %q/%q, want 203.0.113.5 via 10.42.0.15", c, p)
	}
}

// CGNAT is the address space of subscribers - a mobile operator's customers,
// a Tailscale device - so a peer there is an end user, not a proxy.
func TestCarrierNATIsNotAProxy(t *testing.T) {
	withTrusted(t, "")
	if got := ClientIP(reqFrom("100.64.3.4:5000", "1.1.1.1")); got != "100.64.3.4" {
		t.Fatalf("a CGNAT peer named its address: %q", got)
	}
	if got := ClientIP(reqFrom("169.254.1.1:5000", "1.1.1.1")); got != "169.254.1.1" {
		t.Fatalf("a link-local peer named its address: %q", got)
	}
}

// "none" trusts nobody: even the loopback's header is ignored.
func TestNoneTrustsNobody(t *testing.T) {
	withTrusted(t, "none")
	if got := ClientIP(reqFrom("10.42.0.15:5000", "203.0.113.5")); got != "10.42.0.15" {
		t.Fatalf("with none, a header was believed: %q", got)
	}
	r := reqFrom("127.0.0.1:5000")
	r.Header.Set("X-Forwarded-Proto", "https")
	if Secure(r) {
		t.Fatal("with none, X-Forwarded-Proto made a plain request secure")
	}
}

// A list REPLACES the default: naming the CDN's public range trusts it, and
// stops trusting the private ranges nobody named.
func TestAListReplacesTheDefault(t *testing.T) {
	withTrusted(t, "198.51.100.0/24, 192.0.2.10")
	if got := ClientIP(reqFrom("198.51.100.7:5000", "203.0.113.5")); got != "203.0.113.5" {
		t.Fatalf("a named range was not believed: %q", got)
	}
	if got := ClientIP(reqFrom("192.0.2.10:5000", "203.0.113.5")); got != "203.0.113.5" {
		t.Fatalf("a named address was not believed: %q", got)
	}
	if got := ClientIP(reqFrom("10.42.0.15:5000", "203.0.113.5")); got != "10.42.0.15" {
		t.Fatalf("the default stood beside the list: %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if p, err := ParseTrustedProxies(""); err != nil || p != nil {
		t.Fatalf("empty: %v %v, want the default (nil)", p, err)
	}
	if p, err := ParseTrustedProxies(" None "); err != nil || p == nil || len(p) != 0 {
		t.Fatalf("none: %v %v, want an empty set", p, err)
	}
	p, err := ParseTrustedProxies("10.1.2.3, 2001:db8::/32,192.168.7.9/16")
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("10.1.2.3/32"), netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	if len(p) != len(want) {
		t.Fatalf("parsed %v", p)
	}
	for i := range want {
		if p[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, p[i], want[i])
		}
	}
	for _, bad := range []string{"proxy.example", "10.0.0.0/33", "none, 10.0.0.1", ","} {
		if _, err := ParseTrustedProxies(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// X-Forwarded-Proto follows the same rule: a stranger saying "https" does not
// skip the redirect nor make a cookie Secure for it.
func TestForwardedProtoNeedsATrustedPeer(t *testing.T) {
	withTrusted(t, "")
	r := reqFrom("203.0.113.9:4000")
	r.Header.Set("X-Forwarded-Proto", "https")
	if Secure(r) {
		t.Fatal("a public peer's X-Forwarded-Proto was believed")
	}
	r = reqFrom("10.42.0.15:5000")
	r.Header.Set("X-Forwarded-Proto", "https")
	if !Secure(r) {
		t.Fatal("the Ingress's X-Forwarded-Proto was not believed")
	}
}

// What a service is told about the browser's hop is what the gateway believes:
// a trusted proxy's https is passed on, a stranger's is not, a stranger's
// chain is replaced, and Forwarded says the same three things.
func TestTellUpstream(t *testing.T) {
	for _, c := range []struct {
		name, peer, host, xfp, xff  string
		wantProto, wantFor, wantFwd string
	}{
		{"behind a trusted proxy ending TLS", "10.0.0.7:5000", "canopy.example.com", "https", "203.0.113.9",
			"https", "203.0.113.9, 10.0.0.7", "for=203.0.113.9;host=canopy.example.com;proto=https"},
		{"a stranger claiming https", "198.51.100.4:5000", "canopy.example.com", "https", "1.2.3.4",
			"http", "198.51.100.4", "for=198.51.100.4;host=canopy.example.com;proto=http"},
		{"plain, nobody in front", "198.51.100.4:5000", "gw.example.com:8080", "", "",
			"http", "198.51.100.4", `for=198.51.100.4;host="gw.example.com:8080";proto=http`},
		{"an IPv6 client", "[2001:db8::7]:5000", "gw.example.com", "", "",
			"http", "2001:db8::7", `for="[2001:db8::7]";host=gw.example.com;proto=http`},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := httptest.NewRequest("GET", "http://"+c.host+"/", nil)
			in.RemoteAddr = c.peer
			if c.xfp != "" {
				in.Header.Set("X-Forwarded-Proto", c.xfp)
			}
			if c.xff != "" {
				in.Header.Set("X-Forwarded-For", c.xff)
			}
			out := http.Header{}
			// What httputil's SetXForwarded writes before us: the chain plus
			// the peer, and the scheme of the gateway's own connection.
			peer := PeerIP(in)
			if c.xff != "" {
				out.Set("X-Forwarded-For", c.xff+", "+peer)
			} else {
				out.Set("X-Forwarded-For", peer)
			}
			out.Set("X-Forwarded-Proto", "http")
			TellUpstream(in, out)
			if got := out.Get("X-Forwarded-Proto"); got != c.wantProto {
				t.Errorf("X-Forwarded-Proto %q, want %q", got, c.wantProto)
			}
			if got := out.Get("X-Forwarded-For"); got != c.wantFor {
				t.Errorf("X-Forwarded-For %q, want %q", got, c.wantFor)
			}
			if got := out.Get("Forwarded"); got != c.wantFwd {
				t.Errorf("Forwarded %q, want %q", got, c.wantFwd)
			}
		})
	}
}
