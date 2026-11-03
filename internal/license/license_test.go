package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func testLicense() License {
	return License{
		ID:        "LIC-1",
		Licensee:  "ACME Corp",
		Edition:   EditionTeam,
		Tenants:   1,
		Instances: 1,
		IssuedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestParseValid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, err := Sign(testLicense(), priv)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	lic, err := Parse(data, []ed25519.PublicKey{pub}, now)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if lic.ID != "LIC-1" || lic.Licensee != "ACME Corp" || lic.Edition != EditionTeam || lic.Tenants != 1 || lic.Instances != 1 {
		t.Fatalf("unexpected payload: %+v", lic)
	}
}

func TestParseRejectsWrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	_, err := Parse(data, []ed25519.PublicKey{otherPub}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrSignature) {
		t.Fatalf("want ErrSignature, got %v", err)
	}
}

func TestParseRejectsTampering(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	data[len(fileHeader)+40] ^= 0x01
	if _, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("tampered license accepted")
	}
}

// A perpetual fallback: long past its date a license still validates and still
// names its edition, and whoever paid keeps the builds from the term - cutting
// authentication over a lapsed subscription would make this undeployable in
// front of a production.
func TestParseStaysValidPastExpiry(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	lic, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("a perpetual license must survive its expiry date: %v", err)
	}
	if lic.Edition != EditionTeam {
		t.Fatal("the edition must still be carried")
	}
	// What the date DOES say: how far updates were paid for. A build from
	// before it is covered, one from after is not.
	if !lic.Covered(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a build from the term must be covered")
	}
	if lic.Covered(time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a build from after the term must be reported as uncovered")
	}
	if lic.Ended(time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a license that is sold never ends")
	}
}

// A file cannot be revoked, so a test license ends at its date: it says so in
// what is signed, and a copy with the line taken out does not verify.
func TestATestLicenseEndsAtItsDate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	sample := testLicense()
	sample.Expires = ExpiresHard
	data, err := Sign(sample, priv)
	if err != nil {
		t.Fatal(err)
	}
	lic, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("a test license past its date must still be read, to say it ended: %v", err)
	}
	if lic.Ended(sample.ExpiresAt) {
		t.Fatal("it ends after its date, not on it")
	}
	if !lic.Ended(sample.ExpiresAt.AddDate(0, 0, 1)) {
		t.Fatal("a test license must end at its date")
	}
}

// The date is a day: everything built on it, up to midnight UTC, is covered, and
// a test license runs until then. The issuer writes noon so that the date reads
// the same in every country.
func TestTheLastDayCountsToItsEnd(t *testing.T) {
	lic := &License{ExpiresAt: time.Date(2027, 12, 31, 12, 0, 0, 0, time.UTC), Expires: ExpiresHard}
	evening := time.Date(2027, 12, 31, 23, 59, 59, 0, time.UTC)
	if !lic.Covered(evening) {
		t.Fatal("a build from the evening of the last day must be covered")
	}
	if lic.Ended(evening) {
		t.Fatal("a test license must run until the end of its last day")
	}
	midnight := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	if lic.Covered(midnight) || !lic.Ended(midnight) {
		t.Fatal("the next day is past the license")
	}
	// The same instant written in another zone is the same build.
	paris := time.FixedZone("CET", 3600)
	if !lic.Covered(time.Date(2028, 1, 1, 0, 30, 0, 0, paris)) {
		t.Fatal("00:30 in Paris is still the last day in UTC")
	}
}

func TestParseRejectsNotYetValid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	_, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNotYet) {
		t.Fatalf("want ErrNotYet, got %v", err)
	}
}

func TestParseRejectsNoKeys(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	if _, err := Parse(data, nil, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrSignature) {
		t.Fatalf("want ErrSignature, got %v", err)
	}
}

// The shape the issuer writes, by hand rather than through Sign: the 80-column
// body, the fields Meerkat does not read, and the copy's own expiry a month out,
// which must not end the license.
func TestParseReadsTheFileTheIssuerWrites(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	enc := base64.StdEncoding.EncodeToString([]byte(`{"data":{"id":"7fdff601","type":"licenses","attributes":{
		"name":"Test team","key":"030557-V3","expiry":"2027-10-31T17:00:00.000Z","status":"ACTIVE","metadata":{}},
		"relationships":{"product":{"data":{"type":"products","id":"` + ProductID + `"}}}},
		"included":[{"id":"70b1","type":"policies","attributes":{"name":"Team",
		"metadata":{"edition":"team","tenants":1,"instances":1}}}],
		"meta":{"issued":"2026-11-01T15:22:59.367Z","expiry":"2026-12-01T15:22:59.367Z","ttl":2629746}}`))
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("license/"+enc)))
	body := base64.StdEncoding.EncodeToString([]byte(`{"enc":"` + enc + `","sig":"` + sig + `","alg":"base64+ed25519"}`))
	var wrapped strings.Builder
	for len(body) > 60 {
		wrapped.WriteString(body[:60] + "\n")
		body = body[60:]
	}
	file := "-----BEGIN LICENSE FILE-----\n" + wrapped.String() + body + "\n-----END LICENSE FILE-----\n"

	lic, err := Parse([]byte(file), []ed25519.PublicKey{pub}, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("the issuer's file, past the expiry of its copy: %v", err)
	}
	if lic.ID != "7fdff601" || lic.Licensee != "Test team" || lic.Edition != EditionTeam || lic.Tenants != 1 ||
		lic.Instances != 1 || lic.ExpiresAt.Format(time.DateOnly) != "2027-10-31" {
		t.Fatalf("unexpected license: %+v", lic)
	}
}

func TestParseRefusesAnotherProduct(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, err := signFor(testLicense(), priv, "another-product")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrProduct) {
		t.Fatalf("want ErrProduct, got %v", err)
	}
}

// What the policy says is signed, and a mistake in it is refused rather than
// read as Enterprise.
func TestParseRefusesWhatAPolicyDoesNotSay(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	for name, change := range map[string]func(*License){
		"an unknown edition":     func(l *License) { l.Edition = "tean" },
		"no edition":             func(l *License) { l.Edition = "" },
		"Team without a ceiling": func(l *License) { l.Tenants = 0 },
		"no licensee":            func(l *License) { l.Licensee = "" },
		"an unknown expiry":      func(l *License) { l.Expires = "soon" },
	} {
		lic := testLicense()
		change(&lic)
		data, err := Sign(lic, priv)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(data, []ed25519.PublicKey{pub}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// Keygen's algorithm is asserted before the signature means anything.
func TestParseRefusesAnotherAlgorithm(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data, _ := Sign(testLicense(), priv)
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(data)), fileHeader), fileFooter)
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(inner))
	raw = []byte(strings.Replace(string(raw), algorithm, "aes-256-gcm+ed25519", 1))
	file := fileHeader + "\n" + base64.StdEncoding.EncodeToString(raw) + "\n" + fileFooter
	if _, err := Parse([]byte(file), []ed25519.PublicKey{pub}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)); err == nil ||
		!strings.Contains(err.Error(), "aes-256-gcm+ed25519") {
		t.Fatalf("got %v", err)
	}
}

// A machine a few hours behind keeps its license: it is read again every time.
func TestParseToleratesAClockBehind(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	lic := testLicense()
	data, _ := Sign(lic, priv)
	if _, err := Parse(data, []ed25519.PublicKey{pub}, lic.IssuedAt.Add(-6*time.Hour)); err != nil {
		t.Fatalf("six hours behind: %v", err)
	}
}

// The build carries the key of the account that issues Meerkat's licenses.
func TestTheBuildCarriesASigningKey(t *testing.T) {
	if keys := ProductionKeys(); len(keys) == 0 || len(keys[0]) != ed25519.PublicKeySize {
		t.Fatalf("keys: %v", keys)
	}
}
