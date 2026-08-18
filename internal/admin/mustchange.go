package admin

import (
	"errors"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// Force a password change at the next sign-in (AUTH-10), for one account or
// for the whole installation.
//
// Distinct from a RESET on purpose: a reset replaces the password with a
// temporary one that has to be carried to its owner somehow, which is a phone
// call per person. This keeps the password everyone already knows and simply
// refuses to go further with it - the person signs in as usual and is sent
// straight to the change page.
//
// LOCAL accounts only, and the count says how many were touched. An account
// born at a directory or an OIDC provider has no password here to change:
// flagging it would send someone to a form that could not help them, in front
// of an authority that would let them in anyway.

// putMustChangePassword flags ONE account.
func (a *API) putMustChangePassword(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	target, err := a.st.GetUserByID(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if target.PasswordHash == "" {
		writeErr(w, http.StatusUnprocessableEntity,
			"this account has no password here: it signs in through its authority, which owns its credentials")
		return
	}
	if err := a.st.SetMustChangePassword(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "user.must-change-password", "user", id, target.Username, "",
		"must change password at next sign-in")
	writeJSON(w, http.StatusOK, map[string]int{"users": 1})
}

// postMustChangePasswordAll flags every local account - the answer to a leak,
// where the question is not which account but all of them. Root only: it is
// the one action here that reaches every person on the installation at once.
//
// The ACTOR is spared. Being signed out of the console by one's own click, in
// the middle of an incident, is the wrong lesson at the wrong moment - and the
// admin can flag themselves from the list like anyone else.
func (a *API) postMustChangePasswordAll(w http.ResponseWriter, r *http.Request, actor store.User) {
	n, err := a.st.SetMustChangePasswordForAll(r.Context(), actor.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "users.must-change-password", "users", "", "", "",
		"every local account must change its password at next sign-in")
	writeJSON(w, http.StatusOK, map[string]int{"users": int(n)})
}
