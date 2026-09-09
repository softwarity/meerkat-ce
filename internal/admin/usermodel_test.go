package admin

import (
	"strings"
	"testing"

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
