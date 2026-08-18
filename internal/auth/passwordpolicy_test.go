package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The policy is enforced where a password is TYPED, and the page says what it
// asks for before the person guesses. Both halves come from the same policy,
// so a checklist can never promise what the server will refuse.
func TestPasswordPolicyReachesThePageAndTheRefusal(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingPasswordPolicy, store.PasswordPolicy{
		MinLength: 12, MinUpper: 1, MinDigits: 2, MinSpecial: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// Sign in with the seeded password, which lands on the forced change only
	// if the account asks for one - so go through the profile page instead,
	// which any signed-in account may open.
	login := postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}})
	sc := sessionCookieOf(login)
	if sc == nil {
		t.Fatal("no session cookie")
	}

	page := do(t, mux, "GET", "/profile/password", nil, sc)
	body := bodyString(page)
	// The rules that are asked for are drawn, with their numbers.
	for _, want := range []string{`<ul class="pw-rules"`, `data-rule="length"`, `data-need="12"`,
		`data-rule="upper"`, `data-rule="digit"`, `data-need="2"`, `data-rule="special"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the checklist is missing %s", want)
		}
	}
	// And the rules that are NOT asked for stay off the page: a policy that
	// does not care about lowercase must not show a line about lowercase.
	if strings.Contains(body, `data-rule="lower"`) {
		t.Error("a rule nobody asked for was shown")
	}

	// A password the checklist would refuse is refused, and the page comes back
	// with the rules still on it.
	refused := do(t, mux, "POST", "/profile/password", url.Values{
		"current": {"s3cret"}, "password": {"short1!A"}, "confirm": {"short1!A"},
	}, sc)
	if refused.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a password below the policy answered %d", refused.Code)
	}
	if !strings.Contains(bodyString(refused), `<ul class="pw-rules"`) {
		t.Error("the refusal dropped the checklist")
	}

	// And one that satisfies every rule goes through.
	ok := do(t, mux, "POST", "/profile/password", url.Values{
		"current": {"s3cret"}, "password": {"Longenough12!"}, "confirm": {"Longenough12!"},
	}, sc)
	if ok.Code != http.StatusSeeOther {
		t.Fatalf("a password meeting the policy answered %d: %s", ok.Code, bodyString(ok))
	}
}

// The sign-in page asks for the password that already EXISTS. Telling a
// visitor the shape of what they are guessing helps only the guesser.
func TestSignInPageCarriesNoChecklist(t *testing.T) {
	mux, st := setupWithStore(t)
	if err := st.SetSetting(context.Background(), store.SettingPasswordPolicy,
		store.PasswordPolicy{MinLength: 14, MinSpecial: 2}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if strings.Contains(string(body), `<ul class="pw-rules"`) {
		t.Error("the sign-in page advertised the password policy")
	}
}

// A password may not come back. The one IN USE counts as history too: "do not
// reuse the last 3" that let someone re-set the very password they already
// have would have its hole exactly where people aim.
func TestPasswordHistoryRefusesAReturn(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	// Six, so the seeded "s3cret" is a candidate the LENGTH rule accepts:
	// otherwise every assertion below would pass on the wrong rule.
	if err := st.SetSetting(ctx, store.SettingPasswordPolicy, store.PasswordPolicy{
		MinLength: 6, History: 2,
	}); err != nil {
		t.Fatal(err)
	}
	sc := sessionCookieOf(postLogin(t, mux, url.Values{
		"username": {"admin"}, "password": {"s3cret"}}))
	if sc == nil {
		t.Fatal("no session cookie")
	}
	change := func(current, next string) int {
		return do(t, mux, "POST", "/profile/password", url.Values{
			"current": {current}, "password": {next}, "confirm": {next}}, sc).Code
	}

	// The password in use cannot be "changed" to itself.
	if got := change("s3cret", "s3cret"); got != http.StatusUnprocessableEntity {
		t.Errorf("re-setting the current password answered %d", got)
	}
	// Two real changes, then a return to the first: refused.
	if got := change("s3cret", "first-one-99"); got != http.StatusSeeOther {
		t.Fatalf("first change answered %d", got)
	}
	if got := change("first-one-99", "second-one-99"); got != http.StatusSeeOther {
		t.Fatalf("second change answered %d", got)
	}
	if got := change("second-one-99", "first-one-99"); got != http.StatusUnprocessableEntity {
		t.Errorf("a remembered password came back: %d", got)
	}
	// Past the window the policy remembers, it may be used again - that is
	// what a number means.
	if got := change("second-one-99", "third-one-99"); got != http.StatusSeeOther {
		t.Fatalf("third change answered %d", got)
	}
	if got := change("third-one-99", "s3cret"); got != http.StatusSeeOther {
		t.Errorf("a password older than the window was still refused: %d", got)
	}
}

// Expiry is checked at SIGN-IN: an expired password does not end a session
// someone is working in, it refuses the next login and sends them to change it.
func TestExpiredPasswordSendsToTheChangePage(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingPasswordPolicy, store.PasswordPolicy{
		MinLength: 8, ExpiryDays: 30,
	}); err != nil {
		t.Fatal(err)
	}
	// A password whose age is UNKNOWN never expires: treating the zero as 1970
	// would expire every account at once, the first time someone typed a
	// number into the expiry box.
	if loc := postLogin(t, mux, url.Values{
		"username": {"admin"}, "password": {"s3cret"}}).Header().Get("Location"); loc == "/update-password" {
		t.Fatal("an account of unknown password age was expired")
	}

	// The rule itself, on the ages the flow reads.
	h := New(st, session.NewManager(st))
	fresh := store.User{ID: "u1", PasswordHash: "x", PasswordChangedAt: time.Now().Add(-2 * 24 * time.Hour).Unix()}
	if h.passwordExpired(ctx, fresh) {
		t.Error("a two-day-old password expired under a thirty-day rule")
	}
	old := store.User{ID: "u1", PasswordHash: "x", PasswordChangedAt: time.Now().Add(-40 * 24 * time.Hour).Unix()}
	if !h.passwordExpired(ctx, old) {
		t.Error("a forty-day-old password did not expire under a thirty-day rule")
	}
	// An account with no local password (born at an authority) has nothing to
	// expire: sending it to a change page it cannot complete would lock it out
	// of a gateway its authority still vouches for.
	external := store.User{ID: "u2", PasswordChangedAt: time.Now().Add(-400 * 24 * time.Hour).Unix()}
	if h.passwordExpired(ctx, external) {
		t.Error("an account without a local password was expired")
	}
}
