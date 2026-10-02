package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Group modes (RBAC-03): how a member's groups combine within an organisation.
const (
	GroupModeMultiple = "MULTIPLE" // every assigned group cumulates
	GroupModeSingle   = "SINGLE"   // one group, chosen at login
)

// Role is a named authority in the GLOBAL catalogue (RBAC-01). Roles form a
// hierarchy: a parent role IMPLIES its children, so holding a role grants it and
// all its descendants. System roles are protected from deletion. Tags classify
// them in the console. The catalogue is gateway-wide; per-org GROUPS assemble
// subsets of it.
//
// THE NAME IS THE IDENTITY. There is no id beside it: an access rule names roles
// by name, a JWT carries names, and a generated id only made a configuration
// document unreadable (`parentId: d040c48d...`) and incomparable between two
// installations that both have ROLE_ADMIN.
type Role struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parent      string   `json:"parent"` // "" = top-level
	Tags        []string `json:"tags"`   // classification (e.g. per microservice)
	System      bool     `json:"system"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
	// Rev is the revision this row was READ at, carried back by a save so a
	// write built on a version somebody has replaced is refused. Zero means "I
	// read no version" and still wins - see rev.go.
	Rev int64 `json:"rev,omitempty"`
}

// Group is a per-organisation bundle of catalogue roles (RBAC-02). A member is
// assigned groups PER TENANT, so the same user carries different roles in
// different tenants.
type Group struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Roles are the catalogue roles this group grants, BY NAME - the only
	// identity a role has.
	Roles     []string `json:"roles"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
	// Rev is the revision this row was READ at, carried back by a save so a
	// write built on a version somebody has replaced is refused. Zero means "I
	// read no version" and still wins - see rev.go.
	Rev int64 `json:"rev,omitempty"`
}

// scanner is the shared surface of *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

// ── Roles (global catalogue) ─────────────────────────────────────────────────

// SaveRole inserts or updates a role, BY NAME. It rejects a parent that would
// create a cycle (a role can never be its own ancestor).
//
// IT NEVER RENAMES, and nothing else does either: a role IS its name, and the
// name is what leaves this gateway. A JWT carries it to every service behind
// the gateway, a customer's configuration carries it in a git repository, and
// their own code spells it out. Meerkat could rewrite its own rules and nothing
// else, so a rename would be an operation that LOOKS complete and is not -
// which is worse than one that is plainly a decision. Changing a role's name
// means deleting it and declaring the new one, through the refusal that names
// every rule still pointing at it.
func (s *Store) SaveRole(ctx context.Context, r Role) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return fmt.Errorf("store: role name is required")
	}
	r.Parent = strings.TrimSpace(r.Parent)
	if r.Parent != "" {
		if r.Parent == r.Name {
			return fmt.Errorf("store: role %q cannot be its own parent", r.Name)
		}
		cyclic, err := s.roleReaches(ctx, r.Parent, r.Name)
		if err != nil {
			return err
		}
		if cyclic {
			return fmt.Errorf("store: role %q: that parent would create a cycle", r.Name)
		}
	}
	// See rev.go: a write built on a version somebody has replaced is refused.
	if err := s.checkRevBy(ctx, "roles", "name", "role", r.Name, r.Rev); err != nil {
		return err
	}
	// [] rather than null when there are none: the column is read back into the
	// API's answer, and a list the console indexes into must never arrive null.
	if r.Tags == nil {
		r.Tags = []string{}
	}
	tags, _ := json.Marshal(r.Tags)
	var parent any
	if r.Parent != "" {
		parent = r.Parent
	}
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO roles (name, description, parent, tags, system, created_at, updated_at, rev)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(name) DO UPDATE SET
		   description = excluded.description, parent = excluded.parent,
		   tags = excluded.tags, system = excluded.system,
		   updated_at = excluded.updated_at, rev = roles.rev + 1`,
		r.Name, r.Description, parent, string(tags), r.System, now, now)
	if err != nil {
		return fmt.Errorf("store: save role %q: %w", r.Name, err)
	}
	return nil
}

// GetRole returns one role by name.
func (s *Store) GetRole(ctx context.Context, name string) (Role, error) {
	return scanRole(s.db.QueryRowContext(ctx,
		`SELECT name, description, parent, tags, system, created_at, updated_at, rev FROM roles WHERE name = ?`, name))
}

// ListRoles returns the whole catalogue, ordered by name.
func (s *Store) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, description, parent, tags, system, created_at, updated_at, rev FROM roles ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list roles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var roles []Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// DeleteRole removes a role. System roles are protected; children are re-parented
// to the top level (ON DELETE SET NULL) and the role drops out of every group
// (group_roles cascade).
func (s *Store) DeleteRole(ctx context.Context, name string) (bool, error) {
	r, err := s.GetRole(ctx, name)
	if err != nil {
		return false, nil //nolint:nilerr // absent = nothing to delete
	}
	if r.System {
		return false, fmt.Errorf("store: role %q is a system role and cannot be deleted", r.Name)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("store: delete role %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanRole(row scanner) (Role, error) {
	var r Role
	var parent sql.NullString
	var tags string
	if err := row.Scan(&r.Name, &r.Description, &parent, &tags, &r.System, &r.CreatedAt, &r.UpdatedAt, &r.Rev); err != nil {
		return Role{}, fmt.Errorf("store: get role: %w", err)
	}
	r.Parent = parent.String
	if err := json.Unmarshal([]byte(tags), &r.Tags); err != nil {
		return Role{}, fmt.Errorf("store: role %q: bad tags: %w", r.Name, err)
	}
	// Rows written before the line above, and rows a document imported without
	// tags at all, hold the four letters "null". The console indexes into this
	// list; it must be a list.
	if r.Tags == nil {
		r.Tags = []string{}
	}
	return r, nil
}

// roleReaches reports whether target is on the ancestor chain of start (walking
// parent upward) - used to reject hierarchy cycles.
func (s *Store) roleReaches(ctx context.Context, start, target string) (bool, error) {
	parents, err := s.roleParents(ctx)
	if err != nil {
		return false, err
	}
	for cur, hops := start, 0; cur != "" && hops <= len(parents); cur, hops = parents[cur], hops+1 {
		if cur == target {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) roleParents(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, COALESCE(parent, '') FROM roles`)
	if err != nil {
		return nil, fmt.Errorf("store: load role parents: %w", err)
	}
	defer func() { _ = rows.Close() }()
	parents := map[string]string{}
	for rows.Next() {
		var name, p string
		if err := rows.Scan(&name, &p); err != nil {
			return nil, fmt.Errorf("store: scan role parent: %w", err)
		}
		parents[name] = p
	}
	return parents, rows.Err()
}

// ── Groups (per tenant) ──────────────────────────────────────────────────────

// SaveGroup inserts or updates a group and replaces its role set, atomically.
func (s *Store) SaveGroup(ctx context.Context, g Group) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("store: group name is required")
	}
	if g.TenantID == "" {
		return fmt.Errorf("store: group %q: a tenant is required", g.Name)
	}
	// See rev.go: a write built on a version somebody has replaced is refused.
	if err := s.checkRev(ctx, "groups", "group", g.ID, g.Rev); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: save group %q: %w", g.Name, err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO groups (id, tenant_id, name, description, created_at, updated_at, rev)
		 VALUES (?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, description = excluded.description,
		   updated_at = excluded.updated_at, rev = groups.rev + 1`,
		g.ID, g.TenantID, g.Name, g.Description, now, now); err != nil {
		return fmt.Errorf("store: save group %q: %w", g.Name, err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM group_roles WHERE group_id = ?`, g.ID); err != nil {
		return fmt.Errorf("store: save group %q: %w", g.Name, err)
	}
	for _, name := range g.Roles {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO group_roles (group_id, role) VALUES (?, ?) ON CONFLICT DO NOTHING`, g.ID, name); err != nil {
			return fmt.Errorf("store: save group %q roles: %w", g.Name, err)
		}
	}
	return tx.Commit()
}

// GetGroup returns one group by id.
func (s *Store) GetGroup(ctx context.Context, id string) (Group, error) {
	var g Group
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, description, created_at, updated_at, rev FROM groups WHERE id = ?`, id).
		Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt, &g.Rev)
	if err != nil {
		return Group{}, fmt.Errorf("store: get group %q: %w", id, err)
	}
	if g.Roles, err = s.groupRoles(ctx, []string{id}); err != nil {
		return Group{}, err
	}
	if g.Roles == nil {
		g.Roles = []string{} // JSON [] - never null (the console indexes into it)
	}
	return g, nil
}

// ListGroups returns a tenant's groups (with their role sets), ordered by name.
func (s *Store) ListGroups(ctx context.Context, tenantID string) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, description, created_at, updated_at, rev FROM groups WHERE tenant_id = ? ORDER BY name ASC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list groups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var groups []Group
	for rows.Next() {
		// Roles starts as an empty slice, never nil: a role-less group must
		// serialise as [] (the console indexes into it).
		g := Group{Roles: []string{}}
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt, &g.Rev); err != nil {
			return nil, fmt.Errorf("store: scan group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list groups: %w", err)
	}
	// One pass for the role links, mapped back onto the final slice.
	byID := make(map[string]*Group, len(groups))
	for i := range groups {
		byID[groups[i].ID] = &groups[i]
	}
	rr, err := s.db.QueryContext(ctx,
		`SELECT gr.group_id, gr.role FROM group_roles gr
		 JOIN groups g ON g.id = gr.group_id WHERE g.tenant_id = ?`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list group roles: %w", err)
	}
	defer func() { _ = rr.Close() }()
	for rr.Next() {
		var gid, name string
		if err := rr.Scan(&gid, &name); err != nil {
			return nil, fmt.Errorf("store: scan group role: %w", err)
		}
		if g := byID[gid]; g != nil {
			g.Roles = append(g.Roles, name)
		}
	}
	return groups, rr.Err()
}

// DeleteGroup removes a group (its role links and member assignments cascade).
func (s *Store) DeleteGroup(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: delete group %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// groupRoles returns the role NAMES a set of groups grants directly, without
// the hierarchy.
func (s *Store) groupRoles(ctx context.Context, groupIDs []string) ([]string, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	args := make([]any, len(groupIDs))
	for i, g := range groupIDs {
		args[i] = g
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(groupIDs)), ",")
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT role FROM group_roles WHERE group_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: group roles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("store: scan group role: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// ── Member ↔ groups (per tenant) ─────────────────────────────────────────────

// SetMemberGroups replaces the groups a user holds in a tenant, atomically.
func (s *Store) SetMemberGroups(ctx context.Context, tenantID, userID string, groupIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: set member groups: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Only the rows an administrator owns are replaced. What a GROUP RULE
	// granted (RBAC-10) is not this call's to remove: it would come straight
	// back at the next sign-in, so taking it away here would only look like it
	// worked. Changing it means changing the rule.
	if _, err = tx.ExecContext(ctx,
		`DELETE FROM member_groups WHERE tenant_id = ? AND user_id = ? AND source = ''`,
		tenantID, userID); err != nil {
		return fmt.Errorf("store: set member groups: %w", err)
	}
	for _, gid := range groupIDs {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO member_groups (tenant_id, user_id, group_id, source) VALUES (?, ?, ?, '')
			 ON CONFLICT DO NOTHING`,
			tenantID, userID, gid); err != nil {
			return fmt.Errorf("store: set member groups: %w", err)
		}
	}
	return tx.Commit()
}

// MemberGroups returns the groups a user holds in a tenant WITH their names -
// the select-group step and the user button list them.
func (s *Store) MemberGroups(ctx context.Context, tenantID, userID string) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.tenant_id, g.name, g.created_at, g.updated_at
		 FROM member_groups mg JOIN groups g ON g.id = mg.group_id
		 WHERE mg.tenant_id = ? AND mg.user_id = ? ORDER BY g.name ASC`,
		tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("store: member groups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan member group: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SessionRoleNames resolves the role names a SESSION actually carries - the
// single place the group mode applies (RBAC-03). Cumulative: every assigned
// group counts. Exclusive (SINGLE): only the session's ACTIVE group counts,
// re-validated against the memberships on every call (a user pulled from a
// group loses its roles at the very next request); none chosen = no roles.
func (s *Store) SessionRoleNames(ctx context.Context, userID, tenantID, groupID string) ([]string, error) {
	if tenantID == "" {
		return nil, nil
	}
	ids, err := s.MemberGroupIDs(ctx, tenantID, userID)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	if s.EffectiveGroupMode(ctx, tenantID) == GroupModeSingle {
		for _, id := range ids {
			if id == groupID {
				return s.EffectiveRoleNames(ctx, []string{groupID})
			}
		}
		return nil, nil
	}
	return s.EffectiveRoleNames(ctx, ids)
}

// RolesReachableIn returns every role a user could hold in a tenant: the union
// over all their groups there, whatever the tenant's group mode.
//
// It answers a DIFFERENT question from SessionRoleNames, which says what they
// hold right now - in exclusive mode, nothing at all until a group is chosen.
// The difference is the whole point when deciding whether switching
// organisation would lift a refusal, because the group is chosen after the
// organisation is: asking what she holds there before she has been there
// answers no every time.
func (s *Store) RolesReachableIn(ctx context.Context, userID, tenantID string) ([]string, error) {
	if tenantID == "" || userID == "" {
		return nil, nil
	}
	ids, err := s.MemberGroupIDs(ctx, tenantID, userID)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return s.EffectiveRoleNames(ctx, ids)
}

// MemberGroupIDs returns the groups a user holds in a tenant.
func (s *Store) MemberGroupIDs(ctx context.Context, tenantID, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id FROM member_groups WHERE tenant_id = ? AND user_id = ? ORDER BY group_id`,
		tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("store: member group ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan member group id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ── Group mode (per tenant, RBAC-03) ─────────────────────────────────────────

// GetGroupMode returns the tenant's raw group_mode column ("" = default cumulative).
func (s *Store) GetGroupMode(ctx context.Context, tenantID string) (string, error) {
	var m string
	err := s.db.QueryRowContext(ctx, `SELECT group_mode FROM tenants WHERE id = ?`, tenantID).Scan(&m)
	if err != nil {
		return "", fmt.Errorf("store: get group mode %q: %w", tenantID, err)
	}
	return m, nil
}

// EffectiveGroupMode resolves the mode that ACTUALLY applies to a tenant:
// the tenant's own value when forced, otherwise cumulative. The group mode is a
// per-tenant responsibility (RBAC-03); there is no gateway-wide default.
func (s *Store) EffectiveGroupMode(ctx context.Context, tenantID string) string {
	if m, err := s.GetGroupMode(ctx, tenantID); err == nil && m != "" {
		return m
	}
	return GroupModeMultiple
}

// SetGroupMode stores the tenant's group mode (SINGLE, MULTIPLE, "" = default
// cumulative).
func (s *Store) SetGroupMode(ctx context.Context, tenantID, mode string) error {
	if mode != GroupModeSingle && mode != GroupModeMultiple && mode != "" {
		return fmt.Errorf("store: group mode %q is not allowed: allowed modes are SINGLE, MULTIPLE, or empty (default cumulative)", mode)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE tenants SET group_mode = ?, updated_at = ? WHERE id = ?`, mode, time.Now().Unix(), tenantID)
	if err != nil {
		return fmt.Errorf("store: set group mode %q: %w", tenantID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: set group mode %q: %w", tenantID, ErrNoRows)
	}
	return nil
}

// ── Effective roles ──────────────────────────────────────────────────────────

// ExpandRoleNames expands role NAMES down the hierarchy, the same implication
// a real session gets (a role implies its descendants). Names outside the
// catalogue pass through untouched: a simulated identity may pose roles that
// only exist in the target application. Sorted and unique.
func (s *Store) ExpandRoleNames(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	children, err := s.roleChildren(ctx)
	if err != nil {
		return nil, err
	}
	return walkDown(children, names), nil
}

// EffectiveRoleNames resolves the role NAMES a set of groups grants: the union
// of the groups' roles, expanded DOWN the hierarchy (a role implies its
// descendants). Sorted and unique - ready for the JWT (RBAC-09) and the session.
func (s *Store) EffectiveRoleNames(ctx context.Context, groupIDs []string) ([]string, error) {
	direct, err := s.groupRoles(ctx, groupIDs)
	if err != nil || len(direct) == 0 {
		return nil, err
	}
	children, err := s.roleChildren(ctx)
	if err != nil {
		return nil, err
	}
	return walkDown(children, direct), nil
}

// roleChildren is the hierarchy by name: who each role implies.
//
// Since a role IS its name, this is the whole index the two expansions need -
// where they used to carry a map from id to name and another from name to id,
// and walk the first to answer in the second's terms.
func (s *Store) roleChildren(ctx context.Context) (map[string][]string, error) {
	roles, err := s.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	children := map[string][]string{}
	for _, r := range roles {
		if r.Parent != "" {
			children[r.Parent] = append(children[r.Parent], r.Name)
		}
	}
	return children, nil
}

// walkDown closes a set of names over the hierarchy, sorted and unique. A name
// nobody declared is kept as it stands: it may belong to the application rather
// than to this catalogue.
func walkDown(children map[string][]string, from []string) []string {
	seen := map[string]bool{}
	stack := append([]string(nil), from...)
	for len(stack) > 0 {
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[name] {
			continue
		}
		seen[name] = true
		stack = append(stack, children[name]...)
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
