package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The system mark is the product's, not a caller's: a write that clears it
// made a system role deletable, and one that sets it minted a role nobody
// could delete again.
func TestTheSystemMarkOfARoleIsReadOnly(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.SaveRole(ctx, store.Role{Name: "ROLE_CORE", System: true}); err != nil {
		t.Fatal(err)
	}

	if code, out := f.call(t, "PUT", "/api/roles/ROLE_CORE",
		`{"name":"ROLE_CORE","description":"edited","system":false}`, f.rootC); code != http.StatusOK {
		t.Fatalf("editing a system role: %d %s", code, out)
	}
	if saved, _ := f.api.st.GetRole(ctx, "ROLE_CORE"); !saved.System || saved.Description != "edited" {
		t.Fatalf("an edit changed the system mark or lost the description: %+v", saved)
	}
	if code, out := f.call(t, "DELETE", "/api/roles/ROLE_CORE", "", f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("deleting a system role after an edit: %d %s", code, out)
	}

	if code, out := f.call(t, "POST", "/api/roles", `{"name":"ROLE_MINE","system":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("creating a role: %d %s", code, out)
	}
	if saved, _ := f.api.st.GetRole(ctx, "ROLE_MINE"); saved.System {
		t.Fatalf("a created role took the system mark from its body")
	}
	if code, out := f.call(t, "DELETE", "/api/roles/ROLE_MINE", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("deleting a role created with system:true: %d %s", code, out)
	}
}
