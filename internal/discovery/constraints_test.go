package discovery

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// testCA is an authority made for one test, with the DNS constraints given
// and every IP excluded, as the canopy cluster's.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newTestCA(t *testing.T, name string, parent *testCA, permitted, excluded []string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		PermittedDNSDomainsCritical: len(permitted)+len(excluded) > 0,
		PermittedDNSDomains:         permitted, ExcludedDNSDomains: excluded,
	}
	if len(permitted)+len(excluded) > 0 {
		_, v4, _ := net.ParseCIDR("0.0.0.0/0")
		_, v6, _ := net.ParseCIDR("::/0")
		tpl.ExcludedIPRanges = []*net.IPNet{v4, v6}
	}
	signer, signerKey := tpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return &testCA{cert: c, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

// leaf is what cert-manager's CA issuer writes: whatever names were asked,
// signed without a look at the constraints.
func (ca *testCA) leaf(t *testing.T, names ...string) (*x509.Certificate, []byte) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), DNSNames: names,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return c, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

var canopyPermitted = []string{"canopy", "cluster.local", "localhost", "meerkat-meerkat-internal", "rabbitmq", "svc", "understory.com"}

// TestTheMatcherIsCryptoX509s: for each constraint set and name, the matcher
// says what crypto/x509 decides when it verifies a leaf carrying that one
// name - permitted subtrees, exclusions winning, a leading dot for
// subdomains only, an empty permitted list, and the IP exclusions that have
// no say on a DNS name.
func TestTheMatcherIsCryptoX509s(t *testing.T) {
	cases := []struct {
		permitted, excluded []string
		name                string
		want                bool
	}{
		{canopyPermitted, nil, "meerkat-meerkat.canopy.svc.cluster.local", true},
		{canopyPermitted, nil, "meerkat-meerkat.canopy.svc", true},
		{canopyPermitted, nil, "meerkat-meerkat.canopy", true},
		{canopyPermitted, nil, "meerkat-meerkat", false},
		{canopyPermitted, nil, "meerkat-meerkat-internal", true},
		{canopyPermitted, nil, "svc", true},
		{canopyPermitted, nil, "xsvc", false},
		{canopyPermitted, nil, "Meerkat.CANOPY", true},
		{[]string{".svc"}, nil, "svc", false},
		{[]string{".svc"}, nil, "a.svc", true},
		{[]string{".svc"}, nil, "a.b.svc", true},
		{nil, []string{"svc"}, "a.svc", false},
		{nil, []string{"svc"}, "meerkat-meerkat", true},
		{[]string{"svc"}, []string{"secret.svc"}, "a.secret.svc", false},
		{[]string{"svc"}, []string{"secret.svc"}, "a.svc", true},
		{[]string{"svc"}, []string{".svc"}, "svc", true},
		{[]string{"svc"}, []string{".svc"}, "a.svc", false},
		{nil, []string{"a.svc"}, "*.svc", false},
		{nil, []string{"other"}, "*.svc", true},
	}
	ca := map[string]*testCA{}
	for _, c := range cases {
		got := dnsPermitted(c.name, c.permitted, c.excluded)
		if got != c.want {
			t.Errorf("%q under permitted %v, excluded %v: got %v", c.name, c.permitted, c.excluded, got)
		}
		key := strings.Join(c.permitted, ",") + "|" + strings.Join(c.excluded, ",")
		if ca[key] == nil {
			ca[key] = newTestCA(t, "ca "+key, nil, c.permitted, c.excluded)
		}
		leaf, _ := ca[key].leaf(t, c.name)
		roots := x509.NewCertPool()
		roots.AddCert(ca[key].cert)
		_, err := leaf.Verify(x509.VerifyOptions{Roots: roots})
		if (err == nil) != got {
			t.Errorf("%q under permitted %v, excluded %v: matcher %v, crypto/x509 %v", c.name, c.permitted, c.excluded, got, err)
		}
	}
}

// TestTheChainIsReadFromTheSecret: the constraints of the certificates that
// signed the leaf, an intermediate in tls.crt or the authority in ca.crt;
// not those of another authority in the bundle, and none from an authority
// that only excludes IPs or constrains nothing.
func TestTheChainIsReadFromTheSecret(t *testing.T) {
	root := newTestCA(t, "root", nil, nil, nil)
	inter := newTestCA(t, "intermediate", root, canopyPermitted, nil)
	other := newTestCA(t, "other", nil, []string{"elsewhere"}, nil)
	_, leaf := inter.leaf(t, "meerkat-meerkat.canopy", "meerkat-meerkat")
	l := limitsOf(append(slices.Clone(leaf), inter.pem...), append(slices.Clone(other.pem), root.pem...))
	if len(l.cas) != 1 || l.cas[0].Subject.CommonName != "intermediate" {
		t.Fatalf("constraining: %v", l)
	}
	kept, refused := l.split([]string{"meerkat-meerkat.canopy", "meerkat-meerkat", "meerkat-meerkat-internal"})
	if !slices.Equal(kept, []string{"meerkat-meerkat.canopy", "meerkat-meerkat-internal"}) || !slices.Equal(refused, []string{"meerkat-meerkat"}) {
		t.Fatalf("kept %v, refused %v", kept, refused)
	}
	if !strings.Contains(l.String(), "permitted canopy, cluster.local") {
		t.Fatalf("said: %s", l)
	}

	// The same chain, the intermediate only in ca.crt.
	if l := limitsOf(leaf, append(slices.Clone(inter.pem), root.pem...)); len(l.cas) != 1 {
		t.Fatalf("intermediate in ca.crt: %v", l)
	}

	// IPs only: nothing to say about DNS names.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, v4, _ := net.ParseCIDR("0.0.0.0/0")
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "ip only"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		PermittedDNSDomainsCritical: true, ExcludedIPRanges: []*net.IPNet{v4},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ipOnly := &testCA{key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
	ipOnly.cert, _ = x509.ParseCertificate(der)
	_, ipLeaf := ipOnly.leaf(t, "meerkat-meerkat")
	if l := limitsOf(ipLeaf, ipOnly.pem); !l.none() {
		t.Fatalf("IP exclusions taken for DNS ones: %v", l)
	}
	if l := limitsOf(leaf, nil); !l.none() {
		t.Fatalf("no authority given, yet constraints: %v", l)
	}
}

// certManagerCluster is an API server holding one Certificate: what it
// answers, and every request it received.
type certManagerCluster struct {
	mu       sync.Mutex
	dnsNames []string
	issuer   string
	seen     []string
	written  [][]string
}

func (c *certManagerCluster) serve(t *testing.T) *CertManager {
	return fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		defer c.mu.Unlock()
		c.seen = append(c.seen, r.Method)
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"spec": map[string]any{
				"dnsNames": c.dnsNames, "issuerRef": map[string]string{"kind": "ClusterIssuer", "name": c.issuer}}})
		case http.MethodPost:
			w.WriteHeader(http.StatusConflict)
		case http.MethodPatch:
			var p struct {
				Spec struct {
					DNSNames  []string `json:"dnsNames"`
					IssuerRef struct {
						Name string `json:"name"`
					} `json:"issuerRef"`
				} `json:"spec"`
			}
			_ = json.Unmarshal(b, &p)
			c.dnsNames = p.Spec.DNSNames
			if p.Spec.IssuerRef.Name != "" {
				c.issuer = p.Spec.IssuerRef.Name
			}
			c.written = append(c.written, p.Spec.DNSNames)
		}
	})
}

func (c *certManagerCluster) requests() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.seen
	c.seen = nil
	return out
}

// secretEvent is the Secret cert-manager writes: the leaf, the authority in
// ca.crt, the issuer in its annotations.
func secretEvent(t *testing.T, leaf, ca []byte) json.RawMessage {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{"name": "meerkat-internal-issued", "annotations": map[string]string{
			"cert-manager.io/issuer-kind": "ClusterIssuer", "cert-manager.io/issuer-name": "cluster-ca"}},
		"data": map[string][]byte{"tls.crt": leaf, "tls.key": []byte("key"), "ca.crt": ca},
	})
	return b
}

var serviceNames = []string{"meerkat-meerkat.canopy.svc.cluster.local", "meerkat-meerkat.canopy.svc", "meerkat-meerkat.canopy", "meerkat-meerkat"}

// TestTheNamesFollowANameConstrainedAuthority: once the Secret shows an
// authority that may not sign the bare Service name, the Certificate is
// brought to the names it permits, and stays there - asked again of the same
// issuer, the bare name does not come back; asked of another, the whole set
// goes, its own Secret to be read in turn.
func TestTheNamesFollowANameConstrainedAuthority(t *testing.T) {
	ca := newTestCA(t, "canopy", nil, canopyPermitted, nil)
	_, leaf := ca.leaf(t, serviceNames...)
	cl := &certManagerCluster{dnsNames: serviceNames, issuer: "cluster-ca"}
	m := cl.serve(t)
	ctx := context.Background()
	fitted := serviceNames[:3]

	if !m.holdSecret("ADDED", secretEvent(t, leaf, ca.pem)) {
		t.Fatal("the pair is not taken")
	}
	m.fitNames(ctx)
	if len(cl.written) != 1 || !slices.Equal(cl.written[0], fitted) {
		t.Fatalf("written: %v", cl.written)
	}

	// Reissued with the fitted names: nothing more to write.
	_, leaf2 := ca.leaf(t, fitted...)
	cl.requests()
	m.holdSecret("MODIFIED", secretEvent(t, leaf2, ca.pem))
	m.fitNames(ctx)
	if got := cl.requests(); !slices.Equal(got, []string{"GET"}) {
		t.Fatalf("after the reissue: %v", got)
	}

	// Asked again from the screen, the runtime still naming the bare Service.
	if err := m.Ensure(ctx, "ClusterIssuer", "cluster-ca", serviceNames); err != nil {
		t.Fatal(err)
	}
	if last := cl.written[len(cl.written)-1]; !slices.Equal(last, fitted) {
		t.Fatalf("asked again: %v", last)
	}
	// Another issuer: its authority is not this one.
	if err := m.Ensure(ctx, "ClusterIssuer", "vault-pki", serviceNames); err != nil {
		t.Fatal(err)
	}
	if last := cl.written[len(cl.written)-1]; !slices.Equal(last, serviceNames) {
		t.Fatalf("another issuer: %v", last)
	}
	// Its old Secret still there, the Certificate now another issuer's: left
	// alone.
	cl.requests()
	m.fitNames(ctx)
	if got := cl.requests(); !slices.Equal(got, []string{"GET"}) {
		t.Fatalf("another issuer's Certificate: %v", got)
	}
	if st, _ := m.Status(ctx); st.Refused != "" {
		t.Fatalf("refused: %s", st.Refused)
	}
}

// TestAnOrdinaryAuthorityChangesNothing: no name constraints, no read and no
// write beyond what was asked.
func TestAnOrdinaryAuthorityChangesNothing(t *testing.T) {
	ca := newTestCA(t, "plain", nil, nil, nil)
	_, leaf := ca.leaf(t, serviceNames...)
	cl := &certManagerCluster{dnsNames: serviceNames, issuer: "cluster-ca"}
	m := cl.serve(t)
	m.holdSecret("ADDED", secretEvent(t, leaf, ca.pem))
	m.fitNames(context.Background())
	if got := cl.requests(); len(got) != 0 {
		t.Fatalf("requests: %v", got)
	}
	if err := m.Ensure(context.Background(), "ClusterIssuer", "cluster-ca", serviceNames); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cl.written[0], serviceNames) {
		t.Fatalf("asked: %v", cl.written)
	}
}

// TestNoPermittedNameLeavesTheCertificateAndSaysWhy: an empty list is no
// request, so the Certificate keeps its names, and the status names the ones
// refused and the constraints refusing them.
func TestNoPermittedNameLeavesTheCertificateAndSaysWhy(t *testing.T) {
	ca := newTestCA(t, "canopy", nil, canopyPermitted, nil)
	_, leaf := ca.leaf(t, "meerkat-meerkat")
	cl := &certManagerCluster{dnsNames: []string{"meerkat-meerkat"}, issuer: "cluster-ca"}
	m := cl.serve(t)
	ctx := context.Background()
	m.holdSecret("ADDED", secretEvent(t, leaf, ca.pem))
	m.fitNames(ctx)
	if got := cl.requests(); !slices.Equal(got, []string{"GET"}) {
		t.Fatalf("requests: %v", got)
	}
	st, err := m.Status(ctx)
	if err != nil || !strings.Contains(st.Refused, "(meerkat-meerkat)") || !strings.Contains(st.Refused, "permitted canopy") {
		t.Fatalf("status: %+v %v", st, err)
	}
	if err := m.Ensure(ctx, "ClusterIssuer", "cluster-ca", []string{"meerkat-meerkat"}); err != nil {
		t.Fatal(err)
	}
	if len(cl.written) != 1 || !slices.Equal(cl.written[0], []string{"meerkat-meerkat"}) {
		t.Fatalf("asked: %v", cl.written)
	}
	// The Secret gone, nothing is refused any more.
	m.holdSecret("DELETED", nil)
	m.fitNames(ctx)
	if st, _ := m.Status(ctx); st.Refused != "" {
		t.Fatalf("after the Secret went: %s", st.Refused)
	}
}
