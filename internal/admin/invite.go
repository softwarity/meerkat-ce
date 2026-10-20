package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/store"
)

// An administrator gives a person their way in by one of two means, never
// both (AUTH-25): a temporary password shown once on screen, or a mail with a
// link to the page where the person chooses their own - an invitation for a
// new account, a reset link for an existing one. The mail needs a relay, an
// address on the account, local passwords allowed, and a public name the
// gateway vouches for: a mailed link goes to no other (audit P0-01).

// accessMailOrigin is where a mailed link points, or why no link can be
// mailed right now - the sentence the console shows beside the option.
func (a *API) accessMailOrigin(ctx context.Context) (origin, why string) {
	switch {
	case a.Mailer == nil || !a.st.GetSMTP(ctx).Configured():
		return "", "no mail relay is configured (Infra > Mail relay)"
	case !a.st.LocalSignInEnabled(ctx):
		return "", "local passwords are turned off (Authorities)"
	}
	if origin = a.vouchedDataOrigin(ctx); origin == "" {
		return "", "the application has no public name the gateway can vouch for (Domains & TLS)"
	}
	return origin, ""
}

// sendAccessMail mails u the invitation (invite) or the reset link.
func (a *API) sendAccessMail(r *http.Request, invite bool, u store.User) error {
	if u.Email == "" {
		return fmt.Errorf("account %q has no e-mail address: give it one, or show a temporary password", u.Username)
	}
	origin, why := a.accessMailOrigin(r.Context())
	if why != "" {
		return fmt.Errorf("no link can be mailed: %s; show a temporary password instead", why)
	}
	msg, err := auth.AccessMail(r.Context(), a.st, invite, u, origin, r.Header.Get("Accept-Language"))
	if err != nil {
		return err
	}
	return a.Mailer(r.Context(), msg)
}
