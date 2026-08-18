package config

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Who may do what is configuration: an organisation and its groups survive a
// reset, which is what makes a seeded gateway usable rather than merely
// routed. Accounts do not travel - they carry credentials - so a group's
// MEMBERS are not in the file, only the group and the roles it grants.
func TestTenantsAndGroupsTravel(t *testing.T) {
	ctx := context.Background()
	src := openTemp(t)

	if err := src.SaveRole(ctx, store.Role{ID: "r1", Name: "reader"}); err != nil {
		t.Fatal(err)
	}
	if err := src.CreateUser(ctx, store.User{ID: "u1", Username: "boss", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveTenant(ctx, store.Tenant{
		ID: "acme", Name: "Acme", Enabled: true, OwnerID: "u1", CreatedBy: "u1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveGroup(ctx, store.Group{
		ID: "g1", TenantID: "acme", Name: "ROOT", RoleIDs: []string{"r1"},
	}); err != nil {
		t.Fatal(err)
	}

	doc, _, err := Export(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Tenants) == 0 || len(doc.Groups) == 0 {
		t.Fatalf("the export carries no organisation or group: %+v", doc.Tenants)
	}

	// Another install, with an account of its own and none of the first's.
	dst := openTemp(t)
	if err := dst.CreateUser(ctx, store.User{ID: "other", Username: "admin", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, dst, doc, false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The default organisation of a fresh install is still there: an import
	// adds, it does not replace what it says nothing about.
	tenants, err := dst.ListTenants(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var acme *store.Tenant
	for i := range tenants {
		if tenants[i].ID == "acme" {
			acme = &tenants[i]
		}
	}
	if acme == nil || acme.Name != "Acme" {
		t.Fatalf("organisation did not land: %+v", tenants)
	}
	// The owner was remapped: u1 does not exist here, and OwnerID names a real
	// account or the organisation could not be saved at all.
	if acme.OwnerID != "other" {
		t.Errorf("owner = %q, want the importing install's own account", acme.OwnerID)
	}
	groups, err := dst.ListGroups(ctx, "acme")
	if err != nil || len(groups) != 1 {
		t.Fatalf("group did not land: %v %v", groups, err)
	}
	if len(groups[0].RoleIDs) != 1 || groups[0].RoleIDs[0] != "r1" {
		t.Errorf("the group lost the roles it grants: %+v", groups[0])
	}
}
