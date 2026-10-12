package auth

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/filters"
)

// WHERE A MAILED LINK MAY POINT, and what is mailed when nowhere is safe.
//
// A reset, a registration and an address change each mail something that
// proves the mailbox. It used to be a link built from the request's Host -
// and the Host is whatever the caller wrote. `POST /forgot-password` for
// somebody else's address with `Host: attacker.example` mailed that person a
// genuine message whose button carried a genuine token to the attacker: one
// click, and the account was theirs.
//
// So a link is mailed only when the gateway can vouch for the name in it: the
// request reached ITS OWN TLS door, and a certificate placed on that plane
// covers the Host. Somebody who can make a browser trust that certificate for
// that name is the operator. Anything else - plain HTTP, a proxy in front
// that terminated TLS, a Host no certificate carries - and the mail carries a
// CODE instead, and the page that asked waits for it, in that browser only.
// A code mailed to the victim is worthless to whoever asked: they never see
// it, and it opens nothing in another browser.
//
// The code is the one the e-mail sign-in uses (AUTH-16): six digits, ten
// minutes, bound to the browser by an HttpOnly cookie, a few tries.
//
// Behind a proxy that terminates TLS - nginx, an ingress, a cloud balancer -
// the first rule never holds: the request reaches the gateway in clear, and
// every mail would carry a code. So the operator can vouch for a name the
// gateway cannot: one declared, on that door, as reached through such a
// proxy (Domains & TLS, certs.Settings.ProxiedApp). The Host is still what
// the caller wrote, and that is fine: a forged Host that matches a declared
// name sends the link to OUR name, which is where the victim would have gone
// anyway. What a declared name does not need is r.TLS - the HTTPS is the
// proxy's - and what it never gets is anything but https: the operator said
// somebody serves it over HTTPS, so http:// is never written.

// linkOrigin is "https://<host>" when a mailed link may point at the host
// this request named, and "" when the mail must carry a code. The host keeps
// the port the request carried: the one the browser typed, which a proxy that
// passes the Host on passes on too.
func (h *Handler) linkOrigin(r *http.Request) string {
	host := r.Host
	name := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		name = hh
	}
	name = strings.Trim(strings.ToLower(name), "[]")
	if name == "" {
		return ""
	}
	if r.TLS != nil && h.Covers != nil && h.Covers(name) {
		return "https://" + host
	}
	if h.Proxied != nil && h.Proxied(name) {
		return "https://" + host
	}
	return ""
}

const (
	mailCodeCookie = "MEERKAT_MAILCODE"
	// What each code stands for: one purpose per flow, so a reset code never
	// confirms an address and the other way round.
	resetCodePurpose       = "reset-code"
	confirmCodePurpose     = "confirm-code"
	emailChangeCodePurpose = "email-change-code"
)

// bindBrowser gives this browser the request id its code will be bound to.
// Set whatever happens next - an account found or not - so the answer says
// nothing about the address.
func (h *Handler) bindBrowser(w http.ResponseWriter, r *http.Request) (string, error) {
	reqID, err := randToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name: mailCodeCookie, Value: reqID, Path: "/", HttpOnly: true,
		Secure: filters.Secure(r), SameSite: http.SameSiteLaxMode,
		MaxAge: int(signinCodeTTL.Seconds()),
	})
	return reqID, nil
}

// putMailCode stores a fresh code for the account and the purpose, bound to
// the browser's request id, and returns it to be mailed. Only the latest code
// of a purpose works.
func (h *Handler) putMailCode(r *http.Request, reqID, userID, purpose, payload string) (string, error) {
	code, err := numericCode(6)
	if err != nil {
		return "", err
	}
	_ = h.st.ClearEmailTokens(r.Context(), userID, purpose)
	if err := h.st.PutEmailTokenWith(r.Context(), hashTrust(reqID+":"+code), userID, purpose, payload,
		time.Now().Add(signinCodeTTL).Unix()); err != nil {
		return "", err
	}
	return code, nil
}

// codeOutcome is what checking a typed code came to.
type codeOutcome int

const (
	codeTaken     codeOutcome = iota // right: spent, the account and payload returned
	codeWrong                        // wrong or expired
	codeTooMany                      // this browser has tried too often
	codeNoBinding                    // no cookie: not the browser that asked
)

// takeMailCode spends the code typed in this browser, for this purpose.
func (h *Handler) takeMailCode(w http.ResponseWriter, r *http.Request, purpose string) (userID, payload string, out codeOutcome) {
	c, err := r.Cookie(mailCodeCookie)
	if err != nil || c.Value == "" {
		return "", "", codeNoBinding
	}
	tries := "mailcodetry|" + c.Value
	if h.regLimit.count(r.Context(), tries, signinCodeTTL) >= signinMaxTries {
		return "", "", codeTooMany
	}
	code := strings.TrimSpace(r.PostFormValue("code"))
	userID, payload, err = h.st.TakeEmailTokenPayload(r.Context(), hashTrust(c.Value+":"+code), purpose, time.Now().Unix())
	if err != nil || userID == "" {
		h.regLimit.hit(r.Context(), tries)
		return "", "", codeWrong
	}
	h.regLimit.reset(r.Context(), tries)
	http.SetCookie(w, &http.Cookie{
		Name: mailCodeCookie, Value: "", Path: "/", HttpOnly: true,
		Secure: filters.Secure(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	return userID, payload, codeTaken
}
