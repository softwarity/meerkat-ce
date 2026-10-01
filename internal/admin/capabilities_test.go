package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Four features were root's for no reason that survived being asked, and each
// now belongs to the capability whose domain it is. This holds the line where
// it was drawn - including the half nobody states: root has all of it.
//
// "Passed the guard" means anything but a 403. Some of these answer 200, some
// 422 for a body a real screen would never send; either way the capability
// was accepted, which is the only question here.
func TestFeaturesBelongToTheirCapability(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, u := range []store.User{
		{ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true},
		{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	who := map[string]*http.Cookie{
		"root":        f.rootC,
		"infra-admin": issue(t, f.api.sm, "infra"),
		"app-admin":   issue(t, f.api.sm, "app"),
		"plain":       f.plainC,
	}

	cases := []struct {
		method, path, body string
		allowed            []string
	}{
		// Control-plane tokens: anybody who administers a domain, for their own.
		{"GET", "/api/admin-tokens", "", []string{"root", "infra-admin", "app-admin"}},
		{"POST", "/api/admin-tokens", `{"name":"probe","scope":"readonly"}`, []string{"root", "infra-admin", "app-admin"}},
		// The agent door: infrastructure, read by the gateway scope.
		{"GET", "/api/settings/agent", "", []string{"root", "infra-admin"}},
		{"PUT", "/api/settings/agent", `{"enabled":false}`, []string{"root", "infra-admin"}},
		// Identity: the bulk password change and the shape of the organisations.
		{"POST", "/api/users/must-change-password", "", []string{"root", "app-admin"}},
		{"PUT", "/api/settings/tenancy", `{"tenancy":"single"}`, []string{"root", "app-admin"}},
	}
	for _, c := range cases {
		for name, cookie := range who {
			code, out := f.call(t, c.method, c.path, c.body, cookie)
			want := false
			for _, a := range c.allowed {
				if a == name {
					want = true
				}
			}
			switch {
			case want && code == http.StatusForbidden:
				t.Errorf("%s %s: %s was refused, and it is theirs: %s", c.method, c.path, name, out)
			case !want && code != http.StatusForbidden:
				t.Errorf("%s %s: %s got %d, and it is not theirs", c.method, c.path, name, code)
			}
		}
	}
}

// A token is its owner's: each person sees and manages their own, and root's
// are not an infra admin's business - revoking somebody else's credential is
// not administration, it is sabotage.
func TestEachAdminSeesOnlyTheirOwnTokens(t *testing.T) {
	f := setup(t)
	if err := f.api.st.CreateUser(context.Background(),
		store.User{ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	infraC := issue(t, f.api.sm, "infra")

	if code, out := f.call(t, "POST", "/api/admin-tokens", `{"name":"roots-own","scope":"full"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("root mint: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/admin-tokens", `{"name":"infras-own","scope":"full"}`, infraC); code != http.StatusCreated {
		t.Fatalf("infra mint: %d %s", code, out)
	}
	_, list := f.call(t, "GET", "/api/admin-tokens", "", infraC)
	if strings.Contains(list, "roots-own") {
		t.Errorf("an infra admin sees root's token: %s", list)
	}
	if !strings.Contains(list, "infras-own") {
		t.Errorf("an infra admin does not see their own token: %s", list)
	}
}
