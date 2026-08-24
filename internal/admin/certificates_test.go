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
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/store"
)

func pemPair(t *testing.T, host string) (certPEM, keyPEM string) {
	t.Helper()
	m, err := certs.SelfSigned(certs.Request{DNSNames: []string{host}}, time.Now())
	if err != nil {
		t.Fatalf("SelfSigned: %v", err)
	}
	return m.CertPEM, m.KeyPEM
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestTheFourDoors walks each way a certificate can arrive, because the
// argument for having four is that the rooms Meerkat runs in are not alike -
// and an untested door is a room without TLS.
func TestTheFourDoors(t *testing.T) {
	f := setup(t)

	// 1. Import: what a CA e-mailed back, what certbot wrote.
	certPEM, keyPEM := pemPair(t, "imported.example.com")
	body := `{"name":"imported","certPem":` + jsonString(certPEM) + `,"keyPem":` + jsonString(keyPEM) + `}`
	code, out := f.call(t, "POST", "/api/certificates/import", body, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("import: %d %s", code, out)
	}
	if strings.Contains(out, "PRIVATE KEY") {
		t.Fatal("a private key must never travel back to the console")
	}

	// 2. Self-signed: HTTPS in one click, honestly labelled.
	code, out = f.call(t, "POST", "/api/certificates/self-signed",
		`{"name":"lab","dnsNames":["lab.example.com"],"keyType":"ecdsa-p256"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("self-signed: %d %s", code, out)
	}
	var lab certView
	if err := json.Unmarshal([]byte(out), &lab); err != nil {
		t.Fatal(err)
	}
	if !lab.Info.SelfSigned {
		t.Fatal("a self-signed certificate must be reported as such")
	}

	// 3. The offline door: a request leaves, the key stays.
	code, out = f.call(t, "POST", "/api/certificates/signing-request",
		`{"name":"intranet","dnsNames":["intranet.acme.test"],"organization":"Acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("signing request: %d %s", code, out)
	}
	var pending certView
	if err := json.Unmarshal([]byte(out), &pending); err != nil {
		t.Fatal(err)
	}
	if !pending.Pending || pending.CSR == nil {
		t.Fatalf("a fresh signing request must show as pending, with what it asked for: %s", out)
	}
	code, csr := f.call(t, "GET", "/api/certificates/"+pending.ID+"/signing-request", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(csr, "BEGIN CERTIFICATE REQUEST") {
		t.Fatalf("the request must be downloadable - that is the whole point: %d %s", code, csr)
	}
	// It serves nothing yet, and says so rather than pretending.
	if code, out := f.call(t, "GET", "/api/certificates/"+pending.ID+"/pem", "", f.rootC); code != http.StatusNotFound {
		t.Fatalf("a pending request has no certificate to download: %d %s", code, out)
	}

	// The authority signs it, offline, and the answer comes home.
	signed := signCSR(t, csr)
	code, out = f.call(t, "POST", "/api/certificates/"+pending.ID+"/adopt",
		`{"certPem":`+jsonString(signed)+`}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("adopt: %d %s", code, out)
	}
	var adopted certView
	if err := json.Unmarshal([]byte(out), &adopted); err != nil {
		t.Fatal(err)
	}
	if adopted.Pending || adopted.Info.SelfSigned {
		t.Fatalf("an adopted certificate is neither pending nor self-signed: %s", out)
	}

	// 4. The automatic door, pointed at a PRIVATE authority: nothing about it
	// may depend on Let's Encrypt existing.
	code, out = f.call(t, "PUT", "/api/settings/tls",
		`{"data":false,"admin":false,"acme":{"enabled":true,"directoryUrl":"https://step-ca.internal/acme/directory",`+
			`"domains":["app.example.com"],"acceptTos":true,"email":"ops@example.com"}}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("acme settings: %d %s", code, out)
	}

	code, out = f.call(t, "GET", "/api/certificates", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, out)
	}
	var list []certView
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("three certificates went in, %d came out", len(list))
	}
	if strings.Contains(out, "PRIVATE KEY") {
		t.Fatal("the list must never carry key material")
	}
}

// signCSR stands in for whoever signs offline: the company authority, an
// operator with a laptop and openssl, a smart card. What matters is that the
// answer carries the SAME public key the request did.
func signCSR(t *testing.T, csrPEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(strings.TrimSpace(csrPEM)))
	if block == nil {
		t.Fatal("not a signing request")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "Acme Internal CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: csr.Subject,
		DNSNames: csr.DNSNames, IPAddresses: csr.IPAddresses,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(0, 6, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, csr.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestAdoptRefusesTheCertificateOfAnotherOrder(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "POST", "/api/certificates/signing-request",
		`{"name":"mine","dnsNames":["mine.example.com"]}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("signing request: %d %s", code, out)
	}
	var pending certView
	if err := json.Unmarshal([]byte(out), &pending); err != nil {
		t.Fatal(err)
	}
	// A certificate for the right name, signed from someone else's request.
	stranger, _ := pemPair(t, "mine.example.com")
	code, out = f.call(t, "POST", "/api/certificates/"+pending.ID+"/adopt",
		`{"certPem":`+jsonString(stranger)+`}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("adopt of a foreign certificate: %d %s", code, out)
	}
	if !strings.Contains(out, "not signed from this request") {
		t.Fatalf("the refusal must name the mistake, not fail at handshake time: %s", out)
	}
}

func TestImportNamesWhatItRefuses(t *testing.T) {
	f := setup(t)
	certPEM, _ := pemPair(t, "a.example.com")
	_, keyPEM := pemPair(t, "b.example.com")
	code, out := f.call(t, "POST", "/api/certificates/import",
		`{"name":"mixed","certPem":`+jsonString(certPEM)+`,"keyPem":`+jsonString(keyPEM)+`}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched pair: %d %s", code, out)
	}
	if !strings.Contains(out, "two different pairs") {
		t.Fatalf("the error must say what happened: %s", out)
	}
	if code, out := f.call(t, "POST", "/api/certificates/import", `{"name":"empty"}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("importing nothing: %d %s", code, out)
	}
}

// TestTheExternalAccountKeyFollowsTheVaultRule is VAULT-05 applied to the one
// secret the TLS screen holds: a literal never travels back, and a blank field
// means "leave it alone" rather than "erase it".
func TestTheExternalAccountKeyFollowsTheVaultRule(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	body := `{"data":false,"admin":false,"acme":{"enabled":true,"domains":["app.example.com"],` +
		`"acceptTos":true,"eabKeyId":"kid-1","eabHmacKey":"c2VjcmV0LWhtYWMta2V5"}}`
	code, out := f.call(t, "PUT", "/api/settings/tls", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("save: %d %s", code, out)
	}
	if strings.Contains(out, "c2VjcmV0LWhtYWMta2V5") {
		t.Fatal("a literal secret must never travel back to the console")
	}
	var view tlsPayload
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if !view.EABSecretSet {
		t.Fatal("the console has to know something IS configured, even without seeing it")
	}

	// Saving again with a blank secret keeps the stored one: the console never
	// received it, so it cannot resend it.
	code, out = f.call(t, "PUT", "/api/settings/tls",
		`{"data":false,"admin":false,"acme":{"enabled":true,"domains":["app.example.com"],`+
			`"acceptTos":true,"eabKeyId":"kid-2","eabHmacKey":""}}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("second save: %d %s", code, out)
	}
	stored := f.api.st.RawTLS(ctx)
	if stored.ACME.EABHMACKey != "c2VjcmV0LWhtYWMta2V5" {
		t.Fatalf("a blank field erased the secret: %q", stored.ACME.EABHMACKey)
	}
	if stored.ACME.EABKeyID != "kid-2" {
		t.Fatal("the rest of the form must still save")
	}
}

func TestChangingAuthorityForgetsTheOldAccount(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.ACMECachePut(ctx, "acme_account+key", "registered at the old directory"); err != nil {
		t.Fatal(err)
	}
	code, out := f.call(t, "PUT", "/api/settings/tls",
		`{"data":false,"admin":false,"acme":{"enabled":true,"directoryUrl":"https://other.authority/directory",`+
			`"domains":["app.example.com"],"acceptTos":true}}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("save: %d %s", code, out)
	}
	// An account key registered at one directory is worthless at another, and
	// keeping it is how a renewal fails months later on an unknown account.
	if _, err := f.api.st.ACMECacheGet(ctx, "acme_account+key"); err == nil {
		t.Fatal("switching authority must forget the account registered at the previous one")
	}
}

func TestACMEIsRefusedAtSaveTimeRatherThanAtRenewalTime(t *testing.T) {
	f := setup(t)
	// No domains: an open host policy lets anyone reach the gateway with a
	// made-up name and burn the authority's rate limit.
	code, out := f.call(t, "PUT", "/api/settings/tls",
		`{"data":false,"admin":false,"acme":{"enabled":true,"acceptTos":true}}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("acme with no domain: %d %s", code, out)
	}
	// Accepting terms is a legal act: it is never assumed on an operator's
	// behalf, so the tick box is what carries it.
	code, out = f.call(t, "PUT", "/api/settings/tls",
		`{"data":false,"admin":false,"acme":{"enabled":true,"domains":["app.example.com"]}}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "terms of service") {
		t.Fatalf("acme without accepted terms: %d %s", code, out)
	}
}

// TestSwitchingHTTPSOnWithNothingToPresentSaysSo runs the supervisor for real:
// a port that binds and then refuses every visitor is worse than a door that
// stayed shut and said why.
func TestSwitchingHTTPSOnWithNothingToPresentSaysSo(t *testing.T) {
	f := setup(t)
	f.api.TLS = certs.NewSupervisor(f.api.st, isNoRows,
		certs.NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewRedirect(),
	)
	t.Cleanup(func() { _ = f.api.TLS.Stop(context.Background()) })

	code, out := f.call(t, "PUT", "/api/settings/tls", `{"data":true,"admin":false,"redirect":true,"acme":{}}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("save: %d %s", code, out)
	}
	var view tlsPayload
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.State.Data {
		t.Fatal("no certificate is installed: the HTTPS door must stay shut")
	}
	if len(view.State.Problems) == 0 || !strings.Contains(strings.Join(view.State.Problems, " "), "no certificate is installed") {
		t.Fatalf("the reason must be on screen, not only in a log: %+v", view.State)
	}

	// Install one, and the same setting now opens the door.
	certPEM, keyPEM := pemPair(t, "app.example.com")
	code, out = f.call(t, "POST", "/api/certificates/import",
		`{"name":"app","certPem":`+jsonString(certPEM)+`,"keyPem":`+jsonString(keyPEM)+`,"default":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("import: %d %s", code, out)
	}
	code, out = f.call(t, "GET", "/api/settings/tls", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("read back: %d %s", code, out)
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if !view.State.Data {
		t.Fatalf("with a certificate installed the door must open: %+v", view.State)
	}
	if len(view.State.Problems) != 0 {
		t.Fatalf("nothing should be wrong now: %v", view.State.Problems)
	}
}

func TestCertificatesAreInfrastructure(t *testing.T) {
	f := setup(t)
	if code, _ := f.call(t, "GET", "/api/certificates", "", nil); code != http.StatusUnauthorized {
		t.Fatal("anonymous must not list certificates")
	}
	if code, _ := f.call(t, "GET", "/api/certificates", "", f.plainC); code != http.StatusForbidden {
		t.Fatal("a plain account must not list certificates")
	}
}

func isNoRows(err error) bool { return errors.Is(err, store.ErrNoRows) }
