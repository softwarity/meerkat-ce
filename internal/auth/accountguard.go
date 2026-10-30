package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// The account's own counter (AUTH-11): failed sign-ins on one NAME, whatever
// address they come from.
//
// The counter per address and account (doLogin's loginKey) stops one machine
// guessing. It cannot see a thousand machines guessing a little each - a
// botnet, a pool of rotating proxies - because each address stays under its own
// budget forever. What they all have in common is the name they are trying,
// so that is what this one counts.
//
// Past the threshold the account is NOT locked. A lockout is exactly what an
// attacker wants: type a name five times and its owner cannot work. The
// sign-in for that name asks for the picture code as well - the one the
// registration form already uses - which costs a person three seconds and a
// script its whole economy. The right password and the right code still come
// in, and clear the counter.
//
// Unknown names get the same counter, the same code, the same words. Treating
// them differently would answer the question the sign-in page refuses to
// answer everywhere else (SEC-09): does this account exist.

// The counter's keys. The name is normalised the way loginKey normalises it,
// so "Alice" and "alice" are one target, as they are to whoever types them.
// The per-address rows hash the name: what follows the prefix is matched as a
// prefix, and a name that contained "|" would otherwise reach into its
// neighbour's rows.
func accountKey(name string) string        { return "acct|" + name }
func accountAddrPrefix(name string) string { return "acctip|" + hashTrust(name) + "|" }
func accountMailKey(name string) string    { return "acctmail|" + name }

// accountChallenged says whether a sign-in for this name must carry the
// picture code: the name has collected the policy's failures in the window.
func (h *Handler) accountChallenged(ctx context.Context, pol store.RateLimitPolicy, window time.Duration, name string) bool {
	return pol.AccountAttempts > 0 && h.regLimit.count(ctx, accountKey(name), window) >= pol.AccountAttempts
}

// accountFailed counts one failure against the name and the address it came
// from, and reports whether the name is now challenged. The failure that
// crosses the threshold writes one line of the security trail and warns the
// owner; the ones after it write nothing - a botnet at a thousand tries a
// second must not become a thousand rows a second, nor a thousand mails.
func (h *Handler) accountFailed(r *http.Request, pol store.RateLimitPolicy, window time.Duration,
	name string, u store.User, found bool) bool {
	if pol.AccountAttempts <= 0 {
		return false
	}
	ctx := r.Context()
	h.regLimit.hit(ctx, accountKey(name))
	h.regLimit.hit(ctx, accountAddrPrefix(name)+clientIP(r))
	n := h.regLimit.count(ctx, accountKey(name), window)
	if n == pol.AccountAttempts {
		h.security(r, secSigninLocked, u, refusedAccountThrottled)
		if found {
			h.alertOwner(r, u, name, window)
		}
	}
	return n >= pol.AccountAttempts
}

// accountForgive clears the name's counter and its addresses: the owner just
// proved who they are.
func (h *Handler) accountForgive(ctx context.Context, name string) {
	h.regLimit.reset(ctx, accountKey(name))
	if err := h.st.ClearAttemptsPrefix(ctx, accountAddrPrefix(name)); err != nil {
		slog.Warn("account attempts not cleared", "err", err)
	}
}

// alertOwner mails the owner of an account that just crossed the threshold:
// how many failures, from how many addresses, and what to do if it was not
// them. Once per window, whatever happens after - a second mail tells them
// nothing the first did not, and a thousand would be a way to flood an inbox
// through us.
//
// OFF the request, and that is the anti-enumeration rule again rather than a
// matter of speed: only a real account with an address gets a mail, and a
// failure that waited for an SMTP relay would answer measurably later than the
// same failure on a name that does not exist.
func (h *Handler) alertOwner(r *http.Request, u store.User, name string, window time.Duration) {
	// Nobody to tell, or nobody worth telling: an address a self-registered
	// account never confirmed may be somebody else's, and a disabled account
	// cannot act on the advice.
	if u.Email == "" || !u.Enabled || (u.SelfRegistered && !u.EmailVerified) || h.Mailer == nil {
		return
	}
	ctx := context.WithoutCancel(r.Context())
	h.pending.Add(1)
	go func() {
		defer h.pending.Done()
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if !h.st.GetSMTP(ctx).Configured() {
			return
		}
		if !h.regLimit.allow(ctx, accountMailKey(name), 1, window) {
			return
		}
		n := h.regLimit.count(ctx, accountKey(name), window)
		addrs, err := h.st.CountAttemptKeys(ctx, accountAddrPrefix(name), time.Now().Add(-window))
		if err != nil || addrs < 1 {
			addrs = 1
		}
		t := messagesFor(u.Locale)
		_, brand, _ := h.chrome()
		msg := h.buildMail(ctx, u.Email, signinAttemptsSpec(t, brand.AppName, n, addrs, minutesLabel(t, window)))
		if err := h.sendMail(ctx, msg); err != nil {
			slog.Warn("failed sign-ins notice not sent", "user", u.Username, "err", err)
		}
	}()
}

// signinAttemptsSpec is the notice, written once for the real mail and the
// relay screen's sample so the two never word it differently. Counts as
// "label: number" lines rather than sentences: a plural in twenty-one languages is
// twenty-one grammars, and "1 addresses" is the kind of line that makes a security
// mail read like spam.
func signinAttemptsSpec(t map[string]string, app string, failures, addrs int, window string) mail.Spec {
	subject := fmt.Sprintf(t["mailSigninAttemptsSubject"], app)
	return mail.Spec{
		Subject:   subject,
		Preheader: subject,
		Heading:   subject,
		Intro: []string{
			fmt.Sprintf(t["mailSigninAttemptsIntro"], app),
			fmt.Sprintf(t["mailSigninAttemptsCount"], window, failures),
			fmt.Sprintf(t["mailSigninAttemptsAddresses"], addrs),
		},
		Outro: []string{t["mailSigninAttemptsOutro"]},
	}
}

// doLoginCaptcha hands the login page's refresh button a fresh image - the
// registration's endpoint, for the sign-in. Open whenever the account counter
// is on, since which names are challenged is precisely what it must not say.
func (h *Handler) doLoginCaptcha(w http.ResponseWriter, r *http.Request) {
	if h.st.GetRateLimitPolicy(r.Context()).AccountAttempts <= 0 {
		http.NotFound(w, r)
		return
	}
	h.writeCaptcha(w, r)
}
