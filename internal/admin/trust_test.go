package admin

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/discovery"
)

// TestTrustedAuthoritiesAreImportedPEMs: an authority is a named PEM; one that
// holds no certificate, or holds a private key, is refused at save time; the
// list says what each one holds.
func TestTrustedAuthoritiesAreImportedPEMs(t *testing.T) {
	f := setupBare(t)
	upstream := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)
	authority := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))
	body := func(v map[string]string) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "company CA", "pem": authority}), f.rootC)
	if code != http.StatusCreated || !strings.Contains(out, `"fingerprint"`) {
		t.Fatalf("imported: %d %s", code, out)
	}
	key := authority + "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "oops", "pem": key}), f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "certificate pool") {
		t.Fatalf("a private key: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "junk", "pem": "not a pem"}), f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("no certificate: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "x", "pem": authority}), f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d %s", code, out)
	}
	code, out = f.call(t, "GET", "/api/trusted-authorities", "", f.rootC)
	var list []trustView
	if err := json.Unmarshal([]byte(out), &list); err != nil || code != http.StatusOK || len(list) != 1 || len(list[0].Certificates) != 1 {
		t.Fatalf("the list: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/trusted-authorities/"+list[0].ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, out)
	}
}

// TestPlatformAuthoritiesAreFoundThenTaken: what the platform publishes is
// listed without what is trusted already, taken by its reference - read again
// from its source, never sent by the browser - and refused a second time.
func TestPlatformAuthoritiesAreFoundThenTaken(t *testing.T) {
	f := setupBare(t)
	upstream := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)
	authority := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))

	oldFind, oldTake := findAuthorities, takeAuthority
	t.Cleanup(func() { findAuthorities, takeAuthority = oldFind, oldTake })
	found := discovery.PlatformAuthorities{Kubernetes: true,
		Found: []discovery.FoundAuthority{
			{Ref: "openshift/service-ca", Source: "openshift", Name: "openshift-service-ca", PEM: authority, Ready: true},
			{Ref: "clusterissuer/company-ca", Source: "cert-manager", Name: "company-ca", Ready: true},
		},
		Notes: []discovery.SourceNote{{Source: "trust-manager", Absent: true, Why: "trust-manager is not installed in this cluster"}},
	}
	findAuthorities = func(context.Context) discovery.PlatformAuthorities { return found }
	takeAuthority = func(_ context.Context, ref string) (discovery.FoundAuthority, error) {
		for _, a := range found.Found {
			if a.Ref == ref {
				a.PEM = authority // an issuer's is asked of cert-manager when taken
				return a, nil
			}
		}
		return discovery.FoundAuthority{}, errors.New(ref + " is not offered by this platform")
	}
	list := func() platformAnswer {
		t.Helper()
		code, out := f.call(t, "GET", "/api/trusted-authorities/platform", "", f.rootC)
		var a platformAnswer
		if err := json.Unmarshal([]byte(out), &a); err != nil || code != http.StatusOK {
			t.Fatalf("the platform's authorities: %d %s", code, out)
		}
		return a
	}
	if a := list(); !a.Kubernetes || len(a.Found) != 2 || len(a.Found[0].Certificates) != 1 || len(a.Notes) != 1 {
		t.Fatalf("found: %+v", a)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"openshift/service-ca"}`, f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"secret/anything"}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("a reference the platform does not offer: %d %s", code, out)
	}
	code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"openshift/service-ca"}`, f.rootC)
	if code != http.StatusCreated || !strings.Contains(out, `"openshift-service-ca"`) {
		t.Fatalf("taken: %d %s", code, out)
	}
	// Trusted now: the file is no longer offered, and the issuer that signs
	// with the same authority is refused when taken.
	if a := list(); len(a.Found) != 1 || a.Found[0].Ref != "clusterissuer/company-ca" {
		t.Fatalf("after taking it: %+v", a)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"clusterissuer/company-ca","name":"Company"}`, f.rootC); code != http.StatusConflict {
		t.Fatalf("the same authority twice: %d %s", code, out)
	}
	// An issuer's certificate is learnt only when taken: once taken, its
	// reference is what keeps it from being offered again.
	found.Found = append(found.Found, discovery.FoundAuthority{Ref: "issuer/other-ca", Source: "cert-manager", Name: "other-ca", Ready: true})
	otherPEM := newAuthorityPEM(t, "other-ca")
	takeAuthority = func(_ context.Context, ref string) (discovery.FoundAuthority, error) {
		return discovery.FoundAuthority{Ref: ref, Name: "other-ca", PEM: otherPEM}, nil
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"issuer/other-ca"}`, f.rootC); code != http.StatusCreated || !strings.Contains(out, `"source":"issuer/other-ca"`) {
		t.Fatalf("an issuer taken: %d %s", code, out)
	}
	for _, p := range list().Found {
		if p.Ref == "issuer/other-ca" {
			t.Fatalf("an issuer taken is offered again: %+v", p)
		}
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities/platform", `{"ref":"issuer/other-ca"}`, f.rootC); code != http.StatusConflict {
		t.Fatalf("an issuer taken twice: %d %s", code, out)
	}
}

// newAuthorityPEM is a fresh self-signed authority: the httptest servers all
// share one certificate, which would read as the same authority.
func newAuthorityPEM(t *testing.T, cn string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
