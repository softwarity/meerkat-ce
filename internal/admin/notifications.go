package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/notifier"
	"github.com/softwarity/meerkat/internal/store"
)

// The notifications on the control plane (NOTIF-05): where an administrator
// turns them on, says who may publish, and sees what was sent and who read
// it. The services publish on the internal plane (internalplane.go); nothing
// here sends anything.
//
// Mounted on every image, and answering 404 on the community one: the screen
// asks whether the feature is here before drawing anything, and an endpoint
// that does not exist would be the same answer said less clearly.

func (a *API) registerNotifications(mux Mux) {
	mux.Handle("GET /api/notifications/settings", a.domainAdmin(a.notifyLinked(a.getNotifySettings)))
	// Root writes its level, its switch and the cap; an app-admin writes its
	// level. The handler tells which from the caller.
	mux.Handle("PUT /api/notifications/settings", a.appAdmin(a.notifyLinked(a.putNotifySettings)))
	// Who may publish is who may reach a port of the gateway: the routing
	// plane's business, like the scheduler's access.
	mux.Handle("PUT /api/notifications/access", a.infraAdmin(a.notifyLinked(a.putNotifyAccess)))
	mux.Handle("GET /api/notifications", a.appAdmin(a.notifyLinked(a.listNotifications)))
	mux.Handle("GET /api/notifications/{id}/readers", a.appAdmin(a.notifyLinked(a.notificationReaders)))
	mux.Handle("DELETE /api/notifications/{id}", a.appAdmin(a.notifyLinked(a.withdrawNotification)))
	// The tenant level: its administrator's switch, and what was sent to that
	// tenant alone. Never the access nor the cap.
	mux.Handle("GET /api/tenants/{id}/notifications/settings", a.tenantScoped(a.notifyLinked(a.getTenantNotify)))
	mux.Handle("PUT /api/tenants/{id}/notifications/settings", a.tenantScoped(a.notifyLinked(a.putTenantNotify)))
	mux.Handle("GET /api/tenants/{id}/notifications", a.tenantScoped(a.notifyLinked(a.listNotifications)))
	mux.Handle("GET /api/tenants/{id}/notifications/{nid}/readers",
		a.tenantScoped(a.notifyLinked(a.notificationReaders)))
	mux.Handle("DELETE /api/tenants/{id}/notifications/{nid}", a.tenantScoped(a.notifyLinked(a.withdrawNotification)))
	// Trying one out from the tenant's page: what the builder offers, and
	// the message published to that tenant through the internal plane's own
	// checks, with the administrator as its sender.
	mux.Handle("GET /api/tenants/{id}/notifications/compose", a.tenantScoped(a.notifyLinked(a.composeOptions)))
	mux.Handle("POST /api/tenants/{id}/notifications", a.tenantScoped(a.notifyLinked(a.publishToTenant)))
}

// notifyLinked refuses on an image that does not carry the notifications.
func (a *API) notifyLinked(next userHandler) userHandler {
	return func(w http.ResponseWriter, r *http.Request, actor store.User) {
		if a.Notify == nil || !notifier.Linked() {
			writeErr(w, http.StatusNotFound, "notifications are part of the Enterprise edition, and this is the Community edition")
			return
		}
		next(w, r, actor)
	}
}

// notifyLevel is the level a caller decides at on the Data plane screen: root,
// or the application admin. Anybody else reads root's view and writes nothing.
func notifyLevel(actor store.User) string {
	switch {
	case actor.Root:
		return store.NotifyLevelRoot
	case actor.AppAdmin:
		return store.NotifyLevelAppAdmin
	}
	return ""
}

// notifyTenant is one tenant as the caller's level sees it.
type notifyTenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// State is on, off (by the caller's level), offBelow (allowed by it,
	// turned off under it) or offAbove (turned off over it, out of its hands).
	State string `json:"state"`
	// OffBy names the level that turned it off: root, app-admin or
	// tenant-admin. Empty when on.
	OffBy string `json:"offBy,omitempty"`
	// BelowBy names the first level under the caller's that turned it off,
	// whatever the caller decided: what allowing it again would find.
	BelowBy string `json:"belowBy,omitempty"`
	// Effective says it receives notifications, every level read.
	Effective bool `json:"effective"`
}

// notifySettingsView is the setting and what the screen needs around it.
type notifySettingsView struct {
	// Enabled is root's switch for the whole feature.
	Enabled bool `json:"enabled"`
	// Level is the caller's: root or app-admin, empty for a reader only.
	Level   string         `json:"level"`
	Tenants []notifyTenant `json:"tenants"`
	// Single says the gateway has one tenant: the screen shows one switch,
	// never names it, and the tenant level does not exist.
	Single     bool               `json:"single"`
	Cap        int                `json:"cap"`
	DefaultCap int                `json:"defaultCap"`
	Access     internalAccessView `json:"access"`
	// Path is where the services publish, on the internal plane.
	Path string `json:"path"`
	// Editable says the caller may turn tenants off and on at its level;
	// RootEditable that it may change the switch and the cap (root);
	// AccessEditable that it may change who publishes.
	Editable       bool `json:"editable"`
	RootEditable   bool `json:"rootEditable"`
	AccessEditable bool `json:"accessEditable"`
}

func (a *API) getNotifySettings(w http.ResponseWriter, r *http.Request, actor store.User) {
	ctx := r.Context()
	n := a.st.NotifySettings(ctx)
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		a.internal(w, err)
		return
	}
	multi := a.st.Tenancy(ctx) == store.TenancyMulti
	level := notifyLevel(actor)
	view := notifySettingsView{
		Enabled: n.Enabled, Level: level, Tenants: []notifyTenant{}, Single: !multi,
		Cap: n.Cap, DefaultCap: store.NotificationDefaultCap, Path: notifier.InternalPath,
		Editable: level != "", RootEditable: actor.Root, AccessEditable: actor.Root || actor.InfraAdmin,
	}
	if !multi {
		// The one tenant served: the others stay stored, out of sight.
		if t, err := a.st.PrimaryTenant(ctx); err == nil {
			tenants = []store.Tenant{t}
		}
	}
	seenAs := level
	if seenAs == "" {
		seenAs = store.NotifyLevelRoot
	}
	for _, t := range tenants {
		state, by := n.View(seenAs, t.ID, multi)
		view.Tenants = append(view.Tenants, notifyTenant{ID: t.ID, Name: t.Name, State: state, OffBy: by,
			BelowBy: n.BelowBy(seenAs, t.ID, multi), Effective: n.Effective(t.ID, multi)})
	}
	modes := []string{store.InternalAccessNone, store.InternalAccessToken}
	if a.onKubernetes() {
		modes = append(modes, store.InternalAccessCluster)
	}
	view.Access = internalAccessView{InternalAccess: n.Access, Modes: modes, Editable: view.AccessEditable}
	writeJSON(w, http.StatusOK, view)
}

// notifySettingsIn is what the console writes: the tenants the caller's level
// turns off, allows again, or forces on (allows, and clears what the levels
// below decided); and, for root alone, the switch and the cap. The access is
// written on its own, by other hands.
type notifySettingsIn struct {
	Enabled *bool    `json:"enabled,omitempty"`
	Cap     *int     `json:"cap,omitempty"`
	Off     []string `json:"off"`
	On      []string `json:"on"`
	Force   []string `json:"force"`
}

// notifyAudited is what the trail records of the setting: every level's
// decisions, never the access (it has its own entry).
type notifyAudited struct {
	Enabled   bool     `json:"enabled"`
	Cap       int      `json:"cap"`
	RootOff   []string `json:"rootOff"`
	AdminOff  []string `json:"adminOff"`
	TenantOff []string `json:"tenantOff"`
}

func auditedNotify(n store.NotifySettings) notifyAudited {
	return notifyAudited{Enabled: n.Enabled, Cap: n.Cap, RootOff: n.RootOff, AdminOff: n.AdminOff, TenantOff: n.TenantOff}
}

func (a *API) putNotifySettings(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in notifySettingsIn
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed notification settings: "+err.Error())
		return
	}
	level := notifyLevel(actor)
	if level != store.NotifyLevelRoot && (in.Enabled != nil || in.Cap != nil) {
		writeErr(w, http.StatusForbidden, "the switch for the whole gateway and the audience cap are root's: "+
			"an app-admin sends off, on and force only")
		return
	}
	ctx := r.Context()
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		a.internal(w, err)
		return
	}
	known := map[string]bool{}
	for _, t := range tenants {
		known[t.ID] = true
	}
	var unknown []string
	for _, id := range slices.Concat(in.Off, in.On, in.Force) {
		if !known[id] && !slices.Contains(unknown, id) {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		writeErr(w, http.StatusUnprocessableEntity, "no such tenant: "+strings.Join(unknown, ", "))
		return
	}
	before := a.st.NotifySettings(ctx)
	next := before
	next.RootOff, next.AdminOff, next.TenantOff = slices.Clone(before.RootOff), slices.Clone(before.AdminOff),
		slices.Clone(before.TenantOff)
	multi := a.st.Tenancy(ctx) == store.TenancyMulti
	// An app-admin cannot turn on what root turned off: allowing it at its
	// own level would change nothing, and say otherwise on the screen.
	if level == store.NotifyLevelAppAdmin {
		for _, id := range slices.Concat(in.On, in.Force) {
			if state, _ := before.View(level, id, multi); state == store.NotifyOffAbove {
				writeErr(w, http.StatusUnprocessableEntity, "tenant "+id+" is turned off by root: only root turns it back on")
				return
			}
		}
	}
	if err := next.Decide(level, in.Off, in.On, in.Force); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Enabled != nil {
		next.Enabled = *in.Enabled
	}
	if in.Cap != nil {
		next.Cap = *in.Cap
	}
	if err := a.st.SetNotifySettings(ctx, next); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(ctx, actor, "settings.notifications", "settings", "", "", "",
		auditedNotify(before), auditedNotify(a.st.NotifySettings(ctx)))
	a.getNotifySettings(w, r, actor)
}

// tenantNotifyView is the tenant level's switch, as its administrator sees it.
type tenantNotifyView struct {
	// State is on, off (by the tenant admin) or offAbove (by root or the
	// application admin, named in OffBy): then the switch cannot be turned on.
	State     string `json:"state"`
	OffBy     string `json:"offBy,omitempty"`
	Effective bool   `json:"effective"`
}

// tenantLevel refuses on a single-tenant gateway, where the tenant level
// does not exist, and on a tenant that does not.
func (a *API) tenantLevel(w http.ResponseWriter, r *http.Request) (store.Tenant, bool) {
	ctx := r.Context()
	if a.st.Tenancy(ctx) != store.TenancyMulti {
		writeErr(w, http.StatusUnprocessableEntity, "a single-tenant gateway has no tenant level for notifications: "+
			"root and the application admin turn them on and off on Data plane, Notifications")
		return store.Tenant{}, false
	}
	t, err := a.st.GetTenant(ctx, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "tenant not found")
		return store.Tenant{}, false
	}
	return t, true
}

func (a *API) getTenantNotify(w http.ResponseWriter, r *http.Request, _ store.User) {
	t, ok := a.tenantLevel(w, r)
	if !ok {
		return
	}
	n := a.st.NotifySettings(r.Context())
	state, by := n.View(store.NotifyLevelTenant, t.ID, true)
	writeJSON(w, http.StatusOK, tenantNotifyView{State: state, OffBy: by, Effective: n.Effective(t.ID, true)})
}

func (a *API) putTenantNotify(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in struct {
		On bool `json:"on"`
	}
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed switch: "+err.Error()+` (send {"on": true} or {"on": false})`)
		return
	}
	t, ok := a.tenantLevel(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	before := a.st.NotifySettings(ctx)
	if state, by := before.View(store.NotifyLevelTenant, t.ID, true); in.On && state == store.NotifyOffAbove {
		writeErr(w, http.StatusUnprocessableEntity, "notifications for "+t.Name+" are turned off by "+
			store.NotifyLevelWho(by)+": only that level turns them back on")
		return
	}
	next := before
	next.TenantOff = slices.Clone(before.TenantOff)
	off, on := []string{t.ID}, []string(nil)
	if in.On {
		off, on = nil, []string{t.ID}
	}
	if err := next.Decide(store.NotifyLevelTenant, off, on, nil); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SetNotifySettings(ctx, next); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(ctx, actor, "tenant.notifications", "tenant", t.ID, t.Name, t.ID,
		map[string]bool{"notifications": !slices.Contains(before.TenantOff, t.ID)}, map[string]bool{"notifications": in.On})
	a.getTenantNotify(w, r, actor)
}

func (a *API) putNotifyAccess(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in store.InternalAccess
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed access: "+err.Error())
		return
	}
	if err := store.SanitizeInternalAccess(&in); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Mode == store.InternalAccessCluster && !a.onKubernetes() {
		writeErr(w, http.StatusUnprocessableEntity,
			"mode cluster checks a ServiceAccount token with the Kubernetes API, and this gateway does not "+
				"run on Kubernetes: allowed here are "+store.InternalAccessNone+" and "+store.InternalAccessToken)
		return
	}
	ctx := r.Context()
	before := a.st.NotifySettings(ctx)
	next := before
	next.Access = in
	if err := a.st.SetNotifySettings(ctx, next); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(ctx, actor, "settings.notifications-access", "settings", "", "", "", before.Access, in)
	a.getNotifySettings(w, r, actor)
}

// sentNotification is one notification as the console lists it: how many
// were told, how many read, and - when it asks to be acknowledged - how many
// acknowledged it and who closed a single one.
type sentNotification struct {
	store.Notification
	Reads        int                       `json:"reads"`
	Acknowledged int                       `json:"acknowledged"`
	AckBy        *store.NotificationPerson `json:"ackBy,omitempty"`
}

// sentNotifications counts the reads and the acknowledgements of a list.
func (a *API) sentNotifications(ctx context.Context, list []store.Notification) ([]sentNotification, error) {
	ids := make([]string, len(list))
	for i, n := range list {
		ids[i] = n.ID
	}
	reads, err := a.st.NotificationReads(ctx, ids)
	if err != nil {
		return nil, err
	}
	acks, err := a.st.NotificationAcks(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]sentNotification, len(list))
	for i, n := range list {
		out[i] = sentNotification{Notification: n, Reads: reads[n.ID], Acknowledged: acks[n.ID]}
		if n.AckBy != "" {
			p := a.st.NotificationPersonOf(ctx, n.AckBy)
			out[i].AckBy = &p
		}
	}
	return out, nil
}

// The global list and readers take the notification at {id}; the tenant's
// take the tenant at {id} and the notification at {nid}, which must be that
// tenant's: an administrator of one tenant reads and withdraws only its own.
func scopedTenant(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/api/tenants/") {
		return r.PathValue("id")
	}
	return ""
}

func notificationID(r *http.Request) string {
	if scopedTenant(r) != "" {
		return r.PathValue("nid")
	}
	return r.PathValue("id")
}

// scopedNotification reads the notification a request names, refused when
// it is not the tenant's the request is scoped to.
func (a *API) scopedNotification(w http.ResponseWriter, r *http.Request) (store.Notification, bool) {
	id := notificationID(r)
	n, err := a.st.GetNotification(r.Context(), id)
	if err == nil && (scopedTenant(r) == "" || n.TenantID == scopedTenant(r)) {
		return n, true
	}
	if err != nil && !errors.Is(err, store.ErrNotificationNotFound) {
		a.internal(w, err)
		return n, false
	}
	writeErr(w, http.StatusNotFound, "no notification "+id)
	return n, false
}

func (a *API) listNotifications(w http.ResponseWriter, r *http.Request, _ store.User) {
	q := r.URL.Query()
	f := store.NotificationFilter{TenantID: q.Get("tenant"), Sender: q.Get("sender"), Limit: 200}
	if t := scopedTenant(r); t != "" {
		f.TenantID = t
	}
	list, err := a.st.ListNotifications(r.Context(), f)
	if err != nil {
		a.internal(w, err)
		return
	}
	out, err := a.sentNotifications(r.Context(), list)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) notificationReaders(w http.ResponseWriter, r *http.Request, _ store.User) {
	n, ok := a.scopedNotification(w, r)
	if !ok {
		return
	}
	readers, err := a.st.NotificationReaders(r.Context(), n.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, readers)
}

func (a *API) withdrawNotification(w http.ResponseWriter, r *http.Request, actor store.User) {
	n, ok := a.scopedNotification(w, r)
	if !ok {
		return
	}
	if err := a.Notify.Withdraw(r.Context(), n.ID); err != nil {
		if errors.Is(err, store.ErrNotificationNotFound) {
			writeErr(w, http.StatusNotFound, "notification "+n.ID+" was already withdrawn")
			return
		}
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "notification.withdraw", "notification", n.ID, n.Title.Pick("en"), n.TenantID,
		map[string]bool{"withdrawn": false}, map[string]bool{"withdrawn": true})
	w.WriteHeader(http.StatusNoContent)
}

// composeView is what the tenant page's notification builder offers: the
// roles and members an audience names, the routes a link leads to as a
// service names them, and whether a mail can go with it.
type composeView struct {
	Tenant     string            `json:"tenant"`
	Levels     []string          `json:"levels"`
	Roles      []composeRole     `json:"roles"`
	Members    []composeMember   `json:"members"`
	Routes     []notifier.Target `json:"routes"`
	Mail       bool              `json:"mail"`
	MaxActions int               `json:"maxActions"`
	// Path is where a service publishes the same message, on the internal
	// plane.
	Path string `json:"path"`
}

type composeRole struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type composeMember struct {
	Username string `json:"username"`
	Fullname string `json:"fullname,omitempty"`
}

// composeMaxActions is the internal plane's bound (ee/notify), said to the
// builder so it stops offering a sixth.
const composeMaxActions = 5

func (a *API) composeOptions(w http.ResponseWriter, r *http.Request, _ store.User) {
	ctx := r.Context()
	t, err := a.st.GetTenant(ctx, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "tenant not found")
		return
	}
	view := composeView{Tenant: t.Name, Levels: store.NotificationLevels, Roles: []composeRole{},
		Members: []composeMember{}, Mail: a.st.GetSMTP(ctx).Configured(),
		MaxActions: composeMaxActions, Path: notifier.InternalPath}
	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		a.internal(w, err)
		return
	}
	for _, rl := range roles {
		view.Roles = append(view.Roles, composeRole{Name: rl.Name, Description: rl.Description})
	}
	members, err := a.st.ListMembers(ctx, t.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	for _, m := range members {
		if m.Enabled {
			view.Members = append(view.Members, composeMember{Username: m.Username, Fullname: m.Fullname})
		}
	}
	if view.Routes, err = a.Notify.Targets(ctx); err != nil {
		a.internal(w, err)
		return
	}
	if view.Routes == nil {
		view.Routes = []notifier.Target{}
	}
	writeJSON(w, http.StatusOK, view)
}

// refusalOut is a publication refused, with the field of the message it
// concerns when it concerns one.
type refusalOut struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// publishToTenant publishes a message to the tenant the request is scoped
// to, exactly as a service would on the internal plane - the same checks,
// the same refusals, the same delivery - with the administrator as sender.
func (a *API) publishToTenant(w http.ResponseWriter, r *http.Request, actor store.User) {
	ctx := r.Context()
	t, err := a.st.GetTenant(ctx, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "tenant not found")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "malformed notification: "+err.Error())
		return
	}
	p, err := a.Notify.Publish(ctx, raw, notifier.Sender{Identity: actor.Username, Via: store.CallerConsole}, t.ID)
	if err != nil {
		var rf *notifier.Refusal
		if errors.As(err, &rf) {
			writeJSON(w, rf.Status, refusalOut{Error: rf.Message, Field: rf.Field})
			return
		}
		a.internal(w, err)
		return
	}
	if p.Existing {
		writeJSON(w, http.StatusOK, p)
		return
	}
	// The answer names it by the id its sender knows it by; the trail by
	// the gateway's own, as every other line about a notification.
	// Published without an id, it is the administrator's latest one there.
	title, id := "", ""
	list, err := a.st.SenderNotifications(ctx, actor.Username, p.ID)
	if err == nil && len(list) == 0 {
		list, err = a.st.ListNotifications(ctx, store.NotificationFilter{Sender: actor.Username, TenantID: t.ID, Limit: 1})
	}
	if err == nil && len(list) > 0 {
		title, id = list[0].Title.Pick("en"), list[0].ID
	}
	detail := fmt.Sprintf("%d recipients", p.Recipients)
	if p.ID != "" {
		detail += ", id " + p.ID
	}
	a.auditEvent(ctx, actor, "notification.publish", "notification", id, title, t.ID, detail)
	writeJSON(w, http.StatusCreated, p)
}

// The agent's view of the notifications: what was sent, who read it, and
// taking one back - the console's view, no more. Publishing is absent on
// purpose: a notification is a service telling its users about its own
// objects, and an agent has no objects of its own to tell anybody about.
func (a *API) notificationTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_notifications", Allow: administersIdentity, Title: "List the notifications sent", ReadOnly: true,
			Description: "The notifications the services behind this gateway sent their users (NOTIF-05), newest " +
				"first: who sent it, to which tenant, its title, how many people it was given to and how " +
				"many read it, whether it asks to be acknowledged and how many did, when it expires, and " +
				"whether its sender withdrew it. Pass id to read one in full, with every recipient and when each " +
				"read and acknowledged it. Empty on an image that does not carry them.",
			Schema: object(map[string]any{
				"id":     str("Optional: one notification's id, to read who was told and who read it."),
				"sender": str("Optional: only what this sender published (a token's name, a ServiceAccount, a workload)."),
			}),
			Call: a.toolListNotifications,
		},
		{
			Name: "withdraw_notification", Allow: administersIdentity, Title: "Withdraw a notification",
			Description: "Take a notification back, whoever sent it: it shows as withdrawn on every recipient's page and leaves their badge at " +
				"once. The trail keeps who had read it. Take the id from list_notifications.",
			Schema: object(map[string]any{"id": str("The notification's id, as list_notifications gives it.")}, "id"),
			Call:   a.toolWithdrawNotification,
		},
	}
}

func (a *API) toolListNotifications(ctx context.Context, raw json.RawMessage) (any, error) {
	var in struct {
		ID     string `json:"id"`
		Sender string `json:"sender"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
	}
	if !notifier.Linked() {
		return []store.Notification{}, nil
	}
	if in.ID != "" {
		n, err := a.st.GetNotification(ctx, in.ID)
		if errors.Is(err, store.ErrNotificationNotFound) {
			return nil, fmt.Errorf("no notification %q on this gateway", in.ID)
		}
		if err != nil {
			return nil, err
		}
		readers, err := a.st.NotificationReaders(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"notification": n, "recipients": readers}, nil
	}
	list, err := a.st.ListNotifications(ctx, store.NotificationFilter{Sender: in.Sender, Limit: 100})
	if err != nil {
		return nil, err
	}
	return a.sentNotifications(ctx, list)
}

func (a *API) toolWithdrawNotification(ctx context.Context, raw json.RawMessage) (any, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("which notification: pass the id list_notifications gives")
	}
	if a.Notify == nil || !notifier.Linked() {
		return nil, fmt.Errorf("notifications are part of the Enterprise edition, and this is the Community edition")
	}
	n, err := a.st.GetNotification(ctx, in.ID)
	if errors.Is(err, store.ErrNotificationNotFound) {
		return nil, fmt.Errorf("no notification %q on this gateway", in.ID)
	}
	if err != nil {
		return nil, err
	}
	if err := a.Notify.Withdraw(ctx, in.ID); err != nil {
		return nil, fmt.Errorf("notification %q: %w", in.ID, err)
	}
	a.auditUpdate(ctx, mcpActor(ctx), "notification.withdraw", "notification", n.ID, n.Title.Pick("en"), n.TenantID,
		map[string]bool{"withdrawn": false}, map[string]bool{"withdrawn": true})
	return map[string]any{"withdrawn": n.ID}, nil
}
