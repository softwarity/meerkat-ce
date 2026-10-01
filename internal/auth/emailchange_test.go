package auth

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/store"
)

var confirmEmailLink = regexp.MustCompile(`/confirm-email\?token=[A-Za-z0-9_-]+`)

// A new address takes effect only once a link sent TO it comes back
// (AUTH-22); the old address is told at once. Until then the account keeps
// the address its recovery goes to.
func TestANewAddressIsConfirmedBeforeItCounts(t *testing.T) {
	mux, _, st, box := registerSetup(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass-1234-x"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "alice", Username: "alice", PasswordHash: string(hash),
		Email: "alice@old.example", Enabled: true, EmailVerified: true}); err != nil {
		t.Fatal(err)
	}
	login := do(t, mux, "POST", "/login", url.Values{"username": {"alice"}, "password": {"pass-1234-x"}}, nil)
	cookie := sessionCookieOf(login)

	res := do(t, mux, "POST", "/profile/email", url.Values{"email": {"alice@new.example"}}, cookie)
	if res.Code != http.StatusOK || !strings.Contains(bodyString(res), "alice@new.example") {
		t.Fatalf("the request does not say where the link went: %d", res.Code)
	}
	if u, _ := st.GetUserByID(ctx, "alice"); u.Email != "alice@old.example" {
		t.Fatalf("the address changed before being confirmed: %q", u.Email)
	}
	if n := len(box.forRecipient("alice@old.example")); n != 1 {
		t.Fatalf("the old address was told %d time(s)", n)
	}
	sent := box.forRecipient("alice@new.example")
	if len(sent) != 1 {
		t.Fatalf("mails to the new address: %d", len(sent))
	}
	if !strings.Contains(sent[0].Text, "24") {
		t.Errorf("the mail does not say how long the link lives:\n%s", sent[0].Text)
	}
	link := confirmEmailLink.FindString(sent[0].Text)
	if link == "" {
		t.Fatalf("no confirmation link in %q", sent[0].Text)
	}

	done := do(t, mux, "GET", link, nil, nil)
	if done.Code != http.StatusSeeOther || done.Header().Get("Location") != "/profile?email=confirmed" {
		t.Fatalf("confirm: %d %q", done.Code, done.Header().Get("Location"))
	}
	u, _ := st.GetUserByID(ctx, "alice")
	if u.Email != "alice@new.example" || !u.EmailVerified {
		t.Fatalf("after confirmation: %q verified=%v", u.Email, u.EmailVerified)
	}
	// Spent on use.
	if again := do(t, mux, "GET", link, nil, nil); again.Code != http.StatusUnprocessableEntity {
		t.Fatalf("replayed link: %d", again.Code)
	}
}

// A lifetime that has no sentence to say it is refused.
func TestTheConfirmLifetimeIsAChoice(t *testing.T) {
	for _, h := range []int{0, 24, 48, 168} {
		if err := store.SanitizeRegistration(&store.RegistrationPolicy{ConfirmHours: h}); err != nil {
			t.Errorf("%d refused: %v", h, err)
		}
	}
	if err := store.SanitizeRegistration(&store.RegistrationPolicy{ConfirmHours: 36}); err == nil {
		t.Error("36 hours was accepted")
	}
}
