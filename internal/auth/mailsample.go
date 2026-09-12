package auth

import (
	"context"
	"fmt"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// Sending a real message is the only test that tells the truth about a mail: a
// relay that connects can still produce a message a client mangles, and the
// look depends on the theme, the branding and the language. So the relay screen
// can send a SAMPLE of any type it supports - fake values, real rendering - in
// any language, to see the actual thing land in an inbox before an account ever
// triggers it.
//
// The samples live here because this package owns the catalogue and the specs
// the real mails are built from; the admin screen calls SampleMail and sends
// what it gets back through the relay under test.

// MailSampleKind is one testable message type. The digest is English-only (it
// has no catalogue), the rest follow the chosen language.
type MailSampleKind struct {
	Key   string
	Label string // English label for the console's combo
}

// MailSampleKinds is the closed list the console offers and SampleMail accepts.
var MailSampleKinds = []MailSampleKind{
	{"confirm", "Account confirmation"},
	{"reset", "Password reset"},
	{"password-changed", "Password changed"},
	{"otp", "Sign-in code"},
	{"digest", "Expiring accounts"},
}

// SampleMail renders a themed sample of one kind in one language. base is the
// gateway origin a link should point at. ok is false for an unknown kind.
//
// The identity follows the AUDIENCE, exactly as the real mails do: an account
// message wears the application's brand and theme, while the digest - which
// goes to administrators - wears the console's own (Meerkat), so the test shows
// the true thing and not a data-plane-coloured lookalike.
func SampleMail(ctx context.Context, st *store.Store, kind, locale, base string) (mail.Message, bool) {
	t := messagesFor(locale)
	brand, palette := sampleBrand(ctx, st), samplePalette(ctx, st)
	if kind == "digest" {
		brand = mail.Brand{AppName: store.MeerkatBranding().AppName, Meerkat: true}
		palette = store.DefaultTheme().Light
	}
	spec, ok := sampleSpec(kind, t, sampleAppName(ctx, st), base)
	if !ok {
		return mail.Message{}, false
	}
	return mail.Compose("", brand, palette, spec), true
}

// sampleAppName is the data-plane application's name, used in a sample's text
// (the digest subject names the installation even though its look is console's).
func sampleAppName(ctx context.Context, st *store.Store) string {
	b := store.DefaultBranding()
	if err := st.GetSetting(ctx, store.SettingBranding, &b); err != nil || b.AppName == "" {
		b = store.DefaultBranding()
	}
	return b.AppName
}

// sampleSpec is the fake-valued spec for each kind, drawn from the same
// catalogue keys the real mails use - so the test shows the real wording, not a
// paraphrase that could drift from it.
func sampleSpec(kind string, t map[string]string, app, base string) (mail.Spec, bool) {
	switch kind {
	case "confirm":
		return mail.Spec{
			Subject:   fmt.Sprintf(t["mailConfirmSubject"], app),
			Preheader: fmt.Sprintf(t["mailConfirmHeading"], app),
			Heading:   fmt.Sprintf(t["mailConfirmHeading"], app),
			Intro:     []string{t["mailConfirmIntro"]},
			Button:    &mail.Button{Label: t["mailConfirmCta"], URL: base + "/confirm?token=SAMPLE"},
			Outro:     []string{t["mailConfirmOutro"]},
		}, true
	case "reset":
		return mail.Spec{
			Subject:   fmt.Sprintf(t["mailResetSubject"], app),
			Preheader: t["mailResetHeading"],
			Heading:   t["mailResetHeading"],
			Intro:     []string{fmt.Sprintf(t["mailResetIntro"], app)},
			Button:    &mail.Button{Label: t["mailResetCta"], URL: base + "/reset-password?token=SAMPLE"},
			Outro:     []string{t["mailResetOutro"]},
		}, true
	case "password-changed":
		return mail.Spec{
			Subject:   fmt.Sprintf(t["mailPwChangedSubject"], app),
			Preheader: fmt.Sprintf(t["mailPwChangedSubject"], app),
			Heading:   fmt.Sprintf(t["mailPwChangedSubject"], app),
			Intro:     []string{fmt.Sprintf(t["mailPwChangedBody"], app)},
		}, true
	case "otp":
		return mail.Spec{
			Subject:   fmt.Sprintf(t["mailOtpSubject"], app),
			Preheader: t["mailOtpHeading"],
			Heading:   t["mailOtpHeading"],
			Intro:     []string{fmt.Sprintf(t["mailOtpIntro"], app)},
			Code:      "123456",
			CodeNote:  fmt.Sprintf(t["mailOtpNote"], "10 "+t["minutes"]),
			Outro:     []string{t["mailOtpOutro"]},
		}, true
	case "digest":
		// English on purpose: the digest speaks to administrators and has no
		// catalogue (NOTIF-04). The rows are fake but shaped exactly like the
		// real ones.
		return mail.Spec{
			Subject:   app + ": 2 accounts lose access within 7 days",
			Preheader: "2 accounts lose access within 7 days",
			Heading:   "Expiring accounts",
			Groups: []mail.Group{
				{Title: "Losing access within 7 days", Items: []string{
					"contractor (Ada Byron) - last day 2026-01-18",
					"intern (Leo Marx) - last day 2026-01-19",
				}},
				{Title: "No longer able to sign in", Items: []string{
					"temp (Sam Vale) - last day was 2026-01-11",
				}},
			},
			Outro: []string{"An account outside its window is refused at the next sign-in, with the date. " +
				"Nobody is signed out mid-work by this. Change a window under Application, Users."},
		}, true
	}
	return mail.Spec{}, false
}

// sampleBrand and samplePalette read the DATA plane's identity straight from the
// store - the same brand and active theme the real mails wear.
func sampleBrand(ctx context.Context, st *store.Store) mail.Brand {
	b := store.DefaultBranding()
	if err := st.GetSetting(ctx, store.SettingBranding, &b); err != nil || b.AppName == "" {
		b = store.DefaultBranding()
	}
	return mail.Brand{AppName: b.AppName, LogoURL: b.Logo}
}

func samplePalette(ctx context.Context, st *store.Store) map[string]string {
	t, err := st.GetActiveTheme(ctx)
	if err != nil || len(t.Light) == 0 {
		t = store.DefaultTheme()
	}
	return t.Light
}
