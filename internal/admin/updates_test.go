package admin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// feedServer answers the list of releases and counts who asked, with what.
type feedServer struct {
	*httptest.Server
	calls atomic.Int64
	seen  atomic.Pointer[http.Request]
	body  atomic.Pointer[string]
}

func serveFeed(t *testing.T, body string) *feedServer {
	t.Helper()
	f := &feedServer{}
	f.body.Store(&body)
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		f.seen.Store(r)
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(*f.body.Load()))
	}))
	t.Cleanup(f.Close)
	return f
}

func runningAs(t *testing.T, v string) {
	t.Helper()
	before := version.Version
	t.Cleanup(func() { version.Version = before })
	version.Version = v
}

func updatesOf(t *testing.T, out string) updatesInfo {
	t.Helper()
	var info updatesInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return info
}

const twoReleases = `{"releases":[
	{"version":"1.1.0","built":"2026-10-01T10:00:00Z","notes":"- older"},
	{"version":"1.3.0","built":"2026-12-01T10:00:00Z","migrates":true,"notes":"- newest"},
	{"version":"1.2.0","notes":"- middle"},
	{"version":"nonsense"}]}`

// The promise on the site: a gateway calls nobody until root says so. The watch
// runs on every installation, so this is what keeps an isolated one isolated.
func TestNothingLeavesTheGatewayUntilRootTurnsTheCheckOn(t *testing.T) {
	f := setupBare(t)
	runningAs(t, "1.1.0")
	feed := serveFeed(t, twoReleases)
	f.api.UpdateFeed = feed.URL

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	f.api.WatchUpdates(ctx)
	f.api.lookForUpdates(context.Background())
	code, out := f.call(t, "GET", "/api/updates", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("read: %d %s", code, out)
	}
	if info := updatesOf(t, out); info.Enabled || info.CheckedAt != nil || len(info.Releases) != 0 {
		t.Fatalf("off by default, and nothing known: %s", out)
	}
	if n := feed.calls.Load(); n != 0 {
		t.Fatalf("the gateway asked %d time(s) with the check off", n)
	}
}

// On, it reads at once, keeps what is newer, newest first, and sends nothing
// about itself. Off, it forgets, and stops asking.
func TestTheCheckReadsOnceAndSendsNothingAboutThisGateway(t *testing.T) {
	f := setupBare(t)
	runningAs(t, "1.1.0")
	feed := serveFeed(t, twoReleases)
	f.api.UpdateFeed = feed.URL + "/releases.json"
	moved := 0
	f.api.UpdatesMoved = func() { moved++ }

	code, out := f.call(t, "PUT", "/api/updates", `{"enabled":true}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("turn on: %d %s", code, out)
	}
	info := updatesOf(t, out)
	if !info.Enabled || info.CheckedAt == nil || info.Error != "" || len(info.Releases) != 2 {
		t.Fatalf("what was found: %s", out)
	}
	if info.Releases[0].Version != "1.3.0" || info.Releases[1].Version != "1.2.0" || !info.Releases[0].Migrates {
		t.Fatalf("newest first, the running one out, the rest kept as said: %s", out)
	}
	if moved == 0 {
		t.Fatal("the consoles must hear of it")
	}
	r := feed.seen.Load()
	if r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" ||
		strings.Contains(r.UserAgent(), "meerkat") || strings.Contains(r.UserAgent(), "1.1.0") {
		t.Fatalf("the request says something about this gateway: %q %v", r.URL.RawQuery, r.Header)
	}

	// Not due yet: a restart does not read it again.
	f.api.lookForUpdatesIfDue(context.Background())
	if n := feed.calls.Load(); n != 1 {
		t.Fatalf("read %d times within the day", n)
	}

	code, out = f.call(t, "PUT", "/api/updates", `{"enabled":false}`, f.rootC)
	if info := updatesOf(t, out); code != http.StatusOK || info.Enabled || info.CheckedAt != nil || len(info.Releases) != 0 {
		t.Fatalf("off forgets: %d %s", code, out)
	}
	f.api.lookForUpdates(context.Background())
	if n := feed.calls.Load(); n != 1 {
		t.Fatalf("still asking once off: %d", n)
	}
}

// A gateway that cannot reach the list says so on the screen. What it knew stays
// known: a release does not stop existing because the network went away.
func TestAFailedCheckIsALineOnTheScreen(t *testing.T) {
	f := setupBare(t)
	runningAs(t, "1.1.0")
	feed := serveFeed(t, twoReleases)
	f.api.UpdateFeed = feed.URL
	if code, out := f.call(t, "PUT", "/api/updates", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("turn on: %d %s", code, out)
	}

	f.api.UpdateFeed = feed.URL + "/gone"
	f.api.lookForUpdates(context.Background())
	_, out := f.call(t, "GET", "/api/updates", "", f.rootC)
	info := updatesOf(t, out)
	if !strings.Contains(info.Error, "404") || len(info.Releases) != 2 {
		t.Fatalf("the reason, and what was known: %s", out)
	}

	feed.Close()
	f.api.lookForUpdates(context.Background())
	_, out = f.call(t, "GET", "/api/updates", "", f.rootC)
	if info := updatesOf(t, out); !strings.Contains(info.Error, "could not be read") {
		t.Fatalf("no network: %s", out)
	}

	garbled := serveFeed(t, "<html>a captive portal</html>")
	f.api.UpdateFeed = garbled.URL
	f.api.lookForUpdates(context.Background())
	_, out = f.call(t, "GET", "/api/updates", "", f.rootC)
	if info := updatesOf(t, out); !strings.Contains(info.Error, "not a list of releases") {
		t.Fatalf("not the list: %s", out)
	}
}

// A gateway updated since the last look must not offer what it now runs, and a
// development build compares from the last release its notes carry.
func TestWhatIsAvailableFollowsTheRunningVersion(t *testing.T) {
	f := setupBare(t)
	runningAs(t, "1.1.0")
	feed := serveFeed(t, twoReleases)
	f.api.UpdateFeed = feed.URL
	if code, out := f.call(t, "PUT", "/api/updates", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("turn on: %d %s", code, out)
	}

	version.Version = "1.3.0"
	_, out := f.call(t, "GET", "/api/updates", "", f.rootC)
	if info := updatesOf(t, out); len(info.Releases) != 0 {
		t.Fatalf("up to date: %s", out)
	}

	version.Version = "dev"
	f.api.ReleaseNotes = "## NEXT RELEASE\n\n- soon\n\n## 1.2.0\n\n- middle\n"
	_, out = f.call(t, "GET", "/api/updates", "", f.rootC)
	if info := updatesOf(t, out); len(info.Releases) != 1 || info.Releases[0].Version != "1.3.0" {
		t.Fatalf("a development build of 1.2.0: %s", out)
	}

	f.api.ReleaseNotes = ""
	_, out = f.call(t, "GET", "/api/updates", "", f.rootC)
	if info := updatesOf(t, out); len(info.Releases) != 0 {
		t.Fatalf("no version to compare with, no indicator: %s", out)
	}
}

// The verdict on the license is the gateway's, with the rule the gateway applies:
// a release built after the date is out of it, one without a date is not judged.
func TestAReleasePastTheLicenseIsSaidBeforeItIsInstalled(t *testing.T) {
	f := setupEE(t)
	runningAs(t, "1.1.0")
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f.api.LicenseKeys = []ed25519.PublicKey{pub}
	lic := signedEdition(t, priv, license.License{ID: "LIC-T", Licensee: "Small Co",
		Edition: license.EditionTeam, Tenants: 1, Instances: 1})
	if code, out := f.call(t, "PUT", "/api/license", licenseBody(t, lic), f.rootC); code != http.StatusOK {
		t.Fatalf("license: %d %s", code, out)
	}
	parsed, err := license.Parse([]byte(lic), []ed25519.PublicKey{pub}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	within, past := parsed.ExpiresAt.Add(-time.Hour), parsed.ExpiresAt.AddDate(0, 0, 1)
	if err := f.api.st.SetUpdateCheck(context.Background(), store.UpdateCheck{Enabled: true, Releases: []store.UpdateRelease{
		{Version: "1.4.0", Built: &past},
		{Version: "1.3.0", Built: &within},
		{Version: "1.2.0"},
	}}); err != nil {
		t.Fatal(err)
	}

	_, out := f.call(t, "GET", "/api/updates", "", f.rootC)
	info := updatesOf(t, out)
	if info.LicenseUntil == nil || !info.LicenseUntil.Equal(parsed.ExpiresAt) || len(info.Releases) != 3 {
		t.Fatalf("the date, and the three releases: %s", out)
	}
	for i, want := range []bool{true, false, false} {
		if info.Releases[i].OutOfLicense != want {
			t.Fatalf("%s out of the license: %v, want %v: %s", info.Releases[i].Version, !want, want, out)
		}
	}
}

// The address is the deployment's, never a request's: a console that could name
// one would make this gateway fetch whatever it was told.
func TestTheCheckTakesNoAddressFromTheAPI(t *testing.T) {
	f := setupBare(t)
	if code, out := f.call(t, "PUT", "/api/updates", `{"enabled":true,"url":"http://elsewhere"}`, f.rootC); code != http.StatusBadRequest {
		t.Fatalf("no address can be given here: %d %s", code, out)
	}
}
