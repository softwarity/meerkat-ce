package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// otpSetup is mfaSetup with a working relay wired and a capturing mailer, so a
// test can assert what was mailed. The account carries an address.
func otpSetup(t *testing.T) (*http.ServeMux, *store.Store, *[]mail.Message) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{
		ID: "u1", Username: "admin", PasswordHash: string(hash),
		Email: "admin@example.com", Root: true, Enabled: true,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := st.SetSetting(ctx, store.SettingSMTP, mail.Config{Host: "smtp.example.com", Port: 587, From: "gw@example.com"}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	h := New(st, sm)
	var sent []mail.Message
	h.Mailer = func(_ context.Context, m mail.Message) error { sent = append(sent, m); return nil }
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, st, &sent
}

// challenged logs in an enrolled user and returns the pending-challenge cookie.
func challenged(t *testing.T, mux *http.ServeMux) *http.Cookie {
	t.Helper()
	rec := postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}, "next": {"/app"}})
	if loc := rec.Header().Get("Location"); loc != "/totp" {
		t.Fatalf("expected /totp, got %q", loc)
	}
	return rec.Result().Cookies()[0]
}

// The fallback is off by default: the link is not shown, and the endpoint 404s.
// A control that appears and then refuses is worse than one that is absent.
func TestEmailOTPHiddenUntilEnabled(t *testing.T) {
	mux, st, sent := otpSetup(t)
	enrol(t, st)
	cookie := challenged(t, mux)

	page := do(t, mux, "GET", "/totp", nil, cookie)
	if strings.Contains(bodyString(page), "/totp/email") {
		t.Fatalf("the fallback link showed while the option was off")
	}
	if rec := do(t, mux, "POST", "/totp/email", nil, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("send endpoint answered %d while off, want 404", rec.Code)
	}
	if len(*sent) != 0 {
		t.Fatalf("a code was mailed while the option was off")
	}
}

// Enabled, with an enrolled account that has an address and a relay: the link
// shows, a code is mailed, and typing it back finishes the flow.
func TestEmailOTPFallbackSignsIn(t *testing.T) {
	mux, st, sent := otpSetup(t)
	enrol(t, st)
	if err := st.SetSetting(context.Background(), store.SettingMFAEmailOTP, true); err != nil {
		t.Fatal(err)
	}
	cookie := challenged(t, mux)

	page := do(t, mux, "GET", "/totp", nil, cookie)
	if !strings.Contains(bodyString(page), "/totp/email") {
		t.Fatalf("the fallback link is missing when it should show")
	}

	send := do(t, mux, "POST", "/totp/email", nil, cookie)
	if send.Code != http.StatusOK {
		t.Fatalf("send: %d", send.Code)
	}
	if len(*sent) != 1 {
		t.Fatalf("want one mail, got %d", len(*sent))
	}
	code := digitsOf(t, (*sent)[0].Text)

	// A wrong code still fails; the mailed one completes to the destination.
	if bad := do(t, mux, "POST", "/totp", url.Values{"code": {"000000"}}, cookie); bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wrong code: %d", bad.Code)
	}
	ok := do(t, mux, "POST", "/totp", url.Values{"code": {code}}, cookie)
	if ok.Code != http.StatusSeeOther || ok.Header().Get("Location") != "/app" {
		t.Fatalf("mailed code did not sign in: %d %q", ok.Code, ok.Header().Get("Location"))
	}
}

// A mailed code is single-use and bound to its account.
func TestEmailOTPCodeIsSingleUse(t *testing.T) {
	mux, st, sent := otpSetup(t)
	enrol(t, st)
	if err := st.SetSetting(context.Background(), store.SettingMFAEmailOTP, true); err != nil {
		t.Fatal(err)
	}
	cookie := challenged(t, mux)
	do(t, mux, "POST", "/totp/email", nil, cookie)
	code := digitsOf(t, (*sent)[0].Text)

	if ok := do(t, mux, "POST", "/totp", url.Values{"code": {code}}, cookie); ok.Code != http.StatusSeeOther {
		t.Fatalf("first use failed: %d", ok.Code)
	}
	// A second challenge with the same code is refused: it was burned.
	cookie2 := challenged(t, mux)
	if reused := do(t, mux, "POST", "/totp", url.Values{"code": {code}}, cookie2); reused.Code == http.StatusSeeOther {
		t.Fatalf("a burned mailed code signed in again")
	}
}

// An account with no e-mail cannot use the fallback even when it is enabled:
// there is nowhere to send the code.
func TestEmailOTPNeedsAnAddress(t *testing.T) {
	mux, st, _ := otpSetup(t)
	enrol(t, st)
	if err := st.SetSetting(context.Background(), store.SettingMFAEmailOTP, true); err != nil {
		t.Fatal(err)
	}
	// Strip the address.
	u, _ := st.GetUserByID(context.Background(), "u1")
	u.Email = ""
	if err := st.UpdateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	cookie := challenged(t, mux)
	if rec := do(t, mux, "POST", "/totp/email", nil, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("send with no address answered %d, want 404", rec.Code)
	}
}

// A second send inside the resend floor mails nothing, but still answers the
// page: a held-down button neither floods a mailbox nor tells the clicker it
// was throttled.
func TestEmailOTPResendIsThrottled(t *testing.T) {
	mux, st, sent := otpSetup(t)
	enrol(t, st)
	if err := st.SetSetting(context.Background(), store.SettingMFAEmailOTP, true); err != nil {
		t.Fatal(err)
	}
	cookie := challenged(t, mux)
	do(t, mux, "POST", "/totp/email", nil, cookie)
	do(t, mux, "POST", "/totp/email", nil, cookie)
	if len(*sent) != 1 {
		t.Fatalf("throttle let %d mails through, want 1", len(*sent))
	}
}

// An account never enrolled in TOTP does not get the fallback: that is forced
// enrolment's job, not a way around it.
func TestEmailOTPIsFallbackNotEnrolment(t *testing.T) {
	mux, st, _ := otpSetup(t)
	if err := st.SetSetting(context.Background(), store.SettingMFAEmailOTP, true); err != nil {
		t.Fatal(err)
	}
	// No enrol(): u1 has no TOTP secret. Force MFA so login reaches enrolment.
	if err := st.SetSetting(context.Background(), store.SettingMFARequired, true); err != nil {
		t.Fatal(err)
	}
	rec := postLogin(t, mux, url.Values{"username": {"admin"}, "password": {"s3cret"}, "next": {"/app"}})
	// A non-enrolled account under mandatory MFA is sent to enrolment, never to
	// the code challenge - so /totp/email has no pending challenge to serve.
	if loc := rec.Header().Get("Location"); loc != "/totp-enroll" {
		t.Fatalf("expected forced enrolment, got %q", loc)
	}
}

// digitsOf pulls the 6-digit code out of a plain-text mail body.
func digitsOf(t *testing.T, text string) string {
	t.Helper()
	for _, f := range strings.Fields(text) {
		f = strings.TrimSpace(f)
		if len(f) == 6 && isAllDigits(f) {
			return f
		}
	}
	t.Fatalf("no 6-digit code in mail:\n%s", text)
	return ""
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
