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
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/session"
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
	f := setupEE(t)

	// 1. Import: what a CA e-mailed back, what certbot wrote.
	certPEM, keyPEM := pemPair(t, "imported.example.com")
	body := `{"app":true,"certPem":` +
		jsonString(certPEM) + `,"keyPem":` + jsonString(keyPEM) + `}`
	code, out := f.call(t, "POST", "/api/certificates/import", body, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("import: %d %s", code, out)
	}
	if strings.Contains(out, "PRIVATE KEY") {
		t.Fatal("a private key must never travel back to the console")
	}

	// 2. Self-signed: HTTPS in one click, honestly labelled.
	code, out = f.call(t, "POST", "/api/certificates/self-signed",
		`{"names":["localhost"],"console":true,"keyType":"ecdsa-p256"}`, f.rootC)
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
		`{"names":["intranet.acme.test"],"organization":"Acme"}`, f.rootC)
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
	f.api.DiscoverACME = func(context.Context, certs.Authority) error { return nil }
	code, out = f.call(t, "POST", "/api/acme/authorities",
		`{"provider":"custom","name":"Step","directoryUrl":"https://step-ca.internal/acme/directory",`+
			`"acceptTos":true,"email":"ops@example.com"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("authority: %d %s", code, out)
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

// A signing request takes what it uses: the signer sets the validity, and it
// is placed once signed. Asking for either is refused rather than ignored.
func TestASigningRequestRefusesWhatItWouldIgnore(t *testing.T) {
	f := setup(t)
	for _, body := range []string{
		`{"names":["x.example.com"],"days":30}`,
		`{"names":["x.example.com"],"app":true}`,
		`{"names":["x.example.com"],"console":true,"replace":true}`,
	} {
		code, out := f.call(t, "POST", "/api/certificates/signing-request", body, f.rootC)
		if code != http.StatusBadRequest {
			t.Errorf("%s: %d %s, want 400", body, code, out)
		}
	}
	if _, list := f.call(t, "GET", "/api/certificates", "", f.rootC); strings.Contains(list, "x.example.com") {
		t.Fatalf("a refused request was kept: %s", list)
	}
}

func TestAdoptRefusesTheCertificateOfAnotherOrder(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "POST", "/api/certificates/signing-request",
		`{"names":["mine.example.com"]}`, f.rootC)
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
		`{"certPem":`+jsonString(certPEM)+
			`,"keyPem":`+jsonString(keyPEM)+`}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched pair: %d %s", code, out)
	}
	if !strings.Contains(out, "two different pairs") {
		t.Fatalf("the error must say what happened: %s", out)
	}
	if code, out := f.call(t, "POST", "/api/certificates/import",
		`{}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("importing nothing: %d %s", code, out)
	}
}

// authority creates one through the API, the directory check stood in for.
func authority(t *testing.T, f fixture, body string) authorityView {
	t.Helper()
	f.api.DiscoverACME = func(context.Context, certs.Authority) error { return nil }
	code, out := f.call(t, "POST", "/api/acme/authorities", body, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("authority %s: %d %s", body, code, out)
	}
	var v authorityView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestTheExternalAccountKeyFollowsTheVaultRule is VAULT-05 applied to the one
// secret an authority holds: a literal never travels back, and a blank field
// means "leave it alone" rather than "erase it".
func TestTheExternalAccountKeyFollowsTheVaultRule(t *testing.T) {
	f := setupEE(t)
	ctx := context.Background()
	v := authority(t, f, `{"provider":"zerossl","acceptTos":true,"eabKeyId":"kid-1","eabHmacKey":"c2VjcmV0LWhtYWMta2V5"}`)
	if v.EABHMACKey != "" || !v.EABSecretSet {
		t.Fatalf("the literal stays home, and the console knows one is held: %+v", v)
	}
	code, out := f.call(t, "PUT", "/api/acme/authorities/"+v.ID,
		`{"provider":"zerossl","acceptTos":true,"eabKeyId":"kid-2","eabHmacKey":""}`, f.rootC)
	if code != http.StatusOK || strings.Contains(out, "c2VjcmV0LWhtYWMta2V5") {
		t.Fatalf("second save: %d %s", code, out)
	}
	got, _ := f.api.st.RawTLS(ctx).ACME.AuthorityByID(v.ID)
	if got.EABHMACKey != "c2VjcmV0LWhtYWMta2V5" || got.EABKeyID != "kid-2" {
		t.Fatalf("a blank field keeps the secret, the rest saves: %+v", got)
	}
}

// An authority goes back as it was listed: what only the server writes
// (eabSecretSet, uses) is taken and dropped, never a 400, and the held secret
// stays held.
func TestAnAuthorityGoesBackAsItWasListed(t *testing.T) {
	f := setupEE(t)
	authority(t, f, `{"provider":"zerossl","acceptTos":true,"eabKeyId":"kid-1","eabHmacKey":"c2VjcmV0LWhtYWMta2V5"}`)
	_, out := f.call(t, "GET", "/api/acme/authorities", "", f.rootC)
	var listed struct {
		Authorities []map[string]any `json:"authorities"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil || len(listed.Authorities) != 1 {
		t.Fatalf("list: %v %s", err, out)
	}
	v := listed.Authorities[0]
	v["name"], v["uses"] = "Renamed", []string{"shop.example.com"}
	body, _ := json.Marshal(v)
	code, out := f.call(t, "PUT", "/api/acme/authorities/"+v["id"].(string), string(body), f.rootC)
	if code != http.StatusOK || !strings.Contains(out, `"eabSecretSet":true`) || strings.Contains(out, "shop.example.com") {
		t.Fatalf("the listed view sent back: %d %s", code, out)
	}
	got, _ := f.api.st.RawTLS(t.Context()).ACME.AuthorityByID(v["id"].(string))
	if got.Name != "Renamed" || got.EABHMACKey != "c2VjcmV0LWhtYWMta2V5" {
		t.Fatalf("the rename saves and the secret stays: %+v", got)
	}
}

// TestAnAuthorityIsRefusedAtSaveTimeRatherThanAtRenewalTime: what is missing
// is said when the authority is saved, naming where to find it.
func TestAnAuthorityIsRefusedAtSaveTimeRatherThanAtRenewalTime(t *testing.T) {
	f := setupEE(t)
	f.api.DiscoverACME = func(context.Context, certs.Authority) error { return nil }
	for body, want := range map[string]string{
		`{"provider":"letsencrypt"}`:                          "terms of service",
		`{"provider":"zerossl","acceptTos":true}`:             "ZeroSSL dashboard",
		`{"provider":"custom","acceptTos":true}`:              "no directory URL",
		`{"provider":"letsencrypt","acceptTos":true}`:         "",
		`{"provider":"letsencrypt-staging","acceptTos":true}`: "",
	} {
		code, out := f.call(t, "POST", "/api/acme/authorities", body, f.rootC)
		if want == "" {
			if code != http.StatusCreated {
				t.Errorf("%s: %d %s", body, code, out)
			}
			continue
		}
		if code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
			t.Errorf("%s: %d %s, want %q", body, code, out, want)
		}
	}
	// And an authority that does not answer is said at once.
	f.api.DiscoverACME = func(context.Context, certs.Authority) error { return errors.New("connection refused") }
	code, out := f.call(t, "POST", "/api/acme/authorities", `{"provider":"custom","directoryUrl":"https://nowhere.invalid/dir","acceptTos":true}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "connection refused") {
		t.Fatalf("an authority that does not answer: %d %s", code, out)
	}
}

// TestAPublicAuthorityKeepsNothingOfAPrivateOne: a root and a binding left
// from a custom authority would ride along unseen once Let's Encrypt is
// chosen - and a private root in place of the public ones fails every call.
func TestAPublicAuthorityKeepsNothingOfAPrivateOne(t *testing.T) {
	f := setupEE(t)
	v := authority(t, f, `{"provider":"custom","directoryUrl":"https://ca.internal/dir","acceptTos":true,"eabKeyId":"kid","eabHmacKey":"c2VjcmV0"}`)
	f.api.DiscoverACME = func(context.Context, certs.Authority) error { return nil }
	if code, out := f.call(t, "PUT", "/api/acme/authorities/"+v.ID, `{"provider":"letsencrypt","acceptTos":true,"eabKeyId":"kid"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("to Let's Encrypt: %d %s", code, out)
	}
	got, _ := f.api.st.RawTLS(context.Background()).ACME.AuthorityByID(v.ID)
	if got.EABKeyID != "" || got.EABHMACKey != "" || got.RootCA != "" || got.DirectoryURL != certs.LetsEncryptURL {
		t.Fatalf("Let's Encrypt keeps nothing of the private authority: %+v", got)
	}
	if got.CachePrefix == v.CachePrefix || got.CachePrefix == "" {
		t.Fatalf("another directory is another account, in another corner of the cache: %q", got.CachePrefix)
	}
}

// TestTheDoorFollowsTheCertificate replaces the old "switched on but nothing
// installed" case, which cannot happen any more: there is no switch. Adding a
// certificate opens the plane's door, deleting it closes it.
func TestTheDoorFollowsTheCertificate(t *testing.T) {
	f := setup(t)
	f.api.TLS = certs.NewSupervisor(f.api.st, isNoRows,
		certs.NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewRedirect(),
	)
	t.Cleanup(func() { _ = f.api.TLS.Stop(context.Background()) })
	if err := f.api.TLS.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	code, out := f.call(t, "GET", "/api/settings/tls", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("read: %d %s", code, out)
	}
	var view tlsPayload
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.State.Console {
		t.Fatal("nothing installed: the console door is shut")
	}

	code, out = f.call(t, "POST", "/api/certificates/self-signed",
		`{"names":["localhost"],"console":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("generate: %d %s", code, out)
	}
	var made certView
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		t.Fatal(err)
	}
	// localhost is reached by name AND by address depending on who types it.
	if len(made.Info.IPAddresses) == 0 {
		t.Fatalf("localhost must carry 127.0.0.1 too: %+v", made.Info)
	}

	_, out = f.call(t, "GET", "/api/settings/tls", "", f.rootC)
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if !view.State.Console {
		t.Fatalf("having a certificate IS the activation: %+v", view.State)
	}

	if code, out := f.call(t, "DELETE", "/api/certificates/"+made.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, out)
	}
	_, out = f.call(t, "GET", "/api/settings/tls", "", f.rootC)
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.State.Console {
		t.Fatal("deleting the certificate is what closes the door")
	}
}

// TestPlacementSaysWhoIsInTheWay: two certificates answering for the same
// name on one door is a question nobody was asked. The placement names the
// one in the way, and replacing it is a word the caller says.
func TestPlacementSaysWhoIsInTheWay(t *testing.T) {
	f := setup(t)
	generate := func(body string) certView {
		t.Helper()
		code, out := f.call(t, "POST", "/api/certificates/self-signed", body, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("generate %s: %d %s", body, code, out)
		}
		var v certView
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	old := generate(`{"names":["shop.example.com","www.example.com"],"app":true}`)
	spare := generate(`{"names":["shop.example.com"]}`)
	if spare.Console || spare.App {
		t.Fatal("a certificate made without a placement waits in reserve")
	}

	code, out := f.call(t, "PUT", "/api/certificates/"+spare.ID+"/placement", `{"app":true}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "shop.example.com on the application") ||
		!strings.Contains(out, old.ID) {
		t.Fatalf("placing over another must name it: %d %s", code, out)
	}
	// The console is another door: nothing is in the way there.
	if code, out := f.call(t, "PUT", "/api/certificates/"+spare.ID+"/placement", `{"console":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("console: %d %s", code, out)
	}
	code, out = f.call(t, "PUT", "/api/certificates/"+spare.ID+"/placement",
		`{"console":true,"app":true,"replace":true}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("replace: %d %s", code, out)
	}
	back, err := f.api.st.FindCertificate(context.Background(), old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.App {
		t.Fatal("the replaced certificate leaves the door")
	}

	// A request still waiting has nothing to present.
	code, out = f.call(t, "POST", "/api/certificates/signing-request", `{"names":["later.example.com"]}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("signing request: %d %s", code, out)
	}
	var pending certView
	if err := json.Unmarshal([]byte(out), &pending); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "PUT", "/api/certificates/"+pending.ID+"/placement", `{"app":true}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("a pending request placed: %d %s", code, out)
	}
}

// TestAGenerationNamesTheNameItRefuses: a certificate answers for names,
// and the one that is not a name is the one the refusal points at.
func TestAGenerationNamesTheNameItRefuses(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "POST", "/api/certificates/self-signed", `{"names":["ok.example.com","not a host"]}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, `not a host`) {
		t.Fatalf("a bad name: %d %s", code, out)
	}
	code, out = f.call(t, "POST", "/api/certificates/self-signed", `{"names":[]}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "at least one") {
		t.Fatalf("no name: %d %s", code, out)
	}
	code, out = f.call(t, "POST", "/api/certificates/self-signed", `{"names":["*.apps.example.com","10.0.0.7"]}`, f.rootC)
	if code != http.StatusCreated || !strings.Contains(out, "*.apps.example.com") || !strings.Contains(out, "10.0.0.7") {
		t.Fatalf("a wildcard and an address: %d %s", code, out)
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

// The gateway-wide HSTS (SSL-06) is kept, and a lifetime past two years - or
// below zero - is refused by a sentence that says what is allowed: a typo in a
// promise browsers keep for its whole length is not one to store.
func TestHSTSIsKeptAndBounded(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "PUT", "/api/settings/tls",
		`{"hstsMaxAge":86400}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(out, `"hstsMaxAge":86400`) {
		t.Fatalf("a day: %d %s", code, out)
	}
	for _, bad := range []string{"-1", "999999999"} {
		code, out = f.call(t, "PUT", "/api/settings/tls",
			`{"hstsMaxAge":`+bad+`}`, f.rootC)
		if code != http.StatusUnprocessableEntity || !strings.Contains(out, "two years") {
			t.Fatalf("hstsMaxAge %s: %d %s", bad, code, out)
		}
	}
}

// TestAnOrderToTheAuthorityLivesInThePool: asked like a certificate is made,
// placed like one, in conflict like one, deleted like one. An authority
// still asked cannot be deleted.
func TestAnOrderToTheAuthorityLivesInThePool(t *testing.T) {
	f := setupEE(t)
	ctx := context.Background()
	if code, out := f.call(t, "POST", "/api/certificates/acme", `{"authority":"nobody","names":["shop.example.com"]}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "no authority") {
		t.Fatalf("an order with no authority: %d %s", code, out)
	}
	ca := authority(t, f, `{"provider":"custom","name":"Step","directoryUrl":"https://step-ca.internal/acme/directory","acceptTos":true}`)
	if code, out := f.call(t, "POST", "/api/certificates/acme", `{"authority":"`+ca.ID+`","names":["*.example.com"]}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "wildcard") {
		t.Fatalf("a wildcard order: %d %s", code, out)
	}
	code, out := f.call(t, "POST", "/api/certificates/acme", `{"authority":"`+ca.ID+`","names":["shop.example.com","www.example.com"],"app":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("order: %d %s", code, out)
	}
	var order certView
	if err := json.Unmarshal([]byte(out), &order); err != nil {
		t.Fatal(err)
	}
	if order.Source != store.CertSourceACME || !order.App || !order.Waiting || order.AuthorityName != "Step" {
		t.Fatalf("an order placed and not issued yet, naming its authority: %s", out)
	}
	if got := f.api.st.TLSSettings(ctx).ACME.DomainsOn(store.PlaneApp); len(got) != 2 {
		t.Fatalf("placed on the application, its names are asked there: %v", got)
	}
	if code, out := f.call(t, "DELETE", "/api/acme/authorities/"+ca.ID, "", f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "shop.example.com") {
		t.Fatalf("an authority still asked: %d %s", code, out)
	}

	// A certificate made by hand for one of the same names: the order is named.
	code, out = f.call(t, "POST", "/api/certificates/self-signed", `{"names":["shop.example.com"],"app":true}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, order.ID) {
		t.Fatalf("over an order: %d %s", code, out)
	}
	code, out = f.call(t, "POST", "/api/certificates/self-signed", `{"names":["shop.example.com"],"app":true,"replace":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("replace the order: %d %s", code, out)
	}
	if got := f.api.st.TLSSettings(ctx).ACME.DomainsOn(store.PlaneApp); len(got) != 0 {
		t.Fatalf("the replaced order leaves the door: %v", got)
	}
	if code, out := f.call(t, "POST", "/api/certificates/"+order.ID+"/retry", "", f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "not placed") {
		t.Fatalf("retry an order on no door: %d %s", code, out)
	}

	// The list holds both kinds; the TLS settings never carry the orders.
	_, out = f.call(t, "GET", "/api/certificates", "", f.rootC)
	if !strings.Contains(out, `"source":"acme"`) || !strings.Contains(out, `"source":"self-signed"`) {
		t.Fatalf("the pool lists both kinds: %s", out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls", `{"redirect":false}`, f.rootC); code != http.StatusOK || strings.Contains(out, "orders") {
		t.Fatalf("save the settings: %d %s", code, out)
	}
	if len(f.api.st.RawTLS(ctx).ACME.Orders) != 1 {
		t.Fatal("saving the settings must keep the orders")
	}
	if code, out := f.call(t, "DELETE", "/api/certificates/"+order.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete the order: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/acme/authorities/"+ca.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("an authority nobody asks: %d %s", code, out)
	}
}

// Where the console opens a route, and what a vendor is told to register as
// a callback: by a name the gateway can vouch for, never the console's own
// address when one exists - a name declared behind a proxy on the
// Application door first, then a name a certificate placed there carries.
func TestTheDataOriginIsANameTheGatewayVouchesFor(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.api.TLS = certs.NewSupervisor(f.api.st, isNoRows,
		certs.NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewRedirect(),
	)
	t.Cleanup(func() { _ = f.api.TLS.Stop(ctx) })
	origin := func() string {
		t.Helper()
		code, out := f.call(t, "GET", "/api/edition", "", f.rootC)
		if code != http.StatusOK {
			t.Fatalf("edition: %d %s", code, out)
		}
		var e struct{ DataOrigin string }
		if err := json.Unmarshal([]byte(out), &e); err != nil {
			t.Fatal(err)
		}
		return e.DataOrigin
	}
	// Nothing vouched for: the console's host, the data plane's port.
	if got := origin(); strings.HasPrefix(got, "https://shop") {
		t.Fatalf("no name declared yet, and the origin is %q", got)
	}
	// Behind a proxy: the first name declared, over HTTPS.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["shop.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("proxied: %d %s", code, out)
	}
	if got := origin(); got != "https://shop.example.org" {
		t.Fatalf("a name behind a proxy, and the origin is %q", got)
	}
	// The other way: the names off, a certificate placed - its name.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"names":[]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("clearing: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/certificates/self-signed", `{"names":["*.example.com","www.example.com"],"app":true}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("generate: %d %s", code, out)
	}
	if got := origin(); !strings.HasPrefix(got, "https://www.example.com") {
		t.Fatalf("a certificate on the application, and the origin is %q", got)
	}
}

// tlsSupervisor gives the fixture a supervisor on throwaway ports, as main
// wires it: the runtime names come from the API.
func tlsSupervisor(t *testing.T, f fixture) {
	t.Helper()
	f.api.TLS = certs.NewSupervisor(f.api.st, isNoRows,
		certs.NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil),
		certs.NewRedirect(),
	)
	f.api.TLS.RuntimeNames = f.api.PlaneNames
	t.Cleanup(func() { _ = f.api.TLS.Stop(context.Background()) })
}

// A door's public names (SSL-14): stored per plane, cleaned, refused when
// they are not a name the operator can vouch for, audited with what changed,
// and loaded by the supervisor for the sign-in pages to ask.
func TestADoorIsDeclaredPerPlane(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tlsSupervisor(t, f)

	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["shop.example.org"]}`, f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d %s", code, out)
	}
	for _, bad := range []struct{ name, says string }{
		{"https://shop.example.org", "scheme"},
		{"shop.example.org:443", "port"},
		{"10.0.0.7", "IP address"},
		{"[::1]", "IP address"},
		{"shop.example.org/reset", "path"},
		{"shop_example.org", "not a host name"},
		{"*.example.org", "no wildcard"},
		{"a.*.example.org", "no wildcard"},
	} {
		code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["`+bad.name+`"]}`, f.rootC)
		if code != http.StatusUnprocessableEntity || !strings.Contains(out, bad.says) {
			t.Errorf("%s: %d %s, want 422 naming the %s", bad.name, code, out, bad.says)
		}
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/internal", `{"proxied":true,"names":["meerkat.example.org"]}`, f.rootC); code != http.StatusNotFound {
		t.Fatalf("the internal plane has no public names: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"proxyAddresses":["a proxy"]}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "proxy addresses") {
		t.Fatalf("a bad proxy address: %d %s", code, out)
	}

	code, out := f.call(t, "PUT", "/api/settings/tls/doors/app",
		`{"proxied":true,"names":["Shop.Example.org.", "shop.example.org", " EU.Shop.example.org ", ""],"proxyAddresses":["203.0.113.10"]}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("save: %d %s", code, out)
	}
	var view tlsPayload
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if strings.Join(view.Doors.App.Names, " ") != "shop.example.org eu.shop.example.org" || !view.Doors.App.Proxied ||
		len(view.Doors.Console.Names) != 0 || strings.Join(view.Doors.App.ProxyAddresses, " ") != "203.0.113.10/32" {
		t.Fatalf("lowercased, without the dot, once each, on the application only: %+v", view.Doors)
	}
	if !f.api.TLS.Proxied(certs.PlaneApp)("eu.shop.example.org") || f.api.TLS.Proxied(certs.PlaneConsole)("shop.example.org") {
		t.Fatal("saving is applying: the supervisor answers for the application's names, and only there")
	}
	// A name reaches one plane.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/console", `{"proxied":true,"names":["shop.example.org"]}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "application's") {
		t.Fatalf("a name on both planes: %d %s", code, out)
	}

	events, err := f.api.st.ListAuditEvents(ctx, store.AuditFilter{Target: "settings"})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range events {
		if e.Action == "tls.door" && e.TargetID == store.PlaneApp {
			for _, c := range e.Changes {
				if c.Field == "names" && strings.Contains(jsonOf(t, c.To), "shop.example.org") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("no tls.door event with the names before and after: %+v", events)
	}
	// Nothing placed, nothing opened: the names leave the doors and the
	// redirect alone.
	if code, out := f.call(t, "PUT", "/api/settings/tls", `{"redirect":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("redirect: %d %s", code, out)
	}
	if len(f.api.st.RawTLS(ctx).AppDoor.Names) != 2 {
		t.Fatal("saving the redirect dropped the door")
	}
	if f.api.TLS.State().Redirecting || f.api.TLS.State().App {
		t.Fatalf("a proxied name opened a door or a redirect: %+v", f.api.TLS.State())
	}
}

// Served by this gateway, every declared name has a certificate placed on the
// plane that covers it; behind a proxy, a public certificate stays placed but
// unused, and a certificate carrying the plane's runtime names serves the hop.
func TestADoorServesWhatItsModeFits(t *testing.T) {
	f := setup(t)
	tlsSupervisor(t, f)
	f.api.TLS.Ports("127.0.0.1:18080", "127.0.0.1:19090", "127.0.0.1:17070")
	f.api.LookupInternal = func(_ context.Context, port int) discovery.Internal {
		if port == 18080 {
			return discovery.Internal{Names: []string{"meerkat.canopy.svc.cluster.local", "meerkat.canopy.svc", "meerkat"}, Source: "kubernetes"}
		}
		return discovery.Internal{}
	}

	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"names":["shop.example.org"]}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "no certificate placed on the application covers shop.example.org") {
		t.Fatalf("a name the gateway cannot present: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/certificates/self-signed", `{"names":["*.example.org"],"app":true}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("public certificate: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"names":["shop.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("covered by the wildcard certificate: %d %s", code, out)
	}
	if !f.api.TLS.App.Answers("shop.example.org") {
		t.Fatal("the gateway serves its public name")
	}

	// Behind a proxy now: the public certificate stays placed, unused; a
	// cluster certificate is placed beside it and serves the hop.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["shop.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("proxied: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/certificates/self-signed", `{"names":["meerkat.canopy.svc"],"app":true}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("a cluster certificate on a proxied plane: %d %s", code, out)
	}
	if !f.api.TLS.App.Answers("meerkat.canopy.svc") || f.api.TLS.App.Answers("shop.example.org") {
		t.Fatal("behind a proxy the plane serves the cluster certificate, not the public one")
	}
	_, out := f.call(t, "GET", "/api/certificates", "", f.rootC)
	var list []certView
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatal(err)
	}
	unused := map[string][]string{}
	for _, c := range list {
		unused[c.Name()] = c.Unused
	}
	if strings.Join(unused["*.example.org"], ",") != store.PlaneApp || len(unused["meerkat.canopy.svc"]) != 0 {
		t.Fatalf("the public certificate is the unused one: %v", unused)
	}
	// And back: the public one serves again, the cluster one waits.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"names":["shop.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("back to this gateway: %d %s", code, out)
	}
	if !f.api.TLS.App.Answers("shop.example.org") {
		t.Fatal("switching back brings the public certificate back")
	}
}

// The console's door is the one the operator stands in. Reached through a
// proxy that says the browser's hop is HTTPS, a door that would stop
// believing it is refused: it would see this very session as plain http and
// sign the operator out (2026-10-15, on the eval).
func TestTheConsoleDoorNeverLocksOutWhoChangesIt(t *testing.T) {
	f := setup(t)
	tlsSupervisor(t, f)
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/console", `{"proxied":true,"names":["admin.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("behind a proxy: %d %s", code, out)
	}
	viaProxy := func(body string) (int, string) {
		req, err := http.NewRequest("PUT", f.adminSrv.URL+"/api/settings/tls/doors/console", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-Proto", "https")
		// Over HTTPS the session cookie is the __Host- one.
		req.AddCookie(&http.Cookie{Name: session.SecurePrefix + f.rootC.Name, Value: f.rootC.Value})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res.StatusCode, string(b)
	}
	if code, out := viaProxy(`{"names":["admin.example.org"]}`); code != http.StatusConflict || !strings.Contains(out, "sign you out") {
		t.Fatalf("served by Meerkat, through the proxy: %d %s", code, out)
	}
	if code, out := viaProxy(`{"proxied":true,"names":["admin.example.org"],"proxyAddresses":["203.0.113.0/24"]}`); code != http.StatusConflict {
		t.Fatalf("a proxy named elsewhere than the one in front: %d %s", code, out)
	}
	if code, out := viaProxy(`{"proxied":true,"names":["admin.example.org"],"proxyAddresses":["127.0.0.1"]}`); code != http.StatusOK {
		t.Fatalf("the proxy in front, named: %d %s", code, out)
	}
	// Not over HTTPS, nothing to lose: the switch goes through.
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/console", `{"names":["admin.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("plain http: %d %s", code, out)
	}
}

// A plane's plain door (SSL-15): HTTP goes off only while HTTPS serves; on
// the console, never the door in use. And a plane's last HTTPS certificate is
// not removed while its HTTP is off, nor, on the console, while HTTPS is in
// use.
func TestAPlaneKeepsADoorItIsReachedBy(t *testing.T) {
	f := setup(t)
	tlsSupervisor(t, f)
	f.api.DoorTraffic = certs.NewTraffic()
	put := func(plane, body string) (int, string) {
		return f.call(t, "PUT", "/api/settings/tls/ports/"+plane, body, f.rootC)
	}
	if code, out := put("app", `{"http":false}`); code != http.StatusUnprocessableEntity || !strings.Contains(out, "no certificate serves HTTPS") {
		t.Fatalf("HTTP off with nothing on HTTPS: %d %s", code, out)
	}
	if code, out := put("app", `{"http":true,"https":false}`); code != http.StatusBadRequest {
		t.Fatalf("HTTPS has no switch: %d %s", code, out)
	}
	if code, out := put("plug", `{"http":true}`); code != http.StatusNotFound {
		t.Fatalf("an unknown plane: %d %s", code, out)
	}
	code, out := f.call(t, "POST", "/api/certificates/self-signed", `{"names":["shop.example.org"],"app":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("application certificate: %d %s", code, out)
	}
	var appCert struct{ ID string }
	_ = json.Unmarshal([]byte(out), &appCert)
	if code, out := put("app", `{"http":false}`); code != http.StatusOK || !strings.Contains(out, `"app":["http"]`) {
		t.Fatalf("the application's HTTP off: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/certificates/"+appCert.ID, "", f.rootC); code != http.StatusConflict || !strings.Contains(out, "switch HTTP on first") {
		t.Fatalf("the last certificate with HTTP off: %d %s", code, out)
	}
	if code, out := put("app", `{"http":true}`); code != http.StatusOK || !strings.Contains(out, `"closed":{}`) {
		t.Fatalf("back on: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/certificates/"+appCert.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("with HTTP on, the last certificate goes: %d %s", code, out)
	}

	// The console, reached here over plain HTTP: its HTTP is in use.
	code, out = f.call(t, "POST", "/api/certificates/self-signed", `{"names":["localhost"],"console":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("console certificate: %d %s", code, out)
	}
	var consoleCert struct{ ID string }
	_ = json.Unmarshal([]byte(out), &consoleCert)
	if code, out := put("console", `{"http":false}`); code != http.StatusConflict || !strings.Contains(out, "access the console through HTTPS") {
		t.Fatalf("the door in use: %d %s", code, out)
	}
	// A proxy calls the console's HTTPS door: its last certificate stays.
	seen := httptest.NewRequest("GET", "/api/me", nil)
	seen.RemoteAddr, seen.TLS = "10.42.1.178:40000", &tls.ConnectionState{}
	f.api.DoorTraffic.Wrap(store.PlaneConsole, http.NotFoundHandler()).ServeHTTP(httptest.NewRecorder(), seen)
	if code, out := f.call(t, "DELETE", "/api/certificates/"+consoleCert.ID, "", f.rootC); code != http.StatusConflict || !strings.Contains(out, "from 10.42.1.178") {
		t.Fatalf("the console's last certificate while HTTPS is in use: %d %s", code, out)
	}
	if out := func() string { _, o := f.call(t, "GET", "/api/settings/tls", "", f.rootC); return o }(); !strings.Contains(out, `"peer":"10.42.1.178"`) || !strings.Contains(out, `"trafficWindow":30`) {
		t.Fatalf("the console's traffic is shown: %s", out)
	}
}
