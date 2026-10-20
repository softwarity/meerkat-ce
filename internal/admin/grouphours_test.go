package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// A group created or renamed without a word about hours keeps none of its own
// - which is what the console's dialog sends - and hours of its own are held
// only in exclusive mode (TENANT-04).
func TestAGroupsHoursNeedExclusiveMode(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "acme", Enabled: true,
		BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	code, out := f.call(t, "POST", "/api/tenants/t1/groups", `{"name":"Finance","description":"","roles":[]}`, f.rootC)
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("a plain group: %d %s", code, out)
	}
	var g store.Group
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatal(err)
	}
	if !g.BusinessAccess.Inherited {
		t.Fatalf("a group created without hours got some: %+v", g.BusinessAccess)
	}
	night := `{"timezone":"UTC","days":[{"day":1,"from":"20:00","to":"23:59"}]}`
	body := `{"name":"Finance","description":"","roles":[],"businessAccess":` + night + `}`
	code, out = f.call(t, "PUT", "/api/tenants/t1/groups/"+g.ID, body, f.rootC)
	if !edition.Enterprise {
		if code != http.StatusForbidden {
			t.Fatalf("hours of its own on the community image: %d %s", code, out)
		}
		return
	}
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "exclusive") {
		t.Fatalf("hours of its own in cumulative mode: %d %s", code, out)
	}
	if err := f.api.st.SetGroupMode(ctx, "t1", store.GroupModeSingle); err != nil {
		t.Fatal(err)
	}
	if code, out = f.call(t, "PUT", "/api/tenants/t1/groups/"+g.ID, body, f.rootC); code != http.StatusOK {
		t.Fatalf("hours of its own in exclusive mode: %d %s", code, out)
	}
	// A rename that says nothing about hours keeps them.
	if code, out = f.call(t, "PUT", "/api/tenants/t1/groups/"+g.ID, `{"name":"Night","description":"","roles":[]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("rename: %d %s", code, out)
	}
	if saved, _ := f.api.st.GetGroup(ctx, g.ID); saved.BusinessAccess.Inherited || len(saved.BusinessAccess.Days) != 1 {
		t.Fatalf("a rename dropped the group's hours: %+v", saved.BusinessAccess)
	}
}
