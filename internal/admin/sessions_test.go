package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Root reads every live session; an application administrator the
// applications' only, and a console session under their eyes reads as absent
// (CONSOLE-08). Ending one is a line of the trail.
func TestSessionsAreListedAndEndedWithinThePerimeter(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.CreateUser(ctx, store.User{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.CreateUser(ctx, store.User{ID: "alice", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour).Unix()
	for _, s := range []store.Session{
		{TokenHash: "aaaaaaaaaaaaaaaa1111", UserID: "alice", Plane: store.PlaneData, ExpiresAt: future, CreatedAt: 10, IP: "203.0.113.9", Agent: "Mozilla/5.0 (Macintosh) Chrome/120"},
		{TokenHash: "bbbbbbbbbbbbbbbb2222", UserID: "app", Plane: store.PlaneAdmin, ExpiresAt: future, CreatedAt: 20},
	} {
		if err := f.api.st.CreateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	appC := issue(t, f.api.sm, "app")
	read := func(c *http.Cookie) sessionPage {
		code, out := f.call(t, "GET", "/api/sessions", "", c)
		if code != http.StatusOK {
			t.Fatalf("list: %d %s", code, out)
		}
		var p sessionPage
		if err := json.Unmarshal([]byte(out), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, s := range read(appC).Sessions {
		if s.Plane == store.PlaneAdmin {
			t.Fatalf("an application administrator read a console session: %+v", s)
		}
	}
	if code, _ := f.call(t, "DELETE", "/api/sessions/bbbbbbbbbbbbbbbb", "", appC); code != http.StatusNotFound {
		t.Fatalf("an application administrator ended a console session: %d", code)
	}
	all := read(f.rootC)
	seen := map[string]bool{}
	for _, s := range all.Sessions {
		seen[s.ID] = true
		if s.ID == "aaaaaaaaaaaaaaaa" && s.Label != "Chrome - macOS" {
			t.Errorf("label %q", s.Label)
		}
	}
	if !seen["aaaaaaaaaaaaaaaa"] || !seen["bbbbbbbbbbbbbbbb"] {
		t.Fatalf("root does not read both planes: %+v", all.Sessions)
	}
	if code, _ := f.call(t, "DELETE", "/api/sessions/aaaaaaaaaaaaaaaa", "", appC); code != http.StatusNoContent {
		t.Fatalf("ending an application session: %d", code)
	}
	if _, err := f.api.st.GetSession(ctx, "aaaaaaaaaaaaaaaa1111"); err == nil {
		t.Fatal("the session is still there")
	}
	events, _ := f.api.st.ListAuditEvents(ctx, store.AuditFilter{Target: "user", TargetID: "alice"})
	if len(events) == 0 || events[0].Action != "session.revoke" {
		t.Fatalf("not in the trail: %+v", events)
	}
}

// Disabling an account ends its sessions at once (SEC-07), on both planes.
func TestDisablingAnAccountEndsItsSessions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if err := f.api.st.CreateUser(ctx, store.User{ID: "alice", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.CreateSession(ctx, store.Session{TokenHash: "cccccccccccccccc3333", UserID: "alice",
		Plane: store.PlaneData, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	u, _ := f.api.st.GetUserByID(ctx, "alice")
	body, _ := json.Marshal(map[string]any{"username": "alice", "enabled": false, "rev": u.Rev})
	if code, out := f.call(t, "PUT", "/api/users/alice", string(body), f.rootC); code != http.StatusOK {
		t.Fatalf("disable: %d %s", code, out)
	}
	if _, err := f.api.st.GetSession(ctx, "cccccccccccccccc3333"); err == nil {
		t.Fatal("a disabled account kept its session")
	}
}

// An application administrator reads every account's application tokens and
// revokes one; the owner's cache forgets it and the trail says who did it
// (AUTH-09). Somebody who administers nothing reads nothing.
func TestAdministratorsSeeAndRevokeApplicationTokens(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, u := range []store.User{
		{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true},
		{ID: "alice", Username: "alice", PasswordHash: "x", Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.api.st.AddAPIToken(ctx, store.NewToken{ID: "tk-ci", UserID: "alice", Name: "ci-script", TokenHash: "h", Prefix: "mk_x"}); err != nil {
		t.Fatal(err)
	}
	appC := issue(t, f.api.sm, "app")
	code, out := f.call(t, "GET", "/api/data-tokens?q=alice", "", appC)
	if code != http.StatusOK || !strings.Contains(out, `"ownerName":"alice"`) || !strings.Contains(out, "ci-script") {
		t.Fatalf("list: %d %s", code, out)
	}
	if code, _ := f.call(t, "GET", "/api/data-tokens", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("somebody who administers nothing: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/data-tokens/tk-ci", "", appC); code != http.StatusNoContent {
		t.Fatalf("revoke: %d", code)
	}
	if left, _ := f.api.st.ListAPITokens(ctx, "alice", store.PlaneData); len(left) != 0 {
		t.Fatalf("the token survived: %+v", left)
	}
	events, _ := f.api.st.ListAuditEvents(ctx, store.AuditFilter{Target: "user", TargetID: "alice"})
	if len(events) == 0 || events[0].Action != "token.revoke" || events[0].Detail != "ci-script" {
		t.Fatalf("not in the trail: %+v", events)
	}
}

// An organisation's administrator sees the sessions open in the organisations
// they administer, and ends those - nothing outside them.
func TestATenantAdministratorSeesTheirOrganisationsSessions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, id := range []string{"t1", "t2"} {
		if err := f.api.st.SaveTenant(ctx, store.Tenant{ID: id, Name: id, Enabled: true,
			BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.api.st.SaveMembership(ctx, store.Membership{UserID: "bob", TenantID: "t1", Type: store.MemberAdmin,
		Enabled: true, BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.CreateUser(ctx, store.User{ID: "carol", Username: "carol", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour).Unix()
	for _, s := range []store.Session{
		{TokenHash: "cccccccccccccccc1111", UserID: "carol", TenantID: "t1", Plane: store.PlaneData, ExpiresAt: future, CreatedAt: 10},
		{TokenHash: "dddddddddddddddd2222", UserID: "carol", TenantID: "t2", Plane: store.PlaneData, ExpiresAt: future, CreatedAt: 20},
	} {
		if err := f.api.st.CreateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	code, out := f.call(t, "GET", "/api/sessions", "", f.plainC)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, out)
	}
	var p sessionPage
	_ = json.Unmarshal([]byte(out), &p)
	if len(p.Sessions) != 1 || p.Sessions[0].ID != "cccccccccccccccc" {
		t.Fatalf("want t1's session only, got %+v", p.Sessions)
	}
	if code, _ := f.call(t, "DELETE", "/api/sessions/dddddddddddddddd", "", f.plainC); code != http.StatusNotFound {
		t.Fatalf("ended a session of another organisation: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/sessions/cccccccccccccccc", "", f.plainC); code != http.StatusNoContent {
		t.Fatalf("ending a session of their organisation: %d", code)
	}
}

// Somebody who administers nothing reads no session at all.
func TestSessionsAreRefusedToSomebodyWhoAdministersNothing(t *testing.T) {
	f := setup(t)
	if code, _ := f.call(t, "GET", "/api/sessions", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account read sessions: %d", code)
	}
}
