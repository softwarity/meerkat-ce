package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// perimeterOf asks the endpoint what it would hand the channel for this caller.
//
// A plain GET is not an upgrade, so there is no socket to speak on: what the
// test needs is the ANSWER the funnel computes, and putting a recorder in the
// channel's place is the only way to read it without a websocket client in a
// unit test.
func perimeterOf(t *testing.T, f fixture, cookie *http.Cookie) (int, LivePerimeter) {
	t.Helper()
	var seen LivePerimeter
	f.api.Live = func(p LivePerimeter, w http.ResponseWriter, _ *http.Request) {
		seen = p
		writeJSON(w, http.StatusOK, p)
	}
	code, _ := f.call(t, "GET", "/api/live", "", cookie)
	return code, seen
}

// The live channel answers an ADMINISTRATOR, and what each one may watch is the
// trail's own partition (RBAC-05) rather than a second rule written for the
// socket.
func TestLivePerimeterFollowsTheCapability(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for id, u := range map[string]store.User{
		"gwa": {ID: "gwa", Username: "gwa", PasswordHash: "x", InfraAdmin: true, Enabled: true},
		"apa": {ID: "apa", Username: "apa", PasswordHash: "x", AppAdmin: true, Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	// Root: every kind, named, and both planes' own topics.
	code, root := perimeterOf(t, f, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("root refused: %d", code)
	}
	if len(root.Named) != len(store.AuditTargetKinds()) || len(root.Quiet) != 0 {
		t.Errorf("root must be told everything, named: %+v", root)
	}
	if !root.RoutingPlane || !root.ApplicationPlane {
		t.Errorf("root administers both planes: %+v", root)
	}

	// The routing plane: its own kinds, and the traffic curves.
	_, infra := perimeterOf(t, f, issue(t, f.api.sm, "gwa"))
	if !slices.Contains(infra.Named, "route") {
		t.Errorf("an infra-admin must be told its routes moved: %+v", infra)
	}
	if slices.Contains(infra.Named, "user") || slices.Contains(infra.Quiet, "user") {
		t.Errorf("an infra-admin has no business hearing about accounts: %+v", infra)
	}
	if !infra.RoutingPlane || infra.ApplicationPlane {
		t.Errorf("an infra-admin administers the routing plane only: %+v", infra)
	}
	// The vault has an entry per plane AND per organisation, so it is announced
	// without being described - one shared read cannot check which entries this
	// reader may see.
	if !slices.Contains(infra.Quiet, "vault") || slices.Contains(infra.Named, "vault") {
		t.Errorf("the vault must be announced quietly: %+v", infra)
	}

	// The application's plane: its own kinds, and the scheduled calls - whose own
	// API answers an app-admin, which is the hole this closed.
	_, app := perimeterOf(t, f, issue(t, f.api.sm, "apa"))
	for _, kind := range []string{"user", "role", "theme", "schedule"} {
		if !slices.Contains(app.Named, kind) {
			t.Errorf("an app-admin must be told %q moved: %+v", kind, app)
		}
	}
	if slices.Contains(app.Named, "route") {
		t.Errorf("an app-admin does not run the routing plane: %+v", app)
	}
	if app.RoutingPlane || !app.ApplicationPlane {
		t.Errorf("an app-admin administers the application's plane only: %+v", app)
	}

	// Two callers of the same perimeter share one read; two perimeters do not.
	if infra.Key == app.Key || infra.Key == root.Key {
		t.Errorf("two perimeters share one key: %q %q %q", root.Key, infra.Key, app.Key)
	}
}

// An organisation's administrator hears that their kinds moved and is told
// nothing about them: their partition is by organisation, and one shared read
// cannot check that this group belongs to an organisation THIS reader
// administers. The screen asks the API, which checks it.
func TestLivePerimeterOfATenantAdminSaysNothingAboutWhatMoved(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "POST", "/api/tenants", `{"name":"acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create tenant: %d %s", code, body)
	}
	var acme store.Tenant
	if err := json.Unmarshal([]byte(body), &acme); err != nil {
		t.Fatal(err)
	}
	if code, _ = f.call(t, "PUT", "/api/tenants/"+acme.ID+"/members/bob",
		`{"type":"ADMIN","enabled":true,"businessAccess":{"inherited":true}}`, f.rootC); code != http.StatusOK {
		t.Fatalf("add member: %d", code)
	}

	code, p := perimeterOf(t, f, f.plainC)
	if code != http.StatusOK {
		t.Fatalf("a tenant administrator was refused the channel: %d", code)
	}
	if len(p.Named) != 0 {
		t.Errorf("nothing may be named to a tenant administrator: %+v", p)
	}
	for _, kind := range []string{"tenant", "membership", "group", "grouprule"} {
		if !slices.Contains(p.Quiet, kind) {
			t.Errorf("a tenant administrator must be told %q moved: %+v", kind, p)
		}
	}
	if p.RoutingPlane || p.ApplicationPlane {
		t.Errorf("a tenant administrator administers neither plane: %+v", p)
	}
}

// A person who administers nothing is refused, the way the trail refuses them:
// the console's live channel is an administrative view, not an empty page. And a
// build with no channel says so rather than upgrading a socket nothing feeds.
func TestLiveChannelRefusesWhoAdministersNothing(t *testing.T) {
	f := setup(t)
	f.api.Live = func(LivePerimeter, http.ResponseWriter, *http.Request) {
		t.Error("the channel was served to somebody who administers nothing")
	}
	if code, body := f.call(t, "GET", "/api/live", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain user must be refused: %d %s", code, body)
	}
	f.api.Live = nil
	if code, body := f.call(t, "GET", "/api/live", "", f.rootC); code != http.StatusServiceUnavailable {
		t.Fatalf("a build with no channel must say so: %d %s", code, body)
	}
}
