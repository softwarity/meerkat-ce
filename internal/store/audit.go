package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// NewEventID mints an identifier that SORTS BY TIME: twelve hex digits of unix
// milliseconds, then ten random bytes.
//
// The randomness alone was enough to be unique and useless for anything else.
// Both tables that use this stamp their rows with a timestamp in SECONDS, and
// ask "which is the newest" - a question a second cannot answer when two rows
// share one. SQLite's rowid was answering it, silently, and PostgreSQL has no
// such thing.
//
// So the id answers it instead. Lexicographic order is now chronological to
// the millisecond, ties inside one millisecond fall to the random half, and
// two instances writing in the same millisecond are genuinely concurrent -
// any order between them is as true as another.
//
// A prefix rather than a sequence, deliberately: a sequence would need a
// number handed out by somebody, and with several gateways that somebody is a
// round trip and a retry on every write. A clock needs nobody.
// Exported because the audit funnel stamps the event BEFORE writing it: the
// same identifier then names it in the table and on the console's live channel,
// and the version a screen compares is that identifier rather than a second
// (two writes inside one second on the same kind would have shared a version,
// and the second would never have been published).
//
// Inside one millisecond a sequence follows the clock, so two events one node
// writes back to back keep their order: a refused sign-in and the lock-out it
// caused land in the same millisecond, and a trail that showed the lock first
// would tell the story backwards. Two NODES in one millisecond still order at
// random, which nobody can read into anything.
func NewEventID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	ms, seq := eventClock.next()
	// Twelve hex digits hold milliseconds until the year 10889, and keep the
	// width fixed - which is what makes the comparison lexicographic. Four
	// more for the sequence, and the width stays the 32 it always was.
	return fmt.Sprintf("%012x%04x%s", ms, seq, hex.EncodeToString(b[:]))
}

var eventClock clockSeq

type clockSeq struct {
	mu   sync.Mutex
	last int64
	seq  uint16
}

func (c *clockSeq) next() (int64, uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= c.last {
		// The same millisecond, or a clock stepped back: stay on the last one
		// and count, so the order this node wrote in is the order it reads.
		ms = c.last
		c.seq++
		if c.seq == 0 {
			// 65536 events in one millisecond: borrow the next one.
			ms++
		}
	} else {
		c.seq = 0
	}
	c.last = ms
	return ms, c.seq
}

// AuditEvent is one recorded administrative mutation (the audit trail). It
// captures WHO (actor), WHAT (action + target), and the exact field-level diff
// - only what changed, with before/after - so a group-mode edit reads as
// "groupMode: MULTIPLE -> SINGLE", never "modified tenant X".
type AuditEvent struct {
	ID string `json:"id"`
	At int64  `json:"at"`
	// ActorID survives the actor's deletion (no FK); ActorName is joined on read.
	ActorID   string `json:"actorId"`
	ActorName string `json:"actorName,omitempty"`
	// ActorToken names the control-plane token that acted, when one did
	// (MCP-03). An agent holds a token minted on somebody's account, so the
	// account alone would read as if that person had done it by hand.
	ActorToken string `json:"actorToken,omitempty"`
	// Action is a dotted verb: "tenant.update", "member.remove", "settings.update"...
	Action string `json:"action"`
	// Target is the object kind ("tenant", "user", "membership", "role",
	// "group", "settings", "route"); TargetID/TargetName identify the instance
	// (TargetName captured at write time so it reads even after a delete).
	Target     string `json:"target"`
	TargetID   string `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	// TenantID scopes tenant-admin visibility ("" = a global/application event).
	TenantID string `json:"tenantId,omitempty"`
	// TenantName is the organisation's current name, joined on read like the
	// actor's - an id is not something a person reading the trail recognises.
	TenantName string `json:"tenantName,omitempty"`
	// Changes is the field-level diff; Detail is a free note for create/delete
	// and events that have no meaningful before/after.
	Changes []FieldChange `json:"changes,omitempty"`
	Detail  string        `json:"detail,omitempty"`
	// IP is the address the gateway resolved for the request, on the events
	// of an account's own security (a sign-in, a refusal, a new factor). A
	// refused sign-in with no address is a line nobody can act on.
	IP string `json:"ip,omitempty"`
}

// FieldChange is one field's before/after inside an AuditEvent. From/To are the
// decoded JSON values (string, number, bool, or a nested object/array).
type FieldChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// AuditScope is a visibility window: an event is visible when its target is in
// Targets (a whole domain, e.g. gateway = route/theme) OR its tenant_id is in
// TenantIDs (the tenants the caller administers). The two combine as OR, so a
// caller who is both app-admin and a tenant admin sees the union. Both empty
// means "nothing" (the caller administers no domain).
type AuditScope struct {
	Targets   []string
	TenantIDs []string
}

// The kinds of object an administrative event names, and who administers each.
//
// ONE table, read by two things that must not disagree: the trail's own
// partition by capability (RBAC-05, admin.auditScope) and the console's live
// channel, which tells a screen that its kind moved (CONSOLE-13). A kind
// classed here is classed for both, and a kind that is NOT here is one nobody
// would ever hear about - which is why a test refuses one.
const (
	AuditDomainInfra  = "infra"
	AuditDomainApp    = "app"
	AuditDomainTenant = "tenant"
)

// AuditTargetAll is the kind that stands for every other one: exporting,
// importing or restoring a configuration replays the whole installation, so
// what moved is not one kind but all of them.
const AuditTargetAll = "config"

// AuditTarget is what is known about one kind, and the two fields answer two
// different questions - which is why they are not one.
type AuditTarget struct {
	// Domains are the administrative capabilities that WRITE this kind, read
	// from the funnel its endpoints are mounted behind. They decide who is told
	// that it moved. Empty means root alone.
	Domains []string
	// Scoped says a row of this kind may belong to an organisation, so the
	// trail partitions it BY ORGANISATION (tenant_id) rather than by domain.
	// Without this, listing the kind for a whole domain would show an
	// application administrator every organisation's rows - which is the one
	// thing the tenant partition exists to prevent. The live channel still
	// wakes the domains above: that a kind moved is not one of its rows.
	Scoped bool
}

// auditTargets is the table. Keep it sorted by domain, then by name.
var auditTargets = map[string]AuditTarget{
	// The routing plane: what an infra-admin runs.
	"route":        {Domains: []string{AuditDomainInfra}},
	"authprovider": {Domains: []string{AuditDomainInfra}},
	"certificate":  {Domains: []string{AuditDomainInfra}},
	// The application's identity and appearance: what an app-admin runs.
	"theme":    {Domains: []string{AuditDomainApp}},
	"locale":   {Domains: []string{AuditDomainApp}},
	"user":     {Domains: []string{AuditDomainApp}},
	"role":     {Domains: []string{AuditDomainApp}},
	"schedule": {Domains: []string{AuditDomainApp}},
	// The installation's settings are a single kind covering both planes - TLS
	// and the proxy's limits as much as the account model - so both are told.
	"settings": {Domains: []string{AuditDomainInfra, AuditDomainApp}},
	// A report is visible to the application's administrators whole, and to a
	// tenant administrator for their own organisations (admin.issueScope), so
	// it is partitioned the same way the endpoint partitions it.
	"issue": {Domains: []string{AuditDomainInfra, AuditDomainApp, AuditDomainTenant}},
	// An organisation and what hangs off it. Every row belongs to one, so the
	// trail shows it by organisation and never by domain.
	"tenant":     {Domains: []string{AuditDomainTenant}, Scoped: true},
	"membership": {Domains: []string{AuditDomainTenant}, Scoped: true},
	"group":      {Domains: []string{AuditDomainTenant}, Scoped: true},
	"grouprule":  {Domains: []string{AuditDomainTenant}, Scoped: true},
	// The vault has an entry per plane AND per organisation (vault.ScopeInfra,
	// ScopeApp, TenantScope), so all three are told that it moved while the
	// trail keeps naming entries to root alone.
	"vault": {Domains: []string{AuditDomainInfra, AuditDomainApp, AuditDomainTenant}, Scoped: true},
	// Root's own business: the control-plane tokens, the configuration as a
	// document, and a snapshot of the database.
	"token":         {},
	"configuration": {},
	AuditTargetAll:  {},
	"backup":        {},
	// The SECURITY of an account rather than its administration (AUD-01): a
	// sign-in, a refusal, a new factor, a password changed by its owner. Written
	// by the identity handlers, never by the control plane, so no screen is
	// woken by them - a user list re-read at every sign-in of anybody would be
	// noise. An account of the applications is the application
	// administrator's business; a sign-in to the CONSOLE is root's alone, since
	// it says where and when the people who run the gateway work.
	// The trail's own export (STORE-06): root reads who took the trail out.
	"audit":            {},
	AuditTargetAccount: {Domains: []string{AuditDomainApp}},
	AuditTargetConsole: {},
}

// The two kinds of the security trail, apart from the administrative one.
const (
	AuditTargetAccount = "account"
	AuditTargetConsole = "console"
)

// AuditSecurityTargets are the kinds a "security" filter keeps and an
// "administration" filter leaves out.
var AuditSecurityTargets = []string{AuditTargetAccount, AuditTargetConsole}

// The two halves of the trail, as a filter names them.
const (
	AuditKindAdmin    = "admin"
	AuditKindSecurity = "security"
)

// AuditTargetOf answers what is known about a kind, and whether it is known at
// all. An unclassed kind is a defect rather than a default: nobody would be
// told it moved, and nobody would see it in the trail but root.
func AuditTargetOf(target string) (AuditTarget, bool) {
	t, ok := auditTargets[target]
	return t, ok
}

// AuditTargetKinds lists every classed kind, sorted - for a status page and for
// the test that refuses an unclassed one.
func AuditTargetKinds() []string {
	kinds := make([]string, 0, len(auditTargets))
	for kind := range auditTargets {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// AuditTargets lists the kinds one capability administers, sorted. What the
// live channel wakes.
func AuditTargets(domain string) []string {
	return auditTargetsWhere(domain, false)
}

// AuditTrailTargets lists the kinds one capability may read WHOLE in the trail,
// sorted: its own kinds, minus the ones partitioned by organisation. What
// admin.auditScope fills AuditScope.Targets with.
func AuditTrailTargets(domain string) []string {
	return auditTargetsWhere(domain, true)
}

func auditTargetsWhere(domain string, globalOnly bool) []string {
	var kinds []string
	for kind, t := range auditTargets {
		if globalOnly && t.Scoped {
			continue
		}
		if slices.Contains(t.Domains, domain) {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// AuditFilter narrows ListAuditEvents. All fields are optional; the zero value
// lists the most recent events across everything.
type AuditFilter struct {
	ActorID  string // exact actor
	Target   string // exact target kind
	TargetID string // exact target instance
	// Scope, when non-nil, restricts visibility (see AuditScope). Nil means no
	// restriction (root, or an internal caller).
	Scope *AuditScope
	// Kind keeps one half of the trail: AuditKindAdmin (what was changed) or
	// AuditKindSecurity (who signed in, and how). Empty keeps both.
	Kind  string
	Since int64 // at >= Since when > 0
	Until int64 // at <= Until when > 0
	Limit int   // capped result count (default 200)
}

// AddAuditEvent appends one event. It stamps ID (if empty) and At (if zero) so
// callers may pass a bare event.
func (s *Store) AddAuditEvent(ctx context.Context, ev AuditEvent) error {
	if ev.ID == "" {
		ev.ID = NewEventID()
	}
	// The row keeps seconds; the copy for the collector keeps the instant. Two
	// identical actions in the same second - two routes saved at once - are
	// otherwise the same line at the same time to Loki, which keeps one.
	now := time.Now()
	if ev.At == 0 {
		ev.At = now.Unix()
	}
	at := time.Unix(ev.At, 0)
	if ev.At == now.Unix() {
		at = now
	}
	changes := "[]"
	if len(ev.Changes) > 0 {
		b, err := json.Marshal(ev.Changes)
		if err != nil {
			return fmt.Errorf("store: audit %q: encode changes: %w", ev.Action, err)
		}
		changes = string(b)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events (id, at, actor_id, actor_token, action, target, target_id, target_name, tenant_id, changes, detail, ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, ev.At, ev.ActorID, ev.ActorToken, ev.Action, ev.Target, ev.TargetID, ev.TargetName, ev.TenantID, changes, ev.Detail, ev.IP)
	if err != nil {
		return fmt.Errorf("store: add audit event %q: %w", ev.Action, err)
	}
	// And a copy to the collector, when the audit is sent there (AUD-03).
	// Queued, never awaited: recording an event does not wait on the network.
	if tracing.Shipping() {
		tracing.ShipAudit(s.auditRecord(ctx, ev, changes, at))
	}
	return nil
}

// auditRecord is ev as it leaves for the collector: the action in the body,
// everything else as attributes - the actor under OpenTelemetry's own user.*
// names, the client address as client.address - and the trace id of the
// request that caused it. The names are resolved here, best-effort, because
// the row only keeps ids and a collector has no table to join them with.
func (s *Store) auditRecord(ctx context.Context, ev AuditEvent, changes string, at time.Time) tracing.AuditRecord {
	attrs := []tracing.Attr{
		tracing.String("audit.id", ev.ID),
		tracing.String("audit.action", ev.Action),
		tracing.String("audit.target", ev.Target),
		tracing.String("audit.target.id", ev.TargetID),
		tracing.String("audit.target.name", ev.TargetName),
		tracing.String("audit.detail", ev.Detail),
		tracing.String("user.id", ev.ActorID),
		tracing.String("meerkat.token", ev.ActorToken),
		tracing.String("meerkat.tenant.id", ev.TenantID),
		tracing.String("client.address", ev.IP),
	}
	if changes != "[]" {
		attrs = append(attrs, tracing.String("audit.changes", changes))
	}
	if ev.ActorID != "" {
		if u, err := s.GetUserByID(ctx, ev.ActorID); err == nil {
			attrs = append(attrs, tracing.String("user.name", u.Username))
		}
	}
	if ev.TenantID != "" {
		if t, err := s.GetTenant(ctx, ev.TenantID); err == nil {
			attrs = append(attrs, tracing.String("meerkat.tenant.name", t.Name))
		}
	}
	return tracing.AuditRecord{
		Time:    at,
		TraceID: tracing.ID(ctx),
		Body:    ev.Action,
		Attrs:   attrs,
		// The data plane's own events are the accounts' sign-ins; everything
		// else - a change, a console sign-in - is the console's.
		Console: ev.Target != AuditTargetAccount,
	}
}

// ListAuditEvents returns matching events newest first, each enriched with the
// actor's current username (best-effort - "" if the actor is gone).
func (s *Store) ListAuditEvents(ctx context.Context, f AuditFilter) ([]AuditEvent, error) {
	var where []string
	var args []any
	if f.ActorID != "" {
		where = append(where, "e.actor_id = ?")
		args = append(args, f.ActorID)
	}
	if f.Target != "" {
		where = append(where, "e.target = ?")
		args = append(args, f.Target)
	}
	if f.TargetID != "" {
		where = append(where, "e.target_id = ?")
		args = append(args, f.TargetID)
	}
	switch f.Kind {
	case "":
	case AuditKindAdmin, AuditKindSecurity:
		not := ""
		if f.Kind == AuditKindAdmin {
			not = "NOT "
		}
		ph := make([]string, len(AuditSecurityTargets))
		for i, tgt := range AuditSecurityTargets {
			ph[i] = "?"
			args = append(args, tgt)
		}
		where = append(where, "e.target "+not+"IN ("+strings.Join(ph, ", ")+")")
	default:
		return nil, fmt.Errorf("store: audit kind %q: expected %q or %q", f.Kind, AuditKindAdmin, AuditKindSecurity)
	}
	if f.Scope != nil {
		if len(f.Scope.Targets) == 0 && len(f.Scope.TenantIDs) == 0 {
			return []AuditEvent{}, nil // no domain, no tenant -> sees nothing
		}
		var ors []string
		if len(f.Scope.Targets) > 0 {
			ph := make([]string, len(f.Scope.Targets))
			for i, tgt := range f.Scope.Targets {
				ph[i] = "?"
				args = append(args, tgt)
			}
			ors = append(ors, "e.target IN ("+strings.Join(ph, ", ")+")")
		}
		if len(f.Scope.TenantIDs) > 0 {
			ph := make([]string, len(f.Scope.TenantIDs))
			for i, id := range f.Scope.TenantIDs {
				ph[i] = "?"
				args = append(args, id)
			}
			ors = append(ors, "e.tenant_id IN ("+strings.Join(ph, ", ")+")")
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if f.Since > 0 {
		where = append(where, "e.at >= ?")
		args = append(args, f.Since)
	}
	if f.Until > 0 {
		where = append(where, "e.at <= ?")
		args = append(args, f.Until)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT e.id, e.at, e.actor_id, COALESCE(u.username, ''), e.actor_token, e.action, e.target,
	             e.target_id, e.target_name, e.tenant_id, COALESCE(t.name, ''), e.changes, e.detail, e.ip
	      FROM audit_events e
	      LEFT JOIN users u ON u.id = e.actor_id
	      LEFT JOIN tenants t ON t.id = e.tenant_id`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	// Newest first, and within one second the LAST written wins: ids are random
	// hex, so ordering by them shuffled events that happened in a known order.
	// It reads wrong on the audit screen, and it made the restore point borrow
	// the words of the change it was undoing rather than its own.
	q += " ORDER BY e.at DESC, e.id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var changes string
		if err := rows.Scan(&ev.ID, &ev.At, &ev.ActorID, &ev.ActorName, &ev.ActorToken, &ev.Action, &ev.Target,
			&ev.TargetID, &ev.TargetName, &ev.TenantID, &ev.TenantName, &changes, &ev.Detail, &ev.IP); err != nil {
			return nil, fmt.Errorf("store: scan audit event: %w", err)
		}
		if changes != "" && changes != "[]" {
			if err := json.Unmarshal([]byte(changes), &ev.Changes); err != nil {
				return nil, fmt.Errorf("store: audit %q: bad changes: %w", ev.ID, err)
			}
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list audit events: %w", err)
	}
	return events, nil
}

// PurgeAuditEventsBefore drops events older than cutoff (retention upkeep) and
// reports how many were removed.
func (s *Store) PurgeAuditEventsBefore(ctx context.Context, cutoff int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: purge audit events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// SettingAuditRetention is how many days the trail keeps an event (AUD-02).
// Root's alone: whoever may shorten the trail may erase their own traces with
// it, so it is not on the screens an application or infrastructure
// administrator writes. It stays with the installation: a retention is a
// compliance decision of this place, not part of a configuration's shape.
const SettingAuditRetention = "auditRetention"

// AuditRetentionChoices are the lifetimes offered, in days: three months to
// five years. A choice rather than a number, because a typo in a retention is
// a trail gone at the next purge.
var AuditRetentionChoices = []int{90, 180, 365, 730, 1825}

// DefaultAuditRetention is a year.
const DefaultAuditRetention = 365

// AuditRetentionDays reads the retention, the default when unset or invalid.
func (s *Store) AuditRetentionDays(ctx context.Context) int {
	var d int
	if err := s.GetSetting(ctx, SettingAuditRetention, &d); err != nil || !slices.Contains(AuditRetentionChoices, d) {
		return DefaultAuditRetention
	}
	return d
}

// SetAuditRetentionDays stores it, refusing a lifetime that is not offered.
func (s *Store) SetAuditRetentionDays(ctx context.Context, days int) error {
	if !slices.Contains(AuditRetentionChoices, days) {
		return fmt.Errorf("retentionDays %d: expected one of %v", days, AuditRetentionChoices)
	}
	return s.SetSetting(ctx, SettingAuditRetention, days)
}
