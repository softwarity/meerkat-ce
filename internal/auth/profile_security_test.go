package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// The security page offers to change the password only to an account that has
// one: an account that signs in through an authority alone would land on a
// form asking for a current password it never had.
func TestThePasswordChangeIsOfferedOnlyWithAPassword(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	sc := sessionCookieOf(postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}}))
	if sc == nil {
		t.Fatal("no session cookie")
	}
	if page := bodyString(do(t, mux, "GET", "/profile/security", nil, sc)); !strings.Contains(page, `href="/profile/password"`) {
		t.Fatalf("an account with a password is not offered to change it:\n%.600s", page)
	}

	u, err := st.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.RehashUserPassword(ctx, u.ID, u.PasswordHash, ""); err != nil || !ok {
		t.Fatalf("clearing the password: %v %v", ok, err)
	}
	if page := bodyString(do(t, mux, "GET", "/profile/security", nil, sc)); strings.Contains(page, `href="/profile/password"`) {
		t.Fatal("an account without a password is offered to change it")
	}
}
