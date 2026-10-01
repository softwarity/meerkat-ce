package auth

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/store"
)

// A person sees where their account is signed in and closes any of those
// sessions, or all but this one (AUTH-14, SEC-07) - never somebody else's.
func TestAPersonListsAndClosesTheirSessions(t *testing.T) {
	mux, sm, st := mfaSetup(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("other-pass-1"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "u2", Username: "bob", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	signIn := func(user, pass string) *http.Cookie {
		return sessionCookieOf(do(t, mux, "POST", "/login", url.Values{"username": {user}, "password": {pass}}, nil))
	}
	here, laptop, phone := signIn("admin", "s3cret"), signIn("admin", "s3cret"), signIn("admin", "s3cret")
	bob := signIn("bob", "other-pass-1")

	page := html.UnescapeString(bodyString(do(t, mux, "GET", "/profile/sessions", nil, here)))
	if strings.Count(page, `name="id"`) != 2 {
		t.Fatalf("the two other sessions are not offered for closing:\n%s", page)
	}
	live := func(c *http.Cookie) bool {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(c)
		_, err := sm.Resolve(ctx, r)
		return err == nil
	}
	idOf := func(c *http.Cookie) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(c)
		return sm.CurrentID(r)
	}

	do(t, mux, "POST", "/profile/sessions", url.Values{"id": {idOf(laptop)}}, here)
	if live(laptop) || !live(here) || !live(phone) {
		t.Fatalf("closing one: laptop=%v here=%v phone=%v", live(laptop), live(here), live(phone))
	}
	// Somebody else's id closes nothing.
	do(t, mux, "POST", "/profile/sessions", url.Values{"id": {idOf(bob)}}, here)
	if !live(bob) {
		t.Fatal("a person closed another account's session")
	}
	do(t, mux, "POST", "/profile/sessions", url.Values{"action": {"others"}}, here)
	if live(phone) || !live(here) || !live(bob) {
		t.Fatalf("everywhere else: phone=%v here=%v bob=%v", live(phone), live(here), live(bob))
	}
	list, _, _ := st.ListSessions(ctx, store.SessionFilter{UserID: "u1"}, time.Now().Unix())
	if len(list) != 1 || list[0].CreatedAt == 0 || list[0].IP == "" {
		t.Fatalf("what is left: %+v", list)
	}
}
