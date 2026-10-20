package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/softwarity/meerkat/internal/store"
)

// Exclusive group mode (RBAC-03): when a tenant's effective mode is SINGLE,
// ONE of the member's groups is active per session and only its roles apply.
// Groups are PER TENANT, so every tenant change resets the choice and re-runs
// this decision: none = nothing to pick, one = picked silently, several =
// the /select-group step (same pattern as select-tenant - a redirect, not a
// blocking pending step: an undecided session simply carries no roles).

// groupForTenant decides the active group for a user entering a tenant:
// the group id to stamp silently, or choice=true when the user must pick.
func (h *Handler) groupForTenant(ctx context.Context, userID, tenantID string) (groupID string, choice bool) {
	if tenantID == "" || h.st.EffectiveGroupMode(ctx, tenantID) != store.GroupModeSingle {
		return "", false
	}
	groups, err := h.st.MemberGroups(ctx, tenantID, userID)
	if err != nil || len(groups) == 0 {
		return "", false
	}
	if len(groups) == 1 {
		return groups[0].ID, false
	}
	return "", true
}

// applyGroupStep runs the decision for an EXISTING session (tenant just set
// on it): stamps a lone group, and returns true when /select-group is owed.
func (h *Handler) applyGroupStep(r *http.Request, userID, tenantID string) bool {
	gid, choice := h.groupForTenant(r.Context(), userID, tenantID)
	if gid != "" {
		_ = h.sm.SetGroup(r.Context(), r, gid)
	}
	return choice
}

// ---- the /select-group step ----

var selectGroupPage = flowPage("select-group", selectGroupBody)

// groupChoice is one group on the chooser: Closed when its hours keep this
// person out right now (TENANT-04) - shown, not offered.
type groupChoice struct {
	store.Group
	Closed bool
}

type selectGroupData struct {
	flowChrome
	Groups []groupChoice
	Active string
	Next   string
	Error  string
}

const selectGroupBody = `    <form method="post" action="/select-group">
      <p class="lead">{{.T.chooseGroupLead}}</p>
      {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
      {{range .Groups}}
      <button class="choice" type="submit" name="group" value="{{.ID}}" {{if or (eq .ID $.Active) .Closed}}disabled{{end}}>
        <span class="choice-name">{{.Name}}</span>
        {{if .Closed}}<span class="choice-type">{{$.T.outsideHours}}</span>{{end}}
      </button>
      {{end}}
      <input type="hidden" name="next" value="{{.Next}}">
    </form>
    <form method="post" action="logout" class="leave">
      <button type="submit">{{.T.signOut}}</button>
    </form>
`

func (h *Handler) showSelectGroup(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.String()), http.StatusSeeOther)
		return
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return
	}
	next := safeNext(sess.Next)
	if q := r.URL.Query().Get("next"); q != "" {
		next = safeNext(q)
	}
	if sess.TenantID == "" || h.st.EffectiveGroupMode(r.Context(), sess.TenantID) != store.GroupModeSingle {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	groups, err := h.st.MemberGroups(r.Context(), sess.TenantID, sess.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	switch len(groups) {
	case 0:
		http.Redirect(w, r, next, http.StatusSeeOther)
	case 1:
		// Nothing to choose - stamp the only group and go.
		if err := h.sm.SetGroup(r.Context(), r, groups[0].ID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	default:
		h.renderSelectGroup(w, r, groups, sess.GroupID, next, "", http.StatusOK)
	}
}

func (h *Handler) doSelectGroup(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	next := safeNext(sess.Next)
	if f := r.PostFormValue("next"); f != "" {
		next = safeNext(f)
	}
	if sess.TenantID == "" {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	groups, err := h.st.MemberGroups(r.Context(), sess.TenantID, sess.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	choice := r.PostFormValue("group")
	var chosen *store.Group
	for i := range groups {
		if groups[i].ID == choice {
			chosen = &groups[i]
			break
		}
	}
	if chosen == nil {
		h.renderSelectGroup(w, r, groups, sess.GroupID, next, h.tr(r, "errGroupForbidden"), http.StatusForbidden)
		return
	}
	// The chosen group's hours, when it has its own, are the ones that hold
	// from here (TENANT-04): checked, and the session bound by them.
	inHours, until := h.windowHours(r.Context(), sess.UserID, sess.TenantID, chosen.ID)
	if !inHours {
		h.securityOfIn(r, secSigninRefused, sess.UserID, sess.TenantID, refusedHours)
		h.renderSelectGroup(w, r, groups, sess.GroupID, next,
			fmt.Sprintf(h.tr(r, "errTenantRefused"), chosen.Name), http.StatusForbidden)
		return
	}
	if err := h.sm.SetGroup(r.Context(), r, chosen.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.boundSession(w, r, until)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) renderSelectGroup(w http.ResponseWriter, r *http.Request, groups []store.Group, active, next, errMsg string, status int) {
	sess, _ := h.sm.Resolve(r.Context(), r)
	choices := make([]groupChoice, 0, len(groups))
	for _, g := range groups {
		open, _ := h.windowHours(r.Context(), sess.UserID, sess.TenantID, g.ID)
		choices = append(choices, groupChoice{Group: g, Closed: !open})
	}
	writeFlow(w, selectGroupPage, selectGroupData{
		flowChrome: h.flowData(r, "titleChooseGroup"),
		Groups:     choices, Active: active, Next: next, Error: errMsg,
	}, status)
}
