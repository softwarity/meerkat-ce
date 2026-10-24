package admin

import (
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A route may only forward what this installation actually knows about a
// person. gateway.Validate cannot check it - it is pure on a route - so the
// check lives here, where there is a store and a person typing.
func TestARouteCannotForwardAFieldNobodyDefined(t *testing.T) {
	f := setupBare(t)
	a := f.api
	ctx := t.Context()

	route := func(field string) store.Route {
		return store.Route{
			ID: "r", Name: "r", Enabled: true, Upstream: "http://svc:80",
			Identity: &store.IdentityForward{
				Mechanism:  "headers",
				Attributes: []store.IdentityAttr{{Field: field}},
			},
		}
	}

	// Built-ins pass with nothing defined at all.
	if err := a.knownFields(ctx, route("email")); err != nil {
		t.Fatalf("a built-in field was refused: %v", err)
	}

	// A custom name nobody defined is refused, and the refusal LISTS what is
	// available - the answer to "then what may I forward?".
	err := a.knownFields(ctx, route("employeeNumber"))
	if err == nil {
		t.Fatal("an undefined field was accepted")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("the refusal does not say what is available: %v", err)
	}

	// Defined, and it passes - named in the list from then on.
	if err := a.st.SetSetting(ctx, store.SettingUserFields, []store.UserField{
		{Name: "employeeNumber", Kind: store.FieldText},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.knownFields(ctx, route("employeeNumber")); err != nil {
		t.Fatalf("a defined field was refused: %v", err)
	}
	err = a.knownFields(ctx, route("costCenter"))
	if err == nil || !strings.Contains(err.Error(), "employeeNumber") {
		t.Errorf("the refusal does not list the defined field: %v", err)
	}
}

// A value is checked against its definition when an account is saved. Written
// as a test because the store has the rule and the API has the definitions -
// two halves that only meet on this path, and a path is where they get missed.
func TestAValueIsCheckedAgainstTheModel(t *testing.T) {
	f := setupBare(t)
	a := f.api
	ctx := t.Context()

	if err := a.st.SetSetting(ctx, store.SettingUserFields, []store.UserField{
		{Name: "costCenter", Kind: store.FieldChoice, Choices: []string{"A100", "B200"}},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := a.checkedFields(ctx, map[string]string{"costCenter": "Z999"}); err == nil {
		t.Fatal("a value outside the choice list was accepted")
	}
	got, err := a.checkedFields(ctx, map[string]string{"costCenter": "B200", "gone": "orphan"})
	if err != nil {
		t.Fatalf("a sound value was refused: %v", err)
	}
	if got["costCenter"] != "B200" {
		t.Errorf("the value was not kept: %v", got)
	}
	if _, still := got["gone"]; still {
		t.Errorf("a value with no definition survived: %v", got)
	}
}

// An organisation's values are trusted by the services behind the gateway, so
// its own administrators read them and do not write them (MODEL-03): root or
// an app-admin does. Saving the rest of the organisation still works for its
// administrator - the screen sends the values back unchanged.
func TestATenantDoesNotFillItsOwnFields(t *testing.T) {
	f := setupBare(t)
	ctx := t.Context()
	if err := f.api.st.SetSetting(ctx, store.SettingTenantFields, []store.UserField{
		{Name: "plan", Kind: store.FieldChoice, Choices: []string{"basic", "premium"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.CreateUser(ctx, store.User{ID: "apa", Username: "apa", PasswordHash: "x", AppAdmin: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bob", "apa"} {
		if code, body := f.call(t, "PUT", "/api/tenants/default/members/"+id,
			`{"type":"ADMIN","enabled":true,"businessAccess":{"inherited":true}}`, f.rootC); code != http.StatusOK {
			t.Fatalf("make %s an admin: %d %s", id, code, body)
		}
	}
	put := func(name, fields string, c *http.Cookie) (int, string) {
		return f.call(t, "PUT", "/api/tenants/default",
			`{"name":"`+name+`","enabled":true,"businessAccess":{"inherited":true},"fields":`+fields+`}`, c)
	}

	if code, body := put("Default", `{"plan":"premium"}`, f.plainC); code != http.StatusForbidden || !strings.Contains(body, "app-admin") {
		t.Fatalf("a tenant admin set its own field: %d %s", code, body)
	}
	if code, body := put("Default", `{"plan":"gold"}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(body, "premium") {
		t.Fatalf("a value outside the list was not refused with the list: %d %s", code, body)
	}
	if code, body := put("Default", `{"plan":"basic","gone":"x"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("root could not set a field: %d %s", code, body)
	}
	if got, _ := f.api.st.GetTenant(ctx, "default"); got.Fields["plan"] != "basic" || got.Fields["gone"] != "" {
		t.Fatalf("stored values: %+v", got.Fields)
	}
	// The organisation's administrator renames it, sending the values back.
	if code, body := put("Acme", `{"plan":"basic"}`, f.plainC); code != http.StatusOK {
		t.Fatalf("a tenant admin could not save its tenant unchanged: %d %s", code, body)
	}
	if code, body := put("Acme", `{"plan":"premium"}`, issue(t, f.api.sm, "apa")); code != http.StatusOK {
		t.Fatalf("an app-admin could not set a field: %d %s", code, body)
	}
}

// Defining an organisation's fields: refused by the community image, which
// serves one organisation, and refused on a name an account field already
// travels under.
func TestTenantFieldsAreEnterpriseAndDisjoint(t *testing.T) {
	f := setupBare(t)
	if !edition.Enterprise {
		code, body := f.call(t, "PUT", "/api/model/tenant-fields", `{"fields":[{"name":"plan","kind":"text"}]}`, f.rootC)
		if code != http.StatusForbidden || !strings.Contains(body, "Enterprise") {
			t.Fatalf("the community image defined tenant fields: %d %s", code, body)
		}
		if code, body := f.call(t, "PUT", "/api/model/tenant-fields", `{"fields":[]}`, f.rootC); code != http.StatusOK {
			t.Fatalf("emptying the list was refused: %d %s", code, body)
		}
		return
	}
	if code, body := f.call(t, "PUT", "/api/model/user-fields", `{"fields":[{"name":"region","kind":"text"}]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("user fields: %d %s", code, body)
	}
	code, body := f.call(t, "PUT", "/api/model/tenant-fields", `{"fields":[{"name":"Region","kind":"text"}]}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "Region") {
		t.Fatalf("a name on both lists was accepted: %d %s", code, body)
	}
	if code, body := f.call(t, "PUT", "/api/model/tenant-fields", `{"fields":[{"name":"plan","kind":"text"}]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("tenant fields: %d %s", code, body)
	}
	// The other way round too: the account list cannot take it back.
	if code, body := f.call(t, "PUT", "/api/model/user-fields", `{"fields":[{"name":"plan","kind":"text"}]}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("an account field took an organisation's name: %d %s", code, body)
	}
	// And a route may name it.
	r := store.Route{ID: "r", Name: "r", Enabled: true, Upstream: "http://svc:80",
		Identity: &store.IdentityForward{Mechanism: "headers", Attributes: []store.IdentityAttr{{Field: "plan"}}}}
	if err := f.api.knownFields(t.Context(), r); err != nil {
		t.Fatalf("a route could not name a tenant field: %v", err)
	}
}

// A field added to the model is ticked at once on every route that declares
// what it knows about the caller, and a field removed leaves them all - page
// stamp included. A route that declares nothing is left alone.
func TestTheRoutesFollowTheModel(t *testing.T) {
	f := setupBare(t)
	ctx := t.Context()
	pred := []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/a/**"}}}}
	if err := f.api.st.SaveRoute(ctx, store.Route{ID: "fwd", Name: "fwd", Enabled: true, Upstream: "http://svc:80",
		Predicates: pred, Identity: &store.IdentityForward{Mechanism: "headers",
			Attributes: []store.IdentityAttr{{Field: "username"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SaveRoute(ctx, store.Route{ID: "bare", Name: "bare", Enabled: true, Upstream: "http://svc:80",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/b/**"}}}}}); err != nil {
		t.Fatal(err)
	}
	code, body := f.call(t, "PUT", "/api/model/user-fields", `{"fields":[{"name":"costCenter","kind":"text"}]}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"routes":["fwd"]`) {
		t.Fatalf("adding a field: %d %s", code, body)
	}
	fwd, _ := f.api.st.GetRoute(ctx, "fwd")
	if n := len(fwd.Identity.Attributes); n != 2 || fwd.Identity.Attributes[1].Field != "costCenter" {
		t.Fatalf("the new field is not ticked: %+v", fwd.Identity.Attributes)
	}
	if bare, _ := f.api.st.GetRoute(ctx, "bare"); bare.Identity != nil {
		t.Fatalf("a route declaring nothing was given an identity: %+v", bare.Identity)
	}
	// Saving the same list again moves nothing.
	if _, body := f.call(t, "PUT", "/api/model/user-fields", `{"fields":[{"name":"costCenter","kind":"text"}]}`, f.rootC); !strings.Contains(body, `"routes":[]`) {
		t.Fatalf("an unchanged model rewrote routes: %s", body)
	}
	if code, body := f.call(t, "PUT", "/api/model/user-fields", `{"fields":[]}`, f.rootC); code != http.StatusOK || !strings.Contains(body, `"routes":["fwd"]`) {
		t.Fatalf("removing a field: %d %s", code, body)
	}
	fwd, _ = f.api.st.GetRoute(ctx, "fwd")
	if len(fwd.Identity.Attributes) != 1 {
		t.Fatalf("the removed field stayed: %+v", fwd.Identity.Attributes)
	}
}

// The page stamp loses a removed field too, and keeps the others.
func TestARemovedFieldLeavesThePage(t *testing.T) {
	r := store.Route{UI: &store.RouteUI{UserInfo: &store.UserInfoConfig{Enabled: true,
		Fields: map[string]string{"username": "", "costCenter": "data-cc"}}}}
	got, moved := followedRoute(r, nil, map[string]bool{"costCenter": true})
	if !moved || len(got.UI.UserInfo.Fields) != 1 || r.UI.UserInfo.Fields["costCenter"] == "" {
		t.Fatalf("moved=%v, fields=%v, original=%v", moved, got.UI.UserInfo.Fields, r.UI.UserInfo.Fields)
	}
}
