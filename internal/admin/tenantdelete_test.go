package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// An installation always keeps one organisation, and a single-organisation
// one keeps the organisation it serves: deleting it put another, hidden until
// then, in service; deleting the last left nothing to serve at all.
func TestTheServedAndTheLastOrganisationStay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "hidden", Enabled: true,
		BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SetTenancy(ctx, store.TenancySingle); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "DELETE", "/api/tenants/"+store.DefaultTenantID, "", f.rootC); code != http.StatusConflict {
		t.Fatalf("deleting the served organisation in single mode: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/tenants/t1", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("deleting a hidden organisation: %d %s", code, out)
	}
	if err := f.api.st.SetTenancy(ctx, store.TenancyMulti); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "DELETE", "/api/tenants/"+store.DefaultTenantID, "", f.rootC); code != http.StatusConflict {
		t.Fatalf("deleting the last organisation: %d %s", code, out)
	}
	if _, err := f.api.st.PrimaryTenant(ctx); err != nil {
		t.Fatalf("no organisation left to serve: %v", err)
	}
}
