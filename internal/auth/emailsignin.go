package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/mail"
)

// Signing in with a code mailed to the address (AUTH-16): no password typed,
// no password remembered.
//
// It is a PASSWORD replacement and never a second factor. The MFA step runs
// afterwards exactly as it does behind a password, because a mailed code and a
// mailed reset link travel the same channel: someone holding the mailbox would
// otherwise hold both halves.
//
// THE MAILBOX BECOMES THE CREDENTIAL, so the frame around it is the feature:
//
//   - DATA PLANE ONLY. The console is never openable by a mailbox, whoever the
//     account belongs to - the whole flow is simply not mounted there.
//   - SHIPS OFF, and needs a relay. An administrator turns it on knowing what
//     it moves.
//   - BOUND TO THE BROWSER that asked. What is stored is the hash of a random
//     request id - held in an HttpOnly cookie - AND the code, so a code read
//     out over the telephone opens nothing on the caller's machine. This is
//     the difference between a one-time code and a password mailed in clear.
//   - Six digits, ten minutes, single use, one live code per account.
//   - No enumeration: the same page and the same timing whatever the address
//     is, and the cookie is set even when no account matched.
//   - Throttled twice: sending, per address, and guessing, per browser.
const (
	signinPurpose = "signin"
	// Long enough to leave the browser, read the mail and come back; short
	// enough that yesterday's inbox is worthless.
	signinCodeTTL = 10 * time.Minute
	// The floor between two sends for one address. A second ask inside it is
	// answered as if it sent - a button that mails on demand is a button that
	// floods a mailbox on demand.
	signinResend = 45 * time.Second
	// How many wrong codes one browser may try before the request is dead. Six
	// digits is a million, so this is not what stops a machine - what stops a
	// machine is the ten minutes. This stops a person guessing over someone's
	// shoulder.
	signinMaxTries = 5
	// How many addresses one source may ask about in a window. This is
	// anti-SPRAY, not anti-brute-force: the ten minutes and the six digits
	// answer guessing, this answers someone walking a list of addresses.
	//
	// It is deliberately NOT the registration counter, which the first
	// version borrowed. That budget is five per quarter of an hour and is
	// shared with sign-up and password reset, so a morning of sign-ups would
	// have silently closed the sign-in door - two different doors spending one
	// budget. The integration suite found it by running the three flows in a
	// row from one address, which is exactly what an office does.
	signinAskPerIP  = 20
	signinAskWindow = 15 * time.Minute
	signinCookie    = "MEERKAT_SIGNIN"
)

var (
	signinCodePage = flowPage("signin-code", signinCodeBody)
	signinSentPage = flowPage("signin-sent", signinSentBody)
)

type signinCodeData struct {
	flowChrome
	Next  string
	Error string
}

type signinSentData struct {
	flowChrome
	Next  string
	Hint  string
	Error string
}

const signinCodeBody = `    <form method="post" action="/login/code">
      <p class="lead">{{.T.signinCodeLead}}</p>
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <p class="hint">{{.T.signinCodeHint}}</p>
      <label class="field">
        <span>{{.T.email}}</span>
        <input name="email" type="email" autocomplete="email" autofocus required>
      </label>
      <input type="hidden" name="next" value="{{.Next}}">
      <button type="submit">{{.T.signinCodeCta}}</button>
    </form>
    <p class="back"><a href="/login">{{.T.backToLogin}}</a></p>
`

const signinSentBody = `    <form method="post" action="/login/code/verify">
      <p class="lead">{{.T.registerSentLead}}</p>
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <p class="hint">{{.Hint}}</p>
      <label class="field">
        <span>{{.T.authCode}}</span>
        <input name="code" inputmode="numeric" autocomplete="one-time-code" autofocus required>
      </label>
      <input type="hidden" name="next" value="{{.Next}}">
      <button type="submit">{{.T.signIn}}</button>
    </form>
    <p class="back"><a href="/login/code">{{.T.signinCodeAgain}}</a></p>
`

// emailSigninOpen says whether the door is open at all. Four conditions, and
// the first is not a setting: the control plane never offers this.
func (h *Handler) emailSigninOpen(r *http.Request) bool {
	return !h.adminPlane && h.Mailer != nil &&
		h.st.EmailSigninEnabled(r.Context()) && h.st.GetSMTP(r.Context()).Configured()
}

func (h *Handler) showEmailSignin(w http.ResponseWriter, r *http.Request) {
	if !h.emailSigninOpen(r) {
		http.NotFound(w, r)
		return
	}
	writeFlow(w, signinCodePage, signinCodeData{
		flowChrome: h.flowData(r, "titleSigninCode"),
		Next:       r.URL.Query().Get("next"),
	}, http.StatusOK)
}

// doEmailSignin mails a code, or does not, and renders the same page either
// way. The cookie is set in both cases: a browser that got no cookie would
// have been told the address is unknown.
func (h *Handler) doEmailSignin(w http.ResponseWriter, r *http.Request) {
	if !h.emailSigninOpen(r) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	next := r.PostFormValue("next")
	if email == "" {
		writeFlow(w, signinCodePage, signinCodeData{
			flowChrome: h.flowData(r, "titleSigninCode"), Next: next, Error: h.tr(r, "errBadEmail"),
		}, http.StatusUnprocessableEntity)
		return
	}
	if !h.regLimit.allow(r.Context(), "signinask|"+clientIP(r), signinAskPerIP, signinAskWindow) {
		writeFlow(w, signinCodePage, signinCodeData{
			flowChrome: h.flowData(r, "titleSigninCode"), Next: next, Error: h.tr(r, "errTooManyAttempts"),
		}, http.StatusTooManyRequests)
		return
	}

	reqID, err := randToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: signinCookie, Value: reqID, Path: "/", HttpOnly: true,
		Secure: filters.Secure(r), SameSite: http.SameSiteLaxMode,
		MaxAge: int(signinCodeTTL.Seconds()),
	})

	// One send per window and per ADDRESS, not per account: the throttle has
	// to work before we know whether an account is behind it.
	key := "signin|" + strings.ToLower(email)
	if h.regLimit.count(r.Context(), key, signinResend) == 0 {
		if userID, err := h.st.UserIDByEmail(r.Context(), email); err == nil {
			if u, err := h.st.GetUserByID(r.Context(), userID); err == nil &&
				u.Enabled && (!u.SelfRegistered || u.EmailVerified) && u.ValidityReason(time.Now()) == "" {
				if err := h.mailSigninCode(r, u.ID, u.Email, u.Locale, reqID); err != nil {
					slog.Error("sign-in code e-mail failed", "user", u.Username, "err", err)
				}
			}
		}
		h.regLimit.hit(r.Context(), key)
	}
	h.renderSigninSent(w, r, next, "", http.StatusOK)
}

func (h *Handler) renderSigninSent(w http.ResponseWriter, r *http.Request, next, errMsg string, status int) {
	data := signinSentData{flowChrome: h.flowData(r, "titleSigninCode"), Next: next, Error: errMsg}
	data.Hint = fmt.Sprintf(data.T["signinCodeSentHint"], minutesLabel(data.T, signinCodeTTL))
	writeFlow(w, signinSentPage, data, status)
}

// mailSigninCode stores one code for the account, bound to the browser that
// asked, and mails it. The stored hash carries the REQUEST ID, so the code is
// worthless anywhere else - including in the hands of whoever talked the
// account holder into reading it out.
func (h *Handler) mailSigninCode(r *http.Request, userID, email, locale, reqID string) error {
	code, err := numericCode(6)
	if err != nil {
		return err
	}
	// Only the latest code works.
	_ = h.st.ClearEmailTokens(r.Context(), userID, signinPurpose)
	if err := h.st.PutEmailToken(r.Context(), hashTrust(reqID+":"+code), userID, signinPurpose,
		time.Now().Add(signinCodeTTL).Unix()); err != nil {
		return err
	}
	if locale == "" {
		locale = prefsOf(r, h.offeredLanguages()).Lang
	}
	t := messagesFor(locale)
	_, brand, _ := h.chrome()
	return h.sendMail(r.Context(), h.buildMail(r.Context(), email, mail.Spec{
		Subject:   fmt.Sprintf(t["mailOtpSubject"], brand.AppName),
		Preheader: t["mailOtpHeading"],
		Heading:   t["mailOtpHeading"],
		Intro:     []string{fmt.Sprintf(t["mailOtpIntro"], brand.AppName)},
		Code:      code,
		CodeNote:  fmt.Sprintf(t["mailOtpNote"], minutesLabel(t, signinCodeTTL)),
		Outro:     []string{t["mailOtpOutro"]},
	}))
}

// doEmailSigninVerify burns a live code and continues the login exactly where
// a correct password would: the forced password change, then the second
// factor, then the organisation.
func (h *Handler) doEmailSigninVerify(w http.ResponseWriter, r *http.Request) {
	if !h.emailSigninOpen(r) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	next := r.PostFormValue("next")
	c, err := r.Cookie(signinCookie)
	if err != nil || c.Value == "" {
		// No cookie, no request: the code belongs to a browser, and this is
		// not it. Start again rather than explain.
		http.Redirect(w, r, "/login/code", http.StatusSeeOther)
		return
	}
	tries := "signintry|" + c.Value
	if h.regLimit.count(r.Context(), tries, signinCodeTTL) >= signinMaxTries {
		h.renderSigninSent(w, r, next, h.tr(r, "errTooManyAttempts"), http.StatusTooManyRequests)
		return
	}
	code := strings.TrimSpace(r.PostFormValue("code"))
	userID, err := h.st.TakeEmailToken(r.Context(), hashTrust(c.Value+":"+code), signinPurpose, time.Now().Unix())
	if err != nil || userID == "" {
		h.regLimit.hit(r.Context(), tries)
		h.renderSigninSent(w, r, next, h.tr(r, "errSigninCode"), http.StatusUnauthorized)
		return
	}
	user, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil || !user.Enabled {
		h.renderSigninSent(w, r, next, h.tr(r, "errSigninCode"), http.StatusUnauthorized)
		return
	}
	// The window may have closed since the code was mailed. Told with its
	// date, like the password path: the person proved they hold the mailbox,
	// so there is nothing left to enumerate.
	if why := user.ValidityReason(time.Now()); why != "" {
		key, when := "errAccessEnded", user.ValidUntil
		if time.Now().Unix() < user.ValidFrom {
			key, when = "errAccessNotOpen", user.ValidFrom
		}
		h.renderSigninSent(w, r, next,
			fmt.Sprintf(h.tr(r, key), time.Unix(when, 0).UTC().Format(time.DateOnly)),
			http.StatusForbidden)
		return
	}
	// Spent: the cookie has nothing left to bind.
	http.SetCookie(w, &http.Cookie{
		Name: signinCookie, Value: "", Path: "/", HttpOnly: true,
		Secure: filters.Secure(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	h.regLimit.reset(r.Context(), tries)
	h.continueAfterCredential(w, r, user, next, loginMethodEmailCode)
}
