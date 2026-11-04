package admin

import (
	"context"
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
	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

func signedLicense(t *testing.T, key ed25519.PrivateKey, licensee string) string {
	t.Helper()
	return signedEdition(t, key, license.License{ID: "LIC-1", Licensee: licensee, Edition: license.EditionEnterprise})
}

func signedEdition(t *testing.T, key ed25519.PrivateKey, lic license.License) string {
	t.Helper()
	t.Cleanup(func() { license.Apply(nil) })
	lic.IssuedAt, lic.ExpiresAt = time.Now().Add(-time.Hour), time.Now().AddDate(1, 0, 0)
	data, err := license.Sign(lic, key)
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
	if code, out := f.call(t, "DELETE", "/api/license", "", f.rootC); code != http.StatusForbidden ||
		!strings.Contains(out, "Community edition") {
		t.Fatalf("license removed on the community image: %d %s", code, out)
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
	if code, out := f.call(t, "DELETE", "/api/license", "", f.rootC); code != http.StatusConflict ||
		!strings.Contains(out, "MEERKAT_LICENSE_FILE") {
		t.Fatalf("a mounted license removed from the console: %d %s", code, out)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, out := f.call(t, "GET", "/api/edition", "", f.rootC); !strings.Contains(out, "is refused") {
		t.Fatalf("a broken mounted license is not said: %s", out)
	}
}

// A Team license sets the ceilings as soon as it is saved: what is there stays
// served and is said, and nothing more is created.
func TestATeamLicenseSetsTheCeilings(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}

	second := `{"name": "Second"}`
	if code, out := f.call(t, "POST", "/api/tenants", second, f.rootC); code >= 400 && code != http.StatusConflict {
		t.Fatalf("no license, no ceiling: %d %s", code, out)
	}
	team := signedEdition(t, priv, license.License{ID: "LIC-T", Licensee: "Small Co",
		Edition: license.EditionTeam, Tenants: 1, Instances: 1})
	code, out := f.call(t, "PUT", "/api/license", licenseBody(t, team), f.rootC)
	if code != http.StatusOK {
		t.Fatalf("team license: %d %s", code, out)
	}
	if !strings.Contains(out, `"plan":"team"`) || !strings.Contains(out, `"tenants":1`) || !strings.Contains(out, `"instances":1`) {
		t.Fatalf("the ceilings are not shown: %s", out)
	}
	if code, out := f.call(t, "POST", "/api/tenants", `{"name": "Third"}`, f.rootC); code != http.StatusForbidden ||
		!strings.Contains(out, "Team license covers 1") {
		t.Fatalf("a tenant past the ceiling: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy": "multi"}`, f.rootC); f.api.st.Tenancy(context.Background()) == "single" &&
		code != http.StatusForbidden {
		t.Fatalf("several tenants on Team: %d %s", code, out)
	}

	enterprise := signedLicense(t, priv, "Grown Co")
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, enterprise), f.rootC); code != http.StatusOK {
		t.Fatalf("enterprise license: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/tenants", `{"name": "Third"}`, f.rootC); code == http.StatusForbidden {
		t.Fatalf("the ceiling outlived the Team license: %d %s", code, out)
	}
}

// Tenants created during an evaluation are not covered by a license for one:
// they stay served, the notice stays and the screen says what ends it. Going to
// a single tenant ends it, with nothing deleted, and there is no way back up.
func TestATeamLicenseDoesNotCoverTheTenantsOfAnEvaluation(t *testing.T) {
	f := setupEE(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	if code, out := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy": "multi"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("several tenants: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/tenants", `{"name": "Second"}`, f.rootC); code >= 400 {
		t.Fatalf("second tenant: %d %s", code, out)
	}
	before, err := f.api.st.CountTenants(ctx)
	if err != nil || before < 2 {
		t.Fatalf("%d tenants, %v", before, err)
	}

	team := signedEdition(t, priv, license.License{ID: "LIC-T", Licensee: "Small Co",
		Edition: license.EditionTeam, Tenants: 1, Instances: 1})
	t.Cleanup(func() { license.ObserveTenants(0) })
	code, out := f.call(t, "PUT", "/api/license", licenseBody(t, team), f.rootC)
	if code != http.StatusOK || !strings.Contains(out, "tenants are served") {
		t.Fatalf("the screen does not say it: %d %s", code, out)
	}
	if evalmark.Shown("mark") == "" {
		t.Fatal("a license for one tenant took the notice off several")
	}

	code, out = f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy": "single"}`, f.rootC)
	if code != http.StatusOK || strings.Contains(out, "tenants are served") {
		t.Fatalf("down to one tenant: %d %s", code, out)
	}
	if evalmark.Shown("mark") != "" {
		t.Fatal("one tenant served: the notice must go")
	}
	if after, err := f.api.st.CountTenants(ctx); err != nil || after != before {
		t.Fatalf("going down deleted tenants: %d, was %d (%v)", after, before, err)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy": "multi"}`, f.rootC); code != http.StatusForbidden {
		t.Fatalf("back up on Team: %d %s", code, out)
	}
}

// Removing the license is the way back to evaluation: root's, on every read at
// once, the notice returns, the ceilings go with it, and the trail keeps who and
// which.
// A version built after the updates ended is an evaluation again, ceilings
// included, and going back to one from the term puts the license back: nothing
// was written down, so there is nothing to undo.
func TestABuildPastTheUpdatesIsAnEvaluationForAsLongAsItRuns(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	dated := version.Date
	t.Cleanup(func() { version.Date = dated })

	team := signedEdition(t, priv, license.License{ID: "LIC-T", Licensee: "Small Co",
		Edition: license.EditionTeam, Tenants: 1, Instances: 1})
	version.Date = time.Now().UTC().Format(time.RFC3339)
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, team), f.rootC); code != http.StatusOK {
		t.Fatalf("team license: %d %s", code, out)
	}
	if license.Current() == nil || evalmark.Shown("mark") != "" {
		t.Fatal("a build from the term must honour the license")
	}

	version.Date = time.Now().UTC().AddDate(2, 0, 0).Format(time.RFC3339)
	code, out := f.call(t, "GET", "/api/edition", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(out, "built after the updates") || !strings.Contains(out, "Small Co") ||
		!strings.Contains(out, `"lapsed":true`) {
		t.Fatalf("the screen must keep the license and say why it does not count: %d %s", code, out)
	}
	if license.Current() != nil || evalmark.Shown("mark") != "mark" {
		t.Fatal("a build past the updates must be an evaluation")
	}
	if code, out := f.call(t, "POST", "/api/tenants", `{"name": "Second"}`, f.rootC); code == http.StatusForbidden {
		t.Fatalf("an evaluation has no ceiling: %d %s", code, out)
	}

	version.Date = time.Now().UTC().Format(time.RFC3339)
	code, out = f.call(t, "GET", "/api/edition", "", f.rootC)
	if code != http.StatusOK || strings.Contains(out, "built after the updates") {
		t.Fatalf("going back to a build from the term must clear it: %d %s", code, out)
	}
	if held := license.Current(); held == nil || held.ID != "LIC-T" {
		t.Fatal("going back to a build from the term must put the license back")
	}
}

// A test license ends at its date, for good, whatever the build: read again by
// every node, it is an evaluation from then on. Nothing else stops.
func TestATestLicenseStopsCountingAtItsDate(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	file, err := license.Sign(license.License{ID: "LIC-X", Licensee: "CI", Edition: license.EditionEnterprise,
		IssuedAt: time.Now().AddDate(0, 0, -3), ExpiresAt: time.Now().AddDate(0, 0, -2), Expires: license.ExpiresHard}, priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { license.Apply(nil) })
	code, out := f.call(t, "PUT", "/api/license", licenseBody(t, string(file)), f.rootC)
	if code != http.StatusOK || !strings.Contains(out, "this trial license ended on") || !strings.Contains(out, `"ends":true`) {
		t.Fatalf("an ended test license: %d %s", code, out)
	}
	if license.Current() != nil || evalmark.Shown("mark") != "mark" {
		t.Fatal("an ended test license must not take the notice away")
	}
}

func TestRemovingTheLicenseGoesBackToEvaluation(t *testing.T) {
	f := setupEE(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	team := signedEdition(t, priv, license.License{ID: "LIC-T", Licensee: "Small Co",
		Edition: license.EditionTeam, Tenants: 1, Instances: 1})
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, team), f.rootC); code != http.StatusOK {
		t.Fatalf("team license: %d %s", code, out)
	}
	if license.Current() == nil || evalmark.Shown("mark") != "" {
		t.Fatal("the license is not in force")
	}

	if code, out := f.call(t, "DELETE", "/api/license", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("a non-root removed the license: %d %s", code, out)
	}
	if license.Current() == nil {
		t.Fatal("a refused removal took the license away")
	}

	code, out := f.call(t, "DELETE", "/api/license", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("removal: %d %s", code, out)
	}
	if strings.Contains(out, "Small Co") || strings.Contains(out, "licenseSource") {
		t.Fatalf("the license is still shown: %s", out)
	}
	if license.Current() != nil || evalmark.Shown("mark") != "mark" {
		t.Fatal("the notice did not come back")
	}
	if text, err := f.api.st.License(ctx); err != nil || text != "" {
		t.Fatalf("the file stayed in the database: %q %v", text, err)
	}
	if code, out := f.call(t, "POST", "/api/tenants", `{"name": "Second"}`, f.rootC); code == http.StatusForbidden {
		t.Fatalf("the ceiling outlived the license: %d %s", code, out)
	}

	// Nothing to remove is no error, and no line in the trail.
	if code, out := f.call(t, "DELETE", "/api/license", "", f.rootC); code != http.StatusOK {
		t.Fatalf("removal with none held: %d %s", code, out)
	}
	events, err := f.api.st.ListAuditEvents(ctx, store.AuditFilter{Target: "license"})
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, ev := range events {
		if ev.Action == "license.delete" {
			removed++
			if ev.TargetID != "LIC-T" || ev.TargetName != "Small Co" {
				t.Fatalf("the trail does not say which: %+v", ev)
			}
		}
	}
	if removed != 1 {
		t.Fatalf("%d license.delete in the trail, want 1", removed)
	}

	// A license refused on reading goes the same way: it is the one to get rid of.
	if err := f.api.st.SetLicense(ctx, "no longer a license"); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "DELETE", "/api/license", "", f.rootC); code != http.StatusOK ||
		strings.Contains(out, "licenseProblem") {
		t.Fatalf("removal of a refused license: %d %s", code, out)
	}
}

// Going past the updates a license covers stops nothing and brings no notice
// back: what ends is updates and support.
func TestALicensePastItsDateStaysInForce(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	t.Cleanup(func() { license.Apply(nil) })
	data, err := license.Sign(license.License{ID: "LIC-OLD", Licensee: "Old Co", Edition: license.EditionEnterprise,
		IssuedAt: time.Now().AddDate(-3, 0, 0), ExpiresAt: time.Now().AddDate(-2, 0, 0)}, priv)
	if err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, string(data)), f.rootC); code != http.StatusOK {
		t.Fatalf("a license past its date: %d %s", code, out)
	}
	if license.Current() == nil || evalmark.Shown("mark") != "" {
		t.Fatal("a license past its date must stay in force")
	}
}

// The real file is about 10 KB: most of it is the issuer's own fields.
func TestALicenseOfTheRealSizeGoesThrough(t *testing.T) {
	f := setupEE(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	text := signedEdition(t, priv, license.License{ID: "LIC-1", Licensee: strings.Repeat("Long Co ", 1500),
		Edition: license.EditionEnterprise})
	if len(text) < 12<<10 {
		t.Fatalf("the file is only %d bytes", len(text))
	}
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, text), f.rootC); code != http.StatusOK {
		t.Fatalf("a large license: %d %.200s", code, out)
	}
}
