package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

type fixture struct {
	api      *API
	adminSrv *httptest.Server // the control plane
	appSrv   *httptest.Server // the data plane, reloaded by the API
	rootC    *http.Cookie
	plainC   *http.Cookie
}

func setup(t *testing.T) fixture {
	t.Helper()
	// These tests are about the API's behaviour, the same in both editions, so
	// they run in both: the community pipeline is the one the public mirror
	// runs on every pull request, and a suite that skipped there proved nothing
	// about the image most installations run.
	return setupBare(t)
}

// setupEE is the fixture for a test that exercises an Enterprise feature
// (several organisations, ACME, a git location, access hours, more than three
// configurations). The community image refuses those BY DESIGN, and its
// refusals are asserted instead (edition_test.go, pagelayout_ee_test.go).
func setupEE(t *testing.T) fixture {
	t.Helper()
	if !edition.Enterprise {
		t.Skip("community build: this test exercises an Enterprise feature")
	}
	return setupBare(t)
}

// setupBare is the same fixture WITHOUT the edition check - what the community
// tests use to assert refusals on an installation that is otherwise identical.
func setupBare(t *testing.T) fixture {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	// And the multi-organisation SHAPE, for the same reason: half of what
	// follows creates organisations, and a single-organisation installation
	// refuses a second one - a mode question, not an API one.
	if err := st.SetTenancy(ctx, store.TenancyMulti); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "root", Username: "root", PasswordHash: "x", Root: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "bob", Username: "bob", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// The data plane and the control plane have SEPARATE session managers, as in
	// production (session.ForAdminPlane) - this is what isolates admin-plane API
	// tokens from data-plane ones.
	sm := session.NewManager(st)
	adminSM := session.NewManager(st, session.ForAdminPlane())
	router := gateway.New(st, sm)
	if err := router.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	api := New(st, adminSM, router)
	// The runtime is not asked from a test: no cluster, and the developer's
	// own Docker is not this gateway's runtime.
	api.LookupInternal = func(context.Context, int) discovery.Internal { return discovery.Internal{} }
	api.LookupPlatform = func(context.Context) discovery.Platform { return discovery.Platform{Name: "binary"} }
	adminMux := http.NewServeMux()
	api.Register(adminMux)

	f := fixture{
		api:      api,
		adminSrv: httptest.NewServer(adminMux),
		appSrv:   httptest.NewServer(router),
		rootC:    issue(t, adminSM, "root"),
		plainC:   issue(t, adminSM, "bob"),
	}
	t.Cleanup(f.adminSrv.Close)
	t.Cleanup(f.appSrv.Close)
	return f
}

func issue(t *testing.T, sm *session.Manager, userID string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(context.Background(), rec, httptest.NewRequest("POST", "/login", nil), userID); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return rec.Result().Cookies()[0]
}

func (f fixture) call(t *testing.T, method, path, body string, cookie *http.Cookie) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.adminSrv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

const validRoute = `{
  "name": "api",
  "order": 1,
  "enabled": true,
  "upstream": "%s",
  "predicates": [{"type": "path", "args": {"patterns": ["/api/**"]}}],
  "filters": [{"type": "strip-prefix", "args": {"parts": 1}}]
}`

func TestAuthz(t *testing.T) {
	f := setup(t)
	if code, _ := f.call(t, "GET", "/api/routes", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d, want 401", code)
	}
	if code, _ := f.call(t, "GET", "/api/routes", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("non-root: %d, want 403", code)
	}
	if code, _ := f.call(t, "GET", "/api/routes", "", f.rootC); code != http.StatusOK {
		t.Fatalf("root: %d, want 200", code)
	}
}

func TestCatalogIsServed(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "GET", "/api/catalog", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("catalog: %d", code)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(body), &entries); err != nil {
		t.Fatalf("catalog not JSON: %v", err)
	}
	kinds := map[string]bool{}
	types := map[string]bool{}
	for _, e := range entries {
		kinds[e["kind"].(string)] = true
		types[e["type"].(string)] = true
	}
	if !kinds["predicate"] || !kinds["filter"] || !types["path"] || !types["strip-prefix"] {
		t.Fatalf("catalog incomplete: %v", body)
	}
}

func TestPutRouteAppliesImmediately(t *testing.T) {
	f := setup(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello from "+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	// Before: the data plane knows nothing.
	res, err := http.Get(f.appSrv.URL + "/api/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("before: %d", res.StatusCode)
	}

	body := strings.Replace(validRoute, "%s", upstream.URL, 1)
	code, out := f.call(t, "PUT", "/api/routes/r1", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("put: %d %s", code, out)
	}

	// After: saving IS applying - no restart, no explicit reload call.
	res, err = http.Get(f.appSrv.URL + "/api/x")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(got) != "hello from /x" {
		t.Fatalf("after: %d %q", res.StatusCode, got)
	}
}

func TestPutInvalidRouteIsRefusedWithEngineError(t *testing.T) {
	f := setup(t)
	bad := `{
	  "name": "broken", "enabled": true, "upstream": "http://up.invalid",
	  "predicates": [{"type": "path", "args": {"pattern": "/x"}}]
	}`
	code, body := f.call(t, "PUT", "/api/routes/r1", bad, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422 (%s)", code, body)
	}
	if !strings.Contains(body, `unknown arg \"pattern\"`) || !strings.Contains(body, "patterns") {
		t.Fatalf("error should carry the engine message: %s", body)
	}
	// Nothing was persisted.
	if _, routes := f.call(t, "GET", "/api/routes", "", f.rootC); routes != "[]\n" {
		t.Fatalf("invalid route persisted: %s", routes)
	}
}

func TestPutRejectsRouteWithoutPredicates(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "PUT", "/api/routes/r1",
		`{"name": "naked", "enabled": true, "upstream": "http://up.invalid", "predicates": []}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "at least one predicate") {
		t.Fatalf("%d %s", code, body)
	}
}

func TestGetAndDeleteRoute(t *testing.T) {
	f := setup(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(upstream.Close)

	body := strings.Replace(validRoute, "%s", upstream.URL, 1)
	if code, out := f.call(t, "PUT", "/api/routes/r1", body, f.rootC); code != http.StatusOK {
		t.Fatalf("put: %d %s", code, out)
	}
	code, out := f.call(t, "GET", "/api/routes/r1", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(out, `"name":"api"`) {
		t.Fatalf("get: %d %s", code, out)
	}

	if code, _ := f.call(t, "DELETE", "/api/routes/r1", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/routes/r1", "", f.rootC); code != http.StatusNotFound {
		t.Fatalf("re-delete: %d, want 404", code)
	}
	// The data plane dropped it too.
	res, err := http.Get(f.appSrv.URL + "/api/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete: %d, want 404", res.StatusCode)
	}
}

func TestPutRejectsUnknownJSONFields(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "PUT", "/api/routes/r1",
		`{"name": "x", "upstrem": "typo"}`, f.rootC)
	if code != http.StatusBadRequest || !strings.Contains(body, "upstrem") {
		t.Fatalf("%d %s", code, body)
	}
}

// The answer to a save is the route as stored: the editor builds its next save
// on it, so an answer carrying the revision it was sent would make the second
// save of the same screen a 409, and one carrying the access rule as typed
// would show what the gateway does not apply.
func TestPutAnswersTheRouteAsStored(t *testing.T) {
	f := setup(t)
	body := strings.Replace(validRoute, "%s", "http://up.invalid", 1)
	body = strings.Replace(body, `"name": "api",`, `"name": "api", "access": {"level": " AUTH ", "tenants": ["t1"], "roles": ["ops"]},`, 1)
	code, out := f.call(t, "PUT", "/api/routes/r1", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, out)
	}
	var saved store.Route
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Rev != 1 {
		t.Fatalf("created route answered at rev %d, want 1", saved.Rev)
	}
	if saved.Access.Level != store.AccessAuth || saved.Access.Tenants != nil || !slices.Equal(saved.Access.Roles, []string{"ops"}) {
		t.Fatalf("access answered as sent, not as stored: %+v", saved.Access)
	}
	// Twice more on the answer alone, as the editor does: each must be taken.
	for i := 2; i <= 3; i++ {
		next, _ := json.Marshal(saved)
		code, out = f.call(t, "PUT", "/api/routes/r1", string(next), f.rootC)
		if code != http.StatusOK {
			t.Fatalf("save %d built on the previous answer: %d %s", i, code, out)
		}
		saved = store.Route{}
		if err := json.Unmarshal([]byte(out), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Rev != int64(i) {
			t.Fatalf("save %d answered rev %d, want %d", i, saved.Rev, i)
		}
	}
}

// A route read with its problem goes back as it was read: the console's enable
// toggle sends the listed route with one field changed, and a field only the
// server writes must not make a broken route the one nobody can touch.
func TestPutTakesBackTheProblemItAnswered(t *testing.T) {
	f := setup(t)
	body := strings.Replace(validRoute, "%s", "http://up.invalid", 1)
	body = strings.Replace(body, `"name": "api",`, `"name": "api", "problem": "references vault entries that hold nothing: x",`, 1)
	code, out := f.call(t, "PUT", "/api/routes/r1", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("put with the read-only problem: %d %s, want 200", code, out)
	}
	if strings.Contains(out, "references vault entries") {
		t.Fatalf("the problem a client sent was kept: %s", out)
	}
}
