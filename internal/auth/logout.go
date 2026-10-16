package auth

import (
	"log/slog"
	"net/http"
	"strings"
)

// Signing out, by POST from our own pages and by GET from the applications
// behind the gateway.
//
// The POST is what the profile and the user button send, and it stays the
// only way on the control plane. The GET exists for the applications: one
// served behind Meerkat signs in through it and has a "sign out" button of its
// own, and the only thing that button can do is send the browser to a sign-out
// address - a link or a redirect, never a form posted to somebody else's page.
// Sending it to the gateway's /logout is what ends the session that actually
// holds the person in, the gateway's; ending only the application's own would
// leave them signed in one click away.
//
// A GET that changes state is what a forged link, an image tag or a prefetch
// abuses, so who sent it decides what it does:
//   - a page of this site (Sec-Fetch-Site same-origin or same-site, which is
//     how a browser stamps an application served on this host or on a
//     neighbouring subdomain) signs out at once, exactly as the POST does;
//   - anybody else - another site, an address typed or bookmarked ("none"), a
//     client that says nothing - gets a page asking first, whose button is the
//     ordinary POST. A sign-out forged from elsewhere costs a click, not a
//     session.
//
// Same-site is let through here though the write guard refuses it on every
// POST (guard.go): there it would let a neighbour WRITE in somebody's name,
// here the worst it can do is end a session, which the person reopens by
// signing in. Refusing it would break the very applications this exists for,
// the ones served on their own subdomain.
//
// Either way the person lands on the sign-in page. No address to go to is
// ever read from the request: a sign-out that redirects where it is told is
// an open redirect with a session-ending side effect.

var signOutPage = flowPage("signout-confirm", signOutBody)

const signOutBody = `    <form method="post" action="/logout">
      <p class="lead">{{.T.signOutAsk}}</p>
      <p class="hint">{{.T.signOutLead}}</p>
      <button type="submit">{{.T.signOut}}</button>
    </form>
    <p class="back"><a href="/profile">{{.T.backToProfile}}</a></p>
`

func (h *Handler) doLogout(w http.ResponseWriter, r *http.Request) {
	h.signOut(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// showLogout is the data plane's GET /logout (see above).
func (h *Handler) showLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, err := h.sm.Resolve(r.Context(), r); err != nil {
		// Nobody to sign out: nothing to confirm either.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !signOutAtOnce(r) {
		writeFlow(w, signOutPage, struct{ flowChrome }{h.flowData(r, "titleSignOut")}, http.StatusOK)
		return
	}
	h.signOut(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// signOutAtOnce says whether a GET /logout comes from a page of this site,
// followed by a person. A browser fetching it ahead of a click (prefetch,
// speculation rules) has not been asked anything yet: it gets the question,
// and the click that may never come stays the person's.
func signOutAtOnce(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Sec-Purpose"), "prefetch") || r.Header.Get("Purpose") == "prefetch" {
		return false
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "same-site":
		return true
	}
	return false
}

// signOut ends the request's session and says so in the security trail.
func (h *Handler) signOut(w http.ResponseWriter, r *http.Request) {
	if sess, err := h.sm.Resolve(r.Context(), r); err == nil {
		h.securityOfIn(r, secSignout, sess.UserID, sess.TenantID, "")
	}
	if err := h.sm.Destroy(r.Context(), w, r); err != nil {
		slog.Error("logout failed", "err", err)
	}
}
