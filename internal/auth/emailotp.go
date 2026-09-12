package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
)

// A second factor by e-mail (MFA-02), and a FALLBACK on purpose: the code in an
// inbox is weaker than the one in an authenticator, so it is not a method a
// person enrols in - it is the door for the day they cannot reach their app.
//
// It opens only when all four hold: the account is already enrolled in TOTP,
// the account carries an address, an administrator turned the fallback on, and a
// relay is configured. Miss any one and the link is not even shown - a control
// that appears and then refuses is worse than one that is absent.
//
// The code is six digits, lives ten minutes, is single-use, and is bound to the
// account: what is stored is the hash of the user id AND the code, so a code is
// worthless on another account even if two happen to collide. Sending is
// rate-limited, because a button that mails on demand is a button that floods a
// mailbox on demand.

const (
	// mfaOTPPurpose tags the code rows in email_tokens.
	mfaOTPPurpose = "mfaotp"
	// mfaOTPTTL is how long a mailed code lives - long enough to leave the app,
	// read the mail and type it back, short enough that a leaked inbox from last
	// week opens nothing.
	mfaOTPTTL = 10 * time.Minute
	// mfaOTPResend is the floor between two sends for one account: a second
	// click inside it is answered as if it sent, but mails nothing.
	mfaOTPResend = 45 * time.Second
)

// emailOTPOffered reports whether the fallback may be shown to this account at
// the challenge. userID is the account being challenged.
func (h *Handler) emailOTPOffered(ctx context.Context, userID string) bool {
	if h.Mailer == nil || !h.st.MFAEmailOTPEnabled(ctx) || !h.st.GetSMTP(ctx).Configured() {
		return false
	}
	u, err := h.st.GetUserByID(ctx, userID)
	if err != nil || u.Email == "" {
		return false
	}
	// Fallback only: an account with no TOTP secret is not "cannot reach the
	// app", it is "never set one up", and that is forced enrolment's job, not
	// this one's.
	if _, err := h.st.GetUserTOTP(ctx, userID); err != nil {
		return false
	}
	return true
}

// sendMFAEmail mails a fresh code for the pending challenge. It answers the
// challenge page either way: whether a code was actually sent is not something
// the page reveals, and the send is rate-limited under the covers.
func (h *Handler) sendMFAEmail(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if sess.Pending != stepTOTP {
		http.Error(w, "no second factor is pending on this session", http.StatusForbidden)
		return
	}
	if !h.emailOTPOffered(r.Context(), sess.UserID) {
		http.NotFound(w, r)
		return
	}
	// One send per window: a second click inside the floor renders the "sent"
	// page without mailing, so a held-down button neither floods nor tells the
	// clicker that it was throttled.
	key := "mfaotpsend|" + sess.UserID
	if h.regLimit.count(r.Context(), key, mfaOTPResend) == 0 {
		if err := h.mailMFACode(r, sess.UserID); err != nil {
			// A relay that refused is worth a line, not a leak: the page still
			// says "sent", and the person falls back to their app.
			h.renderChallengeSent(w, r)
			return
		}
		h.regLimit.hit(r.Context(), key)
	}
	h.renderChallengeSent(w, r)
}

// mailMFACode generates, stores and sends one code.
func (h *Handler) mailMFACode(r *http.Request, userID string) error {
	u, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil || u.Email == "" {
		return fmt.Errorf("email otp: no address")
	}
	code, err := numericCode(6)
	if err != nil {
		return err
	}
	// Only the latest code works: clear the pile before storing the new one.
	_ = h.st.ClearEmailTokens(r.Context(), userID, mfaOTPPurpose)
	if err := h.st.PutEmailToken(r.Context(), hashTrust(userID+":"+code), userID, mfaOTPPurpose,
		time.Now().Add(mfaOTPTTL).Unix()); err != nil {
		return err
	}
	t := messagesFor(u.Locale)
	_, brand, _ := h.chrome()
	return h.sendMail(r.Context(), h.buildMail(r.Context(), u.Email, mail.Spec{
		Subject:   fmt.Sprintf(t["mailOtpSubject"], brand.AppName),
		Preheader: t["mailOtpHeading"],
		Heading:   t["mailOtpHeading"],
		Intro:     []string{fmt.Sprintf(t["mailOtpIntro"], brand.AppName)},
		Code:      code,
		CodeNote:  fmt.Sprintf(t["mailOtpNote"], minutesLabel(t, mfaOTPTTL)),
		Outro:     []string{t["mailOtpOutro"]},
	}))
}

// verifyEmailOTP accepts a live mailed code for the account, burning it. It is
// tried alongside the TOTP and the scratch codes at the same challenge input,
// so the person types whatever they have into one box.
func (h *Handler) verifyEmailOTP(ctx context.Context, userID, code string) bool {
	if code == "" {
		return false
	}
	got, err := h.st.TakeEmailToken(ctx, hashTrust(userID+":"+code), mfaOTPPurpose, time.Now().Unix())
	return err == nil && got == userID
}

// numericCode returns n cryptographically random decimal digits, keeping any
// leading zeros - "004821" is a valid code, and a code that silently became
// four digits would be refused with no way for the person to know why.
func numericCode(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[v.Int64()]
	}
	return string(b), nil
}

// minutesLabel renders the TTL the way the note reads it ("10 minutes"), from
// the catalogue so it is not an English word wedged into a translated sentence.
func minutesLabel(t map[string]string, d time.Duration) string {
	m := int(d.Minutes())
	if unit := t["minutes"]; unit != "" {
		return fmt.Sprintf("%d %s", m, unit)
	}
	return fmt.Sprintf("%d min", m)
}

// clearMFAEmail drops any pending mailed code for the account - called when the
// second factor succeeds, so a code left in an inbox cannot be replayed.
func (h *Handler) clearMFAEmail(ctx context.Context, userID string) {
	_ = h.st.ClearEmailTokens(ctx, userID, mfaOTPPurpose)
}
