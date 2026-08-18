package auth

import (
	"log/slog"
	"net/http"

	"github.com/softwarity/meerkat/internal/idp"
	"github.com/softwarity/meerkat/internal/store"
)

// Connecting an authority to the account one is ALREADY signed in to.
//
// The alternative was matching on a verified e-mail address, which is what a
// first sign-in still tries: an authority vouches for an address, an account
// holds it, they are joined. It works right up to the moment someone's address
// at the authority is not the one an administrator typed here - and then there
// is nothing anybody can do about it, on either side.
//
// This closes it from the other end. The person signs in with what they have,
// goes through the authority's own dance, and comes back attached. No address
// to match, no confirmation mail, no address anyone could claim on someone
// else's behalf: the session says who this is, and the authority says who they
// are over there. Both halves are proved at the moment they are joined.

// showConnect starts the authority's flow in LINK mode. Same dance as a
// sign-in, and deliberately so - a second implementation of an OAuth redirect
// is a second place for a state check to be forgotten.
func (h *Handler) showConnect(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id := r.PathValue("provider")
	driver, _, err := h.driverFor(r.Context(), id)
	if err != nil {
		slog.Error("connect unavailable", "provider", id, "err", err)
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errSignInUnavailable"), http.StatusBadGateway)
		return
	}
	redirect, ok := driver.(idp.Redirect)
	if !ok {
		// A directory is asked with a username and a password, not with a
		// round trip: there is no page to send anyone to.
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errSignInUnavailable"), http.StatusBadRequest)
		return
	}
	req := idp.AuthRequest{
		State: randToken32(), Nonce: randToken32(), Verifier: randToken32(),
		RedirectURI: h.callbackURL(r, id), ProviderID: id,
	}
	url, err := redirect.AuthURL(req)
	if err != nil {
		slog.Error("authorization URL failed", "provider", id, "err", err)
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errSignInUnavailable"), http.StatusBadGateway)
		return
	}
	h.setAuthState(w, r, authState{Req: req, Next: "/profile/authorities", Link: true})
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// completeConnect attaches what the authority just said to the account the
// caller is signed in as. Called from the ordinary callback when the state
// says LINK - the state is signed and short-lived, so the mode cannot be
// flipped by whoever comes back.
func (h *Handler) completeConnect(w http.ResponseWriter, r *http.Request,
	p store.AuthProvider, identity idp.Identity) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		// The session died while the person was away at the authority. Nothing
		// to attach to, and attaching to whoever signs in next would be the
		// one mistake this whole flow exists to avoid.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if identity.Subject == "" {
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errSignInUnavailable"), http.StatusBadGateway)
		return
	}
	// Already someone else's: an authority account identifies ONE person here,
	// or the next sign-in through it would be a coin toss.
	if other, err := h.st.UserByIdentity(r.Context(), p.ID, identity.Subject); err == nil && other.ID != sess.UserID {
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errIdentityTaken"), http.StatusConflict)
		return
	}
	if err := h.st.LinkIdentity(r.Context(), p.ID, identity.Subject, sess.UserID, identity.Groups); err != nil {
		slog.Error("identity link failed", "provider", p.ID, "err", err)
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errSignInUnavailable"), http.StatusInternalServerError)
		return
	}
	// What the authority says about the person's groups is worth applying at
	// once (RBAC-10), exactly as a sign-in through it would.
	if _, err := h.st.SyncMappedGroups(r.Context(), sess.UserID, p.ID, identity.Groups); err != nil {
		slog.Error("group rules could not be applied", "provider", p.ID, "err", err)
	}
	slog.Info("authority connected", "provider", p.ID, "user", sess.UserID)
	http.Redirect(w, r, "/profile/authorities", http.StatusSeeOther)
}

// doDisconnect detaches an authority from the caller's own account.
//
// Refused when it is the LAST way in: an account with no local password whose
// only identity is this one would be locked out by its own click, and the page
// offering the click is behind the login it just closed.
func (h *Handler) doDisconnect(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	provider := r.PathValue("provider")
	u, err := h.st.GetUserByID(r.Context(), sess.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	links, err := h.st.IdentitiesOfUser(r.Context(), sess.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var subject string
	for _, l := range links {
		if l.ProviderID == provider {
			subject = l.ExternalID
		}
	}
	if subject == "" {
		http.Redirect(w, r, "/profile/authorities", http.StatusSeeOther)
		return
	}
	if u.PasswordHash == "" && len(links) == 1 {
		h.renderAuthorities(w, r, sess.UserID, h.tr(r, "errLastWayIn"), http.StatusUnprocessableEntity)
		return
	}
	if _, err := h.st.UnlinkIdentity(r.Context(), provider, subject); err != nil {
		slog.Error("identity unlink failed", "provider", provider, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("authority disconnected", "provider", provider, "user", sess.UserID)
	http.Redirect(w, r, "/profile/authorities", http.StatusSeeOther)
}

// authorityLinks lists the redirect authorities someone may connect to their
// own account, each already answered. Directories are left out: they are asked
// with a username and a password, so there is no round trip to send anyone on
// - what links them is signing in with them.
func (h *Handler) authorityLinks(r *http.Request, userID string) []authorityLink {
	all, err := h.st.ListAuthProviders(r.Context())
	if err != nil {
		slog.Error("auth providers lookup failed", "err", err)
		return nil
	}
	links, err := h.st.IdentitiesOfUser(r.Context(), userID)
	if err != nil {
		slog.Error("identities lookup failed", "err", err)
		return nil
	}
	connected := make(map[string]bool, len(links))
	for _, l := range links {
		connected[l.ProviderID] = true
	}
	u, err := h.st.GetUserByID(r.Context(), userID)
	if err != nil {
		return nil
	}
	// The only way in: no local password and this one link. Answered here so
	// the page can say why the button is not offered, rather than offer it and
	// refuse afterwards.
	sole := u.PasswordHash == "" && len(links) == 1

	var out []authorityLink
	for _, p := range all {
		if !p.Enabled {
			continue
		}
		switch p.Kind {
		case store.ProviderOIDC, store.ProviderSAML, store.ProviderGitHub:
		default:
			continue
		}
		out = append(out, authorityLink{
			ID: p.ID, Name: p.Name, Connected: connected[p.ID],
			Sole: sole && connected[p.ID],
		})
	}
	return out
}

var profileAuthoritiesPage = flowPage("profile-authorities", profileAuthoritiesBody)

type profileAuthoritiesData struct {
	flowChrome
	Error       string
	Authorities []authorityLink
}

// The page, on the model of the sign-in history: a panel that carries its own
// title, one row per authority, and a way back. It used to be a section on the
// Security page, squeezed between the API tokens and the passkeys, where it
// read as a stray line rather than as something one manages.
const profileAuthoritiesBody = `    <style>
      .au-name { flex: 1; min-width: 0; font-size: .9rem; text-align: start; }
      .au-on {
        font-family: var(--mk-mono); font-size: .58rem; letter-spacing: .12em;
        text-transform: uppercase; color: var(--mk-primary);
        padding: 3px 9px; border-radius: 999px; white-space: nowrap;
        background: color-mix(in srgb, var(--mk-primary) 12%, transparent);
      }
      .au-sole { font-size: .68rem; color: var(--mk-on-surface-variant); white-space: nowrap; }
      .au-go, .row button {
        margin: 0; font-size: .78rem; text-decoration: none; white-space: nowrap;
        padding: 5px 14px; border-radius: 999px; width: auto;
        border: 1px solid color-mix(in srgb, var(--mk-primary) 45%, transparent);
        background: none; box-shadow: none; color: var(--mk-primary);
      }
      .row button.au-off {
        border-color: color-mix(in srgb, var(--mk-error) 45%, transparent);
        color: var(--mk-error);
      }
      .au-go:hover { background: color-mix(in srgb, var(--mk-primary) 10%, transparent); }
    </style>
    <div class="panel">
      <h2>{{.T.authorities}}</h2>
      <p class="panel-hint">{{.T.authoritiesHint}}</p>
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      <div class="rows">
      {{range .Authorities}}
      <div class="row">
        <span class="au-name">{{.Name}}</span>
        {{if .Connected}}
        <span class="au-on">{{$.T.connected}}</span>
        {{if .Sole}}
        <span class="au-sole">{{$.T.onlyWayIn}}</span>
        {{else}}
        <form method="post" action="/profile/disconnect/{{.ID}}">
          <button class="au-off" type="submit">{{$.T.disconnect}}</button>
        </form>
        {{end}}
        {{else}}
        <a class="au-go" href="/profile/connect/{{.ID}}">{{$.T.connect}}</a>
        {{end}}
      </div>
      {{end}}
      </div>
    </div>
    <p class="back"><a href="/profile/security">{{.T.back}}</a></p>
`

// showAuthorities renders the page, reached from Security.
func (h *Handler) showAuthorities(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return
	}
	h.renderAuthorities(w, r, sess.UserID, "", http.StatusOK)
}

func (h *Handler) renderAuthorities(w http.ResponseWriter, r *http.Request, userID, errMsg string, status int) {
	writeFlow(w, profileAuthoritiesPage, profileAuthoritiesData{
		flowChrome:  listChrome(h.flowData(r, "titleAuthorities")),
		Error:       errMsg,
		Authorities: h.authorityLinks(r, userID),
	}, status)
}
