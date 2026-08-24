package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
)

func certStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
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

func row(t *testing.T, id, name, host string) Certificate {
	m := material(t, host)
	return Certificate{
		ID: id, Name: name, Source: CertSourceImport,
		CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info,
	}
}

// TestPrivateKeysAreSealedAtRest is the one Archway got wrong in the other
// direction: it kept the keystore password in clear beside the keystore. A
// stolen database file must not be a stolen private key.
func TestPrivateKeysAreSealedAtRest(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	c := row(t, "c1", "app", "app.example.com")
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

func TestOnlyOneCertificateIsTheFallback(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	a, b := row(t, "a", "first", "a.example.com"), row(t, "b", "second", "b.example.com")
	a.Default = true
	if err := st.SaveCertificate(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCertificate(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDefaultCertificate(ctx, "b"); err != nil {
		t.Fatalf("SetDefaultCertificate: %v", err)
	}
	list, err := st.ListCertificates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var defaults []string
	for _, c := range list {
		if c.Default {
			defaults = append(defaults, c.ID)
		}
	}
	if len(defaults) != 1 || defaults[0] != "b" {
		t.Fatalf("defaults = %v, want exactly [b]", defaults)
	}
	// The default sorts first: it is the one an operator looks for.
	if list[0].ID != "b" {
		t.Fatalf("the fallback must head the list, got %q", list[0].ID)
	}
}

func TestASigningRequestServesNothingAndCannotBeTheFallback(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	csrPEM, keyPEM, err := certs.NewCSR(certs.Request{DNSNames: []string{"pending.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	pending := Certificate{ID: "p", Name: "pending", Source: CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM}
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
	err = st.SetDefaultCertificate(ctx, "p")
	if err == nil || !strings.Contains(err.Error(), "no certificate to present") {
		t.Fatalf("a pending request cannot be the fallback, and the refusal must say why: %v", err)
	}
	list, _, _, err := st.CertificateMaterials(ctx)
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
	good := row(t, "good", "good", "good.example.com")
	if err := st.SaveCertificate(ctx, good); err != nil {
		t.Fatal(err)
	}
	bad := row(t, "bad", "corrupted", "bad.example.com")
	bad.CertPEM = "-----BEGIN CERTIFICATE-----\nnot base64 at all\n-----END CERTIFICATE-----\n"
	if err := st.SaveCertificate(ctx, bad); err != nil {
		t.Fatal(err)
	}
	list, fallback, problems, err := st.CertificateMaterials(ctx)
	if err != nil {
		t.Fatalf("one broken row must not fail the whole read: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("materials = %d, want the one good certificate", len(list))
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "corrupted") {
		t.Fatalf("the broken one must be named: %v", problems)
	}
	// A single certificate is obviously the fallback: asking an operator to
	// tick a box is asking a question with one answer.
	if fallback == nil {
		t.Fatal("with exactly one certificate installed, that one answers a handshake carrying no name")
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
	c := row(t, "x", "x", "x.example.com")
	c.Source = "wherever"
	err := st.SaveCertificate(ctx, c)
	if err == nil || !strings.Contains(err.Error(), CertSourceImport) {
		t.Fatalf("the error must list the allowed sources: %v", err)
	}
}
