package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

const declaringSpec = `{
  "openapi": "3.0.0", "info": {"title": "demo", "version": "1"},
  "security": [{"bearer": []}],
  "paths": {
    "/admin": {"get": {"security": [{"bearer": ["ROLE_ADMIN"]}]}},
    "/orders": {"post": {"security": [{"bearer": ["ROLE_USER"]}]}}
  }
}`

// The import: the roles the application declares are listed, created in the
// catalogue under the route's prefix and tag, and tied to the route - in one
// call, refused whole when a target is missing.
func TestTheDeclaredRolesAreImportedAndMapped(t *testing.T) {
	f := setup(t)
	up := specServing(t, declaringSpec)
	route := fmt.Sprintf(`{
	  "name":"demo","order":1,"enabled":true,"upstream":"%s",
	  "predicates":[{"type":"path","args":{"patterns":["/demo/**"]}}],
	  "filters":[{"type":"strip-prefix","args":{"parts":1}}],
	  "api":{"spec":{"type":"upstream","path":"%s/spec.json"}}
	}`, up, up)
	if code, out := f.call(t, "PUT", "/api/routes/r1", route, f.rootC); code != http.StatusOK {
		t.Fatalf("put route: %d %s", code, out)
	}
	code, body := f.call(t, "GET", "/api/routes/r1/operations", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"declaredRoles":["ROLE_ADMIN","ROLE_USER"]`) ||
		!strings.Contains(body, `"upstream":{"access":"roles","roles":["ROLE_ADMIN"]}`) {
		t.Fatalf("operations do not carry what the spec declares: %d %s", code, body)
	}

	// A target that neither exists nor is created: refused, and nothing created.
	missing := `{"roleMap":{"ROLE_ADMIN":"demo-ROLE_ADMIN","ROLE_USER":"nobody"},
	  "create":[{"name":"demo-ROLE_ADMIN","tags":["demo"]}]}`
	if code, out := f.call(t, "PUT", "/api/routes/r1/role-map", missing, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing target: %d %s, want 422", code, out)
	}
	if _, err := f.api.st.GetRole(context.Background(), "demo-ROLE_ADMIN"); err == nil {
		t.Fatal("a refused mapping left a role in the catalogue")
	}

	// An infra admin may map onto what exists, not create catalogue roles.
	if err := f.api.st.CreateUser(context.Background(),
		store.User{ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	infraC := issue(t, f.api.sm, "infra")
	if err := f.api.st.SaveRole(context.Background(), store.Role{Name: "opt-svc"}); err != nil {
		t.Fatal(err)
	}
	importing := `{"roleMap":{"ROLE_ADMIN":"demo-ROLE_ADMIN","ROLE_USER":"opt-svc"},
	  "create":[{"name":"demo-ROLE_ADMIN","tags":["demo"]}]}`
	if code, out := f.call(t, "PUT", "/api/routes/r1/role-map", importing, infraC); code != http.StatusForbidden {
		t.Fatalf("infra admin creating roles: %d %s, want 403", code, out)
	}

	code, out := f.call(t, "PUT", "/api/routes/r1/role-map", importing, f.rootC)
	if code != http.StatusOK || !strings.Contains(out, `"created":["demo-ROLE_ADMIN"]`) {
		t.Fatalf("import: %d %s", code, out)
	}
	role, err := f.api.st.GetRole(context.Background(), "demo-ROLE_ADMIN")
	if err != nil || len(role.Tags) != 1 || role.Tags[0] != "demo" {
		t.Fatalf("created role %+v (%v), want tagged demo", role, err)
	}
	saved, _ := f.api.st.GetRoute(context.Background(), "r1")
	if m := saved.RoleMap(); m["ROLE_ADMIN"] != "demo-ROLE_ADMIN" || m["ROLE_USER"] != "opt-svc" {
		t.Fatalf("saved map %v", m)
	}

	// Mapping onto existing roles only is the infra admin's to do.
	if code, out := f.call(t, "PUT", "/api/routes/r1/role-map", `{"roleMap":{"ROLE_USER":"opt-svc"}}`, infraC); code != http.StatusOK {
		t.Fatalf("infra admin mapping: %d %s", code, out)
	}
	// And an empty map clears it.
	if code, out := f.call(t, "PUT", "/api/routes/r1/role-map", `{"roleMap":{}}`, f.rootC); code != http.StatusOK {
		t.Fatalf("clear: %d %s", code, out)
	}
	saved, _ = f.api.st.GetRoute(context.Background(), "r1")
	if saved.RoleMap() != nil {
		t.Fatalf("cleared map still %v", saved.RoleMap())
	}
}

// specServing is an upstream serving spec at /spec.json; its address.
func specServing(t *testing.T, spec string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, spec)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
