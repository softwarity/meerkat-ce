package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
)

// Changing an account's address, with the new one confirmed first (AUTH-22).
//
// The address is where a password reset lands, so it is a way into the
// account. The change is therefore held until a link sent to the NEW address
// comes back; the OLD address is told at once, which is the alarm the owner
// gets when it was not them. Nothing is stored on the account meanwhile: the
// new address rides in the token itself, and an address nobody confirms simply
// never becomes anything.

const emailChangePurpose = "email-change"

// requestEmailChange mails the confirmation to the new address and the notice
// to the old one.
func (h *Handler) requestEmailChange(r *http.Request, userID, newEmail string) error {
	u, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil {
		return err
	}
	token, err := randToken()
	if err != nil {
		return err
	}
	hours := h.st.GetRegistrationPolicy(r.Context()).ConfirmHours
	if err := h.st.PutEmailTokenWith(r.Context(), hashTrust(token), u.ID, emailChangePurpose, newEmail,
		time.Now().Add(time.Duration(hours)*time.Hour).Unix()); err != nil {
		return err
	}
	link := h.externalURL(r) + "/confirm-email?token=" + token
	t := messagesFor(u.Locale)
	_, brand, _ := h.chrome()
	if err := h.sendMail(r.Context(), h.buildMail(r.Context(), newEmail, mail.Spec{
		Subject:   fmt.Sprintf(t["mailEmailChangeSubject"], brand.AppName),
		Preheader: t["mailEmailChangeHeading"],
		Heading:   t["mailEmailChangeHeading"],
		Intro:     []string{fmt.Sprintf(t["mailEmailChangeIntro"], brand.AppName), linkValidity(t, hours)},
		Button:    &mail.Button{Label: t["mailEmailChangeCta"], URL: link},
		Outro:     []string{t["mailEmailChangeOutro"]},
	})); err != nil {
		return err
	}
	// The old address, told at once. Best effort: the change is still held
	// by the confirmation whatever happens to this one.
	if u.Email != "" && u.Email != newEmail {
		if err := h.sendMail(r.Context(), h.buildMail(r.Context(), u.Email, mail.Spec{
			Subject:   fmt.Sprintf(t["mailEmailChangingSubject"], brand.AppName),
			Preheader: fmt.Sprintf(t["mailEmailChangingSubject"], brand.AppName),
			Heading:   fmt.Sprintf(t["mailEmailChangingSubject"], brand.AppName),
			Intro:     []string{fmt.Sprintf(t["mailEmailChangingIntro"], brand.AppName, newEmail)},
			Outro:     []string{t["mailEmailChangingOutro"]},
		})); err != nil {
			slog.Warn("old address not told of the change", "user", u.Username, "err", err)
		}
	}
	return nil
}

// doConfirmEmail makes the change when the link comes back. Spent on use,
// like every mailed link; the address is checked again, since somebody may
// have taken it while the message sat in an inbox.
func (h *Handler) doConfirmEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	userID, email, err := h.st.TakeEmailTokenPayload(r.Context(), hashTrust(token), emailChangePurpose, time.Now().Unix())
	if err != nil || email == "" {
		http.Error(w, h.tr(r, "errEmailChangeExpired"), http.StatusUnprocessableEntity)
		return
	}
	if id, err := h.st.UserIDByEmail(r.Context(), email); err == nil && id != userID {
		http.Error(w, h.tr(r, "errEmailTaken"), http.StatusConflict)
		return
	}
	if err := h.st.SetUserEmail(r.Context(), userID, email); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Proved: the person holds this mailbox.
	if err := h.st.MarkEmailVerified(r.Context(), userID); err != nil {
		slog.Warn("confirmed address not marked verified", "err", err)
	}
	h.securityOf(r, secEmailChange, userID, email)
	http.Redirect(w, r, "/profile?email=confirmed", http.StatusSeeOther)
}

// linkValidity is the sentence that says how long a mailed link lives, one
// per lifetime offered: "48 hours" is not "24 hours" with another number in
// every language.
func linkValidity(t map[string]string, hours int) string {
	switch hours {
	case 48:
		return t["linkValid48h"]
	case 168:
		return t["linkValid7d"]
	}
	return t["linkValid24h"]
}
