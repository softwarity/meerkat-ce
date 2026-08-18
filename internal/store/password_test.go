package store

import (
	"context"
	"testing"
)

// The checklist and the refusal come from ONE function: what the page shows is
// what the server enforces, rule by rule.
func TestPasswordRulesCountEachKind(t *testing.T) {
	p := PasswordPolicy{MinLength: 10, MinLower: 2, MinUpper: 1, MinDigits: 1, MinSpecial: 1}
	cases := []struct {
		pwd    string
		accept bool
		why    string
	}{
		{"Passw0rd!x", true, "ten characters with one of each kind"},
		{"Passw0rd!", false, "nine characters is one short"},
		{"passw0rd!!!", false, "no uppercase"},
		{"PASSW0RD!!!", false, "no lowercase"},
		{"Password!!!", false, "no digit"},
		{"Passw0rdxyz", false, "no special character"},
		{"", false, "nothing at all"},
	}
	for _, c := range cases {
		if got := p.Accepts(c.pwd); got != c.accept {
			t.Errorf("Accepts(%q) = %v, want %v (%s)", c.pwd, got, c.accept, c.why)
		}
	}

	// Only the rules that are ASKED for are listed: a policy that does not
	// care about digits must not put a line about digits on the page.
	only := PasswordPolicy{MinLength: 8}
	rules := only.Rules("abc")
	if len(rules) != 1 || rules[0].Kind != PasswordRuleLength || rules[0].OK {
		t.Fatalf("a length-only policy showed %+v", rules)
	}
}

// A passphrase in a script without letter case is letters, not punctuation.
// Counting kana as special would tick a box nobody meant and refuse a
// lowercase the keyboard cannot produce.
func TestPasswordCountsCaselessScriptsAsLetters(t *testing.T) {
	p := PasswordPolicy{MinLength: 4, MinSpecial: 1}
	if p.Accepts("ねこすき") {
		t.Error("Japanese letters were counted as special characters")
	}
	if !p.Accepts("ねこすき!") {
		t.Error("a real special character was not counted")
	}

	// And length is in characters, not bytes: four kana are four, not twelve.
	long := PasswordPolicy{MinLength: 5}
	if long.Accepts("ねこすき") {
		t.Error("four characters passed a five-character rule")
	}
}

// A policy nobody can satisfy locks every account out of its own password
// change - and the screen that set it sits behind that same login.
func TestPasswordPolicyIsBounded(t *testing.T) {
	got := PasswordPolicy{MinLength: 9000, MinLower: -3, MinDigits: 99}.Sanitize()
	if got.MinLength != 128 || got.MinLower != 0 || got.MinDigits != 16 {
		t.Fatalf("out-of-range policy stored as %+v", got)
	}
	// The length is raised to the sum of the parts rather than promise a
	// checklist that can never be completed.
	sum := PasswordPolicy{MinLength: 4, MinLower: 2, MinUpper: 2, MinDigits: 2, MinSpecial: 2}.Sanitize()
	if sum.MinLength != 8 {
		t.Fatalf("length = %d, want the sum of the kinds (8)", sum.MinLength)
	}
}

// The sender's display name IS the application's name. It used to be typed a
// second time, in its own box, next to a subject line that already read
// "Reset your <application> password" - two names for one identity, free to
// disagree, and they did.
func TestSenderNameIsTheApplicationName(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	if err := st.SetSetting(ctx, SettingSMTP, map[string]any{
		"host": "relay.example.com", "port": 25,
		"from": "no-reply@example.com", "fromName": "a name typed long ago",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, SettingBranding, Branding{AppName: "Acme Portal"}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetSMTP(ctx).Sender(); got != `"Acme Portal" <no-reply@example.com>` {
		t.Fatalf("sender = %q, want the application's name", got)
	}

	// No application name: whatever the relay holds is better than nothing,
	// and an empty display name would send bare addresses.
	if err := st.SetSetting(ctx, SettingBranding, Branding{AppName: ""}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetSMTP(ctx).FromName; got != "a name typed long ago" {
		t.Fatalf("with no application name, FromName = %q", got)
	}
}
