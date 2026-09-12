// Package expiry sends the daily notice about accounts whose access window is
// closing (MODEL-02).
//
// A validity window is the one thing on an account that acts on its own: a
// date passes and somebody cannot sign in, without anyone touching anything.
// That is exactly what it is for - a contractor's access ends the day the
// contract does, whether or not a human remembers - and it is also why a
// deliberate mechanism looks like a fault on the morning it fires. So the
// gateway says it first, to the people who could act.
//
// Three rules shape everything below:
//
//   - It sends only when there is something to say. A daily message that says
//     "nothing today" three hundred times is a message nobody opens on the day
//     it matters.
//   - It sends once a day, whatever happens to the process. The day it sent is
//     recorded in the database, so a restart at 07:59 does not send a second
//     one and a cluster of four nodes sends one, not four.
//   - It is not a clock the accounts depend on. Nothing here changes what an
//     account may do; the window is enforced at sign-in and on every request,
//     by the store. This only tells.
package expiry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// lockName is the advisory lock the sending node holds. Two nodes waking at
// the same second would otherwise both read "not sent yet" and both send.
const lockName = "expiry-digest"

// DefaultEvery is how often the loop asks whether it is time. Small enough
// that the hour an operator chose is honoured to the quarter, large enough
// that the question costs nothing - it is one settings read.
const DefaultEvery = 10 * time.Minute

// Sender is how a message leaves. The same shape the rest of the product
// passes around, so the trunk hands over the mailer it already built.
type Sender func(context.Context, mail.Message) error

// Digest is the daily notice.
type Digest struct {
	st   *store.Store
	send Sender
	// Now and Every are seams for the tests: a daily loop that could only be
	// observed by waiting a day would not be observed at all.
	Now   func() time.Time
	Every time.Duration
}

// New builds the digest on a store and a mailer.
func New(st *store.Store, send Sender) *Digest {
	return &Digest{st: st, send: send, Now: time.Now, Every: DefaultEvery}
}

// Run serves until ctx ends. It ticks rather than sleeping until the hour: a
// process that computed its next wake-up at start would be wrong the moment
// somebody changed the hour, and asking is cheap.
func (d *Digest) Run(ctx context.Context) {
	t := time.NewTicker(d.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := d.Tick(ctx); err != nil {
				slog.Warn("expiry digest", "err", err)
			}
		}
	}
}

// Tick does one pass: decides whether today's notice is due, and sends it.
// Exported because it is the whole behaviour - the ticker above only repeats
// it.
func (d *Digest) Tick(ctx context.Context) error {
	cfg := d.st.GetExpiryDigest(ctx)
	if !cfg.Enabled {
		return nil
	}
	now := d.Now()
	if now.Hour() < cfg.Hour {
		return nil
	}
	// The day is stamped in the gateway's local time, like the hour: the
	// operator picked "my morning", and a stamp in another calendar would make
	// "already sent today" true at the wrong moment.
	today := now.Format(time.DateOnly)
	if d.st.LastExpiryDigest(ctx) == today {
		return nil
	}
	// No relay, nothing to do - and the day is NOT recorded: the notice is
	// owed, and the morning a relay appears it should go out rather than be
	// found already ticked off.
	if !d.st.GetSMTP(ctx).Configured() {
		return nil
	}
	ran, err := d.st.TryLock(ctx, lockName, func(ctx context.Context) error {
		// Re-read inside the lock: the node that held it a second ago may have
		// been sending exactly this.
		if d.st.LastExpiryDigest(ctx) == today {
			return nil
		}
		return d.sendToday(ctx, cfg, now, today)
	})
	if err != nil {
		return err
	}
	if !ran {
		// Another node is on it. Nothing to report: this is the normal case in
		// a cluster, once a day, on every node but one.
		return nil
	}
	return nil
}

// sendToday builds the two lists and sends them, then records the day.
func (d *Digest) sendToday(ctx context.Context, cfg store.ExpiryDigest, now time.Time, today string) error {
	start := store.DayStart(now)
	// Ending: the last day is today or within the horizon - these accounts
	// still work. Ended: their last day has passed, and passed since the last
	// notice, so nobody is told twice about the same person.
	horizon := start.Add(time.Duration(cfg.Days) * 24 * time.Hour)
	ending, err := d.st.UsersEndingBetween(ctx, start.Unix(), horizon.Unix())
	if err != nil {
		return err
	}
	since := start.Add(-24 * time.Hour)
	if last := d.st.LastExpiryDigest(ctx); last != "" {
		if t, err := time.ParseInLocation(time.DateOnly, last, time.UTC); err == nil && t.Before(since) {
			since = t
		}
	}
	ended, err := d.st.UsersEndingBetween(ctx, since.Unix(), start.Unix())
	if err != nil {
		return err
	}
	if len(ending) == 0 && len(ended) == 0 {
		// Nothing to say: the day is recorded all the same, or every tick for
		// the rest of the day would ask the database the same question.
		return d.st.MarkExpiryDigestSent(ctx, today)
	}
	admins, err := d.st.ListNotifiableAdmins(ctx)
	if err != nil {
		return err
	}
	if len(admins) == 0 {
		slog.Warn("expiry digest: nobody to tell",
			"ending", len(ending), "ended", len(ended),
			"why", "no enabled root or app-admin account carries an e-mail address")
		return d.st.MarkExpiryDigestSent(ctx, today)
	}
	msg := d.message(ctx, cfg, ending, ended)
	// One message per administrator rather than one with everybody in To: a
	// list of colleagues is not a secret, but it is not this message's news
	// either, and a bounce for one address should not lose the others.
	var sent int
	var lastErr error
	for _, a := range admins {
		m := msg
		m.To = []string{a.Email}
		if err := d.send(ctx, m); err != nil {
			lastErr = err
			slog.Warn("expiry digest not delivered", "to", a.Username, "err", err)
			continue
		}
		sent++
	}
	if sent == 0 {
		// The relay refused everyone: leave the day unrecorded so the next
		// tick tries again. A notice that is a day late beats one that was
		// silently dropped.
		return lastErr
	}
	slog.Info("expiry digest sent", "recipients", sent, "ending", len(ending), "ended", len(ended))
	return d.st.MarkExpiryDigestSent(ctx, today)
}

// message writes the notice. English, like the console: its readers are the
// people who administer this gateway, and the pages a visitor sees are the
// translated surface (the catalogue in internal/auth).
func (d *Digest) message(ctx context.Context, cfg store.ExpiryDigest, ending, ended []store.User) mail.Message {
	app := appName(ctx, d.st)
	var groups []mail.Group
	if len(ending) > 0 {
		items := make([]string, len(ending))
		for i, u := range ending {
			items[i] = fmt.Sprintf("%s - last day %s", who(u), day(u.ValidUntil))
		}
		groups = append(groups, mail.Group{Title: fmt.Sprintf("Losing access within %s", days(cfg.Days)), Items: items})
	}
	if len(ended) > 0 {
		items := make([]string, len(ended))
		for i, u := range ended {
			items[i] = fmt.Sprintf("%s - last day was %s", who(u), day(u.ValidUntil))
		}
		groups = append(groups, mail.Group{Title: "No longer able to sign in", Items: items})
	}
	// The digest wears the CONSOLE's identity, not the application's: it goes to
	// administrators, in the tool they run this gateway from, not to the users
	// of the app behind it (NOTIF-01, NOTIF-04). So Meerkat's own mark and
	// palette, never the data plane's theme - the app name still rides in the
	// subject, to say which installation this is about.
	return mail.Compose("", consoleBrand(), consolePalette(), mail.Spec{
		Subject:   fmt.Sprintf("%s: %s", app, headline(ending, ended, cfg.Days)),
		Preheader: headline(ending, ended, cfg.Days),
		Heading:   "Expiring accounts",
		Groups:    groups,
		Outro: []string{
			"An account outside its window is refused at the next sign-in, with the date. " +
				"Nobody is signed out mid-work by this. Change a window under Application, Users.",
		},
	})
}

// consoleBrand and consolePalette are the ADMIN plane's identity - Meerkat's
// own, the look the console wears - shared by every mail that speaks to
// operators rather than to the application's users. Fixed, because the console
// is the product's surface and does not fork with the tenant's theme.
func consoleBrand() mail.Brand {
	return mail.Brand{AppName: store.MeerkatBranding().AppName, Meerkat: true}
}

func consolePalette() map[string]string {
	return store.DefaultTheme().Light
}

// headline is the subject's news, which has to survive being read in a list of
// forty subjects: how many, and how soon.
func headline(ending, ended []store.User, horizon int) string {
	var parts []string
	if n := len(ending); n > 0 {
		parts = append(parts, fmt.Sprintf("%s lose access within %s", accounts(n), days(horizon)))
	}
	if n := len(ended); n > 0 {
		parts = append(parts, fmt.Sprintf("%s can no longer sign in", accounts(n)))
	}
	return strings.Join(parts, ", ")
}

func accounts(n int) string {
	if n == 1 {
		return "1 account"
	}
	return fmt.Sprintf("%d accounts", n)
}

func days(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// who names a person the way the people reading know them: the login they type
// in the console, and the human name when there is one.
func who(u store.User) string {
	if u.Fullname == "" {
		return u.Username
	}
	return fmt.Sprintf("%s (%s)", u.Username, u.Fullname)
}

func day(unix int64) string {
	return time.Unix(unix, 0).UTC().Format(time.DateOnly)
}

func appName(ctx context.Context, st *store.Store) string {
	var b store.Branding
	if err := st.GetSetting(ctx, store.SettingBranding, &b); err == nil && b.AppName != "" {
		return b.AppName
	}
	return "Meerkat"
}
