package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/auth"
)

type localeBody struct {
	Code    string `json:"code"`
	Scope   string `json:"scope"`
	Strings []struct {
		Key, Value, Embedded, Reference string
		Overridden, Missing             bool
	} `json:"strings"`
}

func fetchLocale(t *testing.T, f fixture, path string) localeBody {
	t.Helper()
	status, body := f.call(t, "GET", path, "", f.rootC)
	if status != http.StatusOK {
		t.Fatalf("%s: %d %s", path, status, body)
	}
	var out localeBody
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return out
}

// Asking for one screen's strings returns a handful, not the catalogue. That
// is the whole point of the editor: three hundred keys in a list is how a
// catalogue ends up with holes.
func TestALocaleIsScopedToOneScreen(t *testing.T) {
	f := setupBare(t)
	all := fetchLocale(t, f, "/api/locales/fr")
	one := fetchLocale(t, f, "/api/locales/fr?template=select-tenant")
	if all.Scope != "all" || one.Scope != "select-tenant" {
		t.Fatalf("scopes: %q and %q", all.Scope, one.Scope)
	}
	if len(one.Strings) >= len(all.Strings) {
		t.Errorf("one screen reports %d strings and the catalogue %d", len(one.Strings), len(all.Strings))
	}
	if len(one.Strings) == 0 {
		t.Error("a screen with no strings is a screen the editor cannot serve")
	}
	// The English wording rides along: a key name is not a sentence, and a
	// translator fills against the reference, not against "chooseOrg".
	for _, s := range one.Strings {
		if s.Reference == "" {
			t.Errorf("%s carries no English reference", s.Key)
		}
	}
}

// A correction is stored, shows up as overridden, and comes back out - and
// resetting the language puts the product's own wording back.
func TestACorrectionIsStoredAndCanBePutBack(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	original := auth.EmbeddedString("fr", "signIn")

	status, body := f.call(t, "PUT", "/api/locales/fr", `{"entries":{"signIn":"Entrer"}}`, f.rootC)
	if status != http.StatusOK {
		t.Fatalf("save: %d %s", status, body)
	}

	got := fetchLocale(t, f, "/api/locales/fr?template=login")
	var found bool
	for _, s := range got.Strings {
		if s.Key != "signIn" {
			continue
		}
		found = true
		if s.Value != "Entrer" || !s.Overridden {
			t.Errorf("the correction did not stick: %+v", s)
		}
		// What we ship stays visible beside it, so "put this one back" can be
		// judged before it is done.
		if s.Embedded != original {
			t.Errorf("the shipped wording was lost: %q, want %q", s.Embedded, original)
		}
	}
	if !found {
		t.Fatal("the corrected key is not on the page that renders it")
	}
	// And the data plane sees it, not just the table.
	if auth.EmbeddedString("fr", "signIn") != original {
		t.Error("the override was written into the embedded catalogue")
	}

	if status, body := f.call(t, "DELETE", "/api/locales/fr", "", f.rootC); status != http.StatusNoContent {
		t.Fatalf("reset: %d %s", status, body)
	}
	back := fetchLocale(t, f, "/api/locales/fr?template=login")
	for _, s := range back.Strings {
		if s.Key == "signIn" && (s.Overridden || s.Value != original) {
			t.Errorf("the reset did not put the wording back: %+v", s)
		}
	}
	_ = ctx
}

// A wording equal to what we ship is NOT an override: keeping it would freeze
// that string at today's wording and hide the next fix we release.
func TestAWordingEqualToOursIsNotStored(t *testing.T) {
	f := setupBare(t)
	same := auth.EmbeddedString("fr", "signIn")
	f.call(t, "PUT", "/api/locales/fr", `{"entries":{"signIn":`+strconv.Quote(same)+`}}`, f.rootC)
	layer, _ := f.api.st.LocaleOverrides(context.Background())
	if _, there := layer["fr"]["signIn"]; there {
		t.Errorf("a wording identical to ours was stored as an override: %v", layer["fr"])
	}
}

// The catalogue is fixed: only its wordings belong to the installation. A key
// nothing reads would sit in an export forever with nothing to show it.
func TestAnUnknownKeyIsRefusedByName(t *testing.T) {
	f := setupBare(t)
	status, body := f.call(t, "PUT", "/api/locales/fr", `{"entries":{"notAThing":"x"}}`, f.rootC)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("an unknown key was accepted: %d", status)
	}
	if !strings.Contains(body, "notAThing") {
		t.Errorf("the refusal does not name the key: %s", body)
	}
}

// A screen's strings come back in the order a screen is read - title, the
// sentence under it, fields and buttons, help, then what it says when something
// goes wrong - and each one says which kind it is.
//
// Alphabetical was the order before, which put the title between two buttons
// and the refusal at the top. The editor draws a heading per run, so the order
// IS the grouping: sorted here rather than in the console, where the export and
// the screen would have drifted into two answers.
func TestAScreensStringsComeBackInReadingOrder(t *testing.T) {
	f := setupBare(t)
	res := f.get(t, "/api/locales/en?template=login", f.rootC)
	var body struct {
		Strings []localeString `json:"strings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("locale: %v", err)
	}
	_ = res.Body.Close()
	if len(body.Strings) < 5 {
		t.Fatalf("the sign-in page renders %d strings", len(body.Strings))
	}

	last := -1
	seen := map[string]bool{}
	for _, s := range body.Strings {
		rank, ok := groupOrder[s.Group]
		if !ok {
			t.Fatalf("%q is in group %q, which nothing orders", s.Key, s.Group)
		}
		if rank < last {
			t.Errorf("%q (%s) comes after a %s: the groups are out of order", s.Key, s.Group, body.Strings[0].Group)
		}
		last = rank
		seen[s.Group] = true
	}
	// The page has a title and a refusal, or the order above proves nothing.
	for _, want := range []string{groupTitle, groupError} {
		if !seen[want] {
			t.Errorf("the sign-in page reports no %s at all", want)
		}
	}
}

// The kind is read off the key when the key says it, and off the English
// wording when it does not. English and not the language being edited: a screen
// that reordered itself when one switched language would be a screen one loses
// one's place in.
func TestTheKindOfAStringIsWhatItIs(t *testing.T) {
	cases := []struct{ key, english, want string }{
		{"errInvalidCreds", "Invalid username or password.", groupError},
		{"devFailed", "Could not update the test mode.", groupError},
		{"titleSignIn", "Sign in", groupTitle},
		{"mailOtpSubject", "Your %s sign-in code", groupTitle},
		{"forgotHint", "Enter your account's email address.", groupHint},
		{"devNote", "A developer lens, not a privilege.", groupHint},
		{"forgotLead", "Reset your password", groupMessage},
		{"refusedRoles", "It asks for a role your account does not have.", groupMessage},
		{"backSoon", "Service will be back shortly.", groupMessage},
		{"signIn", "Sign in", groupLabel},
		{"cancel", "Cancel", groupLabel},
		{"apiTokens", "API tokens", groupLabel},
	}
	for _, c := range cases {
		if got := localeGroup(c.key, c.english); got != c.want {
			t.Errorf("%s (%q) is a %s, want %s", c.key, c.english, got, c.want)
		}
	}
}
