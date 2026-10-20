package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// AccessLinkHours is how long a link an administrator mails holds: it reaches
// somebody who may read it tomorrow, not within the hour a reset they asked
// for themselves allows.
const AccessLinkHours = 48

// AccessMail is what an administrator sends INSTEAD of showing a password
// (AUTH-27): an invitation to a new account, or a reset link for an existing
// one. Both open the reset page, where the person chooses their own password,
// so no password ever travels or is read over a shoulder. The link's token is
// stored here, the caller sends the message.
//
// The language is the account's own, else the one the administrator's browser
// asks for (acceptLanguage) when the catalogue has it, else English; the look
// is the application's, as on every account message.
func AccessMail(ctx context.Context, st *store.Store, invite bool, u store.User, origin, acceptLanguage string) (mail.Message, error) {
	token, err := randToken()
	if err != nil {
		return mail.Message{}, err
	}
	if err := st.PutEmailToken(ctx, hashTrust(token), u.ID, resetPurpose,
		time.Now().Add(AccessLinkHours*time.Hour).Unix()); err != nil {
		return mail.Message{}, err
	}
	locale := u.Locale
	if locale == "" {
		locale = matchAcceptLanguage(acceptLanguage, catalogueLanguages())
	}
	spec := accessSpec(invite, messagesFor(locale), sampleAppName(ctx, st), origin+"/reset-password?token="+token)
	return mail.Compose(u.Email, sampleBrand(ctx, st), samplePalette(ctx, st), spec), nil
}

// accessSpec words the two messages from the catalogue, the samples included.
func accessSpec(invite bool, t map[string]string, app, link string) mail.Spec {
	if invite {
		return mail.Spec{
			Subject:   fmt.Sprintf(t["mailInviteSubject"], app),
			Preheader: fmt.Sprintf(t["mailInviteHeading"], app),
			Heading:   fmt.Sprintf(t["mailInviteHeading"], app),
			Intro:     []string{fmt.Sprintf(t["mailInviteIntro"], app), linkValidity(t, AccessLinkHours)},
			Button:    &mail.Button{Label: t["mailInviteCta"], URL: link},
			Outro:     []string{t["mailInviteOutro"]},
		}
	}
	return mail.Spec{
		Subject:   fmt.Sprintf(t["mailResetSubject"], app),
		Preheader: t["mailResetHeading"],
		Heading:   t["mailResetHeading"],
		Intro:     []string{fmt.Sprintf(t["mailResetAdminIntro"], app), linkValidity(t, AccessLinkHours)},
		Button:    &mail.Button{Label: t["mailResetCta"], URL: link},
		Outro:     []string{t["mailResetAdminOutro"]},
	}
}

// catalogueLanguages is every language the account messages are written in.
func catalogueLanguages() []string {
	out := make([]string, 0, len(messages))
	for lang := range messages {
		out = append(out, lang)
	}
	return out
}
