package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The oidc mechanism validates the client and the claims; the client's
// existence is the admin API's to check, where there is a store.
func TestValidateOIDCIdentity(t *testing.T) {
	mk := func(client string, claims ...store.OIDCClaim) store.Route {
		return store.Route{Upstream: "http://up", Identity: &store.IdentityForward{
			Mechanism: store.IdentityOIDC, Client: client,
			Attributes: []store.IdentityAttr{{Field: "username"}, {Field: "roles", Expr: `{{.Roles | join ","}}`}},
			Claims:     claims,
		}}
	}
	ok := mk("app",
		store.OIDCClaim{Name: "preferred_username", Field: "username"},
		store.OIDCClaim{Name: "groups", Field: "roles"},
		store.OIDCClaim{Name: "sub", Value: "shared-operator"},
	)
	if err := Validate(ok); err != nil {
		t.Fatalf("a valid oidc identity was refused: %v", err)
	}
	for _, c := range []struct {
		name  string
		route store.Route
		want  string
	}{
		{"no client", mk(""), "needs a client"},
		{"bad claim name", mk("app", store.OIDCClaim{Name: "a b", Value: "x"}), "letters, digits"},
		{"twice", mk("app", store.OIDCClaim{Name: "x", Value: "1"}, store.OIDCClaim{Name: "x", Value: "2"}), "set twice"},
		{"empty", mk("app", store.OIDCClaim{Name: "x"}), "a field of the Identity section or a fixed value"},
		{"both", mk("app", store.OIDCClaim{Name: "x", Field: "username", Value: "v"}), "one or the other"},
		{"undeclared field", mk("app", store.OIDCClaim{Name: "mail", Field: "email"}),
			`field "email", which the Identity section does not declare: declared fields are username, roles`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := Validate(c.route); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want a refusal saying %q", err, c.want)
			}
		})
	}
	// Every reserved claim is refused BY NAME, and the refusal lists them.
	for _, name := range store.OIDCReservedClaims {
		err := Validate(mk("app", store.OIDCClaim{Name: name, Value: "x"}))
		if err == nil || !strings.Contains(err.Error(), `oidc claim "`+name+`" is not allowed`) ||
			!strings.Contains(err.Error(), "Reserved claims are iss, aud") {
			t.Errorf("reserved claim %q: %v", name, err)
		}
	}
	// A client and claims belong to oidc, and an unknown mechanism names the
	// four there are.
	jwt := store.Route{Upstream: "http://up", Identity: &store.IdentityForward{Mechanism: "jwt", Client: "app"}}
	if err := Validate(jwt); err == nil || !strings.Contains(err.Error(), "belong to the oidc mechanism") {
		t.Errorf("a jwt route with a client: %v", err)
	}
	pigeon := store.Route{Upstream: "http://up", Identity: &store.IdentityForward{Mechanism: "pigeon"}}
	if err := Validate(pigeon); err == nil || !strings.Contains(err.Error(), "headers, jwt, signed-jwt, oidc") {
		t.Errorf("unknown mechanism: %v", err)
	}
}

// An oidc route adds nothing to the request and takes nothing away: the
// Authorization header is the application's own access token, sent by its
// browser code to its server, and purging it would sign everybody out of the
// application on every call.
func TestOIDCRouteLeavesAuthorizationAlone(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "auth="+r.Header.Get("Authorization")+";user="+r.Header.Get("username")+
			";remote="+r.Header.Get(RemoteUserHeader))
	}))
	t.Cleanup(upstream.Close)
	route := store.Route{ID: "r", Name: "app", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Identity: &store.IdentityForward{Mechanism: store.IdentityOIDC, Client: "app",
			Attributes: []store.IdentityAttr{{Field: "username"}},
			Claims:     []store.OIDCClaim{{Name: "preferred_username", Field: "username"}}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/things", nil)
	req.Header.Set("Authorization", "Bearer the-application-token")
	req.AddCookie(rec.Result().Cookies()[0])
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if got := string(body); got != "auth=Bearer the-application-token;user=;remote=" {
		t.Fatalf("an oidc route touched the request: %q", got)
	}
	// And the keys its tokens are signed with exist from the first reload.
	if _, ok, err := st.GetSigningSet(ctx); err != nil || !ok {
		t.Errorf("no signing keys for an oidc route: %v", err)
	}
}
