package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func rbacStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, context.Background()
}

// admin ⊃ editor ⊃ viewer: holding a role grants it and every descendant.
func seedHierarchy(ctx context.Context, t *testing.T, s *Store) {
	t.Helper()
	for _, r := range []Role{
		{Name: "admin"},
		{Name: "editor", Parent: "admin"},
		{Name: "viewer", Parent: "editor"},
	} {
		if err := s.SaveRole(ctx, r); err != nil {
			t.Fatalf("SaveRole %s: %v", r.Name, err)
		}
	}
}

func TestEffectiveRolesClosure(t *testing.T) {
	s, ctx := rbacStore(t)
	seedHierarchy(ctx, t, s)
	if err := s.SaveTenant(ctx, Tenant{ID: "t1", Name: "acme", Enabled: true}); err != nil {
		t.Fatalf("SaveTenant: %v", err)
	}
	// A group granting "editor" must resolve to editor + viewer (its descendant),
	// not admin (its ancestor).
	if err := s.SaveGroup(ctx, Group{ID: "g1", TenantID: "t1", Name: "staff", Roles: []string{"editor"}}); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	got, err := s.EffectiveRoleNames(ctx, []string{"g1"})
	if err != nil {
		t.Fatalf("EffectiveRoleNames: %v", err)
	}
	if want := []string{"editor", "viewer"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("effective roles = %v, want %v", got, want)
	}
	// "admin" pulls the whole chain.
	if err := s.SaveGroup(ctx, Group{ID: "g2", TenantID: "t1", Name: "ops", Roles: []string{"admin"}}); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	got, _ = s.EffectiveRoleNames(ctx, []string{"g2"})
	if want := []string{"admin", "editor", "viewer"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("effective roles = %v, want %v", got, want)
	}
}

func TestRoleCycleRejected(t *testing.T) {
	s, ctx := rbacStore(t)
	seedHierarchy(ctx, t, s)
	// admin is viewer's ancestor -> making admin a child of viewer is a cycle.
	err := s.SaveRole(ctx, Role{Name: "admin", Parent: "viewer"})
	if err == nil {
		t.Fatal("expected a cycle error, got nil")
	}
	// Self-parenting is also rejected.
	if err := s.SaveRole(ctx, Role{Name: "editor", Parent: "editor"}); err == nil {
		t.Fatal("expected self-parent to be rejected")
	}
}

func TestSystemRoleProtected(t *testing.T) {
	s, ctx := rbacStore(t)
	if err := s.SaveRole(ctx, Role{Name: "authenticated", System: true}); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}
	if _, err := s.DeleteRole(ctx, "authenticated"); err == nil {
		t.Fatal("expected system role deletion to be refused")
	}
	if _, err := s.GetRole(ctx, "authenticated"); err != nil {
		t.Fatalf("system role should still exist: %v", err)
	}
}

// A role cannot be renamed, and this is where that is pinned.
//
// The store has no rename and SaveRole keys on the name, so saving under a new
// one DECLARES A SECOND ROLE and leaves the first standing. That is the whole
// decision: a name is what leaves this gateway - in every token a service
// reads, in every configuration held in a repository - so moving it here would
// rewrite our half and none of theirs.
func TestARoleCannotBeRenamedThroughSave(t *testing.T) {
	s, ctx := rbacStore(t)
	seedHierarchy(ctx, t, s)
	if err := s.SaveTenant(ctx, Tenant{ID: "t1", Name: "acme", Enabled: true}); err != nil {
		t.Fatalf("SaveTenant: %v", err)
	}
	if err := s.SaveGroup(ctx, Group{ID: "g1", TenantID: "t1", Name: "staff", Roles: []string{"editor"}}); err != nil {
		t.Fatalf("SaveGroup: %v", err)
	}
	editor, err := s.GetRole(ctx, "editor")
	if err != nil {
		t.Fatalf("GetRole editor: %v", err)
	}
	// Writing the row BACK under another name is refused by the revision guard,
	// and that is the honest answer: a revision belongs to a row, and under the
	// new name there is no row to have read.
	moved := editor
	moved.Name = "redactor"
	if err := s.SaveRole(ctx, moved); err == nil {
		t.Error("a role read at a revision was written back under another name")
	}
	// Declaring the new name outright is allowed - it is a new role, and that
	// is the point: the old one stays until somebody removes it on purpose.
	if err := s.SaveRole(ctx, Role{Name: "redactor", Description: editor.Description}); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}
	if _, err := s.GetRole(ctx, "editor"); err != nil {
		t.Error("declaring another name took the original away: that would be a rename")
	}
	if _, err := s.GetRole(ctx, "redactor"); err != nil {
		t.Error("declaring another name created nothing")
	}
	// And what pointed at the old name still does - which is why the console
	// refuses this outright rather than letting it read as a rename.
	g, err := s.GetGroup(ctx, "g1")
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if !reflect.DeepEqual(g.Roles, []string{"editor"}) {
		t.Errorf("the group no longer names what it named: %v", g.Roles)
	}
}

func TestDeletedRoleReparentsChildren(t *testing.T) {
	s, ctx := rbacStore(t)
	seedHierarchy(ctx, t, s)
	if _, err := s.DeleteRole(ctx, "editor"); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	// viewer survives, promoted to the top level (ON DELETE SET NULL).
	v, err := s.GetRole(ctx, "viewer")
	if err != nil {
		t.Fatalf("GetRole viewer: %v", err)
	}
	if v.Parent != "" {
		t.Fatalf("viewer should be top-level after its parent is deleted, got parent %q", v.Parent)
	}
}

func TestMemberGroupsPerTenant(t *testing.T) {
	s, ctx := rbacStore(t)
	seedHierarchy(ctx, t, s)
	if err := s.CreateUser(ctx, User{ID: "u1", Username: "alice", PasswordHash: "h", Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	for _, id := range []string{"t1", "t2"} {
		if err := s.SaveTenant(ctx, Tenant{ID: id, Name: id, Enabled: true}); err != nil {
			t.Fatalf("SaveTenant %s: %v", id, err)
		}
	}
	// Different groups (hence different roles) per tenant for the same user.
	if err := s.SaveGroup(ctx, Group{ID: "g-t1", TenantID: "t1", Name: "admins", Roles: []string{"admin"}}); err != nil {
		t.Fatalf("SaveGroup t1: %v", err)
	}
	if err := s.SaveGroup(ctx, Group{ID: "g-t2", TenantID: "t2", Name: "readers", Roles: []string{"viewer"}}); err != nil {
		t.Fatalf("SaveGroup t2: %v", err)
	}
	if err := s.SetMemberGroups(ctx, "t1", "u1", []string{"g-t1"}); err != nil {
		t.Fatalf("SetMemberGroups t1: %v", err)
	}
	if err := s.SetMemberGroups(ctx, "t2", "u1", []string{"g-t2"}); err != nil {
		t.Fatalf("SetMemberGroups t2: %v", err)
	}

	g1, _ := s.MemberGroupIDs(ctx, "t1", "u1")
	r1, _ := s.EffectiveRoleNames(ctx, g1)
	if want := []string{"admin", "editor", "viewer"}; !reflect.DeepEqual(r1, want) {
		t.Fatalf("t1 roles = %v, want %v", r1, want)
	}
	g2, _ := s.MemberGroupIDs(ctx, "t2", "u1")
	r2, _ := s.EffectiveRoleNames(ctx, g2)
	if want := []string{"viewer"}; !reflect.DeepEqual(r2, want) {
		t.Fatalf("t2 roles = %v, want %v", r2, want)
	}
}

func TestGroupModePerTenant(t *testing.T) {
	s, ctx := rbacStore(t)
	if err := s.SaveTenant(ctx, Tenant{ID: "t1", Name: "acme", Enabled: true}); err != nil {
		t.Fatalf("SaveTenant: %v", err)
	}
	// An unset tenant mode defaults to cumulative - the group mode is a
	// per-tenant responsibility (RBAC-03), there is no gateway-wide default.
	if m, _ := s.GetGroupMode(ctx, "t1"); m != "" {
		t.Fatalf("default group mode = %q, want empty", m)
	}
	if m := s.EffectiveGroupMode(ctx, "t1"); m != GroupModeMultiple {
		t.Fatalf("effective default = %q, want MULTIPLE", m)
	}
	// A forced tenant value applies as-is.
	if err := s.SetGroupMode(ctx, "t1", GroupModeSingle); err != nil {
		t.Fatalf("SetGroupMode: %v", err)
	}
	if m := s.EffectiveGroupMode(ctx, "t1"); m != GroupModeSingle {
		t.Fatalf("forced effective = %q, want SINGLE", m)
	}
	// Clearing it falls back to cumulative.
	if err := s.SetGroupMode(ctx, "t1", ""); err != nil {
		t.Fatalf("SetGroupMode(clear): %v", err)
	}
	if m := s.EffectiveGroupMode(ctx, "t1"); m != GroupModeMultiple {
		t.Fatalf("cleared effective = %q, want MULTIPLE", m)
	}
	if err := s.SetGroupMode(ctx, "t1", "BOGUS"); err == nil {
		t.Fatal("expected invalid group mode to be rejected")
	}
}
