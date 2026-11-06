package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// The cross beside a passkey asks before it removes: it opens the question in
// that row, and only the button there removes. A passkey lost to a stray click
// is a device that no longer signs in.
func TestAPasskeyIsRemovedOnlyOnceAsked(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	u, err := st.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddPasskey(ctx, "pk1", u.ID, "credAAA", `{"id":"AAA"}`, "MacBook"); err != nil {
		t.Fatal(err)
	}
	sc := sessionCookieOf(postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}}))
	if sc == nil {
		t.Fatal("no session cookie")
	}

	list := bodyString(do(t, mux, "GET", "/profile/passkeys", nil, sc))
	if strings.Contains(list, `action="/profile/passkeys/delete"`) {
		t.Fatal("the list removes on a single click: no question asked")
	}
	if !strings.Contains(list, `href="/profile/passkeys?remove=pk1"`) {
		t.Fatalf("the cross does not open the question:\n%.800s", list)
	}
	ask := bodyString(do(t, mux, "GET", "/profile/passkeys?remove=pk1", nil, sc))
	if !strings.Contains(ask, `action="/profile/passkeys/delete"`) || !strings.Contains(ask, messages["en"]["passkeyRemoveAsk"]) {
		t.Fatalf("the question is not asked in the row:\n%.800s", ask)
	}
}
