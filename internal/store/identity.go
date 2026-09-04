package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
)

// DayRange is one allowed window on one weekday. Hours are WALL-CLOCK in the
// window's Timezone; the server brings now (a UTC instant) into that zone at
// evaluation time, so DST never shifts the human's window. A day may appear
// several times (split days, e.g. around lunch); an absent day is closed.
type DayRange struct {
	Day  int    `json:"day"`  // 1=Monday ... 7=Sunday
	From string `json:"from"` // "09:00"
	To   string `json:"to"`   // "18:00"
}

// BusinessAccess is the working-hours window (TENANT-04). One shape at every
// level: the global setting fills every field; a tenant override sets
// Inherited=false and its own values; a membership override may also bound by
// dates. Inherited=true means "use the level above" and the other fields are
// ignored.
type BusinessAccess struct {
	Inherited bool       `json:"inherited"`
	Timezone  string     `json:"timezone,omitempty"`
	Days      []DayRange `json:"days,omitempty"`     // empty = no weekday/hour restriction
	DateFrom  string     `json:"dateFrom,omitempty"` // membership level only
	DateTo    string     `json:"dateTo,omitempty"`
}

// Restricts reports whether this window actually restricts anything: a
// timezone alone does not, and neither does an empty schedule.
func (b BusinessAccess) Restricts() bool {
	return len(b.Days) > 0 || b.DateFrom != "" || b.DateTo != ""
}

// SanitizeBusinessAccess turns a window that says NOTHING into an inherited
// one - which is what a caller who never mentioned opening hours meant.
//
// The trap it closes: Go decodes a missing "businessAccess" as the zero value,
// whose Inherited is FALSE. The resolution then reads "this level overrides,
// with no restriction" and stops there, so a membership created through the
// API, an import or a group rule silently escaped the hours set on its
// organisation - and the organisation itself escaped the global ones. Nothing
// on screen said so: the console sends inherited:true and was never the
// culprit, which is why the hours looked like they worked.
//
// An EXPLICIT "no hours for this person" is still expressible: it is what a
// window with a timezone and no days means, and it keeps Inherited false.
func SanitizeBusinessAccess(b *BusinessAccess) {
	if !b.Inherited && !b.Restricts() && b.Timezone == "" {
		b.Inherited = true
	}
	if b.Inherited {
		// An inherited window carries nothing of its own: leftovers here would
		// come back to life the day somebody unticks the box.
		b.Timezone, b.Days, b.DateFrom, b.DateTo = "", nil, "", ""
	}
}

// Tenant is an isolation space (an organization - TENANT-01). Its settings
// inherit from the global ones unless overridden; SessionTTL "" means
// inherited (TENANT-05).
type Tenant struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	BusinessAccess BusinessAccess `json:"businessAccess"`
	SessionTTL     string         `json:"sessionTTL"`
	// GroupMode (RBAC-03), a per-tenant call: "" defaults to cumulative,
	// MULTIPLE cumulates every group's roles, SINGLE makes the member pick one.
	GroupMode string `json:"groupMode"`
	// CreatedBy is the id of the user who created the tenant (audit), set once
	// and never changed. OwnerID is the current owner (TENANT-02): always set,
	// the creator by default, transferable in the Danger zone, and INDEPENDENT
	// of membership - an owner need not be a member. CreatedByName/OwnerName are
	// DISPLAY-only, computed by the admin GET (never persisted, harmless on a
	// round-trip).
	CreatedBy     string `json:"createdBy"`
	OwnerID       string `json:"ownerId"`
	CreatedByName string `json:"createdByName,omitempty"`
	OwnerName     string `json:"ownerName,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// The tenancy modes. Single is the default and the only one a community
// build offers: ONE implicit organisation, created at first boot, which the
// console never names - someone protecting three internal applications has no
// reason to learn the word. Multi is the Enterprise mode and is chosen at
// startup, once: the two shapes cannot be swapped under a live database.
const (
	TenancySingle = "single"
	TenancyMulti  = "multi"

	// DefaultTenantID is the organisation every single-tenant installation
	// hangs off. It exists in multi mode too - it is simply the first one -
	// so nothing in the RBAC model needs a special case for "no organisation".
	DefaultTenantID   = "default"
	defaultTenantName = "Default"
)

// Membership types (TENANT-02 revised): ADMINs administer the tenant, USERs
// belong to it. Ownership is NOT a membership type - it lives on the tenant
// (Tenant.OwnerID), decoupled so an owner need not be a member. Tenant
// administration is either this relation (ADMIN) or ownership.
const (
	MemberAdmin = "ADMIN"
	MemberUser  = "USER"
)

func validMemberType(t string) error {
	switch t {
	case MemberAdmin, MemberUser:
		return nil
	}
	return fmt.Errorf("membership type %q is not allowed: allowed types are ADMIN, USER (ownership lives on the tenant, not the membership)", t)
}

// validTristate guards the inherit/override columns (mfa_required): "" defers to
// the level above, "true"/"false" pin the value at this level.
func validTristate(v string) error {
	switch v {
	case "", "true", "false":
		return nil
	}
	return fmt.Errorf("value %q is not allowed: use \"\" (inherit), \"true\" or \"false\"", v)
}

// Membership ties a user to a tenant with a type and per-member overrides
// (TENANT-04/05: business access and session TTL inherit from the tenant
// unless overridden here).
type Membership struct {
	UserID         string         `json:"userId"`
	TenantID       string         `json:"tenantId"`
	Type           string         `json:"type"`
	Enabled        bool           `json:"enabled"`
	BusinessAccess BusinessAccess `json:"businessAccess"`
	SessionTTL     string         `json:"sessionTTL"`
	CreatedAt      int64          `json:"createdAt"`
	UpdatedAt      int64          `json:"updatedAt"`
}

// Member is a membership joined with the user's identity - what the tenant
// administration screen lists.
type Member struct {
	Membership
	Username         string `json:"username"`
	Fullname         string `json:"fullname"`
	Email            string `json:"email"`
	LastConnectionAt int64  `json:"lastConnectionAt"`
}

// UserTenant is a membership joined with the tenant - what /api/me reports
// (drives the console's role classes and the tenant switcher).
type UserTenant struct {
	TenantID   string `json:"tenantId"`
	TenantName string `json:"tenantName"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
}

// SaveTenant inserts or replaces a tenant by ID.
func (s *Store) SaveTenant(ctx context.Context, t Tenant) error {
	// Same trap one level up: an organisation created without opening hours
	// used to OVERRIDE the global ones with nothing, so the installation's
	// window stopped applying to it.
	SanitizeBusinessAccess(&t.BusinessAccess)
	ba, err := json.Marshal(t.BusinessAccess)
	if err != nil {
		return fmt.Errorf("store: tenant %q: encode business access: %w", t.Name, err)
	}
	now := time.Now().Unix()
	// created_by is set on INSERT only - an update NEVER rewrites who created
	// the tenant (it is stamped once at creation). owner_id IS updatable (the
	// caller carries it forward on a plain update, or changes it on transfer).
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tenants (id, name, description, enabled, business_access, session_ttl, group_mode, created_by, owner_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, description = excluded.description, enabled = excluded.enabled,
		   business_access = excluded.business_access, session_ttl = excluded.session_ttl,
		   group_mode = excluded.group_mode, owner_id = excluded.owner_id,
		   updated_at = excluded.updated_at`,
		t.ID, t.Name, t.Description, t.Enabled, string(ba), t.SessionTTL, t.GroupMode, t.CreatedBy, t.OwnerID, now, now)
	if err != nil {
		return fmt.Errorf("store: save tenant %q: %w", t.Name, err)
	}
	return nil
}

// GetTenant returns one tenant by ID, or an error wrapping sql.ErrNoRows.
func (s *Store) GetTenant(ctx context.Context, id string) (Tenant, error) {
	var t Tenant
	var ba string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, enabled, business_access, session_ttl, group_mode, created_by, owner_id, created_at, updated_at
		 FROM tenants WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Description, &t.Enabled, &ba, &t.SessionTTL, &t.GroupMode, &t.CreatedBy, &t.OwnerID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Tenant{}, fmt.Errorf("store: get tenant %q: %w", id, err)
	}
	if err := json.Unmarshal([]byte(ba), &t.BusinessAccess); err != nil {
		return Tenant{}, fmt.Errorf("store: tenant %q: bad business access: %w", id, err)
	}
	return t, nil
}

// ListTenants returns every tenant ordered by name.
func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	return s.listTenants(ctx,
		`SELECT id, name, description, enabled, business_access, session_ttl, group_mode, created_by, owner_id, created_at, updated_at
		 FROM tenants ORDER BY name ASC`)
}

// ListTenantsAdministeredBy returns the tenants the user administers, ordered
// by name - those they OWN (owner_id) or ADMIN (membership). A tenant admin
// sees exactly these; root uses ListTenants instead.
func (s *Store) ListTenantsAdministeredBy(ctx context.Context, userID string) ([]Tenant, error) {
	return s.listTenants(ctx,
		`SELECT t.id, t.name, t.description, t.enabled, t.business_access, t.session_ttl, t.group_mode, t.created_by, t.owner_id, t.created_at, t.updated_at
		 FROM tenants t
		 LEFT JOIN memberships m ON m.tenant_id = t.id AND m.user_id = ?
		 WHERE t.owner_id = ? OR (m.type = 'ADMIN' AND m.enabled = ?)
		 ORDER BY t.name ASC`, userID, userID, true)
}

func (s *Store) listTenants(ctx context.Context, query string, args ...any) ([]Tenant, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		var ba string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Enabled, &ba, &t.SessionTTL, &t.GroupMode, &t.CreatedBy, &t.OwnerID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan tenant: %w", err)
		}
		if err := json.Unmarshal([]byte(ba), &t.BusinessAccess); err != nil {
			return nil, fmt.Errorf("store: tenant %q: bad business access: %w", t.ID, err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list tenants: %w", err)
	}
	return tenants, nil
}

// DeleteTenant removes a tenant (memberships cascade) and reports whether it
// existed.
func (s *Store) DeleteTenant(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: delete tenant %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetTenantOwner reassigns a tenant's owner (TENANT-02 transfer). The new owner
// need not be a member; the previous owner keeps whatever membership they had.
// Reports whether the tenant existed.
func (s *Store) SetTenantOwner(ctx context.Context, tenantID, ownerID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET owner_id = ?, updated_at = ? WHERE id = ?`,
		ownerID, time.Now().Unix(), tenantID)
	if err != nil {
		return false, fmt.Errorf("store: set owner of tenant %q: %w", tenantID, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SaveMembership inserts or replaces the relation between a user and a tenant.
func (s *Store) SaveMembership(ctx context.Context, m Membership) error {
	if err := validMemberType(m.Type); err != nil {
		return fmt.Errorf("store: membership %s@%s: %w", m.UserID, m.TenantID, err)
	}
	// Here rather than in the handler: a membership is also written by an
	// import and by the group rules, and a window nobody stated must mean
	// "inherit" wherever it comes from.
	SanitizeBusinessAccess(&m.BusinessAccess)
	ba, err := json.Marshal(m.BusinessAccess)
	if err != nil {
		return fmt.Errorf("store: membership %s@%s: encode business access: %w", m.UserID, m.TenantID, err)
	}
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO memberships (user_id, tenant_id, type, enabled, business_access, session_ttl, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, tenant_id) DO UPDATE SET
		   type = excluded.type, enabled = excluded.enabled,
		   business_access = excluded.business_access, session_ttl = excluded.session_ttl,
		   updated_at = excluded.updated_at`,
		m.UserID, m.TenantID, m.Type, m.Enabled, string(ba), m.SessionTTL, now, now)
	if err != nil {
		return fmt.Errorf("store: save membership %s@%s: %w", m.UserID, m.TenantID, err)
	}
	return nil
}

// DeleteMembership removes a user from a tenant and reports whether the
// relation existed.
func (s *Store) DeleteMembership(ctx context.Context, userID, tenantID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM memberships WHERE user_id = ? AND tenant_id = ?`, userID, tenantID)
	if err != nil {
		return false, fmt.Errorf("store: delete membership %s@%s: %w", userID, tenantID, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetMembership returns the relation or an error wrapping sql.ErrNoRows.
func (s *Store) GetMembership(ctx context.Context, userID, tenantID string) (Membership, error) {
	var m Membership
	var ba string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, tenant_id, type, enabled, business_access, session_ttl, created_at, updated_at
		 FROM memberships WHERE user_id = ? AND tenant_id = ?`, userID, tenantID).
		Scan(&m.UserID, &m.TenantID, &m.Type, &m.Enabled, &ba, &m.SessionTTL, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return Membership{}, fmt.Errorf("store: get membership %s@%s: %w", userID, tenantID, err)
	}
	if err := json.Unmarshal([]byte(ba), &m.BusinessAccess); err != nil {
		return Membership{}, fmt.Errorf("store: membership %s@%s: bad business access: %w", userID, tenantID, err)
	}
	return m, nil
}

// ListMembers returns the members of a tenant (membership + identity), admins
// first, then users, alphabetically inside each type. Ownership is not a
// membership type (Tenant.OwnerID) - an owner appears here only if also a member.
func (s *Store) ListMembers(ctx context.Context, tenantID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.user_id, m.tenant_id, m.type, m.enabled, m.business_access, m.session_ttl,
		        m.created_at, m.updated_at, u.username, u.fullname, u.email, u.last_connection_at
		 FROM memberships m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.tenant_id = ?
		 ORDER BY CASE m.type WHEN 'ADMIN' THEN 0 ELSE 1 END, u.username ASC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list members of %q: %w", tenantID, err)
	}
	defer func() { _ = rows.Close() }()
	var members []Member
	for rows.Next() {
		var m Member
		var ba string
		if err := rows.Scan(&m.UserID, &m.TenantID, &m.Type, &m.Enabled, &ba, &m.SessionTTL,
			&m.CreatedAt, &m.UpdatedAt, &m.Username, &m.Fullname, &m.Email, &m.LastConnectionAt); err != nil {
			return nil, fmt.Errorf("store: scan member: %w", err)
		}
		if err := json.Unmarshal([]byte(ba), &m.BusinessAccess); err != nil {
			return nil, fmt.Errorf("store: member %s@%s: bad business access: %w", m.UserID, tenantID, err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list members of %q: %w", tenantID, err)
	}
	return members, nil
}

// ListUserTenants returns the tenants a user belongs to, with the membership
// type - the /api/me shape.
func (s *Store) ListUserTenants(ctx context.Context, userID string) ([]UserTenant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, m.type, m.enabled AND t.enabled
		 FROM memberships m
		 JOIN tenants t ON t.id = m.tenant_id
		 WHERE m.user_id = ?
		 ORDER BY t.name ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list tenants of %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []UserTenant
	for rows.Next() {
		var ut UserTenant
		if err := rows.Scan(&ut.TenantID, &ut.TenantName, &ut.Type, &ut.Enabled); err != nil {
			return nil, fmt.Errorf("store: scan user tenant: %w", err)
		}
		out = append(out, ut)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list tenants of %q: %w", userID, err)
	}
	return out, nil
}

// Global setting keys. Every level below (tenant, membership) inherits from
// these unless it overrides (TENANT-04/05). The data plane's "/" trap is NOT a
// setting - it is an ordinary catch-all route ordered last (ROUTE-10).
const (
	SettingBusinessAccess = "business_access"
	SettingSessionTTL     = "session_ttl"
	SettingBranding       = "branding"
	// SettingLanguages is the APPLICATION's locale pool (I18N): free BCP 47
	// tags. It is the master list - routes pick from it, and the flow pages
	// speak its intersection with the languages Meerkat embeds (fallback
	// English). Empty by default: the integrator declares their app's locales.
	SettingLanguages = "languages"
	// SettingMFARequired is the gateway-wide second-factor policy (MFA-04): when
	// true, every user must enrol a TOTP before the flow completes. A per-user
	// override on the user record may force it on/off (MFARequiredForUser).
	SettingMFARequired = "mfa_required"
	// SettingTrustedBrowser is the gateway-wide trusted-browser policy (MFA-03):
	// whether users may skip the TOTP challenge on a remembered browser, and the
	// ISO-8601 duration of that trust (TrustedBrowserPolicy).
	SettingTrustedBrowser = "trusted_browser"
	// SettingSMTP is the gateway-wide outbound e-mail config (mail.Config):
	// confirmations, admin notifications, later password resets.
	SettingSMTP = "smtp"
	// SettingPagesScheme decides the flow pages' light/dark behaviour (THEME-05):
	// "" lets the visitor choose (their system to begin with, then a button that
	// remembers), "light" or "dark" imposes one and removes the button.
	//
	// Imposing is not a whim: the pages sit in FRONT of an application that may
	// only know one look. A sign-in page following the visitor's dark system,
	// handing over to a portal that is light-only, reads as two products - and
	// the integrator cannot fix it from their side.
	SettingPagesScheme = "pages_scheme"
	// SettingPageLayout is how those same pages are ARRANGED (PAGE-02) - see
	// PageLayout. Colours answer "what does it look like", this one answers
	// "where is everything", and they are set on two tabs of one screen.
	SettingPageLayout = "page_layout"
	// SettingTLS is which HTTPS doors are open and whether an authority issues
	// certificates on its own (certs.Settings). The MATERIAL lives in its own
	// table - this holds only the switches, so turning HTTPS off never risks
	// touching a private key.
	SettingTLS = "tls"
	// SettingTLSSeeded records that both planes were given their first name
	// (localhost). Its own marker, so the names can be seeded ONCE on an
	// installation that already had TLS settings - and so removing them sticks.
	SettingTLSSeeded = "tls_seeded"
	// SettingDevMode is the installation-wide developer switch (DEV-01): the
	// master the per-account `dev` capability hangs from. Off, the developer
	// surface of the SERVED applications is not there at all - no Developer
	// entry in the user button, no UI test mode, no route API docs, no
	// developer profile page - however many accounts carry the capability.
	//
	// Two levels rather than one because they answer two questions. The
	// capability says WHO may develop against this gateway; this setting says
	// whether this INSTALLATION is one you develop against. A production
	// gateway turns it off once and stops caring which accounts kept the flag,
	// and a demo stops showing an amber bar to whoever is watching.
	//
	// Ships ON: the capability was the whole gate until now, and an upgrade
	// must not take a developer's tools away without anyone asking for it.
	SettingDevMode = "dev_mode"
	// SettingIssuesEnabled turns on the embedded issue tracker (ISSUE-04):
	// the user-button of proxied apps gains a "Report an issue" panel and the
	// data plane accepts reports. Ships OFF; the switch lives on the Issues
	// screen, next to what it fills.
	SettingIssuesEnabled = "issues_enabled"
	// SettingAgentEnabled opens the agent endpoint (MCP-01): /mcp on the
	// control plane, where an administrator's agent reads this gateway.
	// Ships OFF, like the issue tracker and for the same reason - a surface
	// nobody asked for should not appear on an upgrade. A control-plane token
	// is still required either way; this is the switch that says the door
	// exists at all, and it lives on the Access tokens screen, next to the
	// tokens that open it.
	SettingAgentEnabled = "agent_enabled"
	// SettingTenancy records the mode this installation was FIRST started in:
	// TenancySingle (one implicit organisation, the notion never surfaces) or
	// TenancyMulti. It is chosen at startup and never changes afterwards -
	// switching would either hide live organisations or leave every account
	// belonging nowhere - so this is the memory a later boot is checked
	// against, not a knob the console turns.
	SettingTenancy = "tenancy"
	// SettingTenancyChosen marks the mode as settled: the -tenancy flag seeds
	// it on a first boot, the console owns it afterwards.
	SettingTenancyChosen = "tenancy_chosen"
	// SettingRegistration is the self-registration policy (AUTH-20): who may
	// create their own account. Ships closed.
	SettingRegistration = "registration"
	// SettingRateLimit throttles the credential endpoints (RateLimitPolicy).
	SettingRateLimit = "rate_limit"
	// SettingAPITokens is the gateway-wide personal-API-token policy (AUTH-16):
	// whether users may mint and use them. Missing = allowed.
	SettingAPITokens = "api_tokens"
	// SettingPasskeys is the gateway-wide passkey policy (AUTH-15): whether
	// users may register passkeys and sign in with them. Global, never per
	// tenant: a passkey login happens before the tenant is known.
	SettingPasskeys = "passkeys_allowed"
	// One-time guard: the theme presets have been topped up into an existing
	// install (THEME-04). Prevents resurrecting a preset the admin deleted.
	SettingThemePresetsSeeded = "theme_presets_seeded"
	// SettingConfigSeed records the configuration file this gateway was seeded
	// from and its digest (CFG-03). It is what stops a bootstrap file from
	// being replayed at every restart, quietly resurrecting what an admin
	// deleted in the console. Never exported: it describes this install's
	// history, not how it is configured.
	SettingConfigSeed = "config_seed"
	// SettingVaultSeed records the encrypted vault file this gateway ingested
	// (VAULT-03), same guard and same reason as SettingConfigSeed: a file left
	// in a compose must not be replayed at every restart.
	SettingVaultSeed = "vault_seed"
)

// SupportedLanguages is every language the flow pages can speak - the
// catalogue lives in internal/auth; keep both in sync when adding one.
var SupportedLanguages = []string{"ar", "de", "en", "es", "fr", "he", "hi", "id", "it", "ja", "ko", "nl", "pl", "pt", "ru", "th", "tr", "uk", "vi", "zh-Hans"}

// defaultBusinessAccess opens every day around the clock - restricting is an
// explicit admin act, never a default.
func defaultBusinessAccess() BusinessAccess {
	days := make([]DayRange, 7)
	for i := range days {
		days[i] = DayRange{Day: i + 1, From: "00:00", To: "23:59"}
	}
	return BusinessAccess{Timezone: "UTC", Days: days}
}

// seedDefaultSettings writes the global defaults on first start (missing keys
// only - an existing value is never overwritten).
func (s *Store) seedDefaultSettings() error {
	ba, err := json.Marshal(defaultBusinessAccess())
	if err != nil {
		return fmt.Errorf("store: seed settings: %w", err)
	}
	branding, err := json.Marshal(DefaultBranding())
	if err != nil {
		return fmt.Errorf("store: seed settings: %w", err)
	}
	trusted, err := json.Marshal(TrustedBrowserPolicy{Allowed: false, TTL: "P7D"})
	if err != nil {
		return fmt.Errorf("store: seed settings: %w", err)
	}
	layout, err := json.Marshal(DefaultPageLayout())
	if err != nil {
		return fmt.Errorf("store: seed settings: %w", err)
	}
	pwPolicy, err := json.Marshal(DefaultPasswordPolicy())
	if err != nil {
		return fmt.Errorf("store: seed settings: %w", err)
	}
	defaults := map[string]string{
		SettingBusinessAccess: string(ba),
		SettingSessionTTL:     `"PT30M"`,
		SettingBranding:       string(branding),
		SettingMFARequired:    `false`,
		SettingPasskeys:       `true`,
		SettingTrustedBrowser: string(trusted),
		// AUTH-10: the eight characters the code used to demand in four
		// places, now in one, and raisable.
		SettingPasswordPolicy: string(pwPolicy),
		// The application locale pool ships EMPTY - the integrator declares
		// their app's languages (flow pages then fall back to English).
		SettingLanguages: `[]`,
		// Empty: the visitor decides, which is right until an integrator tells
		// us their application only knows one look.
		SettingPagesScheme: `""`,
		// The arrangement every version until now served: an installation that
		// upgrades must not find its pages redrawn.
		SettingPageLayout: string(layout),
		// Single until someone says otherwise at startup, with a license.
		SettingTenancy: `"` + TenancySingle + `"`,
		// On, because the dev CAPABILITY was the only gate until this setting
		// existed: an installation that upgrades must not silently lose the
		// developer tooling its people already had.
		SettingDevMode: `true`,
	}
	for key, value := range defaults {
		if _, err := s.db.Exec(
			`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`,
			key, value); err != nil {
			return fmt.Errorf("store: seed setting %q: %w", key, err)
		}
	}
	return s.seedTLSNames()
}

// seedTLSNames gives both planes the name every installation answers to on its
// first day.
//
// It carries its OWN marker rather than riding on "is the setting absent",
// because the two are not the same question: an installation that saved TLS
// settings before names existed has the key and no names, and would open on an
// Application block with nothing in it and no obvious first move.
//
// And it is a marker rather than a default applied on read, because a name an
// operator removes must stay removed - "empty means localhost" would put it
// back every time they cleared the last one, which is how a screen starts
// feeling disobedient.
func (s *Store) seedTLSNames() error {
	ctx := context.Background()
	var seeded bool
	if err := s.GetSetting(ctx, SettingTLSSeeded, &seeded); err == nil && seeded {
		return nil
	}
	var cfg certs.Settings
	_ = s.GetSetting(ctx, SettingTLS, &cfg)
	if cfg.ConsoleName == "" {
		cfg.ConsoleName = DefaultConsoleName
	}
	if len(cfg.AppNames) == 0 {
		cfg.AppNames = []string{DefaultConsoleName}
	}
	if err := s.SetSetting(ctx, SettingTLS, cfg); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingTLSSeeded, true)
}

// Tenancy reports the mode this installation runs in, defaulting to single
// for a database written before the setting existed.
func (s *Store) Tenancy(ctx context.Context) string {
	var mode string
	if err := s.GetSetting(ctx, SettingTenancy, &mode); err != nil || mode == "" {
		return TenancySingle
	}
	return mode
}

// TenancyWasChosen reports whether the mode has been settled already - by a
// first boot or by the console. Before that, the flag gets to seed it; after,
// the console owns it.
func (s *Store) TenancyWasChosen(ctx context.Context) (bool, error) {
	var chosen bool
	if err := s.GetSetting(ctx, SettingTenancyChosen, &chosen); err != nil {
		return false, nil // absent on a database written before the setting
	}
	return chosen, nil
}

// SetTenancy records the mode and marks it as chosen. Switching is allowed in
// both directions and at any time: it is read per request, so it takes effect
// immediately and reverses just as fast. Going down to single with several
// organisations does NOT delete anything - the others stop being served, which
// is what the caller is told to confirm.
func (s *Store) SetTenancy(ctx context.Context, mode string) error {
	if mode != TenancySingle && mode != TenancyMulti {
		return fmt.Errorf("tenancy %q is not allowed: use %q or %q", mode, TenancySingle, TenancyMulti)
	}
	if err := s.SetSetting(ctx, SettingTenancy, mode); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingTenancyChosen, true)
}

// PrimaryTenant is the organisation a single-tenant installation serves: the
// FIRST one, not a hard-coded id - an installation that created its own
// organisations has no "default", and picking by id would serve none of them.
//
// Oldest first, and the SEEDED one wins a tie. Insertion order used to answer
// this, through SQLite's rowid - which PostgreSQL does not have, and which was
// standing in for the tie-break rather than for the age: created_at has a
// second's resolution, so an installation that writes its organisations in the
// same second as its boot has several rows claiming to be the oldest.
//
// The seeded organisation is what a fresh install must keep serving, so it
// takes the tie explicitly. Past that it is alphabetical, which means nothing -
// but by then every candidate was created in the same second and nothing else
// would mean more.
func (s *Store) PrimaryTenant(ctx context.Context) (Tenant, error) {
	list, err := s.listTenants(ctx,
		`SELECT id, name, description, enabled, business_access, session_ttl, group_mode, created_by, owner_id, created_at, updated_at
		 FROM tenants
		 ORDER BY created_at ASC, CASE WHEN id = ? THEN 0 ELSE 1 END, id ASC LIMIT 1`, DefaultTenantID)
	if err != nil {
		return Tenant{}, err
	}
	if len(list) == 0 {
		return Tenant{}, fmt.Errorf("store: no organisation at all, which cannot happen: one is seeded at first boot")
	}
	return list[0], nil
}

// CountTenants is what a boot checks before accepting single mode: switching
// an installation that holds several organisations down to one would hide
// live data behind an interface that no longer mentions it.
func (s *Store) CountTenants(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count tenants: %w", err)
	}
	return n, nil
}

// seedDefaultTenant creates the implicit organisation on a database that has
// none. Every installation has one from its first boot, single or multi, so
// groups and memberships always have somewhere to hang and no code path has
// to answer "what if there is no organisation".
func (s *Store) seedDefaultTenant() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&n); err != nil {
		return fmt.Errorf("store: seed default tenant: %w", err)
	}
	if n > 0 {
		return nil
	}
	ba, err := json.Marshal(BusinessAccess{Inherited: true})
	if err != nil {
		return fmt.Errorf("store: seed default tenant: %w", err)
	}
	now := time.Now().Unix()
	if _, err := s.db.Exec(
		`INSERT INTO tenants (id, name, description, enabled, business_access, session_ttl, group_mode, created_by, owner_id, created_at, updated_at)
		 VALUES (?, ?, '', ?, ?, '', '', '', '', ?, ?)`,
		DefaultTenantID, defaultTenantName, true, string(ba), now, now); err != nil {
		return fmt.Errorf("store: seed default tenant: %w", err)
	}
	return nil
}

// GetSetting decodes the global setting stored under key into out.
func (s *Store) GetSetting(ctx context.Context, key string, out any) error {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return fmt.Errorf("store: get setting %q: %w", key, err)
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return fmt.Errorf("store: setting %q: bad value: %w", key, err)
	}
	return nil
}

// SetSetting stores a global setting as JSON under key.
func (s *Store) SetSetting(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("store: setting %q: %w", key, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, string(encoded))
	if err != nil {
		return fmt.Errorf("store: set setting %q: %w", key, err)
	}
	return nil
}

// ErrNoRows re-exports the sentinel for callers that translate absence to 404.
var ErrNoRows = sql.ErrNoRows
