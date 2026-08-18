package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The community edition, with nothing unlocked. setup() enables everything and
// serves several organisations, so communityFixture puts both back where a
// fresh install starts - nothing licensed, one organisation, unnamed. What is
// pinned here is the refusal itself, and above all WHERE it falls: on writes,
// never on what already runs.
func communityFixture(t *testing.T) fixture {
	t.Helper()
	if edition.Enterprise {
		t.Skip("Enterprise build: there is nothing held back to assert here")
	}
	f := setupBare(t)
	if err := f.api.st.SetTenancy(context.Background(), store.TenancySingle); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCommunityEditionRefusesWhatItDoesNotSell(t *testing.T) {
	t.Run("a second organisation", func(t *testing.T) {
		f := communityFixture(t)
		// The first one is seeded at boot, so any creation is the second.
		code, body := f.call(t, "POST", "/api/tenants", `{"name":"acme"}`, f.rootC)
		if code != http.StatusForbidden {
			t.Fatalf("create tenant = %d, want 403", code)
		}
		if !strings.Contains(body, "Enterprise edition") {
			t.Fatalf("body = %s, want it to name the feature", body)
		}
	})

	t.Run("switching to multi-tenant", func(t *testing.T) {
		f := communityFixture(t)
		if code, _ := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy":"multi"}`, f.rootC); code != http.StatusForbidden {
			t.Fatalf("switch = %d, want 403", code)
		}
		// And single stays available: it is the community shape.
		if code, _ := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy":"single"}`, f.rootC); code != http.StatusOK {
			t.Fatalf("staying single = %d, want 200", code)
		}
	})

	t.Run("declaring a directory, while OIDC stays free", func(t *testing.T) {
		f := communityFixture(t)
		ldap := `{"id":"dir","kind":"ldap","name":"Corp","enabled":true,"config":{"url":"ldap://x"}}`
		if code, body := f.call(t, "PUT", "/api/auth-providers/dir", ldap, f.rootC); code != http.StatusForbidden {
			t.Fatalf("declare LDAP = %d %s, want 403", code, body)
		}
		// Without a free path to a modern provider this would be a toll, not
		// an edition: OIDC must go through.
		oidc := `{"id":"okta","kind":"oidc","name":"Okta","enabled":true,` +
			`"config":{"issuer":"https://example.test","clientId":"meerkat","clientSecret":"s3cret"}}`
		if code, body := f.call(t, "PUT", "/api/auth-providers/okta", oidc, f.rootC); code != http.StatusOK {
			t.Fatalf("declare OIDC = %d %s, want 200", code, body)
		}
	})

	t.Run("changing the working hours, while the rest of the settings save", func(t *testing.T) {
		f := communityFixture(t)
		hours := `{"businessAccess":{"inherited":false,"timezone":"Europe/Paris","days":[{"day":1,"from":"09:00","to":"18:00"}]},` +
			`"sessionTTL":"PT30M","mfaRequired":false,"passkeysAllowed":true,"languages":[]}`
		if code, body := f.call(t, "PUT", "/api/settings", hours, f.rootC); code != http.StatusForbidden {
			t.Fatalf("set hours = %d %s, want 403", code, body)
		}
		// Saving the settings WITHOUT touching the hours is not a change, and
		// must not be refused: every screen carrying them saves them along
		// with everything else. This is the seeded window, sent back verbatim.
		open := `{"day":1,"from":"00:00","to":"23:59"}`
		for d := 2; d <= 7; d++ {
			open += `,{"day":` + strconv.Itoa(d) + `,"from":"00:00","to":"23:59"}`
		}
		same := `{"businessAccess":{"timezone":"UTC","days":[` + open + `]},` +
			`"sessionTTL":"PT45M","mfaRequired":false,"passkeysAllowed":true,"languages":[]}`
		if code, body := f.call(t, "PUT", "/api/settings", same, f.rootC); code != http.StatusOK {
			t.Fatalf("save without touching the hours = %d %s, want 200", code, body)
		}
	})
}

// What the console asks before drawing anything: one endpoint, so a locked
// control here and a hidden menu entry there cannot drift apart.
func TestEditionReportsWhatThisInstallationIs(t *testing.T) {
	f := communityFixture(t)

	code, body := f.call(t, "GET", "/api/edition", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("GET /api/edition = %d %s", code, body)
	}
	for _, want := range []string{
		`"enterprise":false`,
		`"tenancy":"` + store.TenancySingle + `"`,
		`"tenancyLocked":true`,
		`"primaryTenant":"` + store.DefaultTenantID + `"`,
		"Enterprise edition", // the roster of what exists, unlocked or not
	} {
		if !strings.Contains(body, want) {
			t.Errorf("edition body misses %q: %s", want, body)
		}
	}

}

// Going back to single with organisations in the way is allowed and deletes
// nothing - they stop being served, and the count is what the console's banner
// says out loud.
func TestSingleModeReportsWhatItHoldsBack(t *testing.T) {
	f := setup(t) // needs the image that can hold several organisations

	// The way it happens for real: the organisation is created while the
	// gateway serves several, and the switch back is what hides it.
	if code, body := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy":"multi"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switch to multi = %d %s", code, body)
	}
	if code, body := f.call(t, "POST", "/api/tenants", `{"name":"acme"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("create tenant = %d %s", code, body)
	}
	if code, _ := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy":"single"}`, f.rootC); code != http.StatusOK {
		t.Fatal("switching back must be allowed: nothing is deleted")
	}
	code, body := f.call(t, "GET", "/api/edition", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"hiddenTenants":1`) {
		t.Fatalf("edition = %d %s, want it to count what is held back", code, body)
	}
	// Still there, just not served.
	if code, body := f.call(t, "GET", "/api/tenants", "", f.rootC); !strings.Contains(body, "acme") {
		t.Fatalf("tenants = %d %s, want acme still stored", code, body)
	}
}

// The stamp is where the console learns what it is BEFORE its first paint.
// Reading /api/edition instead would show the multi-organisation console for a
// frame and then take it away, which reads as a glitch - and a locked control
// appearing unlocked for a frame reads as a tease.
func TestConsoleStampCarriesTheEdition(t *testing.T) {
	f := communityFixture(t)

	stamp := func() string {
		t.Helper()
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(f.rootC)
		return consoleBodyAttrs(req, f.api.st, f.api.sm)
	}

	body := stamp()
	if body == "" {
		t.Fatal("a signed-in console request must be stamped")
	}
	if strings.Contains(body, "multi-tenant") {
		t.Fatalf("single mode must not stamp the multi class: %q", body)
	}
	// ONE class now, not one per feature: the product is sold whole.
	if strings.Contains(body, `class="ee`) || strings.Contains(body, ` ee"`) {
		t.Fatalf("the community image must not stamp the ee class: %q", body)
	}
	if !strings.Contains(body, `data-meerkat-primary-tenant="`+store.DefaultTenantID+`"`) {
		t.Fatalf("the served organisation must travel: %q", body)
	}
}

// Joining someone to an organisation is not a working-hours change, and the
// community edition must be able to do it. Ticking a group in the members
// matrix creates the membership, and that was refused with "business-hours is
// an Enterprise feature" - for a window the tick never touched.
func TestJoiningSomeoneNeedsNoLicence(t *testing.T) {
	f := setup(t)
	// Joining someone is free; giving THEM their own hours is not. On the
	// Enterprise image both are allowed, so the refusal half only means
	// something on the community one - which is where it is asserted.

	code, body := f.call(t, "PUT", "/api/tenants/default/members/bob",
		`{"type":"USER","enabled":true,"businessAccess":{"inherited":true},"sessionTTL":""}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("joining refused in the community edition: %d %s", code, body)
	}

	// A window of their OWN is the Enterprise part, so the image decides.
	want := http.StatusForbidden
	if edition.Enterprise {
		want = http.StatusOK
	}
	code, body = f.call(t, "PUT", "/api/tenants/default/members/bob",
		`{"type":"USER","enabled":true,"businessAccess":{"inherited":false,"timezone":"Europe/Paris","days":[{"day":1,"from":"09:00","to":"18:00"}]},"sessionTTL":""}`,
		f.rootC)
	if code != want {
		t.Fatalf("a per-person window must ask for the Enterprise image, got %d %s", code, body)
	}
}

// A licence is not a shape. With multi-tenant unlocked but the gateway serving
// ONE organisation, creating another would produce something served to nobody
// and invisible in a console that never mentions the notion - so it is refused,
// and the refusal names the switch that opens it.
func TestSingleModeRefusesASecondOrganisation(t *testing.T) {
	// The refusal here is about the MODE, not the edition: even the image that
	// sells several organisations serves one while it is set to single.
	f := setup(t)
	if err := f.api.st.SetTenancy(context.Background(), store.TenancySingle); err != nil {
		t.Fatal(err)
	}

	code, body := f.call(t, "POST", "/api/tenants", `{"name":"acme"}`, f.rootC)
	if code != http.StatusConflict {
		t.Fatalf("create tenant in single mode = %d %s, want 409", code, body)
	}
	if !strings.Contains(body, "single organisation") {
		t.Errorf("the refusal should say what is in the way: %s", body)
	}

	// The switch is the whole difference.
	if code, body := f.call(t, "PUT", "/api/settings/tenancy", `{"tenancy":"multi"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switch to multi = %d %s", code, body)
	}
	if code, body := f.call(t, "POST", "/api/tenants", `{"name":"acme"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("create tenant in multi mode = %d %s", code, body)
	}
}
