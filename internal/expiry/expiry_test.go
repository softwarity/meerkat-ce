package expiry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

// A gateway with a working relay, one administrator who can be told, and a
// clock the test owns.
func fixture(t *testing.T) (*store.Store, *[]mail.Message, *Digest) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingSMTP, mail.Config{
		Host: "smtp.example.com", Port: 587, From: "gateway@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	mkUser(t, st, store.User{
		Username: "boss", Fullname: "The Boss", Email: "boss@example.com",
		Enabled: true, Root: true,
	})
	var sent []mail.Message
	d := New(st, func(_ context.Context, m mail.Message) error {
		sent = append(sent, m)
		return nil
	})
	return st, &sent, d
}

func mkUser(t *testing.T, st *store.Store, u store.User) store.User {
	t.Helper()
	u.ID = strings.ReplaceAll(u.Username, "-", "") + "0000000000000000000000000000000"
	u.ID = u.ID[:32]
	if err := st.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create %q: %v", u.Username, err)
	}
	return u
}

// lastDay is the unix second a window's bound is written at: midnight UTC.
func lastDay(s string) int64 {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t.Unix()
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// The whole point of the feature: an account whose last day is inside the
// horizon is named, with its date, before the date arrives.
func TestItNamesTheAccountAndTheDayBeforeTheDayArrives(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{
		Username: "contractor", Fullname: "Ada Byron", Enabled: true,
		ValidUntil: lastDay("2026-09-14"),
	})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }

	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("want one message to the one administrator, got %d", len(*sent))
	}
	m := (*sent)[0]
	if m.To[0] != "boss@example.com" {
		t.Errorf("wrong recipient: %v", m.To)
	}
	for _, want := range []string{"contractor", "Ada Byron", "2026-09-14"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("the message never says %q:\n%s", want, m.Text)
		}
	}
	if !strings.Contains(m.Subject, "1 account") {
		t.Errorf("the subject does not carry the news: %q", m.Subject)
	}
}

// Once a day, whatever happens to the process: the day it sent is in the
// database, so a restart - or a second node - finds the work done.
func TestItSendsOncePerDayAcrossRestarts(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{Username: "contractor", Enabled: true, ValidUntil: lastDay("2026-09-14")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	// A fresh process on the same database, later the same day.
	other := New(st, d.send)
	other.Now = func() time.Time { return at("2026-09-10T23:50:00Z") }
	if err := other.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("the same day was announced %d times", len(*sent))
	}
	// The next morning, the same account is still inside the horizon: it is
	// announced again, because it has not been dealt with.
	next := New(st, d.send)
	next.Now = func() time.Time { return at("2026-09-11T07:05:00Z") }
	if err := next.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 2 {
		t.Fatalf("the next day was not announced: %d messages", len(*sent))
	}
}

// Before the hour, nothing - and nothing recorded either, or the notice would
// be skipped for the day.
func TestNothingBeforeTheChosenHour(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{Username: "contractor", Enabled: true, ValidUntil: lastDay("2026-09-14")})
	d.Now = func() time.Time { return at("2026-09-10T06:59:00Z") }
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("sent %d messages before the hour", len(*sent))
	}
	if got := st.LastExpiryDigest(context.Background()); got != "" {
		t.Errorf("the day was ticked off without sending: %q", got)
	}
}

// A day with nothing to say is a day with no message. A daily mail that says
// "nothing today" three hundred times is one nobody opens on the day it
// matters.
func TestSilentWhenThereIsNothingToSay(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{Username: "permanent", Enabled: true})
	mkUser(t, st, store.User{Username: "later", Enabled: true, ValidUntil: lastDay("2027-01-01")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("sent %d messages about nothing: %+v", len(*sent), *sent)
	}
	if got := st.LastExpiryDigest(context.Background()); got != "2026-09-10" {
		t.Errorf("a silent day must still be recorded, got %q", got)
	}
}

// An account that has just fallen out of its window is the other half of the
// news: somebody is going to telephone, and the administrator should already
// know why.
func TestItReportsWhatHasJustExpired(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{Username: "gone", Enabled: true, ValidUntil: lastDay("2026-09-09")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("want one message, got %d", len(*sent))
	}
	m := (*sent)[0]
	if !strings.Contains(m.Text, "no longer able to sign in") || !strings.Contains(m.Text, "gone") {
		t.Errorf("the message does not report the account that expired:\n%s", m.Text)
	}
	// The last day it worked, not the day it stopped: that is the date on the
	// account, and the one an administrator would type back.
	if !strings.Contains(m.Text, "2026-09-09") {
		t.Errorf("the message does not name the last day:\n%s", m.Text)
	}
}

// A disabled account is already refused. Announcing that it will also be out
// of its window is news about nothing.
func TestADisabledAccountIsNotNews(t *testing.T) {
	st, sent, d := fixture(t)
	mkUser(t, st, store.User{Username: "off", Enabled: false, ValidUntil: lastDay("2026-09-14")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("a disabled account was announced: %+v", *sent)
	}
}

// No relay, no notice - and the day stays unrecorded, so the morning a relay
// is configured the notice goes out instead of being found already done.
func TestWithoutARelayNothingIsSentAndNothingIsTickedOff(t *testing.T) {
	st, sent, d := fixture(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingSMTP, mail.Config{}); err != nil {
		t.Fatal(err)
	}
	mkUser(t, st, store.User{Username: "contractor", Enabled: true, ValidUntil: lastDay("2026-09-14")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("sent %d messages with no relay", len(*sent))
	}
	if got := st.LastExpiryDigest(ctx); got != "" {
		t.Errorf("the day was ticked off without a relay: %q", got)
	}
}

// Switched off is switched off, relay or no relay.
func TestDisabledSendsNothing(t *testing.T) {
	st, sent, d := fixture(t)
	ctx := context.Background()
	cfg := store.DefaultExpiryDigest()
	cfg.Enabled = false
	if err := st.SetSetting(ctx, store.SettingExpiryDigest, cfg); err != nil {
		t.Fatal(err)
	}
	mkUser(t, st, store.User{Username: "contractor", Enabled: true, ValidUntil: lastDay("2026-09-14")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Fatalf("a switched-off digest sent %d messages", len(*sent))
	}
}

// The horizon is what it says: an account ending the day after it is not
// announced yet.
func TestTheHorizonIsTheHorizon(t *testing.T) {
	st, sent, d := fixture(t)
	ctx := context.Background()
	cfg := store.DefaultExpiryDigest()
	cfg.Days = 3
	if err := st.SetSetting(ctx, store.SettingExpiryDigest, cfg); err != nil {
		t.Fatal(err)
	}
	mkUser(t, st, store.User{Username: "closing", Enabled: true, ValidUntil: lastDay("2026-09-12")})
	mkUser(t, st, store.User{Username: "distant", Enabled: true, ValidUntil: lastDay("2026-09-20")})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("want one message, got %d", len(*sent))
	}
	m := (*sent)[0]
	if !strings.Contains(m.Text, "closing") {
		t.Errorf("the account inside the horizon is missing:\n%s", m.Text)
	}
	if strings.Contains(m.Text, "distant") {
		t.Errorf("an account beyond the horizon was announced:\n%s", m.Text)
	}
}

// A relay that refuses everybody leaves the day unrecorded: a notice a day
// late beats one silently dropped.
func TestAFailedDeliveryIsRetried(t *testing.T) {
	st, _, _ := fixture(t)
	ctx := context.Background()
	mkUser(t, st, store.User{Username: "contractor", Enabled: true, ValidUntil: lastDay("2026-09-14")})
	var tries int
	d := New(st, func(context.Context, mail.Message) error {
		tries++
		return errRelay
	})
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(ctx); err == nil {
		t.Fatal("a refused delivery was reported as a success")
	}
	if got := st.LastExpiryDigest(ctx); got != "" {
		t.Errorf("the day was ticked off after a failure: %q", got)
	}
	if err := d.Tick(ctx); err == nil {
		t.Fatal("the second attempt did not happen")
	}
	if tries != 2 {
		t.Errorf("want two attempts, got %d", tries)
	}
}

var errRelay = relayError("relay refused")

type relayError string

func (e relayError) Error() string { return string(e) }

// A vault entry carrying a reminder date rides the same digest as the accounts,
// in its own section - a reminder to rotate a token before it lapses (VAULT).
// The value never appears; it is a reminder, not a leak.
func TestDigestReportsExpiringVaultEntries(t *testing.T) {
	st, sent, d := fixture(t)
	// A value entry needs no cipher; a plain value with a reminder date is
	// enough to prove the section renders.
	if err := st.SaveVaultEntry(context.Background(), vault.Entry{
		Name: "npm-token", Kind: vault.KindValue, Scope: "infra",
		Value: "npm_xxx", ExpiresAt: lastDay("2026-09-14"),
	}); err != nil {
		t.Fatal(err)
	}
	d.Now = func() time.Time { return at("2026-09-10T07:05:00Z") }
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("want one message, got %d", len(*sent))
	}
	m := (*sent)[0]
	for _, want := range []string{"Vault entries expiring", "npm-token", "2026-09-14"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("the digest does not report the expiring secret (%q):\n%s", want, m.Text)
		}
	}
	// The stored value never rides into the inbox.
	if strings.Contains(m.Text, "npm_xxx") || strings.Contains(m.HTML, "npm_xxx") {
		t.Errorf("the secret's value leaked into the digest")
	}
}
