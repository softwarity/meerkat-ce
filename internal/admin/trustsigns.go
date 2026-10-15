package admin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
)

// What each authority the platform offers signs HERE (SSL-09), to help pick
// one - indicative, nothing is decided from it. Two sources:
//
//   - the routes in https the gateway REFUSES today: their upstream is
//     dialled once each, and the certificate it presents tells who signed it -
//     matched to an authority whose certificate is known, or, for a
//     cert-manager issuer (whose certificate is learnt only when taken),
//     through the names cert-manager records it signed. The authority found
//     is the one that unblocks them. A route that verifies already, or does
//     not answer at all, says nothing about which authority to pick: it is
//     the routes screen's to report;
//   - the services: what cert-manager records each issuer signed (a
//     Certificate's issuer and dnsNames), and the Services OpenShift gives a
//     serving certificate - whether a route calls them yet or not.
//
// A route signed by a public authority is not listed: it verifies already.

// Swapped in tests: the platform's records, and the dialling.
var (
	issuedCertificates = discovery.IssuedCertificates
	serviceOf          = discovery.ServiceOf
	upstreamRoots      = gateway.UpstreamRoots
)

// dialTimeout bounds one handshake: a target that takes longer is said to be
// unreachable rather than holding the dialog.
const dialTimeout = 2 * time.Second

// signedItem is one thing an authority signs: a route (dialled) or a service
// (recorded by the platform).
type signedItem struct {
	Kind    string `json:"kind"` // route | service
	Name    string `json:"name"`
	RouteID string `json:"routeId,omitempty"`
	// Refused: a route whose upstream the gateway refuses today, for want of
	// this authority.
	Refused bool     `json:"refused,omitempty"`
	Names   []string `json:"names,omitempty"`
}

// unmatchedRoute is a route in https refused today whose certificate comes
// from an authority the platform does not publish: to import by hand.
type unmatchedRoute struct {
	RouteID string `json:"routeId"`
	Name    string `json:"name"`
	Why     string `json:"why"`
}

// presented is what one https upstream answered.
type presented struct {
	route    store.Route
	host     string
	chain    []*x509.Certificate
	err      error
	verifies bool // with the gateway's pool, as a call would
	public   bool // with the system's alone
}

// dialPresented asks an upstream for its certificate, verifying nothing, then
// says whether the gateway's pool and the system's accept it for the host.
func dialPresented(ctx context.Context, r store.Route, u *url.URL, pool *x509.CertPool) presented {
	p := presented{route: r, host: u.Hostname()}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout},
		Config: &tls.Config{InsecureSkipVerify: true, ServerName: p.host, MinVersion: tls.VersionTLS12}} //nolint:gosec // read only, verified below
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(p.host, port))
	if err != nil {
		p.err = err
		return p
	}
	p.chain = c.(*tls.Conn).ConnectionState().PeerCertificates
	_ = c.Close()
	if len(p.chain) == 0 {
		p.err = errors.New("no certificate presented")
		return p
	}
	verify := func(roots *x509.CertPool) bool {
		inter := x509.NewCertPool()
		for _, c := range p.chain[1:] {
			inter.AddCert(c)
		}
		_, err := p.chain[0].Verify(x509.VerifyOptions{DNSName: p.host, Roots: roots, Intermediates: inter})
		return err == nil
	}
	p.verifies = verify(pool)
	if sys, err := x509.SystemCertPool(); err == nil {
		p.public = verify(sys)
	}
	return p
}

// matchesName says a certificate name covers host, a wildcard one label deep.
func matchesName(pattern, host string) bool {
	pattern, host = strings.ToLower(strings.TrimSuffix(pattern, ".")), strings.ToLower(strings.TrimSuffix(host, "."))
	if pattern == host {
		return true
	}
	if rest, ok := strings.CutPrefix(pattern, "*."); ok {
		if i := strings.IndexByte(host, '.'); i > 0 && host[i+1:] == rest {
			return true
		}
	}
	return false
}

// signsOf works out, for each offered authority, what it signs here, and
// which https routes none of them accounts for.
func (a *API) signsOf(ctx context.Context, found []discovery.FoundAuthority) (map[string][]signedItem, []unmatchedRoute, []discovery.SourceNote) {
	out := map[string][]signedItem{}
	issued, notes := issuedCertificates(ctx)
	for _, c := range issued {
		out[c.Authority] = append(out[c.Authority], signedItem{Kind: "service", Name: c.Namespace + "/" + c.Name, Names: c.Names})
	}

	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return out, nil, notes
	}
	pool := upstreamRoots()
	ctx, cancel := context.WithTimeout(ctx, 2*dialTimeout+time.Second)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	var dialled []presented
	for _, r := range routes {
		u, err := url.Parse(r.Upstream)
		if !r.Enabled || err != nil || u.Scheme != "https" || u.Hostname() == "" {
			continue
		}
		wg.Add(1)
		go func(r store.Route, u *url.URL) {
			defer wg.Done()
			p := dialPresented(ctx, r, u, pool)
			mu.Lock()
			dialled = append(dialled, p)
			mu.Unlock()
		}(r, u)
	}
	wg.Wait()
	sort.Slice(dialled, func(i, j int) bool { return dialled[i].route.Name < dialled[j].route.Name })

	// The authorities whose certificate is known, parsed once.
	known := map[string][]*x509.Certificate{}
	for _, f := range found {
		if f.PEM != "" {
			if cs, err := certs.ParseAuthorities([]byte(f.PEM)); err == nil {
				known[f.Ref] = cs
			}
		}
	}
	var unmatched []unmatchedRoute
	for _, p := range dialled {
		if p.err != nil || p.verifies || p.public {
			continue // down, or accepted already: nothing to unblock
		}
		top := p.chain[len(p.chain)-1]
		ref := ""
		for r, cs := range known {
			for _, ca := range cs {
				if top.CheckSignatureFrom(ca) == nil || top.Equal(ca) {
					ref = r
				}
			}
		}
		if ref == "" {
			// An issuer, known by what cert-manager records it signed.
			for _, c := range issued {
				for _, n := range c.Names {
					if matchesName(n, p.host) {
						ref = c.Authority
					}
				}
			}
		}
		if ref == "" {
			unmatched = append(unmatched, unmatchedRoute{RouteID: p.route.ID, Name: p.route.Name,
				Why: "signed by " + p.chain[0].Issuer.String() + ", which this cluster does not publish: import its certificate with Add authority"})
			continue
		}
		out[ref] = append(out[ref], signedItem{Kind: "route", Name: p.route.Name, RouteID: p.route.ID, Refused: true})
	}
	return out, unmatched, notes
}

// switchCandidate is a route in http whose service answers in https with a
// certificate the gateway now accepts, under the name and port given.
type switchCandidate struct {
	RouteID string `json:"routeId"`
	Name    string `json:"name"`
	From    string `json:"from"`
	To      string `json:"to"`
}

// listHTTPSSwitch offers the routes in http whose Service answers in https on
// one of its ports with a certificate the gateway accepts now - tried, not
// guessed: the port changes as often as the scheme, and the name must be one
// the certificate carries. Indicative: switching is the person's to confirm.
func (a *API) listHTTPSSwitch(w http.ResponseWriter, r *http.Request, _ store.User) {
	routes, err := a.st.ListRoutes(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	pool := upstreamRoots()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := []switchCandidate{}
	for _, rt := range routes {
		u, err := url.Parse(rt.Upstream)
		if !rt.Enabled || err != nil || u.Scheme != "http" || u.Hostname() == "" {
			continue
		}
		wg.Add(1)
		go func(rt store.Route, u *url.URL) {
			defer wg.Done()
			svc, ok := serviceOf(ctx, u.Hostname())
			if !ok {
				return
			}
			names := []string{u.Hostname(), svc.Name + "." + svc.Namespace + ".svc", svc.Name + "." + svc.Namespace + ".svc.cluster.local"}
			for _, port := range svc.Ports {
				for _, name := range names {
					d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout},
						Config: &tls.Config{RootCAs: pool, ServerName: name, MinVersion: tls.VersionTLS12}}
					c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), strconv.Itoa(port)))
					if err != nil {
						var hostErr x509.HostnameError
						if errors.As(err, &hostErr) {
							continue // another name of the same service may be the one
						}
						break // not TLS here, or not trusted: no name will do
					}
					_ = c.Close()
					to := *u
					to.Scheme, to.Host = "https", net.JoinHostPort(name, strconv.Itoa(port))
					mu.Lock()
					out = append(out, switchCandidate{RouteID: rt.ID, Name: rt.Name, From: rt.Upstream, To: to.String()})
					mu.Unlock()
					return
				}
			}
		}(rt, u)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, out)
}
