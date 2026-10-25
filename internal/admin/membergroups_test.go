package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A member holds the groups of the organisation it is a member of, and only
// those. The foreign key only proves a group exists SOMEWHERE, and the roles
// resolve from the member's rows by organisation alone: another organisation's
// group attached here handed its roles to every session acting in this one.
func TestAMemberHoldsOnlyItsOrganisationsGroups(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, id := range []string{"t1", "t2"} {
		if err := f.api.st.SaveTenant(ctx, store.Tenant{ID: id, Name: id, Enabled: true,
			BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.api.st.SaveRole(ctx, store.Role{Name: "ROLE_PAYROLL"}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SaveGroup(ctx, store.Group{ID: "g1", TenantID: "t1", Name: "staff"}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SaveGroup(ctx, store.Group{ID: "g2", TenantID: "t2", Name: "payroll", Roles: []string{"ROLE_PAYROLL"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SaveMembership(ctx, store.Membership{UserID: "bob", TenantID: "t1", Type: store.MemberUser,
		Enabled: true, BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}

	code, out := f.call(t, "PUT", "/api/tenants/t1/members/bob/groups", `["g1","g2"]`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "another tenant") {
		t.Fatalf("another organisation's group: %d %s", code, out)
	}
	if roles, _ := f.api.st.SessionRoleNames(ctx, "bob", "t1", ""); len(roles) != 0 {
		t.Fatalf("a refused assignment still granted %v in t1", roles)
	}
	if ids, _ := f.api.st.MemberGroupIDs(ctx, "t1", "bob"); len(ids) != 0 {
		t.Fatalf("a refused assignment left rows behind: %v", ids)
	}

	// Somebody who is not a member holds no group there, not even latently.
	if code, out := f.call(t, "PUT", "/api/tenants/t2/members/bob/groups", `["g2"]`, f.rootC); code != http.StatusNotFound {
		t.Fatalf("groups for a non-member: %d %s", code, out)
	}

	if code, out := f.call(t, "PUT", "/api/tenants/t1/members/bob/groups", `["g1"]`, f.rootC); code != http.StatusOK {
		t.Fatalf("the organisation's own group: %d %s", code, out)
	}
	// null clears like [] and is answered as a list, never as null.
	code, out = f.call(t, "PUT", "/api/tenants/t1/members/bob/groups", `null`, f.rootC)
	if code != http.StatusOK || strings.TrimSpace(out) != "[]" {
		t.Fatalf("a null list: %d %s", code, out)
	}
}
