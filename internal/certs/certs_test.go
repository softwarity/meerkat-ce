package certs

import (
	"golang.org/x/crypto/acme/autocert"

	"github.com/softwarity/meerkat/internal/edition"

	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func mustSelfSigned(t *testing.T, req Request) *Material {
	t.Helper()
	m, err := SelfSigned(req, time.Now())
	if err != nil {
		t.Fatalf("SelfSigned(%+v): %v", req, err)
	}
	return m
}

func TestSelfSignedServesTheNamesItWasGiven(t *testing.T) {
	m := mustSelfSigned(t, Request{
		Subject:     Subject{Organization: "Softwarity"},
		DNSNames:    []string{"gateway.example.com", "www.example.com"},
		IPAddresses: []string{"10.0.0.5"},
	})
	if !m.Info.SelfSigned {
		t.Fatal("a self-signed certificate must say so: the console shows that state, and hiding it lets someone believe an authority was involved")
	}
	// The common name follows the first host rather than staying empty: every
	// tool prints it, and a list of unnamed rows helps nobody.
	if !strings.Contains(m.Info.Subject, "gateway.example.com") {
		t.Fatalf("subject %q should carry the first host as its common name", m.Info.Subject)
	}
	for _, name := range []string{"gateway.example.com", "www.example.com", "10.0.0.5"} {
		if err := m.Leaf().VerifyHostname(name); err != nil {
			t.Fatalf("generated certificate refuses %q: %v", name, err)
		}
	}
	if m.Info.KeyType != KeyECDSAP256 {
		t.Fatalf("default key type = %q, want %q", m.Info.KeyType, KeyECDSAP256)
	}
	// Apple refuses a server certificate valid for more than 398 days, even a
	// manually trusted self-signed one. Generating a two-year laboratory
	// certificate is generating one that fails on every Mac.
	span := time.Unix(m.Info.NotAfter, 0).Sub(time.Unix(m.Info.NotBefore, 0))
	if span > 398*24*time.Hour {
		t.Fatalf("default validity %v exceeds the 398-day ceiling Apple enforces", span)
	}
	// And it starts in the past, so a client whose clock runs slow does not
	// meet a certificate that is "not yet valid".
	if time.Unix(m.Info.NotBefore, 0).After(time.Now()) {
		t.Fatal("a freshly generated certificate must already be valid")
	}
}

func TestSelfSignedRefusesWhatCannotWork(t *testing.T) {
	if _, err := SelfSigned(Request{Subject: Subject{CommonName: "nowhere"}}, time.Now()); err == nil {
		t.Fatal("a certificate with no host is a certificate that serves nothing")
	} else if !strings.Contains(err.Error(), "at least one DNS name") {
		t.Fatalf("the error must say what to give: %v", err)
	}
	_, err := SelfSigned(Request{DNSNames: []string{"a.test"}, KeyType: "rsa-1024"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), KeyECDSAP256) {
		// House rule: an error names what failed AND lists what is allowed.
		t.Fatalf("the error must list the allowed key types, got %v", err)
	}
	if _, err := SelfSigned(Request{DNSNames: []string{"a.test"}, IPAddresses: []string{"not-an-ip"}}, time.Now()); err == nil {
		t.Fatal("an unparseable IP address must be refused, not silently dropped")
	}
}

func TestImportRefusesTwoHalvesOfDifferentPairs(t *testing.T) {
	a := mustSelfSigned(t, Request{DNSNames: []string{"a.test"}})
	b := mustSelfSigned(t, Request{DNSNames: []string{"b.test"}})
	_, err := ParsePEM(a.CertPEM, b.KeyPEM)
	if err == nil {
		t.Fatal("a certificate and a foreign key must not import")
	}
	if !strings.Contains(err.Error(), "two different pairs") {
		t.Fatalf("this is the paste-the-wrong-file error and it must say so plainly: %v", err)
	}
}

func TestImportRefusesACertificateWithoutSAN(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "legacy.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM, err := MarshalKey(key)
	if err != nil {
		t.Fatal(err)
	}
	// It would import, and then fail every single handshake with a name
	// mismatch nobody connects back to this screen.
	_, err = ParsePEM(certPEM, keyPEM)
	if err == nil || !strings.Contains(err.Error(), "subject alternative name") {
		t.Fatalf("a certificate with only a common name must be refused, by name: %v", err)
	}
}

func TestPKCS12RoundTrip(t *testing.T) {
	src := mustSelfSigned(t, Request{DNSNames: []string{"keystore.example.com"}})
	pair, err := tls.X509KeyPair([]byte(src.CertPEM), []byte(src.KeyPEM))
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pfx, err := pkcs12.Modern.Encode(pair.PrivateKey, leaf, nil, "changeit")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParsePKCS12(pfx, "changeit")
	if err != nil {
		t.Fatalf("a keystore Meerkat itself could produce must import: %v", err)
	}
	if err := m.Leaf().VerifyHostname("keystore.example.com"); err != nil {
		t.Fatalf("the imported keystore lost its host: %v", err)
	}
	if _, err := ParsePKCS12(pfx, "wrong"); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("a wrong password must say so, not fail obscurely: %v", err)
	}
	// A trust store is the mistake worth naming: it looks like a certificate
	// file to whoever exported it, and imports nowhere.
	trust, err := pkcs12.Modern.EncodeTrustStore([]*x509.Certificate{leaf}, "changeit")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParsePKCS12(trust, "changeit")
	if err == nil || !strings.Contains(err.Error(), "trust store") {
		t.Fatalf("a trust store must be recognised as such: %v", err)
	}
}

// testCA is a company authority, for the offline signing path.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "Acme Internal CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return testCA{cert: cert, key: key}
}

// sign is what the authority does with the request it was handed.
func (ca testCA) sign(t *testing.T, csrPEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		t.Fatal("the signing request is not PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("the request is not self-signed by its own key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               csr.Subject,
		DNSNames:              csr.DNSNames,
		IPAddresses:           csr.IPAddresses,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 6, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, csr.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestSigningRequestTravelsAndComesBack(t *testing.T) {
	csrPEM, keyPEM, err := NewCSR(Request{
		Subject:  Subject{Organization: "Acme"},
		DNSNames: []string{"intranet.acme.test"},
		KeyType:  KeyRSA2048,
	})
	if err != nil {
		t.Fatalf("NewCSR: %v", err)
	}
	info, err := ParseCSR(csrPEM)
	if err != nil {
		t.Fatalf("ParseCSR: %v", err)
	}
	if info.KeyType != KeyRSA2048 || len(info.DNSNames) != 1 {
		t.Fatalf("the pending request must describe what was asked: %+v", info)
	}

	ca := newTestCA(t)
	m, err := Adopt(keyPEM, ca.sign(t, csrPEM))
	if err != nil {
		t.Fatalf("the answer of the authority must adopt: %v", err)
	}
	if err := m.Leaf().VerifyHostname("intranet.acme.test"); err != nil {
		t.Fatalf("adopted certificate refuses its host: %v", err)
	}
	if m.Info.SelfSigned {
		t.Fatal("a CA-signed certificate is not self-signed")
	}

	// The check that earns this flow its existence: paste the certificate from
	// another order and the mistake is named HERE, not at handshake time.
	//
	// Both shapes of that mistake, because the standard library words them
	// differently: another request for the SAME key type, and one for another
	// type. Only testing the second is how the commoner case ships broken.
	for _, other := range []Request{
		{DNSNames: []string{"intranet.acme.test"}, KeyType: KeyRSA2048},
		{DNSNames: []string{"intranet.acme.test"}, KeyType: KeyECDSAP256},
	} {
		csr, _, err := NewCSR(other)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Adopt(keyPEM, ca.sign(t, csr))
		if err == nil || !strings.Contains(err.Error(), "not signed from this request") {
			t.Fatalf("a certificate signed from a %s request must be refused by name: %v", other.KeyType, err)
		}
	}
}

func hello(name string, protos ...string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:        name,
		SupportedProtos:   protos,
		SupportedVersions: []uint16{tls.VersionTLS13},
		CipherSuites:      []uint16{tls.TLS_AES_128_GCM_SHA256},
	}
}

func TestManagerPicksByServerName(t *testing.T) {
	exact := mustSelfSigned(t, Request{DNSNames: []string{"app.example.com"}})
	wild := mustSelfSigned(t, Request{DNSNames: []string{"*.wild.example.com"}})
	fallback := mustSelfSigned(t, Request{DNSNames: []string{"default.example.com"}})
	m := New()
	m.Set([]*Material{exact, wild, fallback}, fallback)

	got, err := m.GetCertificate(hello("app.example.com"))
	if err != nil || got != exact.TLS() {
		t.Fatalf("exact name: %v", err)
	}
	// A wildcard is what makes one certificate serve a whole estate.
	if got, err := m.GetCertificate(hello("anything.wild.example.com")); err != nil || got != wild.TLS() {
		t.Fatalf("wildcard: %v", err)
	}
	// No SNI at all: a client that came by IP address, or something very old.
	if got, err := m.GetCertificate(hello("")); err != nil || got != fallback.TLS() {
		t.Fatalf("no server name: %v", err)
	}
	// A trailing dot is a legal absolute name and must not miss.
	if got, err := m.GetCertificate(hello("APP.example.com.")); err != nil || got != exact.TLS() {
		t.Fatalf("absolute, upper-case name: %v", err)
	}
}

func TestManagerNamesWhatItServesWhenItCannotAnswer(t *testing.T) {
	only := mustSelfSigned(t, Request{DNSNames: []string{"app.example.com"}})
	m := New()
	m.Set([]*Material{only}, nil)
	_, err := m.GetCertificate(hello("unknown.example.com"))
	if err == nil {
		t.Fatal("with no fallback, an unknown name must fail rather than present something at random")
	}
	// A handshake failure is the most opaque event in an operator's day: the
	// error has to carry both the name asked for and what IS served.
	if !strings.Contains(err.Error(), "unknown.example.com") || !strings.Contains(err.Error(), "app.example.com") {
		t.Fatalf("the error must name the miss and list what is installed: %v", err)
	}
	if _, err := m.GetCertificate(hello("")); err == nil || !strings.Contains(err.Error(), "IP address") {
		t.Fatalf("a client sending no name must be told what it did: %v", err)
	}
}

func TestManagerPrefersTheLivingCertificateDuringARenewal(t *testing.T) {
	// The overlap every renewal goes through: the old one is still installed,
	// the new one has just arrived. Serving the old one is a countdown.
	old, err := SelfSigned(Request{DNSNames: []string{"app.example.com"}, Days: 1},
		time.Now().AddDate(0, 0, -400))
	if err != nil {
		t.Fatal(err)
	}
	fresh := mustSelfSigned(t, Request{DNSNames: []string{"app.example.com"}})
	m := New()
	m.Set([]*Material{old, fresh}, nil)
	got, err := m.GetCertificate(hello("app.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if got != fresh.TLS() {
		t.Fatal("an expired certificate must never win over a valid one for the same name")
	}
}

func TestACMERefusesWhatWouldFailLater(t *testing.T) {
	base := ACMEOptions{Hosts: []string{"app.example.com"}, AcceptTOS: true}

	no := base
	no.Hosts = nil
	if _, err := NewACME(no); err == nil || !strings.Contains(err.Error(), "closed list") {
		t.Fatalf("an open host policy lets anyone burn the authority's rate limit: %v", err)
	}

	tos := base
	tos.AcceptTOS = false
	if _, err := NewACME(tos); err == nil || !strings.Contains(err.Error(), "terms of service") {
		t.Fatalf("accepting terms is a legal act and is never assumed: %v", err)
	}

	half := base
	half.EABKeyID = "kid-only"
	if _, err := NewACME(half); err == nil || !strings.Contains(err.Error(), "BOTH") {
		t.Fatalf("an external account binding needs both halves: %v", err)
	}

	root := base
	root.RootCA = "not a pem block"
	if _, err := NewACME(root); err == nil || !strings.Contains(err.Error(), "BEGIN CERTIFICATE") {
		t.Fatalf("the error must say what a root looks like: %v", err)
	}

	// A private directory is the whole point: nothing about the automatic path
	// may depend on Let's Encrypt existing.
	private := base
	private.DirectoryURL = "https://step-ca.internal/acme/acme/directory"
	private.EABKeyID = "kid"
	private.EABHMACKey = "c2VjcmV0LWhtYWMta2V5"
	ca := newTestCA(t)
	private.RootCA = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw}))
	am, err := NewACME(private)
	if err != nil {
		t.Fatalf("an internal authority behind a private root must configure: %v", err)
	}
	if am.Client.DirectoryURL != private.DirectoryURL {
		t.Fatalf("directory = %q, want the private one", am.Client.DirectoryURL)
	}
	if am.Client.HTTPClient == nil {
		t.Fatal("a private root must produce a client that trusts it, or the first call fails on a name nobody connects to their own PKI")
	}
	if am.ExternalAccountBinding == nil || am.ExternalAccountBinding.KID != "kid" {
		t.Fatal("the external account binding must reach the manager")
	}
}

func TestEABKeyAcceptsEveryBase64AnAuthorityPrints(t *testing.T) {
	// RFC 8555 says base64url, and enough authorities paste standard base64
	// that refusing those would be refusing a correct secret over punctuation.
	for _, encoded := range []string{
		"c2VjcmV0LWhtYWMta2V5",
		"c2VjcmV0LWhtYWMta2V5\n",
		"YWJjZA==",
		"a-b_cQ",
	} {
		if _, err := decodeEABKey(encoded); err != nil {
			t.Fatalf("decodeEABKey(%q): %v", encoded, err)
		}
	}
	if _, err := decodeEABKey("not base64 at all !!"); err == nil {
		t.Fatal("garbage must be refused")
	}
}

// TestTheHTTPSDoorSwapsWithoutMovingThePort is the whole design run rather
// than argued: the certificate changes under a live listener, the port never
// moves, HTTP/2 is what a browser gets, and r.TLS is set - the field every
// Secure cookie in this product depends on.
func TestTheHTTPSDoorSwapsWithoutMovingThePort(t *testing.T) {
	first := mustSelfSigned(t, Request{DNSNames: []string{"one.example.com"}})
	second := mustSelfSigned(t, Request{DNSNames: []string{"two.example.com"}})

	m := New()
	m.Set([]*Material{first}, first)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s tls=%v", r.Proto, r.TLS != nil)
	})
	// Bind on a port the operating system picks: the test must not race a
	// hard-coded one against whatever else runs on this machine.
	ln := NewListener("data", "127.0.0.1:0", h, m.TLSConfig())
	if err := ln.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = ln.Stop(context.Background()) })
	addr := ln.ln.Addr().String()

	get := func(host string) (*http.Response, string, error) {
		tr := &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, ServerName: host},
		}
		defer tr.CloseIdleConnections()
		res, err := (&http.Client{Transport: tr, Timeout: 5 * time.Second}).Get("https://" + addr + "/")
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = res.Body.Close() }()
		b, _ := io.ReadAll(res.Body)
		return res, string(b), nil
	}

	res, body, err := get("one.example.com")
	if err != nil {
		t.Fatalf("first handshake: %v", err)
	}
	// HTTP/2 comes with TLS and costs nothing - but only while the protocol
	// list is stated, and this listener states it to carry ACME's.
	if body != "HTTP/2.0 tls=true" {
		t.Fatalf("HTTP/2 and r.TLS must both hold, got %q", body)
	}
	if res.TLS.PeerCertificates[0].Subject.CommonName != "one.example.com" {
		t.Fatalf("served %q", res.TLS.PeerCertificates[0].Subject.CommonName)
	}

	// The swap. No stop, no start, no second port.
	m.Set([]*Material{second}, second)

	res, _, err = get("two.example.com")
	if err != nil {
		t.Fatalf("handshake after the swap: %v", err)
	}
	if res.TLS.PeerCertificates[0].Subject.CommonName != "two.example.com" {
		t.Fatalf("after the swap the listener still serves %q", res.TLS.PeerCertificates[0].Subject.CommonName)
	}
	if got := ln.ln.Addr().String(); got != addr {
		t.Fatalf("the port moved: %s then %s", addr, got)
	}
}

func TestListenerRefusesATakenPortWithoutBreakingAnything(t *testing.T) {
	m := New()
	m.Set([]*Material{mustSelfSigned(t, Request{DNSNames: []string{"a.test"}})}, nil)
	busy := NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), m.TLSConfig())
	if err := busy.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = busy.Stop(context.Background()) })

	clash := NewListener("admin", busy.ln.Addr().String(), http.NotFoundHandler(), m.TLSConfig())
	err := clash.Start()
	if err == nil {
		t.Fatal("binding a taken port must fail")
	}
	// Archway's lesson: the failure arrives BEFORE anything changed.
	if !strings.Contains(err.Error(), "admin") || !strings.Contains(err.Error(), "HTTPS port") {
		t.Fatalf("the error must name the plane and the port: %v", err)
	}
	if clash.Running() {
		t.Fatal("a door that failed to open must not report itself open")
	}
	if !busy.Running() {
		t.Fatal("the door that was already open must be untouched")
	}
}

// TestPlaintextIsSentToTheHTTPSPort: the redirect has to SWAP the port, since
// the request arrived on the plain one - and it has to keep the host the
// caller used, because a gateway fronts many domains and answering with the
// configured one would send half of them somewhere they never asked for.
func TestPlaintextIsSentToTheHTTPSPort(t *testing.T) {
	d := NewRedirect()
	d.Set(true, ":8443")
	h := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	// A 301 makes clients re-issue a POST as a GET, which would silently drop
	// the body of an API call that reached the plain port. 308 preserves both,
	// and only GET and HEAD - with nothing to preserve - take the status every
	// old client understands.
	for method, want := range map[string]int{
		http.MethodGet:    http.StatusMovedPermanently,
		http.MethodHead:   http.StatusMovedPermanently,
		http.MethodPost:   http.StatusPermanentRedirect,
		http.MethodPut:    http.StatusPermanentRedirect,
		http.MethodDelete: http.StatusPermanentRedirect,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "http://shop.example.com:8080/api/orders", nil))
		if rec.Code != want {
			t.Fatalf("%s: %d, want %d", method, rec.Code, want)
		}
		if got := rec.Header().Get("Location"); got != "https://shop.example.com:8443/api/orders" {
			t.Fatalf("%s: Location = %q", method, got)
		}
	}

	// 443 is implied by the scheme, and a browser handed it back writes it
	// into the address bar for the rest of the session.
	d.Set(true, ":443")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://shop.example.com/x", nil))
	if got := rec.Header().Get("Location"); got != "https://shop.example.com/x" {
		t.Fatalf("port 443 must not be written out: %q", got)
	}

	// The liveness probe keeps answering in the clear: a container runtime
	// asks whether the process is alive, not whether it is secured.
	d.Set(true, ":8443")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://shop.example.com:8080/healthz", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("healthz must not be redirected, got %d", rec.Code)
	}

	// And nothing is redirected while it is off, or a plain installation would
	// send everyone to a port that speaks no TLS.
	d.Set(false, ":8443")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://shop.example.com:8080/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("off, nothing may be redirected, got %d", rec.Code)
	}
}

// TestAnExpiredCertificateStandsTheRedirectDown: an expired certificate still
// completes a handshake - a server does not check its own expiry - and every
// client then refuses it. Sending callers there would strand the application
// behind a door none of them will open.
func TestAnExpiredCertificateStandsTheRedirectDown(t *testing.T) {
	expired, err := SelfSigned(Request{DNSNames: []string{"app.example.com"}, Days: 1},
		time.Now().AddDate(0, 0, -400))
	if err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{
		app: []*Material{expired}, appFallback: expired,
		settings: Settings{Redirect: true},
	}
	d := NewRedirect()
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), d)
	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	// Having a certificate IS the activation: no switch was set, and the door
	// opened because there is something to present.
	if !sup.State().App {
		t.Fatal("an expired certificate is still something to present: the door opens")
	}
	if sup.State().Redirecting {
		t.Fatal("but nobody is sent to it")
	}
	if len(sup.State().Problems) == 0 {
		t.Fatal("and the reason has to be on screen")
	}
	rec := httptest.NewRecorder()
	d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.example.com:8080/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("the application plane must keep answering in the clear, got %d", rec.Code)
	}
}

// fakeSource stands in for the store.
type fakeSource struct {
	console     []*Material
	consoleFall *Material
	app         []*Material
	appFallback *Material
	settings    Settings
}

func (f *fakeSource) CertificateMaterials(_ context.Context, plane string) ([]*Material, *Material, []string, error) {
	if plane == PlaneConsole {
		return f.console, f.consoleFall, nil, nil
	}
	return f.app, f.appFallback, nil, nil
}
func (f *fakeSource) TLSSettings(context.Context) Settings                 { return f.settings }
func (f *fakeSource) ACMECacheGet(context.Context, string) (string, error) { return "", errNoRow }
func (f *fakeSource) ACMECachePut(context.Context, string, string) error   { return nil }
func (f *fakeSource) ACMECacheDelete(context.Context, string) error        { return nil }

var errNoRow = errors.New("no row")

// TestCoversFollowsTheRuleX509Applies. A wildcard stands for exactly ONE
// label - the mistake everyone makes is expecting *.example.com to cover the
// bare domain, or two levels down.
func TestCoversFollowsTheRuleX509Applies(t *testing.T) {
	names := []string{"shop.example.com", "*.apps.example.com", "10.0.0.5"}
	for _, host := range []string{"shop.example.com", "SHOP.example.com", "shop.example.com.", "a.apps.example.com", "10.0.0.5"} {
		if !Covers(names, host) {
			t.Fatalf("%q must be covered", host)
		}
	}
	for _, host := range []string{"example.com", "apps.example.com", "a.b.apps.example.com", "other.example.com", ""} {
		if Covers(names, host) {
			t.Fatalf("%q must NOT be covered", host)
		}
	}
	// And what the manager itself would pick has to agree with this, or the
	// screen would promise a coverage the handshake refuses.
	m := mustSelfSigned(t, Request{DNSNames: []string{"*.apps.example.com"}})
	if err := m.Leaf().VerifyHostname("a.apps.example.com"); err != nil {
		t.Fatalf("x509 disagrees with Covers: %v", err)
	}
	if err := m.Leaf().VerifyHostname("apps.example.com"); err == nil {
		t.Fatal("x509 disagrees with Covers on the bare domain")
	}
}

// TestHavingACertificateIsTheActivation: there is no switch to disagree with.
// A plane with material serves TLS; a plane without does not, and no state
// exists in which one says "on" while nothing can be presented.
func TestHavingACertificateIsTheActivation(t *testing.T) {
	console := mustSelfSigned(t, Request{DNSNames: []string{"localhost"}})
	src := &fakeSource{
		console: []*Material{console}, consoleFall: console,
		settings: Settings{},
	}
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), NewRedirect())
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := sup.State()
	if !st.Console {
		t.Fatal("the console has a certificate: its door is open")
	}
	if st.App {
		t.Fatal("the application has none: its door stays shut")
	}
	// And nothing is reported as a problem - an empty plane is not a fault,
	// it is an installation that has not got there yet.
	if len(st.Problems) != 0 {
		t.Fatalf("no certificate is not a problem to report: %v", st.Problems)
	}

	// Remove it, and the door closes on its own.
	src.console, src.consoleFall = nil, nil
	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sup.State().Console {
		t.Fatal("deleting the certificate is what closes the door")
	}
}

// TestEachPlaneOnlyEverPresentsItsOwn: the console door must never hand out an
// application certificate, whatever names it carries.
func TestEachPlaneOnlyEverPresentsItsOwn(t *testing.T) {
	console := mustSelfSigned(t, Request{DNSNames: []string{"admin.example.com"}})
	app := mustSelfSigned(t, Request{DNSNames: []string{"shop.example.com"}})
	src := &fakeSource{
		console: []*Material{console}, consoleFall: console,
		app: []*Material{app}, appFallback: app,
	}
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), NewRedirect())
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := sup.Console.GetCertificate(hello("shop.example.com")); err == nil {
		// It falls back rather than erroring, but it must never be the
		// application's certificate that comes out.
		got, _ := sup.Console.GetCertificate(hello("shop.example.com"))
		if got == app.TLS() {
			t.Fatal("the console door presented an application certificate")
		}
	}
	got, err := sup.App.GetCertificate(hello("shop.example.com"))
	if err != nil || got != app.TLS() {
		t.Fatalf("the application door must present its own: %v", err)
	}
}

// TestOlderAuthoritySettingsFoldIntoOrders: before orders, a name was
// declared per plane and the automatic ones ticked (or, briefly, listed per
// door). What the authority could be asked for, and on which door, survives
// as orders - one per door, one for the names both doors shared.
func TestOlderAuthoritySettingsFoldIntoOrders(t *testing.T) {
	got := Settings{
		LegacyConsoleName: "Admin.example.com",
		LegacyAppNames:    []string{"shop.example.com", "admin.example.com"},
		ACME: ACMESettings{LegacyDomains: []string{
			"admin.example.com", "shop.example.com", "orphan.example.com",
		}},
	}.Normalized()
	if got.LegacyConsoleName != "" || got.LegacyAppNames != nil ||
		got.ACME.LegacyDomains != nil || got.ACME.LegacyConsoleDomains != nil {
		t.Fatalf("the older fields must go: %+v", got)
	}
	if fmt.Sprint(got.ACME.DomainsOn(PlaneConsole)) != "[admin.example.com]" {
		t.Fatalf("the console's name stays on the console's door: %v", got.ACME.DomainsOn(PlaneConsole))
	}
	if fmt.Sprint(got.ACME.DomainsOn(PlaneApp)) != "[admin.example.com shop.example.com orphan.example.com]" {
		t.Fatalf("every name the application could ask for stays there: %v", got.ACME.DomainsOn(PlaneApp))
	}
	if len(got.ACME.Orders) != 2 {
		t.Fatalf("one order for the shared name, one for the application's: %+v", got.ACME.Orders)
	}
	// Folding again changes nothing: an order is named by its names.
	again := got.Normalized()
	if fmt.Sprint(again.ACME.Orders) != fmt.Sprint(got.ACME.Orders) {
		t.Fatalf("normalizing twice must change nothing: %+v", again.ACME.Orders)
	}
	if OrderID("ca", []string{"a.example.com"}) != OrderID("ca", []string{"A.example.com"}) {
		t.Fatal("an order's id does not depend on the case of its names")
	}
}

// TestAnAuthorityOnlyOpensTheDoorItIsPlacedOn: an order placed on the
// application alone arms the authority for that door; the console's must not
// open on an account with nothing to ask for there.
func TestAnAuthorityOnlyOpensTheDoorItIsPlacedOn(t *testing.T) {
	m := New()
	m.SetACME(nil)
	if m.Serves() || m.Live(time.Now()) {
		t.Fatal("an authority with no name on this door serves nothing here")
	}
	m.SetACME(map[string]*autocert.Manager{"shop.example.com": {}})
	if !m.Serves() || !m.Answers("shop.example.com") || m.Answers("other.example.com") {
		t.Fatal("with a name placed, the door opens for that name")
	}
}

// TestADoorClosedFromInsideAnswers: a request arriving through an HTTPS door
// and asking for that door to close must still get its answer - it used to
// wait on Shutdown, which was waiting on it.
func TestADoorClosedFromInsideAnswers(t *testing.T) {
	m, err := SelfSigned(Request{DNSNames: []string{"localhost"}, IPAddresses: []string{"127.0.0.1"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	mgr := New()
	mgr.Set([]*Material{m}, m)
	var l *Listener
	l = NewListener("admin", "127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = l.Sync(r.Context(), false)
		_, _ = w.Write([]byte("closed"))
	}), mgr.TLSConfig())
	// Bind to a free port first, then reuse it: Start binds Addr.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l.Addr = probe.Addr().String()
	_ = probe.Close()
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // a test against its own certificate
	}}
	res, err := client.Get("https://" + l.Addr + "/")
	if err != nil {
		t.Fatalf("the request that closed the door got no answer: %v", err)
	}
	_ = res.Body.Close()
	if l.Running() {
		t.Fatal("and the door is closed")
	}
	// The port is free again at once: the door can reopen.
	if err := l.Start(); err != nil {
		t.Fatalf("reopening: %v", err)
	}
	_ = l.Stop(context.Background())
}

// TestOnlyANameWithACertificateIsRedirected: a service inside the cluster
// calling http://meerkat-meerkat:8080 reaches the gateway by a name no
// certificate carries. Sending it to HTTPS would send it to a handshake it
// cannot complete - its JWKS fetch failed the day HTTPS was forced.
func TestOnlyANameWithACertificateIsRedirected(t *testing.T) {
	d := NewRedirect()
	d.Set(true, ":8443")
	d.SetServes(func(host string) bool { return host == "shop.example.com" })
	h := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	for url, want := range map[string]int{
		"http://shop.example.com:8080/x":                                             http.StatusMovedPermanently,
		"http://meerkat-meerkat.canopy.svc.cluster.local:8080/.well-known/jwks.json": http.StatusTeapot,
		"http://10.1.2.3:8080/x":                                                     http.StatusTeapot,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", url, rec.Code, want)
		}
	}
}

// TestTheOneAccountBecomesTheFirstAuthority: the single account of the
// settings before authorities keeps working - its cache untouched, its
// orders its own.
func TestTheOneAccountBecomesTheFirstAuthority(t *testing.T) {
	got := Settings{ACME: ACMESettings{
		LegacyEnabled: true, LegacyAcceptTOS: true, LegacyEmail: "ops@example.com",
		LegacyDomains: []string{"shop.example.com"},
	}}.Normalized()
	if len(got.ACME.Authorities) != 1 {
		t.Fatalf("one authority: %+v", got.ACME.Authorities)
	}
	a := got.ACME.Authorities[0]
	if a.Provider != ProviderLetsEncrypt || a.CachePrefix != "" || !a.AcceptTOS || a.Email != "ops@example.com" {
		t.Fatalf("Let's Encrypt, the cache as it was: %+v", a)
	}
	if len(got.ACME.Orders) != 1 || got.ACME.Orders[0].Authority != a.ID {
		t.Fatalf("its orders are its own: %+v", got.ACME.Orders)
	}
	if got.ACME.LegacyEnabled || got.ACME.LegacyEmail != "" {
		t.Fatalf("the old fields go: %+v", got.ACME)
	}
}

// TestARefusalSaysWhatToCheck: autocert's "no viable challenge type found"
// is what an unreachable name looks like, and says nothing an operator can
// act on.
func TestARefusalSaysWhatToCheck(t *testing.T) {
	got := explain("shop.example.com", errors.New(`acme/autocert: unable to satisfy "x" for domain "shop.example.com": no viable challenge type found`))
	for _, want := range []string{"port 443", "DNS only", "CAA", "no viable challenge type found"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	if got := explain("x", errors.New("something else")); got != "something else" {
		t.Errorf("an unknown refusal is passed on as it is: %s", got)
	}
}

// TestTheCommunityImageAsksNoAuthority: ACME is Enterprise. Orders a
// configuration brought over are kept, not asked - and the screen says so,
// since a certificate nobody renews is one that ends.
func TestTheCommunityImageAsksNoAuthority(t *testing.T) {
	if edition.Enterprise {
		t.Skip("the community image's refusal")
	}
	src := &fakeSource{settings: Settings{ACME: ACMESettings{
		Authorities: []Authority{{ID: "ca", Name: "LE", Provider: ProviderLetsEncrypt, AcceptTOS: true}},
		Orders:      []ACMEOrder{{ID: "o", Authority: "ca", Names: []string{"shop.example.com"}, App: true}},
	}}}
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), NewRedirect())
	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	st := sup.State()
	if st.ACME || st.App {
		t.Fatalf("nothing armed, no door opened on it: %+v", st)
	}
	if len(st.Problems) == 0 || !strings.Contains(st.Problems[0], "Enterprise") {
		t.Fatalf("and the screen says why: %v", st.Problems)
	}
}
