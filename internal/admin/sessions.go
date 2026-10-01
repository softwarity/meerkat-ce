package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/useragent"
)

// The live sessions, installation-wide (CONSOLE-08, CONSOLE-01): who is
// signed in where right now, and a way to end any one of them.
//
// Root reads both planes. An application administrator reads the
// applications' sessions only: who runs the console, and from where, is
// root's business - the same line the audit trail draws for console sign-ins.
func (a *API) registerSessions(mux Mux) {
	mux.Handle("GET /api/sessions", a.appAdmin(a.listSessions))
	mux.Handle("DELETE /api/sessions/{id}", a.appAdmin(a.revokeSession))
}

type sessionRow struct {
	store.SessionInfo
	// Label is the browser and system, read from the User-Agent.
	Label string `json:"label,omitempty"`
	// Current marks the caller's own console session.
	Current bool `json:"current,omitempty"`
}

type sessionPage struct {
	Sessions []sessionRow `json:"sessions"`
	Total    int          `json:"total"`
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request, actor store.User) {
	q := r.URL.Query()
	f := store.SessionFilter{
		Search: strings.TrimSpace(q.Get("q")),
		Plane:  strings.TrimSpace(q.Get("plane")),
		Limit:  int(atoi64(q.Get("limit"))),
		Offset: int(atoi64(q.Get("offset"))),
	}
	if f.Plane != "" && f.Plane != store.PlaneData && f.Plane != store.PlaneAdmin {
		writeErr(w, http.StatusBadRequest, "plane: expected data or admin")
		return
	}
	if !actor.Root {
		f.Plane = store.PlaneData
	}
	list, total, err := a.st.ListSessions(r.Context(), f, time.Now().Unix())
	if err != nil {
		a.internal(w, err)
		return
	}
	current := a.sm.CurrentID(r)
	out := sessionPage{Sessions: []sessionRow{}, Total: total}
	for _, s := range list {
		row := sessionRow{SessionInfo: s, Current: s.ID == current && s.Plane == store.PlaneAdmin}
		// A session opened before the browser was recorded has nothing to
		// name: no label, rather than "Unknown browser", which would read as
		// a browser we failed to recognise.
		if s.Agent != "" {
			row.Label = useragent.Label(s.Agent)
		}
		out.Sessions = append(out.Sessions, row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) revokeSession(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	_, userID, plane, err := a.st.SessionHashByID(r.Context(), id)
	// Out of the perimeter reads as absent: an application administrator has
	// no business learning that a console session exists under that id.
	if err != nil || (plane == store.PlaneAdmin && !actor.Root) {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if id == a.sm.CurrentID(r) && plane == store.PlaneAdmin {
		writeErr(w, http.StatusUnprocessableEntity, "this is your own session: sign out instead")
		return
	}
	if _, _, err := a.sm.RevokeByID(r.Context(), id); err != nil {
		a.internal(w, err)
		return
	}
	u, _ := a.st.GetUserByID(r.Context(), userID)
	a.auditEvent(r.Context(), actor, "session.revoke", "user", userID, u.Username, "", plane)
	w.WriteHeader(http.StatusNoContent)
}

// The applications' tokens, every account's (AUTH-09): an application
// administrator finds the one a departed colleague's script still carries,
// or the one that leaked, and revokes it. Created only by their owner, on the
// profile: what an administrator does here is see and end, never mint a
// credential in somebody else's name.
func (a *API) registerDataTokens(mux Mux) {
	mux.Handle("GET /api/data-tokens", a.appAdmin(a.listDataTokens))
	mux.Handle("DELETE /api/data-tokens/{id}", a.appAdmin(a.revokeDataToken))
}

func (a *API) listDataTokens(w http.ResponseWriter, r *http.Request, _ store.User) {
	list, err := a.st.ListDataTokens(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		a.internal(w, err)
		return
	}
	if list == nil {
		list = []store.APIToken{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) revokeDataToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	owner, name, err := a.st.DataTokenOwner(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	if _, err := a.st.RevokeAPIToken(r.Context(), owner, id); err != nil {
		a.internal(w, err)
		return
	}
	a.sm.TokenChanged(id)
	u, _ := a.st.GetUserByID(r.Context(), owner)
	a.auditEvent(r.Context(), actor, "token.revoke", "user", owner, u.Username, "", name)
	w.WriteHeader(http.StatusNoContent)
}
