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

var emailChangeCodePage = flowPage("email-change-code", emailChangeCodeBody)

// The page that waits for the code when the mail to the new address carried
// one rather than a link (mailcode.go).
const emailChangeCodeBody = `    <form method="post" action="/profile/email/code">
      <p class="lead">{{.T.registerSentLead}}</p>
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <p class="hint">{{.Hint}}</p>
      <label class="field">
        <span>{{.T.authCode}}</span>
        <input name="code" inputmode="numeric" autocomplete="one-time-code" autofocus required>
      </label>
      <button type="submit">{{.T.mailEmailChangeCta}}</button>
    </form>
    <p class="back"><a href="/profile">{{.T.backToProfile}}</a></p>
`

// requestEmailChange mails the confirmation to the new address - a link
// rooted at origin, or, origin empty, a code bound to the browser reqID
// names - and the notice to the old one.
func (h *Handler) requestEmailChange(r *http.Request, userID, newEmail, origin, reqID string) error {
	u, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil {
		return err
	}
	t := messagesFor(u.Locale)
	_, brand, _ := h.chrome()
	spec := mail.Spec{
		Subject:   fmt.Sprintf(t["mailEmailChangeSubject"], brand.AppName),
		Preheader: t["mailEmailChangeHeading"],
		Heading:   t["mailEmailChangeHeading"],
		Outro:     []string{t["mailEmailChangeOutro"]},
	}
	if origin == "" {
		code, err := h.putMailCode(r, reqID, u.ID, emailChangeCodePurpose, newEmail)
		if err != nil {
			return err
		}
		spec.Intro = []string{fmt.Sprintf(t["mailEmailChangeCodeIntro"], brand.AppName)}
		spec.Code = code
		spec.CodeNote = fmt.Sprintf(t["mailOtpNote"], minutesLabel(t, signinCodeTTL))
	} else {
		token, err := randToken()
		if err != nil {
			return err
		}
		hours := h.st.GetRegistrationPolicy(r.Context()).ConfirmHours
		if err := h.st.PutEmailTokenWith(r.Context(), hashTrust(token), u.ID, emailChangePurpose, newEmail,
			time.Now().Add(time.Duration(hours)*time.Hour).Unix()); err != nil {
			return err
		}
		spec.Intro = []string{fmt.Sprintf(t["mailEmailChangeIntro"], brand.AppName), linkValidity(t, hours)}
		spec.Button = &mail.Button{Label: t["mailEmailChangeCta"], URL: origin + "/confirm-email?token=" + token}
	}
	if err := h.sendMail(r.Context(), h.buildMail(r.Context(), newEmail, spec)); err != nil {
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
	h.applyEmailChange(w, r, userID, email)
}

func (h *Handler) renderEmailChangeCode(w http.ResponseWriter, r *http.Request, errMsg string, status int) {
	data := codeData{flowChrome: h.flowData(r, "titleProfile"), Error: errMsg}
	data.Hint = fmt.Sprintf(data.T["emailChangeCodeHint"], minutesLabel(data.T, signinCodeTTL))
	writeFlow(w, emailChangeCodePage, data, status)
}

// doEmailChangeCode is the link's twin for a mail that carried a code: typed
// in the browser, and the session, that asked for the change.
func (h *Handler) doEmailChangeCode(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userID, email, out := h.takeMailCode(w, r, emailChangeCodePurpose)
	switch {
	case out == codeNoBinding:
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	case out == codeTooMany:
		h.renderEmailChangeCode(w, r, h.tr(r, "errTooManyAttempts"), http.StatusTooManyRequests)
		return
	case out == codeWrong || userID != sess.UserID || email == "":
		h.renderEmailChangeCode(w, r, h.tr(r, "errSigninCode"), http.StatusUnauthorized)
		return
	}
	h.applyEmailChange(w, r, userID, email)
}

// applyEmailChange makes the change once the new address is proved. The
// address is checked again, since somebody may have taken it meanwhile.
func (h *Handler) applyEmailChange(w http.ResponseWriter, r *http.Request, userID, email string) {
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
