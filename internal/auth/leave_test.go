package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A person leaves the organisation their session is in (TENANT-02): the
// membership goes, the session stops carrying it at once, and the
// organisation's administrators read it in their trail. In a single
// installation there is nothing to leave.
func TestAPersonLeavesTheirOrganisation(t *testing.T) {
	mux, sm, st := groupSetup(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingTenancy, store.TenancyMulti); err != nil {
		t.Fatal(err)
	}
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	cookie := sessionCookieOf(login)
	do(t, mux, "POST", "/select-tenant", url.Values{"tenant": {"t2"}}, cookie)

	page := do(t, mux, "GET", "/profile/leave", nil, cookie)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "globex") {
		t.Fatalf("the confirmation does not name the organisation: %d", page.Code)
	}
	if res := do(t, mux, "POST", "/profile/leave", nil, cookie); res.Code != http.StatusSeeOther {
		t.Fatalf("leave: %d", res.Code)
	}
	if _, err := st.GetMembership(ctx, "u1", "t2"); err == nil {
		t.Fatal("the membership is still there")
	}
	if _, err := st.GetMembership(ctx, "u1", "t1"); err != nil {
		t.Fatalf("the other organisation went too: %v", err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	if s, err := sm.Resolve(ctx, req); err != nil || s.TenantID == "t2" {
		t.Fatalf("the session still carries the organisation left: %+v %v", s, err)
	}
	trail, _ := st.ListAuditEvents(ctx, store.AuditFilter{Scope: &store.AuditScope{TenantIDs: []string{"t2"}}})
	found := false
	for _, e := range trail {
		if e.Action == "member.leave" && e.TenantID == "t2" {
			found = true
		}
	}
	if !found {
		t.Fatal("globex's administrators cannot read that somebody left")
	}

	// A single installation: nothing to leave, the page goes back to the profile.
	if err := st.SetSetting(ctx, store.SettingTenancy, store.TenancySingle); err != nil {
		t.Fatal(err)
	}
	if res := do(t, mux, "POST", "/profile/leave", nil, cookie); res.Header().Get("Location") != "/profile" {
		t.Fatalf("single mode: %d %q", res.Code, res.Header().Get("Location"))
	}
}
