package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func certStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func material(t *testing.T, host string) *certs.Material {
	t.Helper()
	m, err := certs.SelfSigned(certs.Request{DNSNames: []string{host}}, time.Now())
	if err != nil {
		t.Fatalf("SelfSigned: %v", err)
	}
	return m
}

func row(t *testing.T, id, host string) Certificate {
	m := material(t, host)
	return Certificate{
		ID: id, App: true, Source: CertSourceImport,
		CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info,
	}
}

// TestPrivateKeysAreSealedAtRest is the one Archway got wrong in the other
// direction: it kept the keystore password in clear beside the keystore. A
// stolen database file must not be a stolen private key.
func TestPrivateKeysAreSealedAtRest(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	c := row(t, "c1", "app.example.com")
	if err := st.SaveCertificate(ctx, c); err != nil {
		t.Fatalf("SaveCertificate: %v", err)
	}
	var stored string
	if err := st.db.QueryRow(`SELECT key_sealed FROM certificates WHERE id = 'c1'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "PRIVATE KEY") || stored == c.KeyPEM {
		t.Fatal("the private key is stored in clear")
	}
	back, err := st.FindCertificate(ctx, "c1")
	if err != nil {
		t.Fatalf("FindCertificate: %v", err)
	}
	if back.KeyPEM != c.KeyPEM {
		t.Fatal("the key did not survive the round trip")
	}
	// And the CERTIFICATE is not sealed: it is public material, handed to
	// every visitor, and a console must be able to show and download it.
	if !strings.Contains(back.CertPEM, "BEGIN CERTIFICATE") {
		t.Fatal("the certificate itself must stay readable")
	}
}

func TestASigningRequestServesNothing(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	csrPEM, keyPEM, err := certs.NewCSR(certs.Request{DNSNames: []string{"pending.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	pending := Certificate{
		ID: "p", Source: CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM,
	}
	if err := st.SaveCertificate(ctx, pending); err != nil {
		t.Fatal(err)
	}
	back, err := st.FindCertificate(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if !back.Pending() {
		t.Fatal("a row with a request and no certificate is pending")
	}
	list, _, _, err := st.CertificateMaterials(ctx, PlaneApp)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatal("a pending request must not reach the TLS manager: it has nothing to present")
	}
}

// TestOneBadCertificateDoesNotSinkTheOthers is the boot case that matters: a
// row that cannot be parsed must be named and left out, not take the whole
// HTTPS listener down with it.
func TestOneBadCertificateDoesNotSinkTheOthers(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	good := row(t, "good", "good.example.com")
	if err := st.SaveCertificate(ctx, good); err != nil {
		t.Fatal(err)
	}
	bad := row(t, "bad", "bad.example.com")
	bad.CertPEM = "-----BEGIN CERTIFICATE-----\nnot base64 at all\n-----END CERTIFICATE-----\n"
	if err := st.SaveCertificate(ctx, bad); err != nil {
		t.Fatal(err)
	}
	list, fallback, problems, err := st.CertificateMaterials(ctx, PlaneApp)
	if err != nil {
		t.Fatalf("one broken row must not fail the whole read: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("materials = %d, want the one good certificate", len(list))
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "bad.example.com") {
		t.Fatalf("the broken one must be named: %v", problems)
	}
	// Something is always presented to a client that sends no name: the
	// oldest certificate placed on the plane.
	if fallback == nil {
		t.Fatal("a client that sends no server name must still be answered")
	}
}

func TestDeletingACertificateIsNotSilentWhenItIsNotThere(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	if err := st.DeleteCertificate(ctx, "ghost"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleting nothing = %v, want ErrNoRows", err)
	}
}

func TestACMECacheIsSealedAndForgettable(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	// This is the account key that signs on the gateway's behalf, and the
	// private keys of every certificate an authority issued.
	if err := st.ACMECachePut(ctx, "acme_account+key", "top secret account key"); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := st.db.QueryRow(`SELECT value FROM acme_cache WHERE key = 'acme_account+key'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "top secret") {
		t.Fatal("the ACME state is stored in clear")
	}
	got, err := st.ACMECacheGet(ctx, "acme_account+key")
	if err != nil || got != "top secret account key" {
		t.Fatalf("round trip: %q %v", got, err)
	}
	if _, err := st.ACMECacheGet(ctx, "absent"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("absence must be ErrNoRows so the cache adapter can translate it: %v", err)
	}
	// Changing authority means starting again: an account registered at one
	// directory is worthless at another, and keeping it is how a renewal fails
	// months later with an error about an unknown account.
	if err := st.ForgetACME(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ACMECacheGet(ctx, "acme_account+key"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("ForgetACME left state behind: %v", err)
	}
}

func TestSaveCertificateNamesWhatItRefuses(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()

	c := row(t, "x", "x.example.com")
	c.Source = "wherever"
	if err := st.SaveCertificate(ctx, c); err == nil || !strings.Contains(err.Error(), CertSourceImport) {
		t.Fatalf("the error must list the allowed sources: %v", err)
	}

	// A signing request has nothing to present: placing it would open a door
	// on a certificate that does not exist yet.
	csrPEM, keyPEM, err := certs.NewCSR(certs.Request{DNSNames: []string{"p.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	pending := Certificate{ID: "p", App: true, Source: CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM}
	if err := st.SaveCertificate(ctx, pending); err == nil || !strings.Contains(err.Error(), "signing request") {
		t.Fatalf("a placed signing request must be refused: %v", err)
	}
}

// TestOnePieceOfMaterialServesBothPlanes is the pool made testable: one row,
// placed twice, reaches both doors; a row in reserve reaches neither.
func TestOnePieceOfMaterialServesBothPlanes(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	both := row(t, "both", "localhost")
	both.Console = true
	if err := st.SaveCertificate(ctx, both); err != nil {
		t.Fatal(err)
	}
	spare := row(t, "spare", "spare.example.com")
	spare.App = false
	if err := st.SaveCertificate(ctx, spare); err != nil {
		t.Fatal(err)
	}
	for _, plane := range []string{PlaneConsole, PlaneApp} {
		list, fallback, _, err := st.CertificateMaterials(ctx, plane)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || fallback == nil || list[0].Info.DNSNames[0] != "localhost" {
			t.Fatalf("%s: got %d materials, want the one placed there", plane, len(list))
		}
	}
	// Taking it off a plane closes that door and only that one.
	if err := st.PlaceCertificate(ctx, "both", false, true); err != nil {
		t.Fatal(err)
	}
	if list, _, _, _ := st.CertificateMaterials(ctx, PlaneConsole); len(list) != 0 {
		t.Fatal("off the console, the console presents nothing")
	}
	if err := st.PlaceCertificate(ctx, "nobody", true, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("placing what is not there must say so: %v", err)
	}
}

// TestTheOldestPlacedIsTheFallback: a client that sends no server name keeps
// receiving what it received, when a certificate is added next to it.
func TestTheOldestPlacedIsTheFallback(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	first := row(t, "b-first", "first.example.com")
	first.CreatedAt = 100
	second := row(t, "a-second", "second.example.com")
	second.CreatedAt = 200
	for _, c := range []Certificate{second, first} {
		if err := st.SaveCertificate(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	_, fallback, _, err := st.CertificateMaterials(ctx, PlaneApp)
	if err != nil {
		t.Fatal(err)
	}
	if fallback == nil || fallback.Info.DNSNames[0] != "first.example.com" {
		t.Fatalf("the fallback must be the oldest placed, got %v", fallback)
	}
}

// TestCertificatesOfANameBecomePlacedMaterial runs v73 on a database of the
// shape before it: one row per declared name, the material both planes
// shared imported twice. It comes out as the same material placed on both,
// once - the oldest row kept - and a request still waiting, in reserve.
func TestCertificatesOfANameBecomePlacedMaterial(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	shared := material(t, "localhost")
	only := material(t, "shop.example.com")
	csrPEM, csrKey, err := certs.NewCSR(certs.Request{DNSNames: []string{"later.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	seal := func(k string) string {
		out, err := st.vaultCipher.Seal(k)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	steps := []string{
		`DROP TABLE certificates`,
		`CREATE TABLE certificates (
		   id TEXT PRIMARY KEY, plane TEXT NOT NULL DEFAULT 'app', host TEXT NOT NULL DEFAULT '',
		   source TEXT NOT NULL DEFAULT 'import', cert_pem TEXT NOT NULL DEFAULT '',
		   key_sealed TEXT NOT NULL DEFAULT '', csr_pem TEXT NOT NULL DEFAULT '',
		   subject TEXT NOT NULL DEFAULT '', issuer TEXT NOT NULL DEFAULT '',
		   serial TEXT NOT NULL DEFAULT '', algo TEXT NOT NULL DEFAULT '',
		   key_type TEXT NOT NULL DEFAULT '', dns_names TEXT NOT NULL DEFAULT '[]',
		   ip_addresses TEXT NOT NULL DEFAULT '[]', chain BIGINT NOT NULL DEFAULT 0,
		   self_signed BOOLEAN NOT NULL DEFAULT FALSE, not_before BIGINT NOT NULL DEFAULT 0,
		   not_after BIGINT NOT NULL DEFAULT 0, created_at BIGINT NOT NULL DEFAULT 0,
		   updated_at BIGINT NOT NULL DEFAULT 0)`,
	}
	for _, q := range steps {
		if _, err := st.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(id, plane, host, cert, key, csr, dns string, at int64) {
		if _, err := st.db.Exec(`INSERT INTO certificates (id, plane, host, cert_pem, key_sealed, csr_pem, dns_names, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, plane, host, cert, seal(key), csr, dns, at); err != nil {
			t.Fatal(err)
		}
	}
	insert("console", "console", "localhost", shared.CertPEM, shared.KeyPEM, "", `["localhost"]`, 1)
	insert("app", "app", "localhost", shared.CertPEM, shared.KeyPEM, "", `["localhost"]`, 2)
	insert("shop", "app", "shop.example.com", only.CertPEM, only.KeyPEM, "", `["shop.example.com"]`, 3)
	insert("later", "app", "later.example.com", "", csrKey, csrPEM, `["later.example.com"]`, 4)
	if _, err := st.db.Exec(`INSERT INTO settings (key, value) VALUES ('tls_seeded', 'true')`); err != nil {
		t.Fatal(err)
	}
	if err := st.db.setSchemaVersion(72); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("the upgrade must open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	list, err := st.ListCertificates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Certificate{}
	for _, c := range list {
		got[c.ID] = c
	}
	if len(list) != 3 {
		t.Fatalf("the twice-imported material must become one row: %d rows", len(list))
	}
	if c := got["console"]; !c.Console || !c.App {
		t.Fatalf("the oldest copy is kept, placed on both planes: %+v", c)
	}
	if c := got["shop"]; c.Console || !c.App {
		t.Fatalf("a row keeps the plane it was filed under: %+v", c)
	}
	if c := got["later"]; c.Console || c.App || !c.Pending() {
		t.Fatalf("a request still waiting stays in reserve: %+v", c)
	}
	var marker int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'tls_seeded'`).Scan(&marker); err != nil || marker != 0 {
		t.Fatalf("the seed marker has nothing left to mark: %d %v", marker, err)
	}
}
