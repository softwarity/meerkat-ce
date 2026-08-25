package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func row(t *testing.T, id, host string) Certificate {
	m := material(t, host)
	return Certificate{
		ID: id, Plane: PlaneApp, Host: host, Source: CertSourceImport,
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
		ID: "p", Plane: PlaneApp, Host: "pending.example.com",
		Source: CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM,
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
	// Something is always presented to a client that sends no name: the first
	// entry of the plane. There is no star to tick, because with one
	// certificate per host there is nothing to choose between.
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

	c = row(t, "y", "y.example.com")
	c.Plane = "somewhere"
	if err := st.SaveCertificate(ctx, c); err == nil || !strings.Contains(err.Error(), PlaneConsole) {
		t.Fatalf("the error must list the allowed planes: %v", err)
	}

	// A certificate belongs to the name it answers for: without one there is
	// nothing to file it under.
	c = row(t, "z", "z.example.com")
	c.Host = ""
	if err := st.SaveCertificate(ctx, c); err == nil || !strings.Contains(err.Error(), "needs a host") {
		t.Fatalf("a certificate with no host must be refused: %v", err)
	}
}

// TestTheTwoPlanesDoNotShare is the simplification made testable: the same
// material used by both is two entries, and each plane's door only ever sees
// its own.
func TestTheTwoPlanesDoNotShare(t *testing.T) {
	st := certStore(t)
	ctx := context.Background()
	m := material(t, "localhost")
	for i, plane := range []string{PlaneConsole, PlaneApp} {
		c := Certificate{
			ID: fmt.Sprintf("c%d", i), Plane: plane, Host: "localhost",
			Source: CertSourceImport, CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info,
		}
		if err := st.SaveCertificate(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, plane := range []string{PlaneConsole, PlaneApp} {
		list, fallback, _, err := st.CertificateMaterials(ctx, plane)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || fallback == nil {
			t.Fatalf("%s: got %d materials", plane, len(list))
		}
	}
}

// TestBothPlanesAreSeededWithLocalhostOnce covers the case that actually bit:
// an installation that already had TLS settings, saved before names existed.
// The seed has to reach it - and then never come back once a name is removed.
func TestBothPlanesAreSeededWithLocalhostOnce(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if got := st.RawTLS(ctx); got.ConsoleName != DefaultConsoleName || len(got.AppNames) != 1 {
		t.Fatalf("a fresh installation starts with both names: %+v", got)
	}

	// An operator clears the application's names, and closes the gateway.
	cfg := st.RawTLS(ctx)
	cfg.AppNames = nil
	if err := st.SetSetting(ctx, SettingTLS, cfg); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening must NOT put it back: a name removed stays removed.
	st, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if got := st.RawTLS(ctx); len(got.AppNames) != 0 {
		t.Fatalf("the seed came back and undid a deliberate removal: %+v", got)
	}
}

// TestAnInstallationThatPredatesNamesGetsThem is the one that failed in
// practice: the tls setting existed, with no names in it, so a seed keyed on
// "is the setting absent" never fired and the screen opened empty.
func TestAnInstallationThatPredatesNamesGetsThem(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Rewind to what such an installation looks like: settings saved, no names,
	// and no marker.
	if err := st.SetSetting(ctx, SettingTLS, certs.Settings{Redirect: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`DELETE FROM settings WHERE key = ?`, SettingTLSSeeded); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	got := st.RawTLS(ctx)
	if got.ConsoleName != DefaultConsoleName || len(got.AppNames) != 1 {
		t.Fatalf("an installation that predates names must receive them: %+v", got)
	}
	if !got.Redirect {
		t.Fatal("and keep what it had already set")
	}
}
