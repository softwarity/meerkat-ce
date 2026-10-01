package auth

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/softwarity/meerkat/internal/store"
)

// Leaving an organisation, from the person's own side (TENANT-02). Until now
// only an administrator could end a membership, so somebody who had moved on
// stayed listed - and kept whatever the organisation's groups granted - until
// someone else thought of it.
//
// Only the organisation the session is IN: that is the one the profile names,
// and the one the person has just been looking at. Leaving it is final from
// here - the way back is an administrator adding them again, which the page
// says before anything happens.

var profileLeavePage = flowPage("profile-leave", profileLeaveBody)

type profileLeaveData struct {
	flowChrome
	TenantName string
}

const profileLeaveBody = `    <style>
      button.danger {
        background: color-mix(in srgb, var(--mk-error) 14%, transparent);
        color: var(--mk-error); border-color: color-mix(in srgb, var(--mk-error) 34%, transparent);
      }
      button.danger:hover { border-color: var(--mk-error); }
      .back { margin: 6px 0 0; text-align: center; font-size: .8rem; }
      .back a { color: var(--mk-primary); text-decoration: none; }
    </style>
    <div class="panel">
      <h2>{{.T.leaveOrganisation}}</h2>
      <p class="lead">{{printf .T.leaveBody .TenantName}}</p>
      <form method="post" action="/profile/leave">
        <button type="submit" class="danger">{{.T.leaveConfirm}}</button>
      </form>
    </div>
    <p class="back"><a href="/profile">{{.T.backToProfile}}</a></p>
`

// leavable resolves the session and the organisation it is in, or answers.
func (h *Handler) leavable(w http.ResponseWriter, r *http.Request) (store.Session, store.Tenant, bool) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.String()), http.StatusSeeOther)
		return store.Session{}, store.Tenant{}, false
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return store.Session{}, store.Tenant{}, false
	}
	// A single installation has ONE organisation, implicit: leaving it would
	// be leaving the application, which is not a membership question.
	if h.st.Tenancy(r.Context()) != store.TenancyMulti || sess.TenantID == "" {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return store.Session{}, store.Tenant{}, false
	}
	t, err := h.st.GetTenant(r.Context(), sess.TenantID)
	if err != nil {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return store.Session{}, store.Tenant{}, false
	}
	return sess, t, true
}

func (h *Handler) showProfileLeave(w http.ResponseWriter, r *http.Request) {
	_, t, ok := h.leavable(w, r)
	if !ok {
		return
	}
	writeFlow(w, profileLeavePage, profileLeaveData{
		flowChrome: h.flowData(r, "titleLeave"), TenantName: t.Name,
	}, http.StatusOK)
}

func (h *Handler) doProfileLeave(w http.ResponseWriter, r *http.Request) {
	sess, t, ok := h.leavable(w, r)
	if !ok {
		return
	}
	removed, err := h.st.DeleteMembership(r.Context(), sess.UserID, t.ID)
	if err != nil {
		slog.Error("leaving an organisation failed", "user", sess.UserID, "tenant", t.ID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if removed {
		// In the administrative trail, stamped with the organisation: its
		// administrators are the ones who need to read that somebody left.
		u, _ := h.st.GetUserByID(r.Context(), sess.UserID)
		if err := h.st.AddAuditEvent(r.Context(), store.AuditEvent{
			ActorID: sess.UserID, Action: "member.leave", Target: "membership",
			TargetID: sess.UserID, TargetName: u.Username, TenantID: t.ID, Detail: t.Name,
		}); err != nil {
			slog.Error("audit write failed", "action", "member.leave", "err", err)
		}
	}
	// Out of it at once: the session no longer carries the organisation, so
	// nothing it granted is used on the next request. A person left with ONE
	// organisation lands in it on the next page; with none, in the waiting
	// room the rest of the product already sends them to.
	if err := h.sm.SetTenant(r.Context(), r, ""); err != nil {
		slog.Warn("clearing the organisation after leaving failed", "err", err)
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}
