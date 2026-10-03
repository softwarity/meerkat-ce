package gateway

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/tracing"
)

// An audited call becomes an event with its caller, the fields the operation
// names and the body with its secrets masked - and the upstream still gets the
// whole body (AUD-04).
func TestAnAuditedCallBecomesAnEventAndTheBodyStillArrives(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("endpoint audit is Enterprise: the community image keeps the rules and applies none (AUD-04)")
	}
	var got sync.Map
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.Store("upstream", string(b))
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(upstream.Close)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	route := pathRoute("r-aud", "orders", 1, "/o/**", upstream.URL)
	route.API = &store.RouteAPI{Audit: []store.EndpointAudit{{
		Method: "POST", Path: "/o/orders/{id}", Description: "Order changed", Body: true,
		Fields: []store.AuditField{
			{Name: "order", From: store.AuditFromPath, Key: "id"},
			{Name: "reason", From: store.AuditFromQuery, Key: "reason"},
			{Name: "client", From: store.AuditFromHeader, Key: "X-Client"},
			{Name: "amount", From: store.AuditFromBody, Key: "/total/amount"},
		},
	}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	signed := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, signed, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}

	sent := make(chan string, 4)
	tracing.RegisterAuditStarter(func(tracing.Config) (func([]byte) error, error) {
		return func(b []byte) error { sent <- string(b); return nil }, nil
	})
	tracing.SetAuditScope(true, false)
	if err := tracing.ApplyAudit(tracing.Config{Endpoint: "http://collector"}, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tracing.ApplyAudit(tracing.Config{}, false); tracing.RegisterAuditStarter(nil) })

	body := `{"total":{"amount":42},"card":{"password":"hunter2"},"note":"rush"}`
	req := httptest.NewRequest("POST", "http://localhost/o/orders/77?reason=refund", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "kiosk-3")
	req.AddCookie(signed.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("the call answered %d", rec.Code)
	}
	if up, _ := got.Load("upstream"); up != body {
		t.Fatalf("the upstream got %q, want the whole body", up)
	}
	select {
	case ev := <-sent:
		for _, want := range []string{
			`"meerkat.stream"`, `"Order changed"`,
			`"audit.field.order","value":{"stringValue":"77"}`,
			`"audit.field.reason","value":{"stringValue":"refund"}`,
			`"audit.field.client","value":{"stringValue":"kiosk-3"}`,
			`"audit.field.amount","value":{"stringValue":"42"}`,
			`"user.name","value":{"stringValue":"alice"}`,
			`"http.response.status_code","value":{"intValue":"201"}`,
			`\"password\":\"***\"`,
		} {
			if !strings.Contains(ev, want) {
				t.Errorf("the event lacks %s:\n%s", want, ev)
			}
		}
		if strings.Contains(ev, "hunter2") {
			t.Errorf("a password left in the audit: %s", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no audit event was sent")
	}

	// An operation that is not audited sends nothing.
	rt.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost/o/other", nil))
	select {
	case ev := <-sent:
		t.Fatalf("an unaudited call was audited: %s", ev)
	case <-time.After(2500 * time.Millisecond):
	}
}

func TestAnAuditFieldMustSayWhereFrom(t *testing.T) {
	for _, bad := range [][]store.EndpointAudit{
		{{Method: "GET", Path: "/x", Fields: []store.AuditField{{Name: "a", From: "cookie", Key: "k"}}}},
		{{Method: "GET", Path: "/x", Fields: []store.AuditField{{Name: "a", From: "body", Key: "order.id"}}}},
		{{Method: "GET", Path: "/x", Fields: []store.AuditField{{Name: "", From: "query", Key: "k"}}}},
		{{Method: "BREW", Path: "/x"}},
		{{Method: "GET", Path: "/x"}, {Method: "get", Path: "/x"}},
	} {
		if err := store.ValidateAudit(bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

// On the community image the rules a configuration carries are kept and not
// applied: the call goes through untouched and no event is built.
func TestTheCommunityImageAppliesNoEndpointAudit(t *testing.T) {
	if edition.Enterprise {
		t.Skip("the community image's behaviour")
	}
	route := pathRoute("r1", "orders", 1, "/**", "http://127.0.0.1:1")
	route.API = &store.RouteAPI{Audit: []store.EndpointAudit{{Method: "GET", Path: "/x"}}}
	rt := newRouter(t, route)
	if rt.Problems()["r1"] != "" {
		t.Fatalf("a route carrying audit rules was left out: %s", rt.Problems()["r1"])
	}
}
