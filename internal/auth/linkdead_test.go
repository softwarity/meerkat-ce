package auth

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/store"
)

// A mailed link that can no longer do its job lands on a page of the sign-in
// flow - theme, branding, the sentence that says what to do, a link to go and
// do it - not on a naked line of text. The status stays what it was.
func TestADeadMailedLinkLandsOnAThemedPage(t *testing.T) {
	mux, _, st, box := registerSetupTLS(t)
	ctx := context.Background()
	en := messagesFor("en")

	themed := func(t *testing.T, path string, cookie *http.Cookie, status int, errKey, back string) {
		t.Helper()
		res := do(t, mux, "GET", path, nil, cookie)
		body := bodyString(res)
		if res.Code != status {
			t.Fatalf("%s: %d, want %d", path, res.Code, status)
		}
		if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s answered %q, not a page", path, ct)
		}
		if !strings.Contains(body, "--mk-primary") || !strings.Contains(body, html.EscapeString(en[errKey])) ||
			!strings.Contains(body, `href="`+back+`"`) {
			t.Fatalf("%s: not the themed page with %s and a way to %s:\n%s", path, errKey, back, body)
		}
	}

	t.Run("an expired or spent token", func(t *testing.T) {
		themed(t, "/confirm?token=spent", nil, http.StatusUnprocessableEntity, "errConfirmExpired", "/login")
		themed(t, "/confirm-email?token=spent", nil, http.StatusUnprocessableEntity, "errEmailChangeExpired", "/profile")
		themed(t, "/reset-password?token=spent", nil, http.StatusUnprocessableEntity, "errResetExpired", "/login")
	})

	t.Run("an address taken while the link sat in the inbox", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("pass-1234-x"), bcrypt.MinCost)
		if err := st.CreateUser(ctx, store.User{ID: "alice", Username: "alice", PasswordHash: string(hash),
			Email: "alice@old.example", Enabled: true, EmailVerified: true}); err != nil {
			t.Fatal(err)
		}
		cookie := sessionCookieOf(do(t, mux, "POST", "/login", url.Values{"username": {"alice"}, "password": {"pass-1234-x"}}, nil))
		if res := do(t, mux, "POST", "/profile/email", url.Values{"email": {"taken@new.example"}, "current": {"pass-1234-x"}}, cookie); res.Code != http.StatusOK {
			t.Fatalf("change request: %d", res.Code)
		}
		sent := box.forRecipient("taken@new.example")
		if len(sent) != 1 {
			t.Fatalf("mails to the new address: %d", len(sent))
		}
		link := confirmEmailLink.FindString(sent[0].Text)
		if err := st.CreateUser(ctx, store.User{ID: "bob", Username: "bob", PasswordHash: string(hash),
			Email: "taken@new.example", Enabled: true}); err != nil {
			t.Fatal(err)
		}
		themed(t, link, nil, http.StatusConflict, "errEmailTaken", "/profile")
	})
}
