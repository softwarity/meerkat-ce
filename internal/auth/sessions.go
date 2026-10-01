package auth

import (
	"net/http"
	"net/url"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/useragent"
)

// The person's own sessions (AUTH-14, SEC-07): every place their account is
// signed in to the applications right now, and a way to close any of them -
// the laptop left open at a client's, the phone that was lost - or all of
// them but this one. A session closed here is gone on every node at once.

var profileSessionsPage = flowPage("profile-sessions", profileSessionsBody)

type profileSessionsData struct {
	flowChrome
	Sessions []sessionView
	Others   bool
}

type sessionView struct {
	ID      string
	Label   string
	Meta    string
	Since   string
	Current bool
}

const profileSessionsBody = `    <style>
      .ss-lines { flex: 1; min-width: 7.5rem; display: grid; gap: 3px; text-align: start; }
      .ss-label { font-size: .88rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .ss-label.here { color: var(--mk-primary); font-weight: 500; }
      .ss-meta { font-family: var(--mk-mono); font-size: .66rem; color: var(--mk-on-surface-variant); overflow-wrap: anywhere; }
      .ss-form { margin: 0; padding: 0; width: auto; background: none; border: 0; box-shadow: none; }
      .ss-form::before { display: none; }
      .ss-out {
        margin: 0; padding: 2px 0; border: 0; background: none; box-shadow: none;
        color: var(--mk-error); font-size: .78rem; cursor: pointer;
      }
      .ss-out:hover { filter: none; box-shadow: none; transform: none; text-decoration: underline; }
      button.danger {
        background: color-mix(in srgb, var(--mk-error) 14%, transparent);
        color: var(--mk-error); border-color: color-mix(in srgb, var(--mk-error) 34%, transparent);
      }
      button.danger:hover { border-color: var(--mk-error); }
      .hint-line { margin: 0; padding: 8px 0 12px; font-size: .74rem; color: var(--mk-on-surface-variant); }
      .back { margin: 6px 0 0; text-align: center; font-size: .8rem; }
      .back a { color: var(--mk-primary); text-decoration: none; }
    </style>
    <div class="panel">
      <h2>{{.T.sessionsActive}}</h2>
      <p class="hint-line">{{.T.sessionsHint}}</p>
      <div class="rows">
      {{range .Sessions}}
      <div class="row">
        <div class="ss-lines">
          <span class="ss-label{{if .Current}} here{{end}}">{{if .Current}}{{$.T.sessionsThisBrowser}}{{else}}{{.Label}}{{end}}</span>
          <span class="ss-meta">{{.Since}}{{if .Meta}} - {{.Meta}}{{end}}</span>
        </div>
        {{if not .Current}}
        <form method="post" action="/profile/sessions" class="ss-form">
          <input type="hidden" name="id" value="{{.ID}}">
          <button type="submit" class="ss-out">{{$.T.sessionsSignOut}}</button>
        </form>
        {{end}}
      </div>
      {{end}}
      </div>
      {{if .Others}}
      <form method="post" action="/profile/sessions">
        <input type="hidden" name="action" value="others">
        <button type="submit" class="danger">{{.T.sessionsSignOutOthers}}</button>
      </form>
      {{end}}
    </div>
    <p class="back"><a href="/profile/security">{{.T.back}}</a></p>
`

func (h *Handler) showProfileSessions(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.String()), http.StatusSeeOther)
		return
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return
	}
	loc := time.Local
	if u, err := h.st.GetUserByID(r.Context(), sess.UserID); err == nil && u.Timezone != "" {
		if l, err := time.LoadLocation(u.Timezone); err == nil {
			loc = l
		}
	}
	list, _, err := h.st.ListSessions(r.Context(), store.SessionFilter{UserID: sess.UserID, Plane: store.PlaneData, Limit: 100}, time.Now().Unix())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	current := h.sm.CurrentID(r)
	data := profileSessionsData{flowChrome: listChrome(h.flowData(r, "titleSessions"))}
	for _, s := range list {
		v := sessionView{ID: s.ID, Label: "-", Meta: s.IP, Current: s.ID == current}
		if s.Agent != "" {
			v.Label = useragent.Label(s.Agent)
		}
		if s.CreatedAt > 0 {
			v.Since = time.Unix(s.CreatedAt, 0).In(loc).Format("2006-01-02 15:04")
		}
		if !v.Current {
			data.Others = true
		}
		data.Sessions = append(data.Sessions, v)
	}
	writeFlow(w, profileSessionsPage, data, http.StatusOK)
}

func (h *Handler) doProfileSessions(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	current := h.sm.CurrentID(r)
	var ids []string
	if r.PostFormValue("action") == "others" {
		list, _, err := h.st.ListSessions(r.Context(), store.SessionFilter{UserID: sess.UserID, Plane: store.PlaneData, Limit: 500}, time.Now().Unix())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		for _, s := range list {
			if s.ID != current {
				ids = append(ids, s.ID)
			}
		}
	} else if id := r.PostFormValue("id"); id != "" && id != current {
		ids = []string{id}
	}
	for _, id := range ids {
		// Only one's own: the id is checked against the account, so a form
		// posted with somebody else's id closes nothing.
		owner, _, err := h.sm.RevokeByIDOf(r.Context(), id, sess.UserID)
		if err == nil && owner != "" {
			h.securityOf(r, secSignout, sess.UserID, "remote")
		}
	}
	http.Redirect(w, r, "/profile/sessions", http.StatusSeeOther)
}
