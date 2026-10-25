package admin

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/license"
)

func signedLicense(t *testing.T, key ed25519.PrivateKey, licensee string) string {
	t.Helper()
	data, err := license.Sign(license.License{ID: "LIC-1", Licensee: licensee, Plan: "team",
		IssuedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().AddDate(1, 0, 0)}, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func licenseBody(t *testing.T, text string) string {
	t.Helper()
	b, err := json.Marshal(licensePayload{License: text})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The community image takes no license: there is nothing in it one would
// license.
func TestLicenseIsRefusedOnTheCommunityImage(t *testing.T) {
	if edition.Enterprise {
		t.Skip("enterprise build")
	}
	f := setupBare(t)
	if code, out := f.call(t, "PUT", "/api/license", `{"license": "x"}`, f.rootC); code != http.StatusForbidden ||
		!strings.Contains(out, "Community edition") {
		t.Fatalf("license on the community image: %d %s", code, out)
	}
}

// A license is pasted by root, verified before it is kept, shown by what it
// says rather than by its file, and refused with the reason a person can act on.
func TestLicensePastedIsVerifiedThenShown(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, other, _ := ed25519.GenerateKey(rand.Reader)

	// A build with no signing key verifies nothing, and says so.
	f.api.LicenseKeys = []ed25519.PublicKey{}
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, signedLicense(t, priv, "Acme")), f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(out, "no Softwarity signing key") {
		t.Fatalf("no key in the build: %d %s", code, out)
	}
	f.api.LicenseKeys = []ed25519.PublicKey{pub}

	for _, c := range []struct{ name, text, want string }{
		{"empty", "  ", "no license given"},
		{"not a file", "hello", "not a license file"},
		{"someone else's", signedLicense(t, other, "Acme"), "signature is not Softwarity's"},
	} {
		if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, c.text), f.rootC); code != http.StatusUnprocessableEntity ||
			!strings.Contains(out, c.want) {
			t.Errorf("%s: %d %s", c.name, code, out)
		}
	}
	good := signedLicense(t, priv, "Acme")
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, good), f.plainC); code != http.StatusForbidden {
		t.Fatalf("a non-root pasted a license: %d %s", code, out)
	}
	code, out := f.call(t, "PUT", "/api/license", licenseBody(t, "\n"+good+"\n"), f.rootC)
	if code != http.StatusOK {
		t.Fatalf("valid license: %d %s", code, out)
	}
	var e editionInfo
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatal(err)
	}
	if e.License == nil || e.License.Licensee != "Acme" || e.License.ID != "LIC-1" || e.LicenseSource != "console" {
		t.Fatalf("the license is not shown: %s", out)
	}
	if strings.Contains(out, "signature") {
		t.Fatalf("the file went back to the browser: %s", out)
	}
}

// A license mounted from a Secret is the deployment's: shown, read again on
// every look, and the console takes no other.
func TestLicenseFromTheFileWins(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	path := filepath.Join(t.TempDir(), "license")
	if err := os.WriteFile(path, []byte(signedLicense(t, priv, "Mounted Co")), 0o600); err != nil {
		t.Fatal(err)
	}
	f.api.LicenseFile = path

	_, out := f.call(t, "GET", "/api/edition", "", f.rootC)
	if !strings.Contains(out, `"licensee":"Mounted Co"`) || !strings.Contains(out, `"licenseSource":"file"`) {
		t.Fatalf("the mounted license is not shown: %s", out)
	}
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, signedLicense(t, priv, "Other")), f.rootC); code != http.StatusConflict ||
		!strings.Contains(out, "MEERKAT_LICENSE_FILE") {
		t.Fatalf("a paste over a mounted license: %d %s", code, out)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, out := f.call(t, "GET", "/api/edition", "", f.rootC); !strings.Contains(out, "is refused") {
		t.Fatalf("a broken mounted license is not said: %s", out)
	}
}
