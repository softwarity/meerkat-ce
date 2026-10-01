package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The audit trail (phase 2). Every administrative mutation records WHO changed
// WHAT, and - for updates - the exact field-level diff (before/after). The diff
// is computed generically by JSON-marshaling the old and new value and
// comparing top-level keys, so it works for tenants, users, memberships, roles,
// groups and the settings payload without per-type code.

// auditIgnore are keys never worth recording as a change: identifiers, server
// timestamps, and the GET-only display fields that ride along on a round-trip.
var auditIgnore = map[string]bool{
	"id": true, "createdAt": true, "updatedAt": true, "lastConnectionAt": true,
	"createdByName": true, "ownerName": true, "tenantName": true,
	// The row's revision, which every write raises (store.checkRev): recording
	// it would put a "rev: 1 -> 2" line beside every real change, on every
	// object, for ever - the server's own bookkeeping read as if somebody had
	// edited a field.
	"rev": true,
}

// The audit domains (RBAC-05): each administrative capability sees its own
// slice of the trail. infra-admin runs the routing plane, app-admin runs the
// application's identity and appearance, a tenant admin sees their tenants'
// events (scoped by tenant_id, not by target). Root sees everything. A user who
// administers nothing sees nothing.
//
// READ FROM THE MODEL rather than listed here, and the two lists this replaced
// were the argument for it: they named three kinds and four out of the twenty
// an event can name, and two of the seven were wrong - a theme is written by an
// app-admin and a control-plane token by root alone, so the trail showed one
// person events they could not cause and hid another's own writes. The table in
// store.AuditTargets says which capability administers each kind, once, for
// this and for the live channel that tells a screen its kind moved.
var (
	infraTargets = store.AuditTrailTargets(store.AuditDomainInfra)
	appTargets   = store.AuditTrailTargets(store.AuditDomainApp)
)

// auditRegisterViewer mounts the read-only audit endpoint. It is a section of
// its own (not under Application): the handler scopes each caller to the
// domains their capabilities cover, so routes show to infra admins and
// identity changes to app admins.
func (a *API) auditRegisterViewer(mux Mux) {
	mux.Handle("GET /api/audit", a.authed(a.listAudit))
	// The trail as a file (STORE-06), the same perimeter as the screen.
	mux.Handle("GET /api/audit/export", a.authed(a.exportAudit))
	// How long the trail keeps an event (AUD-02): root's alone, since whoever
	// may shorten it may erase their own traces with it.
	mux.Handle("GET /api/settings/audit", a.rootOnly(a.getAuditSettings))
	mux.Handle("PUT /api/settings/audit", a.rootOnly(a.putAuditSettings))
}

// audit records one event, best-effort: an audit write must never fail the
// mutation it trails, so an error is logged, not propagated.
//
// The acting TOKEN is stamped here rather than at the call sites (MCP-03):
// there are five of them today and there will be more, and a trail that names
// the agent in four cases out of five is worse than one that never does - it
// reads as if a human had done the fifth.
func (a *API) audit(ctx context.Context, ev store.AuditEvent) {
	ev.ActorToken = actorToken(ctx)
	// Stamped HERE rather than left to the store, because the same event is
	// about to be told twice: written to the table, and published on the
	// console's live channel where that identifier is the version a screen
	// compares. Two writes to one kind inside the same second would otherwise
	// have shared a version, and the second would never have been published.
	if ev.ID == "" {
		ev.ID = store.NewEventID()
	}
	if ev.At == 0 {
		ev.At = time.Now().Unix()
	}
	if err := a.st.AddAuditEvent(ctx, ev); err != nil {
		slog.Error("audit write failed", "action", ev.Action, "target", ev.Target, "err", err)
	}
	// And the request that made it gets the identifier back, so the screen
	// that saved recognises its own write when the live channel reports it.
	noteChange(ctx, ev.ID)
	// And the console hears about it (CONSOLE-13). This funnel is the only
	// place that knows a mutation happened AND what kind of object it touched,
	// which is what makes the live channel complete without an emission at
	// forty call sites - and complete for the endpoint somebody adds next month
	// without knowing this exists.
	a.changed(ctx, ev)
}

// changed publishes one event to the screens of this node, and to the other
// nodes' screens through the bus.
//
// Both halves are nil-tolerant, like announce: most tests build an API with
// neither a live channel nor a cluster, and a change that only matters to a
// console must not need one to be tested.
func (a *API) changed(ctx context.Context, ev store.AuditEvent) {
	if a.Changes != nil {
		a.Changes.Record(ev)
	}
	if a.Bus == nil {
		return
	}
	msg, err := json.Marshal(changeSignal{
		ID: ev.ID, At: ev.At, Action: ev.Action, Target: ev.Target,
		TargetID: ev.TargetID, TargetName: clip(ev.TargetName),
		TenantID: ev.TenantID, Actor: clip(ev.ActorName), ActorToken: clip(ev.ActorToken),
	})
	if err != nil {
		return
	}
	// A SIGNAL and not a topic: there is no table to re-read behind it - what
	// the screens do next is ask the API. Losing one costs a screen that is
	// late until its next reason to read, never a wrong write: a late screen
	// saving over somebody is refused by the row's revision.
	a.Bus.Signal(ctx, store.TopicChanged, string(msg))
}

// changeSignal is what crosses the cluster: the event without its diff. The
// fields before/after are the trail's business and stay in the table - a
// notification carries at most 7000 bytes (store.MaxSignalBytes), and a theme's
// palette alone would blow through that.
//
// The names it does carry are clipped for the same reason. They are short in
// practice, which is exactly what makes the case worth closing: an object named
// by a script rather than by a person would push the message past the limit,
// the notification would be refused, and the other nodes' screens would sit on
// a stale list with only a warning in a log to say why.
type changeSignal struct {
	ID         string `json:"id"`
	At         int64  `json:"at"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	TargetID   string `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	TenantID   string `json:"tenantId,omitempty"`
	Actor      string `json:"actor,omitempty"`
	ActorToken string `json:"actorToken,omitempty"`
}

// clip bounds one name. Long enough for anything a person reads on a screen.
func clip(s string) string {
	const most = 200
	if len(s) <= most {
		return s
	}
	return s[:most]
}

// DecodeChange reads what crossed the bus back into an event. Exported for the
// wiring in main, which hands it to the live journal.
func DecodeChange(arg string) (store.AuditEvent, bool) {
	var sig changeSignal
	if err := json.Unmarshal([]byte(arg), &sig); err != nil || sig.Target == "" {
		return store.AuditEvent{}, false
	}
	return store.AuditEvent{
		ID: sig.ID, At: sig.At, Action: sig.Action, Target: sig.Target,
		TargetID: sig.TargetID, TargetName: sig.TargetName, TenantID: sig.TenantID,
		ActorName: sig.Actor, ActorToken: sig.ActorToken,
	}, true
}

// auditUpdate records an update only when something actually changed - an empty
// diff writes nothing (no noise for a no-op save).
func (a *API) auditUpdate(ctx context.Context, actor store.User, action, target, targetID, targetName, tenantID string, oldV, newV any) {
	changes := diffFields(oldV, newV)
	if len(changes) == 0 {
		return
	}
	a.audit(ctx, store.AuditEvent{
		At: time.Now().Unix(), ActorID: actor.ID, ActorName: actor.Username, Action: action,
		Target: target, TargetID: targetID, TargetName: targetName,
		TenantID: tenantID, Changes: changes,
	})
}

// auditEvent records a create/delete/action event (no diff, an optional note).
func (a *API) auditEvent(ctx context.Context, actor store.User, action, target, targetID, targetName, tenantID, detail string) {
	a.audit(ctx, store.AuditEvent{
		At: time.Now().Unix(), ActorID: actor.ID, ActorName: actor.Username, Action: action,
		Target: target, TargetID: targetID, TargetName: targetName,
		TenantID: tenantID, Detail: detail,
	})
}

// diffFields returns the top-level fields that differ between oldV and newV,
// keyed by JSON field name, each carrying the before/after value. Nested
// objects/arrays compare by their JSON encoding (a change anywhere inside
// surfaces the whole field). Ignored keys are skipped; sensitive leaf values
// (passwords, secrets, tokens, hashes) are redacted in place.
func diffFields(oldV, newV any) []store.FieldChange {
	om := toMap(oldV)
	nm := toMap(newV)
	seen := map[string]bool{}
	var keys []string
	for k := range om {
		if !auditIgnore[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	for k := range nm {
		if !auditIgnore[k] && !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var changes []store.FieldChange
	for _, k := range keys {
		ov, nv := om[k], nm[k]
		if jsonEqual(ov, nv) {
			continue
		}
		changes = append(changes, store.FieldChange{Field: k, From: redact(k, ov), To: redact(k, nv)})
	}
	return changes
}

// toMap renders any struct/map to a map[string]any via JSON (honours json tags,
// drops json:"-" fields). A non-object marshals to an empty map - the diff then
// treats it as "no fields", which is the safe default.
func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// jsonEqual compares two decoded JSON values by their canonical encoding (Go
// sorts map keys, so re-marshaling is order-stable).
func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

// redact replaces sensitive values with "***" so a secret never lands in the
// trail - either because the key itself is sensitive, or because it hides
// inside a nested object (walked recursively).
func redact(key string, v any) any {
	if isSensitiveKey(key) {
		return "***"
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = redact(k, val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redact(key, val)
		}
		return out
	default:
		if s, ok := v.(string); ok && len(s) > 120 && strings.HasPrefix(s, "data:") {
			return summarizeDataURI(s)
		}
		return v
	}
}

// summarizeDataURI keeps the trail readable and bounded. That an image changed
// is worth recording; a megabyte of base64 is not a diff anyone reads, and a
// branding save carries two of them - the before and the after. What is left
// says which kind of image and how big it was, which is all one can act on.
func summarizeDataURI(s string) string {
	head, _, _ := strings.Cut(s, ";")
	return fmt.Sprintf("%s (%d KiB)", head, len(s)*3/4/1024)
}

func isSensitiveKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "password") || strings.Contains(k, "secret") ||
		strings.Contains(k, "token") || strings.Contains(k, "hash")
}

// listAudit serves the audit trail, scoped to the caller. Query params:
// actor, target, targetId, since, until (unix seconds), limit.
func (a *API) listAudit(w http.ResponseWriter, r *http.Request, actor store.User) {
	f := store.AuditFilter{
		ActorID:  strings.TrimSpace(r.URL.Query().Get("actor")),
		Target:   strings.TrimSpace(r.URL.Query().Get("target")),
		TargetID: strings.TrimSpace(r.URL.Query().Get("targetId")),
		Kind:     strings.TrimSpace(r.URL.Query().Get("kind")),
		Since:    atoi64(r.URL.Query().Get("since")),
		Until:    atoi64(r.URL.Query().Get("until")),
		Limit:    int(atoi64(r.URL.Query().Get("limit"))),
	}
	// Scope by capability (RBAC-05): root sees all; otherwise the union of the
	// domains the caller administers. infra-admin -> routing plane targets,
	// app-admin -> identity targets, tenant admin -> their tenants (by tenant_id).
	// Administering nothing -> 403 (the trail is an administrative view, not an
	// empty page).
	scope, ok := a.auditScope(r.Context(), actor)
	if !ok {
		writeErr(w, http.StatusForbidden,
			"the audit trail requires root, a gateway/app-admin capability, or a tenant administration")
		return
	}
	f.Scope = scope
	if f.Kind != "" && f.Kind != store.AuditKindAdmin && f.Kind != store.AuditKindSecurity {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("kind %q: expected %q (the changes) or %q (the sign-ins and the ways into an account)",
			f.Kind, store.AuditKindAdmin, store.AuditKindSecurity))
		return
	}
	events, err := a.st.ListAuditEvents(r.Context(), f)
	if err != nil {
		a.internal(w, err)
		return
	}
	if events == nil {
		events = []store.AuditEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// auditScope is the visibility window of one caller, shared by the endpoint
// and by the agent's read_audit tool. nil scope with ok = the whole trail
// (root); ok false = this caller administers nothing, and the trail is an
// administrative view rather than an empty page.
//
// Shared rather than reimplemented: an agent that saw a wider trail than the
// console would be a way around RBAC-05, and it would be nobody's fault in
// particular.
func (a *API) auditScope(ctx context.Context, actor store.User) (*store.AuditScope, bool) {
	if actor.Root {
		return nil, true
	}
	scope := &store.AuditScope{}
	if actor.InfraAdmin {
		scope.Targets = append(scope.Targets, infraTargets...)
	}
	if actor.AppAdmin {
		scope.Targets = append(scope.Targets, appTargets...)
	}
	if administered, err := a.st.ListTenantsAdministeredBy(ctx, actor.ID); err == nil {
		for _, t := range administered {
			scope.TenantIDs = append(scope.TenantIDs, t.ID)
		}
	}
	if len(scope.Targets) == 0 && len(scope.TenantIDs) == 0 {
		return nil, false
	}
	return scope, true
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// livePerimeter is what one caller may watch on the live channel, and it is
// derived from the trail's partition rather than decided a second time: the same
// capability that shows somebody a kind of event in the audit screen is the one
// that lets the socket name it to them (CONSOLE-13).
//
// ok is false when the caller administers nothing - the same answer the trail
// gives, and for the same reason: the console's live channel is an
// administrative view, not an empty page.
//
// WHAT IS IN THE KEY, and what is deliberately not. The key is what makes two
// callers share one read, so it holds exactly what the rows are built from: the
// kinds, and whether they may be named. The ORGANISATIONS a tenant admin
// administers are not in it, because nothing published depends on them - a
// tenant's kinds stay quiet precisely because a shared read cannot check them -
// so every tenant admin on the installation shares one read instead of opening
// one apiece.
func (a *API) livePerimeter(ctx context.Context, actor store.User) (LivePerimeter, bool) {
	if _, ok := a.auditScope(ctx, actor); !ok {
		return LivePerimeter{}, false
	}
	if actor.Root {
		named := store.AuditTargetKinds()
		return LivePerimeter{
			Key: "root", Named: named,
			RoutingPlane: true, ApplicationPlane: true,
		}, true
	}
	named := map[string]bool{}
	kinds := map[string]bool{}
	p := LivePerimeter{}
	if actor.InfraAdmin {
		p.RoutingPlane = true
		add(named, store.AuditTrailTargets(store.AuditDomainInfra))
		add(kinds, store.AuditTargets(store.AuditDomainInfra))
	}
	if actor.AppAdmin {
		p.ApplicationPlane = true
		add(named, store.AuditTrailTargets(store.AuditDomainApp))
		add(kinds, store.AuditTargets(store.AuditDomainApp))
	}
	// A tenant administrator hears that their kinds moved and is told nothing
	// about them: their partition is by organisation, and one shared read cannot
	// check that this group belongs to an organisation THIS reader administers.
	// The screen asks the API, which checks it.
	if administered, err := a.st.ListTenantsAdministeredBy(ctx, actor.ID); err == nil && len(administered) > 0 {
		add(kinds, store.AuditTargets(store.AuditDomainTenant))
	}
	for kind := range named {
		p.Named = append(p.Named, kind)
		delete(kinds, kind)
	}
	for kind := range kinds {
		p.Quiet = append(p.Quiet, kind)
	}
	sort.Strings(p.Named)
	sort.Strings(p.Quiet)
	p.Key = strings.Join(p.Named, ",") + "|" + strings.Join(p.Quiet, ",")
	return p, true
}

func add(set map[string]bool, kinds []string) {
	for _, kind := range kinds {
		set[kind] = true
	}
}

// auditSettings is the trail's own setting, and what may be chosen.
type auditSettings struct {
	RetentionDays int   `json:"retentionDays"`
	Choices       []int `json:"choices"`
}

func (a *API) getAuditSettings(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, auditSettings{
		RetentionDays: a.st.AuditRetentionDays(r.Context()), Choices: store.AuditRetentionChoices,
	})
}

func (a *API) putAuditSettings(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p auditSettings
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed audit settings: "+err.Error())
		return
	}
	before := a.st.AuditRetentionDays(r.Context())
	if err := a.st.SetAuditRetentionDays(r.Context(), p.RetentionDays); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(r.Context(), actor, "audit.retention", "settings", "", "", "",
		map[string]int{"retentionDays": before}, map[string]int{"retentionDays": p.RetentionDays})
	a.getAuditSettings(w, r, actor)
}

// exportLimit bounds one export. A year of a busy installation's trail fits;
// past it the file says so on its last line rather than stopping silently.
const exportLimit = 100000

// exportAudit writes the trail as CSV (STORE-06): the filters of the screen,
// the perimeter of the caller - an export never shows more than the screen -
// and one row per event. The diff travels as JSON in its own column: a
// spreadsheet reads it as text, a script parses it.
//
// Enterprise, and audited: the file carries addresses and names, and who took
// it out is a question somebody will ask.
func (a *API) exportAudit(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require("exporting the audit trail"); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	scope, ok := a.auditScope(r.Context(), actor)
	if !ok {
		writeErr(w, http.StatusForbidden,
			"the audit trail requires root, a gateway/app-admin capability, or a tenant administration")
		return
	}
	f := store.AuditFilter{
		Kind:   strings.TrimSpace(r.URL.Query().Get("kind")),
		Target: strings.TrimSpace(r.URL.Query().Get("target")),
		Since:  atoi64(r.URL.Query().Get("since")),
		Until:  atoi64(r.URL.Query().Get("until")),
		Limit:  exportLimit + 1,
		Scope:  scope,
	}
	events, err := a.st.ListAuditEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	truncated := len(events) > exportLimit
	if truncated {
		events = events[:exportLimit]
	}
	a.auditEvent(r.Context(), actor, "audit.export", "audit", "", "", "",
		fmt.Sprintf("%d events, kind=%s target=%s", len(events), f.Kind, f.Target))

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="meerkat-audit.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"at", "action", "actor", "token", "target", "targetId", "targetName",
		"organisation", "ip", "detail", "changes"})
	for _, e := range events {
		changes := ""
		if len(e.Changes) > 0 {
			if b, err := json.Marshal(e.Changes); err == nil {
				changes = string(b)
			}
		}
		actorName := e.ActorName
		if actorName == "" {
			actorName = e.ActorID
		}
		org := e.TenantName
		if org == "" {
			org = e.TenantID
		}
		_ = cw.Write([]string{
			time.Unix(e.At, 0).UTC().Format(time.RFC3339), e.Action, actorName, e.ActorToken,
			e.Target, e.TargetID, e.TargetName, org, e.IP, e.Detail, changes,
		})
	}
	if truncated {
		_ = cw.Write([]string{"", "truncated", "", "", "", "", "", "", "", fmt.Sprintf("only the %d most recent events: narrow the period", exportLimit), ""})
	}
	cw.Flush()
}
