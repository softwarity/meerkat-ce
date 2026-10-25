package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Deleting an entry asks the caller's right on its scope FIRST: answering 404
// or 409 before it would tell anybody signed in whether a name exists in a
// scope they do not administer.
func TestAVaultDeletionAsksTheRightFirst(t *testing.T) {
	f := setupBare(t)
	if code, out := f.call(t, "PUT", "/api/vault/infra/db-password", `{"kind":"secret","value":"s3cr3t"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("put: %d %s", code, out)
	}
	for _, name := range []string{"db-password", "nothing-here"} {
		code, out := f.call(t, "DELETE", "/api/vault/infra/"+name, "", f.plainC)
		if code != http.StatusForbidden {
			t.Fatalf("delete %s by somebody who administers no scope: %d %s, want 403 either way", name, code, out)
		}
	}
	if code, out := f.call(t, "DELETE", "/api/vault/infra/nothing-here", "", f.rootC); code != http.StatusNotFound {
		t.Fatalf("delete an absent entry: %d %s, want 404", code, out)
	}
}

// A reference is to a name IN A SCOPE: a route reads the infra entries, a
// schedule its organisation's then the application's. So a route naming
// $shared holds the infra "shared" and not the application one of the same
// name, and a schedule's header holds the application entry it resolves to.
func TestAVaultReferenceHoldsTheEntryItResolvesTo(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	for _, scope := range []string{"infra", "app"} {
		if code, out := f.call(t, "PUT", "/api/vault/"+scope+"/shared", `{"kind":"value","value":"v"}`, f.rootC); code != http.StatusOK {
			t.Fatalf("put %s: %d %s", scope, code, out)
		}
	}
	if code, out := f.call(t, "PUT", "/api/vault/app/svc-key", `{"kind":"secret","value":"k"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("put svc-key: %d %s", code, out)
	}
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "r1", Name: "orders-api", Enabled: true, Upstream: "http://${shared}:8080",
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.CreateSchedule(ctx, store.Schedule{
		ID: "sch_1", Name: "poll", RouteID: "r1", Method: "POST", Path: "/", Every: "PT1H", Overlap: "skip",
		Headers: map[string]string{"X-Key": "$svc-key"},
	}); err != nil {
		t.Fatal(err)
	}

	// The listing says who uses what, scope by scope.
	code, list := f.call(t, "GET", "/api/vault", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, list)
	}
	if strings.Count(list, "route: orders-api") != 1 || !strings.Contains(list, "schedule: poll") {
		t.Fatalf("the listing does not name each entry's own users: %s", list)
	}

	if code, out := f.call(t, "DELETE", "/api/vault/infra/shared", "", f.rootC); code != http.StatusConflict {
		t.Fatalf("delete the infra entry a route reads: %d %s, want 409", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/vault/app/shared", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete an application entry no route can read: %d %s, want 204", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/vault/app/svc-key", "", f.rootC); code != http.StatusConflict ||
		!strings.Contains(out, "schedule: poll") {
		t.Fatalf("delete the entry a schedule's header resolves to: %d %s, want 409 naming it", code, out)
	}
}
