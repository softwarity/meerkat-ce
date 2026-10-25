package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/notifier"
	"github.com/softwarity/meerkat/internal/store"
)

// fakeNotifier stands for the Enterprise notifications, which this package
// does not link: the control plane only reads the store and withdraws.
type fakeNotifier struct{ st *store.Store }

func (fakeNotifier) Internal() []notifier.Endpoint { return nil }
func (fakeNotifier) MountData(notifier.Mux)        {}
func (fakeNotifier) Receive(string)                {}
func (fakeNotifier) Badge(context.Context, store.Session) (notifier.Badge, bool) {
	return notifier.Badge{}, false
}
func (fakeNotifier) Purge(context.Context, int64) (int64, error) { return 0, nil }
func (f fakeNotifier) Withdraw(ctx context.Context, id string) error {
	return f.st.WithdrawAny(ctx, id)
}

// Publish stores what it is given as is, refusing a title "refuse me" the
// way the Enterprise checks refuse, on the field they concern: the control
// plane's part is the scope, the sender and the answer.
func (f fakeNotifier) Publish(ctx context.Context, raw []byte, from notifier.Sender, tenant string) (notifier.Published, error) {
	var m struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return notifier.Published{}, &notifier.Refusal{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if m.Title == "refuse me" {
		return notifier.Published{}, &notifier.Refusal{Status: http.StatusUnprocessableEntity, Field: "title",
			Message: "title refused"}
	}
	n := store.Notification{ID: "ntf_" + m.Title, TenantID: tenant, Sender: from.Name(), SenderVia: from.Via,
		Level: "info", Title: store.Localized{"en": m.Title}, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	saved, existed, err := f.st.CreateNotification(ctx, n, []string{"tina"})
	if err != nil {
		return notifier.Published{}, err
	}
	return notifier.Published{ID: saved.ID, Recipients: saved.Recipients, Existing: existed}, nil
}

func (fakeNotifier) Targets(context.Context) ([]notifier.Target, error) {
	return []notifier.Target{{Route: "r1", Name: "stations", UI: true, Service: "station-ui", Public: "/stations/"}}, nil
}

var linkFake sync.Once

// notifyFixture is a multi-tenant gateway with notifications linked: two
// tenants, A administered by tina, and an app-admin, maud.
type notifyFixture struct {
	fixture
	appC, tinaC *http.Cookie
}

func setupNotify(t *testing.T) notifyFixture {
	t.Helper()
	linkFake.Do(func() {
		if !notifier.Linked() {
			notifier.Register(func(d notifier.Deps) notifier.Notifier { return fakeNotifier{st: d.Store} })
		}
	})
	f := setup(t)
	f.api.Notify = fakeNotifier{st: f.api.st}
	ctx := context.Background()
	st := f.api.st
	for _, u := range []store.User{
		{ID: "maud", Username: "maud", PasswordHash: "x", AppAdmin: true, Enabled: true},
		{ID: "tina", Username: "tina", PasswordHash: "x", Enabled: true},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	for _, tn := range []store.Tenant{
		{ID: "a", Name: "A", Enabled: true, OwnerID: "tina"},
		{ID: "b", Name: "B", Enabled: true},
	} {
		if err := st.SaveTenant(ctx, tn); err != nil {
			t.Fatal(err)
		}
	}
	return notifyFixture{fixture: f, appC: issue(t, f.api.sm, "maud"), tinaC: issue(t, f.api.sm, "tina")}
}

func (f notifyFixture) settings(t *testing.T, c *http.Cookie) notifySettingsView {
	t.Helper()
	code, body := f.call(t, "GET", "/api/notifications/settings", "", c)
	if code != http.StatusOK {
		t.Fatalf("read settings: %d %s", code, body)
	}
	var v notifySettingsView
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func stateOf(v notifySettingsView, id string) string {
	for _, t := range v.Tenants {
		if t.ID == id {
			return t.State + "/" + t.OffBy
		}
	}
	return "absent"
}

func (f notifyFixture) put(t *testing.T, c *http.Cookie, path, body string, want int) string {
	t.Helper()
	code, out := f.call(t, "PUT", path, body, c)
	if code != want {
		t.Fatalf("PUT %s %s: %d %s, want %d", path, body, code, out, want)
	}
	return out
}

// The owner's scenario: root allows A only, the app-admin turns A off, root
// forces it back, the tenant admin turns it off.
func TestTheThreeLevelsOfTheNotifications(t *testing.T) {
	f := setupNotify(t)
	f.put(t, f.rootC, "/api/notifications/settings", `{"enabled":true,"off":["b"]}`, http.StatusOK)

	v := f.settings(t, f.appC)
	if v.Level != store.NotifyLevelAppAdmin || stateOf(v, "a") != "on/" || stateOf(v, "b") != "offAbove/root" {
		t.Fatalf("app-admin sees %+v", v.Tenants)
	}
	f.put(t, f.appC, "/api/notifications/settings", `{"on":["b"]}`, http.StatusUnprocessableEntity)
	f.put(t, f.appC, "/api/notifications/settings", `{"enabled":false}`, http.StatusForbidden)
	f.put(t, f.appC, "/api/notifications/settings", `{"cap":3}`, http.StatusForbidden)

	f.put(t, f.appC, "/api/notifications/settings", `{"off":["a"]}`, http.StatusOK)
	if v := f.settings(t, f.rootC); stateOf(v, "a") != "offBelow/app-admin" {
		t.Fatalf("root sees %+v", v.Tenants)
	}
	f.put(t, f.rootC, "/api/notifications/settings", `{"force":["a"]}`, http.StatusOK)
	if !f.api.st.NotificationsOn(context.Background(), "a") {
		t.Fatal("root forced A: it must be effective again")
	}
	if v := f.settings(t, f.appC); stateOf(v, "a") != "on/" {
		t.Fatalf("forcing resets the app-admin's decision, it sees %+v", v.Tenants)
	}

	// The tenant admin: its switch, out of reach for B.
	f.put(t, f.tinaC, "/api/tenants/a/notifications/settings", `{"on":false}`, http.StatusOK)
	if f.api.st.NotificationsOn(context.Background(), "a") {
		t.Fatal("the tenant admin turned A off")
	}
	if v := f.settings(t, f.appC); stateOf(v, "a") != "offBelow/tenant-admin" {
		t.Fatalf("app-admin sees %+v", v.Tenants)
	}
	if code, _ := f.call(t, "PUT", "/api/tenants/b/notifications/settings", `{"on":false}`, f.tinaC); code != http.StatusForbidden {
		t.Fatalf("tina administers A only, B answered %d", code)
	}
	if code, _ := f.call(t, "GET", "/api/notifications/settings", "", f.tinaC); code != http.StatusForbidden {
		t.Fatalf("the tenant admin reads no gateway setting, got %d", code)
	}
	// The app-admin forces A: the tenant admin's decision goes.
	f.put(t, f.appC, "/api/notifications/settings", `{"force":["a"]}`, http.StatusOK)
	if !f.api.st.NotificationsOn(context.Background(), "a") {
		t.Fatal("the app-admin forced A on")
	}
	// Turned off above, the tenant admin cannot turn it back on.
	f.put(t, f.appC, "/api/notifications/settings", `{"off":["a"]}`, http.StatusOK)
	out := f.put(t, f.tinaC, "/api/tenants/a/notifications/settings", `{"on":true}`, http.StatusUnprocessableEntity)
	if !strings.Contains(out, "the application admin") {
		t.Fatalf("the refusal must name who: %s", out)
	}
	code, body := f.call(t, "GET", "/api/tenants/a/notifications/settings", "", f.tinaC)
	if code != http.StatusOK || !strings.Contains(body, `"offAbove"`) || !strings.Contains(body, `"app-admin"`) {
		t.Fatalf("tenant view: %d %s", code, body)
	}
	// Every write is in the trail.
	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]int{}
	for _, e := range events {
		actions[e.Action]++
	}
	if actions["settings.notifications"] < 5 || actions["tenant.notifications"] != 1 {
		t.Fatalf("audited: %v", actions)
	}
}

// A single-tenant gateway has no tenant level: what it stored is not read,
// and its endpoints refuse.
func TestASingleTenantHasNoTenantLevel(t *testing.T) {
	f := setupNotify(t)
	ctx := context.Background()
	st := f.api.st
	n := st.NotifySettings(ctx)
	n.Enabled, n.TenantOff = true, []string{store.DefaultTenantID}
	if err := st.SetNotifySettings(ctx, n); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTenancy(ctx, store.TenancySingle); err != nil {
		t.Fatal(err)
	}
	if !st.NotificationsOn(ctx, store.DefaultTenantID) {
		t.Fatal("single tenant: the tenant level must be ignored")
	}
	if v := f.settings(t, f.rootC); !v.Single || len(v.Tenants) != 1 || v.Tenants[0].ID != store.DefaultTenantID {
		t.Fatalf("single tenant: the one tenant only, got %+v", v)
	}
	code, _ := f.call(t, "PUT", "/api/tenants/"+store.DefaultTenantID+"/notifications/settings", `{"on":false}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("single tenant: the tenant switch must be refused, got %d", code)
	}
}

// A tenant admin reads and withdraws what was sent to their tenant, never
// another's.
func TestATenantAdminSeesOnlyTheirTenantsNotifications(t *testing.T) {
	f := setupNotify(t)
	ctx := context.Background()
	exp := time.Now().Add(time.Hour).Unix()
	for _, n := range []store.Notification{
		{ID: "ntf_a", TenantID: "a", Sender: "svc", SenderVia: "token", Level: "info", Title: store.Localized{"en": "for A"}, ExpiresAt: exp},
		{ID: "ntf_b", TenantID: "b", Sender: "svc", SenderVia: "token", Level: "info", Title: store.Localized{"en": "for B"}, ExpiresAt: exp},
	} {
		if _, _, err := f.api.st.CreateNotification(ctx, n, []string{"tina"}); err != nil {
			t.Fatal(err)
		}
	}
	code, body := f.call(t, "GET", "/api/tenants/a/notifications", "", f.tinaC)
	if code != http.StatusOK || !strings.Contains(body, "ntf_a") || strings.Contains(body, "ntf_b") {
		t.Fatalf("A's list: %d %s", code, body)
	}
	if code, _ := f.call(t, "GET", "/api/tenants/a/notifications/ntf_b/readers", "", f.tinaC); code != http.StatusNotFound {
		t.Fatalf("B's notification through A: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/tenants/a/notifications/ntf_b", "", f.tinaC); code != http.StatusNotFound {
		t.Fatalf("withdrawing B's through A: %d", code)
	}
	if code, _ := f.call(t, "GET", "/api/tenants/b/notifications", "", f.tinaC); code != http.StatusForbidden {
		t.Fatalf("B's list: %d", code)
	}
	if code, _ := f.call(t, "GET", "/api/notifications", "", f.tinaC); code != http.StatusForbidden {
		t.Fatalf("the gateway's list: %d", code)
	}
	if code, body := f.call(t, "GET", "/api/tenants/a/notifications/ntf_a/readers", "", f.tinaC); code != http.StatusOK || !strings.Contains(body, "tina") {
		t.Fatalf("A's readers: %d %s", code, body)
	}
	if code, body := f.call(t, "DELETE", "/api/tenants/a/notifications/ntf_a", "", f.tinaC); code != http.StatusNoContent {
		t.Fatalf("withdrawing A's: %d %s", code, body)
	}
	if n, _ := f.api.st.GetNotification(ctx, "ntf_a"); n.WithdrawnAt == 0 {
		t.Fatal("A's notification must be withdrawn")
	}
}

// Sending one from the tenant's page: its administrator may, to that tenant
// only, and the list then names them as its sender. A refusal comes back on
// the field it concerns.
func TestATenantAdminSendsANotificationToTheirTenant(t *testing.T) {
	f := setupNotify(t)
	if err := f.api.st.SaveMembership(context.Background(), store.Membership{UserID: "tina", TenantID: "a",
		Type: store.MemberAdmin, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	code, body := f.call(t, "POST", "/api/tenants/a/notifications", `{"title":"hello"}`, f.tinaC)
	if code != http.StatusCreated || !strings.Contains(body, `"id":"ntf_hello"`) {
		t.Fatalf("send to A: %d %s", code, body)
	}
	n, err := f.api.st.GetNotification(context.Background(), "ntf_hello")
	if err != nil || n.TenantID != "a" || n.Sender != "tina" || n.SenderVia != store.CallerConsole {
		t.Fatalf("stored %+v %v; want tenant a, sent by tina from the console", n, err)
	}
	if code, body := f.call(t, "GET", "/api/tenants/a/notifications", "", f.tinaC); code != http.StatusOK ||
		!strings.Contains(body, `"sender":"tina"`) {
		t.Fatalf("A's list must show it, sent by tina: %d %s", code, body)
	}
	if code, _ := f.call(t, "POST", "/api/tenants/b/notifications", `{"title":"x"}`, f.tinaC); code != http.StatusForbidden {
		t.Fatalf("send to B, which tina does not administer: %d", code)
	}
	code, body = f.call(t, "POST", "/api/tenants/a/notifications", `{"title":"refuse me"}`, f.tinaC)
	var out struct{ Error, Field string }
	if err := json.Unmarshal([]byte(body), &out); err != nil || code != http.StatusUnprocessableEntity ||
		out.Field != "title" || out.Error != "title refused" {
		t.Fatalf("a refusal names its field: %d %s", code, body)
	}
	code, body = f.call(t, "GET", "/api/tenants/a/notifications/compose", "", f.tinaC)
	if code != http.StatusOK || !strings.Contains(body, `"service":"station-ui"`) || !strings.Contains(body, `"username":"tina"`) ||
		!strings.Contains(body, `"tenant":"A"`) {
		t.Fatalf("what the builder offers: %d %s", code, body)
	}
	if code, _ := f.call(t, "GET", "/api/tenants/b/notifications/compose", "", f.tinaC); code != http.StatusForbidden {
		t.Fatalf("B's builder: %d", code)
	}
}
