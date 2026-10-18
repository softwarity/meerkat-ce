package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The catalogue says what uses each role, read off the routes: two routes
// naming ROLE_ADMIN are two entries under it, whatever its tags say, and a
// route naming it twice is one. What was read can be written back as it is.
func TestTheCatalogueSaysWhatUsesARole(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	for _, name := range []string{"ROLE_ADMIN", "ROLE_IDLE"} {
		if err := f.api.st.SaveRole(ctx, store.Role{Name: name, Tags: []string{"otel-demo"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"otel", "refdata"} {
		route := `{"name":"` + id + `","upstream":"http://example.test",
			"predicates":[{"type":"path","args":{"patterns":["/` + id + `/**"]}}],
			"access":{"level":"auth","roles":["ROLE_ADMIN","ROLE_ADMIN"]}}`
		if code, body := f.call(t, "PUT", "/api/routes/"+id, route, f.rootC); code != http.StatusOK {
			t.Fatalf("saving route %s: %d %s", id, code, body)
		}
	}

	code, body := f.call(t, "GET", "/api/roles", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, body)
	}
	var roles []catalogueRole
	if err := json.Unmarshal([]byte(body), &roles); err != nil {
		t.Fatal(err)
	}
	got := map[string][]roleUse{}
	for _, r := range roles {
		got[r.Name] = r.UsedBy
	}
	want := []roleUse{{Kind: "route", ID: "otel", Name: "otel"}, {Kind: "route", ID: "refdata", Name: "refdata"}}
	if len(got["ROLE_ADMIN"]) != 2 || got["ROLE_ADMIN"][0] != want[0] || got["ROLE_ADMIN"][1] != want[1] {
		t.Fatalf("ROLE_ADMIN used by %+v, want %+v", got["ROLE_ADMIN"], want)
	}
	if uses, ok := got["ROLE_IDLE"]; !ok || uses == nil || len(uses) != 0 {
		t.Fatalf("ROLE_IDLE used by %+v, want an empty list", uses)
	}

	// A write built on that read carries usedBy back: ignored, not refused.
	back := `{"name":"ROLE_ADMIN","description":"admins","parent":"","tags":[],"usedBy":[{"kind":"route","id":"x","name":"x"}]}`
	code, body = f.call(t, "PUT", "/api/roles/ROLE_ADMIN", back, f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"usedBy":[{"kind":"route","id":"otel"`) {
		t.Fatalf("writing back what was read: %d %s", code, body)
	}
}
