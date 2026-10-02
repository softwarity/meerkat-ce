package auth

import (
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/store"
)

// passwordCost is the bcrypt cost every local password is hashed at (SEC-05).
// ONE place, because raising it is the whole upgrade: a hash stored at a lower
// cost is re-hashed at its owner's next sign-in (rehashIfWeak), the only moment
// the clear password is known. Machines get faster; a stolen table should keep
// costing what it cost the day the passwords were set, at least.
const passwordCost = bcrypt.DefaultCost

// rehashIfWeak re-hashes a password just proved against a weaker hash. Best
// effort, invisible to the person signing in: a failure leaves the old hash,
// which still works, and the next sign-in tries again.
func (h *Handler) rehashIfWeak(r *http.Request, u store.User, password string) {
	cost, err := bcrypt.Cost([]byte(u.PasswordHash))
	if err != nil || cost >= passwordCost {
		return
	}
	stronger, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		return
	}
	if _, err := h.st.RehashUserPassword(r.Context(), u.ID, u.PasswordHash, string(stronger)); err != nil {
		slog.Warn("password rehash failed", "user", u.Username, "err", err)
	}
}

// The security trail of an account (AUD-01): who signed in, who was refused,
// and what changed about the ways in - a password, a second factor, a passkey,
// a token. Written into the same table as the administrative trail, under two
// kinds of its own (store.AuditTargetAccount, store.AuditTargetConsole), so the
// console shows both halves on one screen and one filter separates them.
//
// It is a RECORD, not a debugging aid: best-effort like the rest of the trail
// (a sign-in never fails because its line could not be written), and it never
// says more than the person on the other side was told. A refusal carries the
// real reason here - "disabled", "bad-credentials" - because the reader is an
// administrator, while the page kept saying the same thing for every case.

// The actions, as the trail names them.
const (
	secSignin           = "signin"
	secSigninTenant     = "signin.organisation"
	secSigninRefused    = "signin.refused"
	secSigninLocked     = "signin.locked"
	secSignout          = "signout"
	secPasswordChange   = "password.change"
	secPasswordForgot   = "password.forgot"
	secPasswordReset    = "password.reset"
	secMFAEnroll        = "mfa.enroll"
	secMFARemove        = "mfa.remove"
	secPasskeyAdd       = "passkey.add"
	secPasskeyRemove    = "passkey.remove"
	secEmailChange      = "email.change"
	secEmailChangeAsked = "email.change.request"
	secTokenCreate      = "token.create"
	secTokenRevoke      = "token.revoke"
	secRegister         = "register"
	secRegisterConfirm  = "register.confirm"
	secDevKeyAdd        = "devkey.add"
	secDevKeyRemove     = "devkey.remove"
)

// The reasons a sign-in is refused, as the trail names them.
const (
	refusedCredentials = "bad-credentials"
	refusedDisabled    = "disabled"
	refusedValidity    = "outside-validity"
	refusedUnconfirmed = "unconfirmed"
	refusedTemporary   = "temporary-expired"
	refusedHours       = "outside-hours"
	refusedCode        = "bad-code"
	refusedThrottled   = "throttled"
)

// security writes one event about an account. u may be a bare account carrying
// only the name that was TYPED, for a refusal of somebody unknown: the actor is
// then nobody, and the name is what was tried.
func (h *Handler) security(r *http.Request, action string, u store.User, detail string) {
	h.securityIn(r, action, u, "", detail)
}

// securityIn is security for an event that belongs to an organisation: a
// sign-in into it, a refusal by its hours. Stamped with it, the line is read
// by that organisation's administrators too - the trail partitions by
// tenant_id - and a person who belongs to three says which one they entered.
func (h *Handler) securityIn(r *http.Request, action string, u store.User, tenantID, detail string) {
	target := store.AuditTargetAccount
	if h.adminPlane {
		target = store.AuditTargetConsole
	}
	name := u.Username
	if len(name) > 100 {
		// A name field is a free text box, and the trail must stay bounded
		// whatever somebody pastes into it.
		name = name[:100]
	}
	ev := store.AuditEvent{
		ActorID: u.ID, Action: action, Target: target,
		TargetID: u.ID, TargetName: name, TenantID: tenantID, Detail: detail, IP: clientIP(r),
	}
	if err := h.st.AddAuditEvent(r.Context(), ev); err != nil {
		slog.Error("security trail write failed", "action", action, "user", name, "err", err)
	}
}

// securityOf is security for an account known only by its id.
func (h *Handler) securityOf(r *http.Request, action, userID, detail string) {
	h.securityOfIn(r, action, userID, "", detail)
}

// securityOfIn is securityIn for an account known only by its id.
func (h *Handler) securityOfIn(r *http.Request, action, userID, tenantID, detail string) {
	u, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil {
		u = store.User{ID: userID}
	}
	h.securityIn(r, action, u, tenantID, detail)
}
