package admin

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// testCA is a private authority and a server it signs, for "localhost": what
// a cluster's service in https looks like to the gateway.
type testCA struct {
	pem    string
	pool   *x509.CertPool
	server *httptest.Server
	port   string
}

func newTestCA(t *testing.T) testCA {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "company-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(2),
		Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}, ca, leafKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return testCA{pem: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})), pool: pool, server: srv, port: port}
}

func swapTrustSeams(t *testing.T) {
	oldFind, oldIssued, oldSvc, oldRoots := findAuthorities, issuedCertificates, serviceOf, upstreamRoots
	t.Cleanup(func() {
		findAuthorities, issuedCertificates, serviceOf, upstreamRoots = oldFind, oldIssued, oldSvc, oldRoots
	})
	issuedCertificates = func(context.Context) ([]discovery.IssuedFor, []discovery.SourceNote) { return nil, nil }
	upstreamRoots = func() *x509.CertPool { return x509.NewCertPool() }
}

func saveTestRoute(t *testing.T, f fixture, id, upstream string) {
	t.Helper()
	if err := f.api.st.SaveRoute(context.Background(), store.Route{ID: id, Name: id, Order: 10, Enabled: true, Upstream: upstream,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/" + id + "/**"}}}}}); err != nil {
		t.Fatal(err)
	}
}

func platformList(t *testing.T, f fixture) platformAnswer {
	t.Helper()
	code, out := f.call(t, "GET", "/api/trusted-authorities/platform", "", f.rootC)
	var a platformAnswer
	if err := json.Unmarshal([]byte(out), &a); err != nil || code != http.StatusOK {
		t.Fatalf("the platform's authorities: %d %s", code, out)
	}
	return a
}

// TestWhatEachAuthoritySignsHere: a route in https refused today is matched
// to the authority that signed what its upstream presents - by its
// certificate when known, by what cert-manager records an issuer signed
// otherwise - and that authority comes first, picked in advance: it unblocks
// the route. A route whose upstream does not answer says nothing here; one
// signed by an authority the cluster does not publish is said, to import.
func TestWhatEachAuthoritySignsHere(t *testing.T) {
	f := setupBare(t)
	swapTrustSeams(t)
	ca := newTestCA(t)
	saveTestRoute(t, f, "geo", "https://localhost:"+ca.port)
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	_, deadPort, _ := net.SplitHostPort(closed.Addr().String())
	_ = closed.Close()
	saveTestRoute(t, f, "stations", "https://localhost:"+deadPort)

	findAuthorities = func(context.Context) discovery.PlatformAuthorities {
		return discovery.PlatformAuthorities{Kubernetes: true, Found: []discovery.FoundAuthority{
			{Ref: "cluster/ca", Source: "cluster", Name: "kubernetes-cluster-ca", PEM: newAuthorityPEM(t, "k3s"), Ready: true},
			{Ref: "openshift/service-ca", Source: "openshift", Name: "company", PEM: ca.pem, Ready: true},
		}}
	}
	a := platformList(t, f)
	if len(a.Found) != 2 || a.Found[0].Ref != "openshift/service-ca" || !a.Found[0].Preselect ||
		len(a.Found[0].Signs) != 1 || a.Found[0].Signs[0].RouteID != "geo" || !a.Found[0].Signs[0].Refused {
		t.Fatalf("by its certificate: %+v", a.Found)
	}
	if len(a.Found[1].Signs) != 0 || a.Found[1].Preselect {
		t.Fatalf("an authority that signs nothing here: %+v", a.Found[1])
	}
	if len(a.Unmatched) != 0 {
		t.Fatalf("a route that does not answer is the routes screen's: %+v", a.Unmatched)
	}
	// Signed by an authority the cluster does not publish: to import.
	findAuthorities = func(context.Context) discovery.PlatformAuthorities {
		return discovery.PlatformAuthorities{Kubernetes: true, Found: []discovery.FoundAuthority{
			{Ref: "cluster/ca", Source: "cluster", Name: "kubernetes-cluster-ca", PEM: newAuthorityPEM(t, "k3s"), Ready: true},
		}}
	}
	if a := platformList(t, f); len(a.Unmatched) != 1 || a.Unmatched[0].RouteID != "geo" || len(a.Found[0].Signs) != 0 {
		t.Fatalf("a refused route no published authority signs: %+v", a)
	}
	// Accepted already: nothing to unblock.
	upstreamRoots = func() *x509.CertPool { return ca.pool }
	findAuthorities = func(context.Context) discovery.PlatformAuthorities {
		return discovery.PlatformAuthorities{Kubernetes: true, Found: []discovery.FoundAuthority{
			{Ref: "openshift/service-ca", Source: "openshift", Name: "company", PEM: ca.pem, Ready: true},
		}}
	}
	if a := platformList(t, f); len(a.Found[0].Signs) != 0 || a.Found[0].Preselect || len(a.Unmatched) != 0 {
		t.Fatalf("a route accepted already: %+v", a)
	}
	upstreamRoots = func() *x509.CertPool { return x509.NewCertPool() }

	// An issuer, whose certificate is learnt only when taken: matched by what
	// cert-manager records it signed, and its services listed too.
	findAuthorities = func(context.Context) discovery.PlatformAuthorities {
		return discovery.PlatformAuthorities{Kubernetes: true, Found: []discovery.FoundAuthority{
			{Ref: "clusterissuer/company-ca", Source: "cert-manager", Name: "company-ca", Ready: true},
		}}
	}
	issuedCertificates = func(context.Context) ([]discovery.IssuedFor, []discovery.SourceNote) {
		return []discovery.IssuedFor{
			{Namespace: "shop", Name: "geo-tls", Authority: "clusterissuer/company-ca", Names: []string{"localhost"}},
			{Namespace: "shop", Name: "s3", Authority: "clusterissuer/company-ca", Names: []string{"s3.example.com"}},
		}, nil
	}
	a = platformList(t, f)
	kinds := map[string]int{}
	for _, s := range a.Found[0].Signs {
		kinds[s.Kind]++
	}
	if !a.Found[0].Preselect || kinds["route"] != 1 || kinds["service"] != 2 {
		t.Fatalf("by the issuer's records: %+v", a.Found[0])
	}
}

// TestRoutesInHTTPAreOfferedHTTPSOnlyWhenItWorks: a route in http whose
// Service answers in https on one of its ports, with a certificate the
// gateway accepts, is offered - with that port; one the gateway does not
// accept yet is not.
func TestRoutesInHTTPAreOfferedHTTPSOnlyWhenItWorks(t *testing.T) {
	f := setupBare(t)
	swapTrustSeams(t)
	ca := newTestCA(t)
	plain := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(plain.Close)
	_, plainPort, _ := net.SplitHostPort(plain.Listener.Addr().String())
	saveTestRoute(t, f, "geo", "http://localhost:"+plainPort+"/geoserver")
	saveTestRoute(t, f, "outside", "http://example.invalid:8080")
	tlsPort, _ := strconv.Atoi(ca.port)
	httpPort, _ := strconv.Atoi(plainPort)
	serviceOf = func(_ context.Context, host string) (discovery.ServiceTarget, bool) {
		if host != "localhost" {
			return discovery.ServiceTarget{}, false
		}
		return discovery.ServiceTarget{Name: "localhost", Namespace: "shop", Ports: []int{httpPort, tlsPort}}, true
	}
	list := func() []switchCandidate {
		code, out := f.call(t, "GET", "/api/trusted-authorities/https-switch", "", f.rootC)
		var c []switchCandidate
		if err := json.Unmarshal([]byte(out), &c); err != nil || code != http.StatusOK {
			t.Fatalf("the switch: %d %s", code, out)
		}
		return c
	}
	if c := list(); len(c) != 0 {
		t.Fatalf("offered before its authority is trusted: %+v", c)
	}
	upstreamRoots = func() *x509.CertPool { return ca.pool }
	c := list()
	if len(c) != 1 || c[0].RouteID != "geo" || c[0].To != "https://localhost:"+ca.port+"/geoserver" {
		t.Fatalf("once trusted: %+v", c)
	}
}
