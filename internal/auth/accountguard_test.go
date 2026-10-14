package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// guardSetup is a data-plane handler with one local account that has an
// address, a relay configured and a recording mailbox, under the given policy.
func guardSetup(t *testing.T, pol store.RateLimitPolicy) (*Handler, *http.ServeMux, *store.Store, *fakeMailbox) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{
		ID: "u1", Username: "alice", PasswordHash: string(hash), Email: "alice@example.org",
		Enabled: true, EmailVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingSMTP, mail.Config{Host: "smtp.test", From: "meerkat@test"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingRateLimit, pol); err != nil {
		t.Fatal(err)
	}
	h := New(st, session.NewManager(st))
	box := &fakeMailbox{}
	h.Mailer = box.send
	mux := http.NewServeMux()
	h.Register(mux)
	// The notice goes out off the request: wait for it before the store
	// closes under it.
	t.Cleanup(func() { h.pending.Wait(); _ = st.Close() })
	return h, mux, st, box
}

// signInFrom posts the login form from a given TCP peer, with an optional
// X-Forwarded-For.
func signInFrom(t *testing.T, mux http.Handler, peer, xff string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = peer
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func creds(user, pw string) url.Values { return url.Values{"username": {user}, "password": {pw}} }

// The reproduction of the audit (P0-02): fifty wrong passwords with a fresh
// X-Forwarded-For each, against a budget of three, gave fifty 401 and no 429.
// From a public peer the header is ignored, so the fourth attempt is refused.
//
// The peer is httptest's own default, 192.0.2.1 - a documentation address,
// which is PUBLIC under the trust rule: that is exactly the stranger reaching
// the gateway directly.
func TestRotatingForwardedForCannotDodgeTheSignInThrottle(t *testing.T) {
	_, mux, _, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 3, LoginWindow: "PT15M"})
	refused := 0
	for i := 0; i < 10; i++ {
		rec := signInFrom(t, mux, "192.0.2.1:1234", fmt.Sprintf("198.51.100.%d", i+1), creds("alice", "wrong"))
		if rec.Code == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused != 7 {
		t.Fatalf("%d of 10 refused with 429, want 7: the forged addresses minted fresh budgets", refused)
	}
}

// Behind a proxy of ours the header IS the address: two clients behind the same
// Ingress get a budget each, and a client cannot borrow the other's by
// prepending it.
func TestForwardedForFromAPrivateProxyIsBelieved(t *testing.T) {
	_, mux, st, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 2, LoginWindow: "PT15M"})
	for i := 0; i < 2; i++ {
		signInFrom(t, mux, "10.42.0.15:5000", "203.0.113.5", creds("alice", "wrong"))
	}
	if rec := signInFrom(t, mux, "10.42.0.15:5000", "203.0.113.5", creds("alice", "wrong")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("203.0.113.5 over budget answered %d, want 429", rec.Code)
	}
	if rec := signInFrom(t, mux, "10.42.0.15:5000", "203.0.113.6", creds("alice", "wrong")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("another client behind the same proxy answered %d, want its own budget (401)", rec.Code)
	}
	if rec := signInFrom(t, mux, "10.42.0.15:5000", "203.0.113.5, 203.0.113.7", creds("alice", "wrong")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the proxy's own entry is the client: answered %d, want 401", rec.Code)
	}
	// And the trail says on whose word: the client, via the proxy.
	evs, err := st.ListAuditEvents(context.Background(), store.AuditFilter{Kind: store.AuditKindSecurity})
	if err != nil || len(evs) == 0 {
		t.Fatalf("no security events: %v", err)
	}
	for _, ev := range evs {
		if ev.IP != "" && ev.Peer != "10.42.0.15" {
			t.Fatalf("event %s at %q records peer %q, want the proxy 10.42.0.15", ev.Action, ev.IP, ev.Peer)
		}
	}
}

// A plane that trusts nobody: even a private peer's header is ignored, so
// the rotation is back to buying nothing.
func TestTrustingNobodyIgnoresEveryForwardedFor(t *testing.T) {
	filters.SetPlane("", []netip.Prefix{}, nil)
	t.Cleanup(func() { filters.SetPlane("", nil, nil) })
	_, mux, _, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 2, LoginWindow: "PT15M"})
	codes := []int{}
	for i := 0; i < 3; i++ {
		codes = append(codes, signInFrom(t, mux, "10.42.0.15:5000", fmt.Sprintf("203.0.113.%d", i+1), creds("alice", "wrong")).Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("answers %v: with none, the header still minted budgets", codes)
	}
}

const capField = `name="captcha_id"`

// The account's own counter: failures on one name from DIFFERENT addresses,
// each well under its own budget, still add up - and past the threshold the
// page asks for the picture code. The right password alone is not enough, the
// right password and the right code are, and the counter is then forgiven.
// The account is never locked.
func TestTheAccountCounterAsksForTheCaptchaAcrossAddresses(t *testing.T) {
	_, mux, st, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 10, LoginWindow: "PT15M", AccountAttempts: 3})
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		rec := signInFrom(t, mux, fmt.Sprintf("203.0.113.%d:4000", i), "", creds("alice", "wrong"))
		if rec.Code != http.StatusUnauthorized || strings.Contains(bodyString(rec), capField) {
			t.Fatalf("failure %d: %d, captcha shown too early", i, rec.Code)
		}
	}
	// The third crosses: the answer carries the code, with the name kept.
	rec := signInFrom(t, mux, "203.0.113.3:4000", "", creds("alice", "wrong"))
	body := bodyString(rec)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(body, capField) || !strings.Contains(body, `value="alice"`) {
		t.Fatalf("crossing failure: %d, want 401 with the captcha and the name kept", rec.Code)
	}
	// A fresh address, the RIGHT password, no code: refused, and told why.
	rec = signInFrom(t, mux, "203.0.113.4:4000", "", creds("alice", "s3cret"))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(bodyString(rec), "copy the digits of the image") {
		t.Fatalf("right password without the code: %d, want 401 asking for it", rec.Code)
	}
	// The right password, a WRONG code: refused, and the code is spent.
	if err := st.PutChallenge(ctx, captchaPrefix+"t1", hashTrust("24689"), time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	form := creds("alice", "s3cret")
	form.Set("captcha_id", "t1")
	form.Set("captcha", "11111")
	if rec := signInFrom(t, mux, "203.0.113.4:4000", "", form); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: %d, want 401", rec.Code)
	}
	form.Set("captcha", "24689")
	if rec := signInFrom(t, mux, "203.0.113.4:4000", "", form); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a spent code was accepted again: %d", rec.Code)
	}
	// The right password AND the right code: in.
	if err := st.PutChallenge(ctx, captchaPrefix+"t2", hashTrust("35792"), time.Now().Add(time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	form.Set("captcha_id", "t2")
	form.Set("captcha", "35792")
	if rec := signInFrom(t, mux, "203.0.113.5:4000", "", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("right password and right code: %d, want 303", rec.Code)
	}
	// Forgiven: the next failure is an ordinary one, no code asked.
	if rec := signInFrom(t, mux, "203.0.113.6:4000", "", creds("alice", "wrong")); strings.Contains(bodyString(rec), capField) {
		t.Fatal("a successful sign-in did not clear the account counter")
	}
}

// An unknown name gets the same counter, the same code and the same words: a
// difference anywhere would answer the question the page refuses to answer -
// does this account exist.
func TestAnUnknownNameIsChallengedTheSame(t *testing.T) {
	_, mux, _, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 10, LoginWindow: "PT15M", AccountAttempts: 2})
	shape := func(user string) []string {
		var out []string
		for i := 1; i <= 3; i++ {
			rec := signInFrom(t, mux, fmt.Sprintf("203.0.113.%d:4000", i), "", creds(user, "wrong"))
			body := bodyString(rec)
			out = append(out, fmt.Sprintf("%d captcha=%v invalid=%v needed=%v", rec.Code,
				strings.Contains(body, capField), strings.Contains(body, "Invalid username or password"),
				strings.Contains(body, "copy the digits of the image")))
		}
		return out
	}
	known, unknown := shape("alice"), shape("ghost")
	if strings.Join(known, "|") != strings.Join(unknown, "|") {
		t.Fatalf("a known and an unknown name answer differently:\n known   %v\n unknown %v", known, unknown)
	}
	if !strings.Contains(known[1], "captcha=true") {
		t.Fatalf("the threshold did not ask for the code: %v", known)
	}
}

// The owner is told, once per window, by mail: how many failures, from how
// many addresses. An unknown name mails nobody.
func TestTheOwnerIsToldOncePerWindow(t *testing.T) {
	h, mux, _, box := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 10, LoginWindow: "PT15M", AccountAttempts: 3})
	for i := 1; i <= 6; i++ {
		signInFrom(t, mux, fmt.Sprintf("203.0.113.%d:4000", (i%2)+1), "", creds("alice", "wrong"))
		signInFrom(t, mux, fmt.Sprintf("203.0.113.%d:4000", i), "", creds("ghost", "wrong"))
	}
	h.pending.Wait()
	mails := box.forRecipient("alice@example.org")
	if len(mails) != 1 {
		t.Fatalf("%d notices to the owner, want exactly 1", len(mails))
	}
	text := mails[0].Text
	for _, want := range []string{"Failed attempts in the last 15 minutes: 3", "Addresses they came from: 2", "change your password"} {
		if !strings.Contains(text, want) {
			t.Errorf("the notice does not say %q:\n%s", want, text)
		}
	}
	box.Lock()
	total := len(box.sent)
	box.Unlock()
	if total != 1 {
		t.Fatalf("%d mails sent in all: an unknown name mailed somebody", total)
	}
}

// The sign-in history keeps the proxy beside the address it believed.
func TestTheHistoryRecordsThePeer(t *testing.T) {
	_, mux, st, _ := guardSetup(t, store.RateLimitPolicy{LoginAttempts: 10, LoginWindow: "PT15M", AccountAttempts: 5})
	if rec := signInFrom(t, mux, "10.42.0.15:5000", "203.0.113.5", creds("alice", "s3cret")); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in: %d", rec.Code)
	}
	evs, err := st.ListLoginEvents(context.Background(), "u1")
	if err != nil || len(evs) != 1 {
		t.Fatalf("history: %v %v", evs, err)
	}
	if evs[0].IP != "203.0.113.5" || evs[0].Peer != "10.42.0.15" {
		t.Fatalf("history row = %q via %q, want 203.0.113.5 via 10.42.0.15", evs[0].IP, evs[0].Peer)
	}
}
