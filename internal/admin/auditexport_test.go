package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The retention is root's alone (AUD-02) - whoever may shorten the trail may
// erase their own traces with it - and a lifetime that is not offered is
// refused rather than stored.
func TestTheAuditRetentionIsRootsAndAChoice(t *testing.T) {
	f := setup(t)
	if err := f.api.st.CreateUser(context.Background(), store.User{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	appC := issue(t, f.api.sm, "app")

	code, out := f.call(t, "GET", "/api/settings/audit", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(out, `"retentionDays":365`) {
		t.Fatalf("default: %d %s", code, out)
	}
	if code, _ := f.call(t, "PUT", "/api/settings/audit", `{"retentionDays":90,"choices":[]}`, appC); code != http.StatusForbidden {
		t.Fatalf("an application admin shortened the trail: %d", code)
	}
	if code, out := f.call(t, "PUT", "/api/settings/audit", `{"retentionDays":45,"choices":[]}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("45 days: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/audit", `{"retentionDays":730,"choices":[]}`, f.rootC); code != http.StatusOK || !strings.Contains(out, `"retentionDays":730`) {
		t.Fatalf("two years: %d %s", code, out)
	}
	if got := f.api.st.AuditRetentionDays(context.Background()); got != 730 {
		t.Fatalf("stored %d", got)
	}
}

// The export (STORE-06) is the screen as a file: CSV, the caller's perimeter,
// and a line of its own in the trail. Enterprise; the community image says so.
func TestTheAuditExportsAsCSV(t *testing.T) {
	f := setup(t)
	f.call(t, "PUT", "/api/settings/audit", `{"retentionDays":180,"choices":[]}`, f.rootC)
	res := f.get(t, "/api/audit/export", f.rootC)
	defer func() { _ = res.Body.Close() }()
	if !edition.Enterprise {
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("community image: %d", res.StatusCode)
		}
		return
	}
	body := readAll(t, res)
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("export: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if !strings.HasPrefix(body, "at,action,actor,") || !strings.Contains(body, "audit.retention") {
		t.Fatalf("the file does not carry the trail:\n%s", body)
	}
	events, _ := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{Target: "audit"})
	if len(events) != 1 || events[0].Action != "audit.export" {
		t.Fatalf("the export is not in the trail: %+v", events)
	}
}
