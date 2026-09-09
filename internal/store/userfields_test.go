package store

import (
	"strings"
	"testing"
	"time"
)

// A custom field's name becomes a header name and a template key, so it is
// bounded like one - and what is refused says what is allowed, because the
// person reading it is typing into a form.
func TestAFieldNameIsBoundedLikeAHeaderName(t *testing.T) {
	for _, bad := range []string{"", "9lives", "cost centre", "cost.centre", "-x", "a/b"} {
		if _, err := SanitizeUserFields([]UserField{{Name: bad, Kind: FieldText}}); err == nil {
			t.Errorf("name %q was accepted", bad)
		}
	}
	for _, ok := range []string{"employeeNumber", "cost_center", "cost-center", "a"} {
		if _, err := SanitizeUserFields([]UserField{{Name: ok, Kind: FieldText}}); err != nil {
			t.Errorf("name %q was refused: %v", ok, err)
		}
	}
}

// A custom field that shadows a built-in would be ambiguous in a forwarding
// attribute, in a template and on a page - and silently so, because whichever
// won would look deliberate.
func TestAFieldCannotShadowOneTheGatewayCarries(t *testing.T) {
	for _, name := range []string{"email", "username", "roles", "tenant", "locale"} {
		_, err := SanitizeUserFields([]UserField{{Name: name, Kind: FieldText}})
		if err == nil {
			t.Fatalf("a field named %q was accepted", name)
		}
		if !strings.Contains(err.Error(), "username") {
			t.Errorf("the refusal does not list what is taken: %v", err)
		}
	}
}

// Two names differing only in case are ONE header with two meanings.
func TestTwoNamesThatDifferOnlyInCaseAreOne(t *testing.T) {
	_, err := SanitizeUserFields([]UserField{
		{Name: "costCenter", Kind: FieldText},
		{Name: "costcenter", Kind: FieldText},
	})
	if err == nil {
		t.Fatal("two fields that become the same header were accepted")
	}
}

// A choice with nothing to choose from is a text field that lies about itself,
// and the console would draw an empty select for it.
func TestAChoiceNeedsSomethingToChooseFrom(t *testing.T) {
	if _, err := SanitizeUserFields([]UserField{{Name: "cc", Kind: FieldChoice}}); err == nil {
		t.Fatal("an empty choice was accepted")
	}
	got, err := SanitizeUserFields([]UserField{
		{Name: "cc", Kind: FieldChoice, Choices: []string{" A100 ", "B200", "", "A100"}},
	})
	if err != nil {
		t.Fatalf("a sound choice was refused: %v", err)
	}
	if len(got[0].Choices) != 2 || got[0].Choices[0] != "A100" {
		t.Errorf("choices = %v, want the trimmed list without blanks or repeats", got[0].Choices)
	}
	// A kind that is not a choice carries no list, whatever was sent.
	got, _ = SanitizeUserFields([]UserField{{Name: "n", Kind: FieldText, Choices: []string{"x"}}})
	if got[0].Choices != nil {
		t.Errorf("a text field kept a choice list: %v", got[0].Choices)
	}
}

// The kind is what makes a value worth forwarding: a number that is prose and
// a choice that is not on the list are what this feature exists to prevent.
func TestAValueIsCheckedAgainstItsKind(t *testing.T) {
	defs := []UserField{
		{Name: "num", Kind: FieldNumber},
		{Name: "when", Kind: FieldDate},
		{Name: "cc", Kind: FieldChoice, Choices: []string{"A100", "B200"}},
		{Name: "ext", Kind: FieldBool},
	}
	for _, bad := range []map[string]string{
		{"num": "twelve"}, {"when": "31/12/2026"}, {"cc": "C300"}, {"ext": "yes"},
	} {
		if _, err := ValidateUserValues(defs, bad); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	good := map[string]string{"num": "48213", "when": "2026-12-31", "cc": "B200", "ext": "true"}
	out, err := ValidateUserValues(defs, good)
	if err != nil {
		t.Fatalf("sound values were refused: %v", err)
	}
	if len(out) != 4 {
		t.Errorf("kept %d values, want 4: %v", len(out), out)
	}
}

// A field removed from the settings must stop travelling. Left in the row it
// would come back the day somebody redefines that name for something else.
func TestAValueWithNoDefinitionIsDropped(t *testing.T) {
	defs := []UserField{{Name: "kept", Kind: FieldText}}
	out, err := ValidateUserValues(defs, map[string]string{"kept": "yes", "gone": "orphan"})
	if err != nil {
		t.Fatal(err)
	}
	if _, still := out["gone"]; still {
		t.Errorf("an orphan value survived: %v", out)
	}
}

// No custom field is ever mandatory. A field is defined on an installation
// that already has its accounts, and none of them carry it: refusing to save
// them until somebody fills a box would turn one definition into as many
// blocked screens as there are people.
func TestAnEmptyCustomFieldIsNeverRefused(t *testing.T) {
	defs := []UserField{
		{Name: "employeeNumber", Label: "Employee number", Kind: FieldText},
		{Name: "costCentre", Kind: FieldChoice, Choices: []string{"A100", "B200"}},
	}
	out, err := ValidateUserValues(defs, map[string]string{})
	if err != nil {
		t.Fatalf("an account with no value for a custom field was refused: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("an empty value was stored anyway: %v", out)
	}
}

// The window is DAYS, not instants: an access valid until the 31st works all
// of the 31st. Anything else is a trap - nobody types a date meaning "and it
// stops as that day begins".
func TestTheWindowIsDaysAndNotInstants(t *testing.T) {
	day := func(s string) int64 {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return d.Unix()
	}
	u := User{ValidFrom: day("2026-03-01"), ValidUntil: day("2026-03-31")}
	at := func(s string) time.Time {
		d, _ := time.Parse(time.RFC3339, s)
		return d
	}
	cases := []struct {
		when string
		want bool
	}{
		{"2026-02-28T23:59:00Z", false},
		{"2026-03-01T00:00:00Z", true},
		{"2026-03-31T23:59:00Z", true}, // all of the last day
		{"2026-04-01T00:00:00Z", false},
	}
	for _, c := range cases {
		if got := u.ValidAt(at(c.when)); got != c.want {
			t.Errorf("ValidAt(%s) = %v, want %v", c.when, got, c.want)
		}
	}

	// Unbounded on either side is ordinary: a contract with a start and no end
	// is as common as the reverse.
	open := User{ValidFrom: day("2026-03-01")}
	if !open.ValidAt(at("2030-01-01T00:00:00Z")) {
		t.Error("an open-ended window expired")
	}
	if (User{}).ValidAt(at("2026-03-01T00:00:00Z")) != true {
		t.Error("an account with no window was refused")
	}
}

// The refusal names the DATE. "Your access is not valid" sends somebody to
// support to ask the one question the message could have answered.
func TestTheRefusalNamesTheDate(t *testing.T) {
	d, _ := time.Parse(time.DateOnly, "2026-03-31")
	u := User{ValidUntil: d.Unix()}
	why := u.ValidityReason(d.AddDate(0, 0, 2))
	if !strings.Contains(why, "2026-03-31") {
		t.Errorf("reason = %q, want the date in it", why)
	}
	if u.ValidityReason(d) != "" {
		t.Error("an account inside its window was given a reason")
	}
}

// An end before its start locks an account out forever with nothing saying
// why, so it is refused where it is typed.
func TestAWindowThatEndsBeforeItStartsIsRefused(t *testing.T) {
	if err := ValidFromUntil(2000, 1000); err == nil {
		t.Fatal("a backwards window was accepted")
	}
	for _, ok := range [][2]int64{{0, 0}, {1000, 0}, {0, 2000}, {1000, 2000}} {
		if err := ValidFromUntil(ok[0], ok[1]); err != nil {
			t.Errorf("window %v was refused: %v", ok, err)
		}
	}
}
