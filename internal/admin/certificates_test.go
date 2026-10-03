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
	f := setup(t)
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

// TestAnAuthorityIsRefusedAtSaveTimeRatherThanAtRenewalTime: what is missing
// is said when the authority is saved, naming where to find it.
func TestAnAuthorityIsRefusedAtSaveTimeRatherThanAtRenewalTime(t *testing.T) {
	f := setup(t)
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
	f := setup(t)
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
	f := setup(t)
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
