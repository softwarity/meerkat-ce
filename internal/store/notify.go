package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// The notifications' half of the store (NOTIF-05): what the services behind
// the gateway tell their users, and who was told.
//
// The decisions live in the Enterprise package (ee/notify) - who may send,
// what a link resolves to, who is in the audience. This file only keeps what
// was decided: one row per notification, one per recipient, and the settings
// the console writes.

// SettingNotifications holds the notifications' settings. Not exported with a
// configuration, like the internal plane's access: the callers it lists are
// this platform's own names.
const SettingNotifications = "notifications"

// The bounds of a notification's life: what a sender gets when it says
// nothing, and the most it may ask. Personal data is not kept forever.
const (
	NotificationDefaultLife = 30 * 24 * time.Hour
	NotificationMaxLife     = 90 * 24 * time.Hour
)

// NotificationDefaultCap is the audience a publication may reach before it is
// refused as a targeting mistake, when the console set none.
const NotificationDefaultCap = 5000

// NotificationLevels are the levels, in the order a screen offers them.
var NotificationLevels = []string{"info", "success", "warning", "error"}

// The three levels that decide whether a tenant receives notifications, from
// the top: root, the application admin (app-admin) and the tenant admin
// (owner or ADMIN member). Each one can only take away.
const (
	NotifyLevelRoot     = "root"
	NotifyLevelAppAdmin = "app-admin"
	NotifyLevelTenant   = "tenant-admin"
)

// What a level sees of one tenant (NotifySettings.View).
const (
	// NotifyOn: allowed by this level, and received.
	NotifyOn = "on"
	// NotifyOffBelow: allowed by this level, turned off by a level under it.
	NotifyOffBelow = "offBelow"
	// NotifyOff: turned off by this level.
	NotifyOff = "off"
	// NotifyOffAbove: turned off by a level above it, which this one cannot
	// turn back on.
	NotifyOffAbove = "offAbove"
)

// NotifySettings is the setting.
//
// A tenant receives notifications only when every level allows it: root's
// switch is on, and none of the three lists names it. The lists hold the
// exceptions, so a tenant created tomorrow is allowed everywhere.
type NotifySettings struct {
	// Enabled is root's switch for the whole feature, off at installation.
	Enabled bool `json:"enabled"`
	// RootOff, AdminOff and TenantOff are the tenants each level turned off.
	// TenantOff is not read on a single-tenant gateway: the tenant level does
	// not exist there.
	RootOff   []string `json:"rootOff"`
	AdminOff  []string `json:"adminOff"`
	TenantOff []string `json:"tenantOff"`
	// Legacy is the one list of tenants the setting held before the levels.
	// Read once and dropped at the next write.
	Legacy []string `json:"tenants,omitempty"`
	// Cap bounds one publication's audience; 0 is NotificationDefaultCap.
	Cap int `json:"cap,omitempty"`
	// Access is who may publish on the internal plane: its own mode and list,
	// since the services that notify are not the ones that schedule.
	Access InternalAccess `json:"access"`
}

// cleanIDs trims, drops the empty and the repeated, keeps the order.
func cleanIDs(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// SanitizeNotifySettings normalises the setting and refuses what cannot be
// enforced.
func SanitizeNotifySettings(n *NotifySettings) error {
	n.RootOff, n.AdminOff, n.TenantOff = cleanIDs(n.RootOff), cleanIDs(n.AdminOff), cleanIDs(n.TenantOff)
	n.Legacy = nil
	if n.Cap < 0 {
		return fmt.Errorf("audience cap %d: give a number of people, or 0 for the default (%d)", n.Cap, NotificationDefaultCap)
	}
	return SanitizeInternalAccess(&n.Access)
}

func notifyOff() NotifySettings {
	return NotifySettings{RootOff: []string{}, AdminOff: []string{}, TenantOff: []string{},
		Access: InternalAccess{Mode: InternalAccessToken, Callers: []string{}}}
}

// NotifySettings reads the setting. Absent is off everywhere, a token asked
// of every sender: a feature nobody turned on opens nothing.
func (s *Store) NotifySettings(ctx context.Context) NotifySettings {
	n := notifyOff()
	if err := s.GetSetting(ctx, SettingNotifications, &n); err != nil {
		return notifyOff()
	}
	// The setting before the levels: one list of the tenants it was on for.
	// Kept as root's decision, the others turned off by root.
	if legacy := cleanIDs(n.Legacy); len(legacy) > 0 && !n.Enabled &&
		len(n.RootOff)+len(n.AdminOff)+len(n.TenantOff) == 0 {
		if tenants, err := s.ListTenants(ctx); err == nil {
			n.Enabled = true
			for _, t := range tenants {
				if !slices.Contains(legacy, t.ID) {
					n.RootOff = append(n.RootOff, t.ID)
				}
			}
		}
	}
	if SanitizeNotifySettings(&n) != nil {
		return notifyOff()
	}
	return n
}

// SetNotifySettings stores it, sanitised.
func (s *Store) SetNotifySettings(ctx context.Context, n NotifySettings) error {
	if err := SanitizeNotifySettings(&n); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingNotifications, n)
}

// NotificationsOn reports whether a tenant receives notifications, every
// level read: what the publication, the page and the badges ask.
func (s *Store) NotificationsOn(ctx context.Context, tenantID string) bool {
	return s.NotifySettings(ctx).Effective(tenantID, s.Tenancy(ctx) == TenancyMulti)
}

// AudienceCap is the cap in force.
func (n NotifySettings) AudienceCap() int {
	if n.Cap <= 0 {
		return NotificationDefaultCap
	}
	return n.Cap
}

// NotifyLevels are the levels in force, from the top: the tenant level only
// on a multi-tenant gateway.
func NotifyLevels(multi bool) []string {
	if multi {
		return []string{NotifyLevelRoot, NotifyLevelAppAdmin, NotifyLevelTenant}
	}
	return []string{NotifyLevelRoot, NotifyLevelAppAdmin}
}

// offAt reports whether a level turned a tenant off. Root's switch counts
// as root turning every tenant off, except in root's own view (withSwitch
// false), where the switch is shown on its own.
func (n NotifySettings) offAt(level, tenantID string, withSwitch bool) bool {
	switch level {
	case NotifyLevelRoot:
		return (withSwitch && !n.Enabled) || slices.Contains(n.RootOff, tenantID)
	case NotifyLevelAppAdmin:
		return slices.Contains(n.AdminOff, tenantID)
	case NotifyLevelTenant:
		return slices.Contains(n.TenantOff, tenantID)
	}
	return false
}

// NotifyLevelWho names a level for the person reading a refusal.
func NotifyLevelWho(level string) string {
	switch level {
	case NotifyLevelAppAdmin:
		return "the application admin"
	case NotifyLevelTenant:
		return "the tenant admin"
	}
	return "root"
}

// Effective reports whether a tenant receives notifications: every level
// allows it.
func (n NotifySettings) Effective(tenantID string, multi bool) bool {
	return n.OffBy(tenantID, multi) == ""
}

// OffBy names the highest level that turned a tenant off, "" when none did.
func (n NotifySettings) OffBy(tenantID string, multi bool) string {
	if tenantID == "" {
		return NotifyLevelRoot
	}
	for _, l := range NotifyLevels(multi) {
		if n.offAt(l, tenantID, true) {
			return l
		}
	}
	return ""
}

// View is a tenant as one level sees it: on, off by itself, off by a level
// below it (named), or off by a level above it (named), which it cannot
// undo. Root's own view leaves its switch out: the screen shows it apart.
func (n NotifySettings) View(level, tenantID string, multi bool) (state, by string) {
	levels := NotifyLevels(multi)
	at := slices.Index(levels, level)
	if at < 0 {
		return NotifyOff, NotifyLevelRoot
	}
	for i, l := range levels {
		if !n.offAt(l, tenantID, level != NotifyLevelRoot) {
			continue
		}
		switch {
		case i < at:
			return NotifyOffAbove, l
		case i == at:
			return NotifyOff, l
		default:
			return NotifyOffBelow, l
		}
	}
	return NotifyOn, ""
}

// BelowBy names the first level under the given one that turned a tenant
// off, whatever the given level decided itself: what a level finds again
// when it allows a tenant it had turned off.
func (n NotifySettings) BelowBy(level, tenantID string, multi bool) string {
	levels := NotifyLevels(multi)
	at := slices.Index(levels, level)
	if at < 0 {
		return ""
	}
	for _, l := range levels[at+1:] {
		if n.offAt(l, tenantID, true) {
			return l
		}
	}
	return ""
}

// Decide writes one level's decision on the tenants it names: off turns
// them off, on allows them again. Forcing a tenant on also clears the
// decision of every level below - they may turn it off again afterwards.
// Root's switch is not touched here.
func (n *NotifySettings) Decide(level string, off, on, force []string) error {
	lists := map[string]*[]string{
		NotifyLevelRoot: &n.RootOff, NotifyLevelAppAdmin: &n.AdminOff, NotifyLevelTenant: &n.TenantOff,
	}
	own, ok := lists[level]
	if !ok {
		return fmt.Errorf("notification level %q: allowed are %s, %s and %s", level,
			NotifyLevelRoot, NotifyLevelAppAdmin, NotifyLevelTenant)
	}
	for _, t := range append(slices.Clone(on), force...) {
		if slices.Contains(off, t) {
			return fmt.Errorf("tenant %q is both turned off and on: name it once", t)
		}
	}
	for _, t := range off {
		if !slices.Contains(*own, t) {
			*own = append(*own, t)
		}
	}
	for _, t := range append(slices.Clone(on), force...) {
		*own = slices.DeleteFunc(*own, func(x string) bool { return x == t })
	}
	below := map[string][]string{
		NotifyLevelRoot:     {NotifyLevelAppAdmin, NotifyLevelTenant},
		NotifyLevelAppAdmin: {NotifyLevelTenant},
	}[level]
	for _, l := range below {
		for _, t := range force {
			*lists[l] = slices.DeleteFunc(*lists[l], func(x string) bool { return x == t })
		}
	}
	return nil
}

// Localized is a text by language. "" is the key of a text sent as one
// string, in no language in particular.
type Localized map[string]string

// Pick chooses the text for a reader: their language, then English, then the
// one sent without a language, then the first in alphabetical order - the same
// answer on every node and every reload.
func (l Localized) Pick(lang string) string {
	if len(l) == 0 {
		return ""
	}
	lang = strings.ToLower(lang)
	if v, ok := l[lang]; ok {
		return v
	}
	if base, _, ok := strings.Cut(lang, "-"); ok {
		if v, ok := l[base]; ok {
			return v
		}
	}
	for _, k := range []string{"en", ""} {
		if v, ok := l[k]; ok {
			return v
		}
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return l[keys[0]]
}

// NotificationAction is one thing a reader can do from a notification: a
// request their browser makes to URL, through the data plane. Route is the
// route serving it, "" for an external link. Method is one of
// NotificationMethods, GET for one stored before actions had a method; Body
// is the JSON value sent with it, never with a GET.
type NotificationAction struct {
	Label  Localized       `json:"label"`
	URL    string          `json:"url"`
	Route  string          `json:"route,omitempty"`
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// NotificationMethods are the methods an action may be made with.
var NotificationMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// Notification is one message, as stored.
type Notification struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	Sender    string `json:"sender"`
	SenderVia string `json:"senderVia"`
	// Ref is the id its sender gave it, required - one per sender, shared
	// by the copies of a message sent to every tenant ("" only on what was
	// stored before it was). ID is the gateway's own, for its pages and its
	// console, never handed to the sender.
	Ref         string               `json:"ref"`
	Level       string               `json:"level"`
	Title       Localized            `json:"title"`
	Body        Localized            `json:"body"`
	DetailURL   string               `json:"detailUrl,omitempty"`
	DetailRoute string               `json:"detailRoute,omitempty"`
	Actions     []NotificationAction `json:"actions,omitempty"`
	Meta        map[string]string    `json:"meta,omitempty"`
	Mail        bool                 `json:"mail,omitempty"`
	Recipients  int                  `json:"recipients"`
	CreatedAt   int64                `json:"createdAt"`
	ExpiresAt   int64                `json:"expiresAt"`
	WithdrawnAt int64                `json:"withdrawnAt,omitempty"`
	// BroadcastID is shared by the copies of one message sent to every
	// tenant, "" for a message sent to one. Not said to a sender, nor to the
	// console: each copy is its tenant's notification there.
	BroadcastID string `json:"-"`
	// Ack asks its readers to acknowledge it: AckSingle or AckAll, "" when it
	// does not ask. OnAck is where the sender's service is called at each
	// acknowledgement. AckCompleteAt is when the request was closed - the
	// first acknowledgement of a single one, the last reader's of an all one
	// - and AckBy the account that closed a single one.
	Ack           string            `json:"ack,omitempty"`
	OnAck         *NotificationLink `json:"onAck,omitempty"`
	AckCompleteAt int64             `json:"ackCompleteAt,omitempty"`
	AckBy         string            `json:"-"`
	// EndedAs says how it ended, WithdrawnAt being when: EndedWithdrawn or
	// EndedReplaced, "" while it is live. ReplacedBy names the notification
	// that replaced it.
	EndedAs    string `json:"endedAs,omitempty"`
	ReplacedBy string `json:"replacedBy,omitempty"`
	// Exercise marks a message sent to practise: it keeps a realistic level,
	// and wears the EXERCISE label wherever it shows.
	Exercise bool `json:"exercise,omitempty"`
	// AudienceRoles and AudienceUsers are the audience its sender named,
	// kept (AudienceKept) so an account given the rights afterwards is given
	// it then. Neither: the whole tenant. Sent before they were kept,
	// AudienceKept is false, and it is given to nobody afterwards.
	AudienceRoles []string `json:"audienceRoles,omitempty"`
	AudienceUsers []string `json:"audienceUsers,omitempty"`
	AudienceKept  bool     `json:"-"`
}

// How a notification ends before it expires.
const (
	// EndedWithdrawn: its sender, or an administrator, took it back.
	EndedWithdrawn = "withdrawn"
	// EndedReplaced: its sender published another one in its place.
	EndedReplaced = "replaced"
)

// Ended says how it ended, "" while it is live: a withdrawal written before
// ended_as existed reads as one.
func (n Notification) Ended() string {
	if n.WithdrawnAt == 0 {
		return ""
	}
	if n.EndedAs == "" {
		return EndedWithdrawn
	}
	return n.EndedAs
}

// The ways a notification asks to be acknowledged.
const (
	// AckSingle: the first reader to acknowledge closes it for everybody (an
	// on-call alert).
	AckSingle = "single"
	// AckAll: every reader acknowledges their own, and it is complete when the
	// last one did (a safety instruction).
	AckAll = "all"
)

// NotificationLink is an address a notification calls or leads to, public,
// and the route serving it.
type NotificationLink struct {
	URL   string `json:"url"`
	Route string `json:"route,omitempty"`
}

// Owes reports whether a reader still owes the acknowledgement a live
// notification asks for: their own in AckAll, anybody's in AckSingle.
// ackedAt is the reader's own acknowledgement.
func (n Notification) Owes(ackedAt int64) bool {
	switch n.Ack {
	case AckSingle:
		return n.AckCompleteAt == 0
	case AckAll:
		return ackedAt == 0
	}
	return false
}

// Routes are the routes a reader must be able to open for the notification to
// mean something to them: the detail's, and the actions'.
func (n Notification) Routes() []string {
	var out []string
	if n.DetailRoute != "" {
		out = append(out, n.DetailRoute)
	}
	for _, a := range n.Actions {
		if a.Route != "" && !slices.Contains(out, a.Route) {
			out = append(out, a.Route)
		}
	}
	return out
}

// Received is a notification as one recipient holds it.
type Received struct {
	Notification
	ReadAt int64 `json:"readAt,omitempty"`
	// AckedAt is when this reader acknowledged it, 0 while they have not.
	AckedAt int64 `json:"ackedAt,omitempty"`
}

// ErrNotificationNotFound is what a lookup of an absent, expired or withdrawn
// notification answers.
var ErrNotificationNotFound = errors.New("no such notification")

// ErrNotificationOwed is what deleting one answers while the reader still
// owes the acknowledgement it asks for.
var ErrNotificationOwed = errors.New("the notification waits for an acknowledgement")

const notificationColumns = `id, tenant_id, sender, sender_via, ref, level, title, body, detail_url,
	detail_route, actions, metadata, mail, recipients, created_at, expires_at, withdrawn_at, broadcast_id,
	ack, on_ack, ack_complete_at, ack_by, ended_as, replaced_by, exercise, audience_roles, audience_users,
	audience_kept`

func scanNotification(row interface{ Scan(...any) error }, extra ...any) (Notification, error) {
	var n Notification
	var title, body, actions, meta, onAck, aRoles, aUsers string
	dest := []any{&n.ID, &n.TenantID, &n.Sender, &n.SenderVia, &n.Ref, &n.Level, &title, &body, &n.DetailURL,
		&n.DetailRoute, &actions, &meta, &n.Mail, &n.Recipients, &n.CreatedAt, &n.ExpiresAt, &n.WithdrawnAt,
		&n.BroadcastID, &n.Ack, &onAck, &n.AckCompleteAt, &n.AckBy, &n.EndedAs, &n.ReplacedBy, &n.Exercise, &aRoles, &aUsers, &n.AudienceKept}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return n, err
	}
	for _, f := range []struct {
		raw string
		out any
	}{{title, &n.Title}, {body, &n.Body}, {actions, &n.Actions}, {meta, &n.Meta}, {aRoles, &n.AudienceRoles},
		{aUsers, &n.AudienceUsers}} {
		if f.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(f.raw), f.out); err != nil {
			return n, fmt.Errorf("store: notification %q: %w", n.ID, err)
		}
	}
	// Stored before actions had a method: they were GETs.
	for i := range n.Actions {
		if n.Actions[i].Method == "" {
			n.Actions[i].Method = "GET"
		}
	}
	if onAck != "" {
		n.OnAck = &NotificationLink{}
		if err := json.Unmarshal([]byte(onAck), n.OnAck); err != nil {
			return n, fmt.Errorf("store: notification %q: %w", n.ID, err)
		}
	}
	return n, nil
}

func jsonText(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateNotification stores a notification and its recipients in one
// transaction. When the sender already used the same id (Ref) in the same
// tenant, for a notification that has not expired - live, withdrawn or
// replaced - that one is returned instead and existed is true: a sender
// retrying after a cut does not tell everybody twice, and an id it withdrew
// does not come back by being sent again. The caller looks in the other
// tenants first (SenderNotifications): one id names one notification of its
// sender, or one message sent to every tenant.
//
// An id a replacement or a withdrawal named before it arrived (RememberEnd)
// is stored already ended: the caller then tells nobody, and mails nobody.
func (s *Store) CreateNotification(ctx context.Context, n Notification, userIDs []string) (out Notification, existed bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return n, false, fmt.Errorf("store: create notification: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	if n.Ref != "" {
		row := tx.QueryRowContext(ctx, `SELECT `+notificationColumns+` FROM notifications
			WHERE sender = ? AND ref = ? AND tenant_id = ? AND expires_at > ?
			ORDER BY created_at DESC LIMIT 1`,
			n.Sender, n.Ref, n.TenantID, now)
		prev, err := scanNotification(row)
		if err == nil {
			return prev, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return n, false, fmt.Errorf("store: create notification: %w", err)
		}
		var as, by string
		err = tx.QueryRowContext(ctx, `SELECT ended_as, replaced_by FROM notification_ends
			WHERE sender = ? AND ref = ? AND expires_at > ?`, n.Sender, n.Ref, now).Scan(&as, &by)
		switch {
		case err == nil:
			n.WithdrawnAt, n.EndedAs, n.ReplacedBy = now, as, by
		case !errors.Is(err, sql.ErrNoRows):
			return n, false, fmt.Errorf("store: create notification: %w", err)
		}
	}
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
	n.Recipients = len(userIDs)
	title, err := jsonText(n.Title)
	if err != nil {
		return n, false, err
	}
	body, err := jsonText(n.Body)
	if err != nil {
		return n, false, err
	}
	actions := ""
	if len(n.Actions) > 0 {
		if actions, err = jsonText(n.Actions); err != nil {
			return n, false, err
		}
	}
	meta, err := metaJSON(n.Meta)
	if err != nil {
		return n, false, err
	}
	onAck := ""
	if n.OnAck != nil {
		if onAck, err = jsonText(n.OnAck); err != nil {
			return n, false, err
		}
	}
	aRoles, aUsers := "", ""
	if len(n.AudienceRoles) > 0 {
		if aRoles, err = jsonText(n.AudienceRoles); err != nil {
			return n, false, err
		}
	}
	if len(n.AudienceUsers) > 0 {
		if aUsers, err = jsonText(n.AudienceUsers); err != nil {
			return n, false, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notifications (`+notificationColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.TenantID, n.Sender, n.SenderVia, n.Ref, n.Level, title, body, n.DetailURL,
		n.DetailRoute, actions, meta, n.Mail, n.Recipients, n.CreatedAt, n.ExpiresAt, n.WithdrawnAt, n.BroadcastID,
		n.Ack, onAck, 0, "", n.EndedAs, n.ReplacedBy, n.Exercise, aRoles, aUsers, n.AudienceKept); err != nil {
		return n, false, fmt.Errorf("store: create notification: %w", err)
	}
	for _, u := range userIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notification_recipients (notification_id, user_id) VALUES (?, ?)`, n.ID, u); err != nil {
			return n, false, fmt.Errorf("store: create notification %q, recipient %q: %w", n.ID, u, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return n, false, fmt.Errorf("store: create notification: %w", err)
	}
	return n, false, nil
}

// GetNotification reads one, live or not.
func (s *Store) GetNotification(ctx context.Context, id string) (Notification, error) {
	n, err := scanNotification(s.db.QueryRowContext(ctx,
		`SELECT `+notificationColumns+` FROM notifications WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return n, ErrNotificationNotFound
	}
	return n, err
}

// NotificationFilter narrows a listing. Empty fields do not narrow.
type NotificationFilter struct {
	Sender   string
	TenantID string
	Meta     []MetaMatch
	// Live leaves out what expired or was withdrawn.
	Live bool
	// Limit bounds the answer, newest first; 0 is 500.
	Limit int
}

// ListNotifications lists notifications, newest first.
func (s *Store) ListNotifications(ctx context.Context, f NotificationFilter) ([]Notification, error) {
	q := `SELECT ` + notificationColumns + ` FROM notifications WHERE 1 = 1`
	var args []any
	if f.Sender != "" {
		q += ` AND sender = ?`
		args = append(args, f.Sender)
	}
	if f.TenantID != "" {
		q += ` AND tenant_id = ?`
		args = append(args, f.TenantID)
	}
	if f.Live {
		q += ` AND withdrawn_at = 0 AND expires_at > ?`
		args = append(args, time.Now().Unix())
	}
	q += ` ORDER BY created_at DESC, id`
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list notifications: %w", err)
		}
		if !matchesMeta(n.Meta, f.Meta) {
			continue
		}
		out = append(out, n)
		if len(out) == limit {
			break
		}
	}
	return out, rows.Err()
}

// WithdrawAny takes one back for an administrator, whoever sent it. One
// already ended is not found.
func (s *Store) WithdrawAny(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE notifications SET withdrawn_at = ?, ended_as = ?
		WHERE id = ? AND withdrawn_at = 0`, time.Now().Unix(), EndedWithdrawn, id)
	if err != nil {
		return fmt.Errorf("store: withdraw notification %q: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// SenderNotifications are the notifications a sender's id names, live or
// ended, not expired: one, or every copy of a message sent to every tenant.
// Another sender's are never found.
func (s *Store) SenderNotifications(ctx context.Context, sender, ref string) ([]Notification, error) {
	if ref == "" {
		return []Notification{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+notificationColumns+` FROM notifications
		WHERE sender = ? AND ref = ? AND expires_at > ? ORDER BY created_at, id`, sender, ref, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("store: notifications %q of %q: %w", ref, sender, err)
	}
	defer func() { _ = rows.Close() }()
	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("store: notifications %q of %q: %w", ref, sender, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// TenantNotificationsByRef are the notifications of one tenant its senders
// name by an id, whoever the sender: what an administrator replaces from the
// tenant's page, as they withdraw any of them from there.
func (s *Store) TenantNotificationsByRef(ctx context.Context, tenantID, ref string) ([]Notification, error) {
	if ref == "" {
		return []Notification{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+notificationColumns+` FROM notifications
		WHERE tenant_id = ? AND ref = ? AND expires_at > ? ORDER BY created_at, id`, tenantID, ref, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("store: notifications %q of tenant %q: %w", ref, tenantID, err)
	}
	defer func() { _ = rows.Close() }()
	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("store: notifications %q of tenant %q: %w", ref, tenantID, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// EndNotification ends one live notification, as withdrawn or replaced (by
// the notification named). It answers whether it was live: ending one already
// ended changes nothing, so a sender retrying is not an error.
func (s *Store) EndNotification(ctx context.Context, id, as, replacedBy string) (bool, error) {
	if as != EndedWithdrawn && as != EndedReplaced {
		return false, fmt.Errorf("store: end notification %q as %q: allowed are %s and %s", id, as,
			EndedWithdrawn, EndedReplaced)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE notifications SET withdrawn_at = ?, ended_as = ?, replaced_by = ?
		WHERE id = ? AND withdrawn_at = 0`, time.Now().Unix(), as, replacedBy, id)
	if err != nil {
		return false, fmt.Errorf("store: end notification %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RememberEnd keeps a replacement or a withdrawal naming an id its sender has
// not published yet, until expiresAt: published then, the notification is
// stored already ended. Named again, the first decision stands.
func (s *Store) RememberEnd(ctx context.Context, sender, ref, as, replacedBy string, expiresAt int64) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO notification_ends
		(sender, ref, ended_as, replaced_by, at, expires_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (sender, ref) DO NOTHING`, sender, ref, as, replacedBy, time.Now().Unix(),
		expiresAt); err != nil {
		return fmt.Errorf("store: remember the end of %q of %q: %w", ref, sender, err)
	}
	return nil
}

// NotificationRecipients are the accounts a notification was given to.
func (s *Store) NotificationRecipients(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM notification_recipients WHERE notification_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("store: recipients of %q: %w", id, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// NotificationReader is one recipient's state, for the console.
type NotificationReader struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Fullname string `json:"fullname,omitempty"`
	ReadAt   int64  `json:"readAt,omitempty"`
	HiddenAt int64  `json:"hiddenAt,omitempty"`
	// AckedAt is when they acknowledged it, for one that asks.
	AckedAt int64 `json:"ackedAt,omitempty"`
	// Mail is where their mail stands (MailStates), "" when none was asked;
	// MailCode is the relay's reply code of a failed one.
	Mail     string `json:"mail,omitempty"`
	MailCode int    `json:"mailCode,omitempty"`
}

// NotificationReaders says who was told and who read, readers first.
func (s *Store) NotificationReaders(ctx context.Context, id string) ([]NotificationReader, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.user_id, COALESCE(u.username, ''), COALESCE(u.fullname, ''),
		r.read_at, r.hidden_at, r.acked_at, r.mail_state, r.mail_code FROM notification_recipients r
		LEFT JOIN users u ON u.id = r.user_id
		WHERE r.notification_id = ? ORDER BY r.read_at DESC, u.username`, id)
	if err != nil {
		return nil, fmt.Errorf("store: readers of %q: %w", id, err)
	}
	defer func() { _ = rows.Close() }()
	out := []NotificationReader{}
	for rows.Next() {
		var r NotificationReader
		if err := rows.Scan(&r.UserID, &r.Username, &r.Fullname, &r.ReadAt, &r.HiddenAt, &r.AckedAt, &r.Mail,
			&r.MailCode); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NotificationReads counts the readers of each of ids.
func (s *Store) NotificationReads(ctx context.Context, ids []string) (map[string]int, error) {
	out := map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT notification_id, COUNT(*) FROM notification_recipients
		WHERE read_at > 0 AND notification_id IN (?` + strings.Repeat(", ?", len(ids)-1) + `) GROUP BY notification_id`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: count the readers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// NotificationPerson is somebody a notification names - who acknowledged
// it - as a person reading about it recognises them.
type NotificationPerson struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name,omitempty"`
}

// NotificationPersonOf is the account id as a NotificationPerson; an account
// gone since is its id alone.
func (s *Store) NotificationPersonOf(ctx context.Context, id string) NotificationPerson {
	p := NotificationPerson{ID: id}
	if u, err := s.GetUserByID(ctx, id); err == nil {
		p.Username, p.Name = u.Username, u.Fullname
	}
	return p
}

// NotificationAcks counts the readers who acknowledged each of ids.
func (s *Store) NotificationAcks(ctx context.Context, ids []string) (map[string]int, error) {
	out := map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT notification_id, COUNT(*) FROM notification_recipients
		WHERE acked_at > 0 AND notification_id IN (?` + strings.Repeat(", ?", len(ids)-1) + `) GROUP BY notification_id`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: count the acknowledgements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// ReceivedFilter narrows what one recipient reads.
type ReceivedFilter struct {
	Unread bool
	Level  string
	// TenantID keeps only what was sent in one tenant; "" is every tenant.
	TenantID string
	// Before pages: only what was created before this instant.
	Before int64
	Limit  int
}

// ReceivedNotifications is what a user was given and still holds: not
// expired, not hidden, newest first - ended ones included, which the reader's
// page shows as such. Whether they may still open what it leads to is the
// caller's question, asked with their session.
func (s *Store) ReceivedNotifications(ctx context.Context, userID string, f ReceivedFilter) ([]Received, error) {
	cols := strings.ReplaceAll(notificationColumns, "\n\t", " ")
	var prefixed []string
	for _, c := range strings.Split(cols, ",") {
		prefixed = append(prefixed, "n."+strings.TrimSpace(c))
	}
	q := `SELECT ` + strings.Join(prefixed, ", ") + `, r.read_at, r.acked_at FROM notification_recipients r
		JOIN notifications n ON n.id = r.notification_id
		WHERE r.user_id = ? AND r.hidden_at = 0 AND n.expires_at > ?`
	args := []any{userID, time.Now().Unix()}
	// An ended one stays listed, marked, and is never unread: nothing waits
	// for its reader any more.
	if f.Unread {
		q += ` AND r.read_at = 0 AND n.withdrawn_at = 0`
	}
	if f.TenantID != "" {
		q += ` AND n.tenant_id = ?`
		args = append(args, f.TenantID)
	}
	if f.Level != "" {
		q += ` AND n.level = ?`
		args = append(args, f.Level)
	}
	if f.Before > 0 {
		q += ` AND n.created_at < ?`
		args = append(args, f.Before)
	}
	q += ` ORDER BY n.created_at DESC, n.id`
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	q += fmt.Sprintf(` LIMIT %d`, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: notifications of %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	out := []Received{}
	for rows.Next() {
		var r Received
		n, err := scanNotification(rows, &r.ReadAt, &r.AckedAt)
		if err != nil {
			return nil, fmt.Errorf("store: notifications of %q: %w", userID, err)
		}
		r.Notification = n
		out = append(out, r)
	}
	return out, rows.Err()
}

// copiesHeld is the subquery naming a notification a user holds and, when it
// is a copy of a message sent to every tenant, every other copy of it: one
// item for its reader, read and deleted at once. Its arguments are the id,
// the id again and the user. A copy the user does not hold names nothing more
// than itself.
const copiesHeld = `SELECT c.id FROM notifications c WHERE c.id = ? OR (c.broadcast_id <> '' AND c.broadcast_id =
	(SELECT h.broadcast_id FROM notifications h JOIN notification_recipients hr ON hr.notification_id = h.id
	WHERE h.id = ? AND hr.user_id = ?))`

// ReceivedOne is one notification a user still holds: live, not hidden.
func (s *Store) ReceivedOne(ctx context.Context, userID, id string) (Received, error) {
	cols := strings.ReplaceAll(notificationColumns, "\n\t", " ")
	var prefixed []string
	for _, c := range strings.Split(cols, ",") {
		prefixed = append(prefixed, "n."+strings.TrimSpace(c))
	}
	var r Received
	n, err := scanNotification(s.db.QueryRowContext(ctx, `SELECT `+strings.Join(prefixed, ", ")+`, r.read_at,
		r.acked_at FROM notification_recipients r JOIN notifications n ON n.id = r.notification_id
		WHERE r.user_id = ? AND r.notification_id = ? AND r.hidden_at = 0 AND n.withdrawn_at = 0
		AND n.expires_at > ?`, userID, id, time.Now().Unix()), &r.ReadAt, &r.AckedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotificationNotFound
	}
	if err != nil {
		return r, fmt.Errorf("store: notification %q of %q: %w", id, userID, err)
	}
	r.Notification = n
	return r, nil
}

// ReceivedCopies is every copy a user still holds of the given messages sent
// to every tenant, whatever the tenant: what a listing narrowed to one tenant,
// or cut by its limit, needs to show each message as the one item it is.
func (s *Store) ReceivedCopies(ctx context.Context, userID string, broadcastIDs []string) ([]Received, error) {
	out := []Received{}
	if len(broadcastIDs) == 0 {
		return out, nil
	}
	cols := strings.ReplaceAll(notificationColumns, "\n\t", " ")
	var prefixed []string
	for _, c := range strings.Split(cols, ",") {
		prefixed = append(prefixed, "n."+strings.TrimSpace(c))
	}
	q := `SELECT ` + strings.Join(prefixed, ", ") + `, r.read_at, r.acked_at FROM notification_recipients r
		JOIN notifications n ON n.id = r.notification_id
		WHERE r.user_id = ? AND r.hidden_at = 0 AND n.expires_at > ?
		AND n.broadcast_id IN (?` + strings.Repeat(", ?", len(broadcastIDs)-1) + `)
		ORDER BY n.created_at DESC, n.id`
	args := []any{userID, time.Now().Unix()}
	for _, b := range broadcastIDs {
		args = append(args, b)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: copies held by %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var r Received
		n, err := scanNotification(rows, &r.ReadAt, &r.AckedAt)
		if err != nil {
			return nil, fmt.Errorf("store: copies held by %q: %w", userID, err)
		}
		r.Notification = n
		out = append(out, r)
	}
	return out, rows.Err()
}

// RefBroadcast is the message sent to every tenant a sender's id already
// names while one of its copies has not expired, live or ended, "" when none:
// sent again with the same id, the copies it adds join the ones already
// there.
func (s *Store) RefBroadcast(ctx context.Context, sender, key string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT broadcast_id FROM notifications
		WHERE sender = ? AND ref = ? AND broadcast_id <> '' AND expires_at > ?
		ORDER BY created_at DESC LIMIT 1`, sender, key, time.Now().Unix()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: the message %q of %q: %w", key, sender, err)
	}
	return id, nil
}

// BroadcastRecipients are the accounts holding a live copy of a message sent
// to every tenant, each with the first of its copies they hold: who to tell
// once that it arrived.
func (s *Store) BroadcastRecipients(ctx context.Context, broadcastID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.user_id, n.id FROM notification_recipients r
		JOIN notifications n ON n.id = r.notification_id
		WHERE n.broadcast_id = ? AND n.withdrawn_at = 0 ORDER BY n.id`, broadcastID)
	if err != nil {
		return nil, fmt.Errorf("store: recipients of the message %q: %w", broadcastID, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var u, id string
		if err := rows.Scan(&u, &id); err != nil {
			return nil, err
		}
		if _, ok := out[u]; !ok {
			out[u] = id
		}
	}
	return out, rows.Err()
}

// MarkNotificationRead records that a user read one, and every other copy
// they hold of the same message sent to every tenant. Reading twice keeps the
// first time.
func (s *Store) MarkNotificationRead(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET read_at = ?
		WHERE notification_id IN (`+copiesHeld+`) AND user_id = ? AND read_at = 0`,
		time.Now().Unix(), id, id, userID, userID)
	if err != nil {
		return fmt.Errorf("store: read notification %q: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		var one int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM notification_recipients
			WHERE notification_id = ? AND user_id = ?`, id, userID).Scan(&one); err != nil {
			return ErrNotificationNotFound
		}
	}
	return nil
}

// sentIn is the subquery naming what a user holds from one tenant: what was
// sent there and, for a message sent to every tenant whose copy there they
// still hold, every other copy of it - one item for its reader, read and
// deleted whole whichever tenant asks. Its arguments are the tenant, the
// tenant again and the user.
const sentIn = `SELECT c.id FROM notifications c WHERE c.tenant_id = ? OR (c.broadcast_id <> '' AND c.broadcast_id IN
	(SELECT b.broadcast_id FROM notifications b JOIN notification_recipients br ON br.notification_id = b.id
	WHERE b.tenant_id = ? AND b.broadcast_id <> '' AND br.user_id = ? AND br.hidden_at = 0))`

// MarkAllNotificationsRead reads everything a user holds from one tenant,
// a message sent to every tenant with all its copies (sentIn), or from every
// tenant when tenantID is "".
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID, tenantID string) error {
	q := `UPDATE notification_recipients SET read_at = ? WHERE user_id = ? AND read_at = 0`
	args := []any{time.Now().Unix(), userID}
	if tenantID != "" {
		q += ` AND notification_id IN (` + sentIn + `)`
		args = append(args, tenantID, tenantID, userID)
	}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("store: read every notification of %q (tenant %q): %w", userID, tenantID, err)
	}
	return nil
}

// owed is the condition on a recipient row r of a notification n under which
// its reader still owes the acknowledgement it asks for: their own in
// AckAll, anybody's in AckSingle. A withdrawn or expired one no longer asks.
// Its one argument is now.
const owed = `n.ack <> '' AND n.withdrawn_at = 0 AND n.expires_at > ?
	AND ((n.ack = 'single' AND n.ack_complete_at = 0) OR (n.ack <> 'single' AND r.acked_at = 0))`

// DeleteReceived deletes one notification for one reader, and only for
// them: the sender's message and every other reader's copy are untouched.
// A copy of a message sent to every tenant goes with every other copy the
// reader holds of it: one item for them, one Delete.
// It is for good. The row stays, flagged (hidden_at), so what the sender is
// told - who was told, who read - stays true; but nothing reads a flagged row
// for its reader again: ReceivedNotifications leaves it out, and no write
// clears the flag. Deleting reads it too: what is gone is not waiting to be
// read. Deleting one already deleted is not found, as for anybody else.
// One that waits for an acknowledgement the reader owes is refused
// (ErrNotificationOwed): what asks for proof is not thrown away before it.
func (s *Store) DeleteReceived(ctx context.Context, userID, id string) error {
	now := time.Now().Unix()
	var waiting int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_recipients r
		JOIN notifications n ON n.id = r.notification_id
		WHERE r.notification_id IN (`+copiesHeld+`) AND r.user_id = ? AND r.hidden_at = 0 AND `+owed,
		id, id, userID, userID, now).Scan(&waiting); err != nil {
		return fmt.Errorf("store: delete notification %q of %q: %w", id, userID, err)
	}
	if waiting > 0 {
		return ErrNotificationOwed
	}
	res, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET hidden_at = ?,
		read_at = CASE WHEN read_at = 0 THEN ? ELSE read_at END
		WHERE notification_id IN (`+copiesHeld+`) AND user_id = ? AND hidden_at = 0`, now, now, id, id, userID, userID)
	if err != nil {
		return fmt.Errorf("store: delete notification %q of %q: %w", id, userID, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

// DeleteAllReceived is DeleteReceived for everything a reader still holds
// from one tenant, a message sent to every tenant with all its copies
// (sentIn), or from every tenant when tenantID is "": the list they read, not
// only the part of it a page loaded. What waits for an acknowledgement the
// reader owes is kept. It answers how many rows it deleted, and how many
// items it kept - a message sent to every tenant counted once.
func (s *Store) DeleteAllReceived(ctx context.Context, userID, tenantID string) (deleted, kept int64, err error) {
	now := time.Now().Unix()
	scope, scopeArgs := "", []any{}
	if tenantID != "" {
		scope = ` AND r.notification_id IN (` + sentIn + `)`
		scopeArgs = []any{tenantID, tenantID, userID}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT CASE WHEN n.broadcast_id <> '' THEN n.broadcast_id
		ELSE n.id END) FROM notification_recipients r JOIN notifications n ON n.id = r.notification_id
		WHERE r.user_id = ? AND r.hidden_at = 0 AND `+owed+scope,
		append([]any{userID, now}, scopeArgs...)...).Scan(&kept); err != nil {
		return 0, 0, fmt.Errorf("store: delete every notification of %q (tenant %q): %w", userID, tenantID, err)
	}
	q := `UPDATE notification_recipients SET hidden_at = ?,
		read_at = CASE WHEN read_at = 0 THEN ? ELSE read_at END
		WHERE user_id = ? AND hidden_at = 0 AND notification_id NOT IN (SELECT r.notification_id
		FROM notification_recipients r JOIN notifications n ON n.id = r.notification_id
		WHERE r.user_id = ? AND ` + owed + `)`
	args := []any{now, now, userID, userID, now}
	if tenantID != "" {
		q += ` AND notification_id IN (` + sentIn + `)`
		args = append(args, tenantID, tenantID, userID)
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("store: delete every notification of %q (tenant %q): %w", userID, tenantID, err)
	}
	deleted, _ = res.RowsAffected()
	return deleted, kept, nil
}

// AckChange is one notification an acknowledgement moved, as it stands
// after it.
type AckChange struct {
	Notification Notification
	// Own says the reader's own acknowledgement was recorded on it. A copy of
	// a single message sent to every tenant that they do not hold is closed
	// by them all the same, and Own is false.
	Own bool
	// Acknowledged is how many of its readers acknowledged it.
	Acknowledged int
	// Complete says this acknowledgement closed it: the first of a single
	// one, the last reader's of an all one.
	Complete bool
}

// Acknowledge records a reader's acknowledgement of the copies ids, which
// they hold and may read (the caller checked), and answers what moved. In
// AckAll it is their own, and the notification is complete once every
// reader gave theirs. In AckSingle the first closes it for everybody - a
// message sent to every tenant with every one of its copies - and once
// closed, a later one records nothing: the request is done. Acknowledging
// reads it too. Acknowledging twice changes nothing and is not an error.
func (s *Store) Acknowledge(ctx context.Context, userID string, ids []string) ([]AckChange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: acknowledge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	moved := map[string]*AckChange{}
	var order []string
	note := func(id string) *AckChange {
		c := moved[id]
		if c == nil {
			c = &AckChange{}
			moved[id] = c
			order = append(order, id)
		}
		return c
	}
	// own records the reader's acknowledgement on their row of id, when
	// they hold one and had not given it.
	own := func(id string) (bool, error) {
		res, err := tx.ExecContext(ctx, `UPDATE notification_recipients SET acked_at = ?,
			read_at = CASE WHEN read_at = 0 THEN ? ELSE read_at END
			WHERE notification_id = ? AND user_id = ? AND acked_at = 0 AND hidden_at = 0`, now, now, id, userID)
		if err != nil {
			return false, err
		}
		n, _ := res.RowsAffected()
		return n > 0, nil
	}
	for _, id := range ids {
		nt, err := scanNotification(tx.QueryRowContext(ctx,
			`SELECT `+notificationColumns+` FROM notifications WHERE id = ?`, id))
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
		}
		if nt.WithdrawnAt != 0 || nt.ExpiresAt <= now {
			return nil, ErrNotificationNotFound
		}
		switch nt.Ack {
		case AckAll:
			did, err := own(id)
			if err != nil {
				return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
			}
			if !did {
				continue
			}
			c := note(id)
			c.Own = true
			var given int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_recipients
				WHERE notification_id = ? AND acked_at > 0`, id).Scan(&given); err != nil {
				return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
			}
			if given >= nt.Recipients {
				res, err := tx.ExecContext(ctx, `UPDATE notifications SET ack_complete_at = ?
					WHERE id = ? AND ack_complete_at = 0`, now, id)
				if err != nil {
					return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
				}
				if n, _ := res.RowsAffected(); n > 0 {
					c.Complete = true
				}
			}
		case AckSingle:
			// The copy named, and every other copy of the same message sent to
			// every tenant still open: one acknowledgement closes them all.
			targets := []string{id}
			if nt.BroadcastID != "" {
				rows, err := tx.QueryContext(ctx, `SELECT id FROM notifications WHERE broadcast_id = ?
					AND id <> ? AND ack = ? AND ack_complete_at = 0 AND withdrawn_at = 0 ORDER BY id`,
					nt.BroadcastID, id, AckSingle)
				if err != nil {
					return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
				}
				for rows.Next() {
					var other string
					if err := rows.Scan(&other); err != nil {
						_ = rows.Close()
						return nil, err
					}
					targets = append(targets, other)
				}
				_ = rows.Close()
			}
			for _, t := range targets {
				res, err := tx.ExecContext(ctx, `UPDATE notifications SET ack_complete_at = ?, ack_by = ?
					WHERE id = ? AND ack_complete_at = 0`, now, userID, t)
				if err != nil {
					return nil, fmt.Errorf("store: acknowledge %q: %w", t, err)
				}
				if n, _ := res.RowsAffected(); n == 0 {
					continue
				}
				c := note(t)
				c.Complete = true
				if c.Own, err = own(t); err != nil {
					return nil, fmt.Errorf("store: acknowledge %q: %w", t, err)
				}
			}
		default:
			return nil, fmt.Errorf("store: notification %q does not ask for an acknowledgement", id)
		}
	}
	out := make([]AckChange, 0, len(order))
	for _, id := range order {
		c := moved[id]
		nt, err := scanNotification(tx.QueryRowContext(ctx,
			`SELECT `+notificationColumns+` FROM notifications WHERE id = ?`, id))
		if err != nil {
			return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
		}
		c.Notification = nt
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_recipients
			WHERE notification_id = ? AND acked_at > 0`, id).Scan(&c.Acknowledged); err != nil {
			return nil, fmt.Errorf("store: acknowledge %q: %w", id, err)
		}
		out = append(out, *c)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: acknowledge: %w", err)
	}
	return out, nil
}

// PurgeNotifications deletes what expired, with each recipient's state:
// personal data is not kept past the life its sender gave it. Withdrawn ones
// go with the rest when their time comes.
func (s *Store) PurgeNotifications(ctx context.Context, now int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: purge notifications: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_recipients WHERE notification_id IN
		(SELECT id FROM notifications WHERE expires_at <= ?)`, now); err != nil {
		return 0, fmt.Errorf("store: purge notifications: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM notifications WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("store: purge notifications: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_ends WHERE expires_at <= ?`, now); err != nil {
		return 0, fmt.Errorf("store: purge notifications: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: purge notifications: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GivenLater are the live notifications of a tenant an account holds no line
// of, among those whose audience was kept: what an account that joined the
// tenant, or was granted a role there, may be given now. Whether they target it
// is the caller's question.
func (s *Store) GivenLater(ctx context.Context, tenantID, userID string) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+notificationColumns+` FROM notifications n
		WHERE n.tenant_id = ? AND n.audience_kept = ? AND n.withdrawn_at = 0 AND n.expires_at > ?
		AND NOT EXISTS (SELECT 1 FROM notification_recipients r WHERE r.notification_id = n.id AND r.user_id = ?)
		ORDER BY n.created_at, n.id`, tenantID, true, time.Now().Unix(), userID)
	if err != nil {
		return nil, fmt.Errorf("store: notifications %q may be given in %q: %w", userID, tenantID, err)
	}
	defer func() { _ = rows.Close() }()
	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("store: notifications %q may be given in %q: %w", userID, tenantID, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GiveNotification gives a notification to one more account, after its
// publication, and answers whether this call did: two nodes giving it at
// once give it once. Its count of recipients follows.
func (s *Store) GiveNotification(ctx context.Context, id, userID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: give notification %q to %q: %w", id, userID, err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO notification_recipients (notification_id, user_id) VALUES (?, ?)
		ON CONFLICT (notification_id, user_id) DO NOTHING`, id, userID)
	if err != nil {
		return false, fmt.Errorf("store: give notification %q to %q: %w", id, userID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE notifications SET recipients = recipients + 1 WHERE id = ?`, id); err != nil {
		return false, fmt.Errorf("store: give notification %q to %q: %w", id, userID, err)
	}
	return true, tx.Commit()
}

// HoldsBroadcast says an account holds a copy, other than except, of a
// message sent to every tenant.
func (s *Store) HoldsBroadcast(ctx context.Context, userID, broadcastID, except string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM notification_recipients r JOIN notifications n
		ON n.id = r.notification_id WHERE r.user_id = ? AND n.broadcast_id = ? AND n.id <> ? LIMIT 1`,
		userID, broadcastID, except).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
