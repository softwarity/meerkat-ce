package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// setupAgentFlow is a control plane with the agent endpoint open and a root
// account signed in - the state an administrator is in when they connect an
// agent.
func setupAgentFlow(t *testing.T) (*httptest.Server, *store.Store, *http.Cookie) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "root", Username: "root", PasswordHash: "x", Enabled: true, Root: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingAgentEnabled, true); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st, session.ForAdminPlane())
	mux := http.NewServeMux()
	NewAdmin(st, sm).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "root"); err != nil {
		t.Fatal(err)
	}
	return srv, st, rec.Result().Cookies()[0]
}

func getJSON(t *testing.T, url string, into any) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET %s: %d %s", url, res.StatusCode, body)
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

// The whole flow an agent walks, in the order it walks it (MCP-07). The point
// of testing it end to end rather than endpoint by endpoint: what matters is
// that a client which knows nothing but the url arrives at a working token,
// and every step here is one a real client actually takes.
func TestAnAgentConnectsWithoutACopiedSecret(t *testing.T) {
	srv, st, cookie := setupAgentFlow(t)
	ctx := context.Background()

	// 1. Discovery, from the 401 the agent endpoint answers to a stranger.
	var resource struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	getJSON(t, srv.URL+"/.well-known/oauth-protected-resource", &resource)
	if resource.Resource != srv.URL+"/mcp" || len(resource.AuthorizationServers) != 1 {
		t.Fatalf("the resource metadata does not name this gateway: %+v", resource)
	}
	var meta struct {
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		RegistrationEndpoint  string   `json:"registration_endpoint"`
		Methods               []string `json:"code_challenge_methods_supported"`
	}
	getJSON(t, resource.AuthorizationServers[0]+"/.well-known/oauth-authorization-server", &meta)
	if len(meta.Methods) != 1 || meta.Methods[0] != "S256" {
		t.Fatalf("PKCE S256 must be the only method offered: %v", meta.Methods)
	}

	// 2. The agent registers itself. No secret comes back: a command-line
	// program cannot keep one.
	body := `{"client_name":"Claude Code","redirect_uris":["http://127.0.0.1:51713/callback"]}`
	res, err := http.Post(meta.RegistrationEndpoint, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		ClientID string `json:"client_id"`
		Secret   string `json:"client_secret"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reg); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if reg.ClientID == "" || reg.Secret != "" {
		t.Fatalf("registration answered %+v", reg)
	}

	// 3. The consent page, in the browser of whoever is connecting it. The
	// PORT of the loopback redirect is different from the registered one on
	// purpose: a CLI is handed a free port every time it runs.
	verifier := "a-verifier-long-enough-to-be-one-0123456789"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	ask := url.Values{
		"response_type":         {"code"},
		"client_id":             {reg.ClientID},
		"redirect_uri":          {"http://127.0.0.1:60001/callback"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"xyz"},
		"resource":              {srv.URL + "/mcp"},
	}
	page := fetchPage(t, srv.URL+"/oauth/authorize?"+ask.Encode(), cookie)
	if !strings.Contains(page, "Claude Code") {
		t.Fatalf("the consent page must name who is asking:\n%s", page)
	}
	if !strings.Contains(page, `name="scope"`) || !strings.Contains(page, `name="domain"`) {
		t.Fatalf("the consent page must ask for the perimeter:\n%s", page)
	}

	// 4. Approving, with a perimeter chosen on that page.
	form := url.Values{
		"request": {ask.Encode()},
		"approve": {"yes"},
		"scope":   {store.ScopeFull},
		"domain":  {store.DomainGateway},
	}
	location := postForm(t, srv.URL+"/oauth/authorize", form, cookie)
	back, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	if back.Query().Get("state") != "xyz" {
		t.Errorf("the state must come back untouched: %s", location)
	}
	code := back.Query().Get("code")
	if code == "" {
		t.Fatalf("no code came back: %s", location)
	}

	// 5. A wrong verifier is refused: that is the whole protection of a client
	// that holds no secret.
	if _, err := exchange(t, srv.URL+meta.TokenEndpoint[len(srv.URL):], url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"client_id": {reg.ClientID}, "code_verifier": {"not-the-verifier"},
	}); err == nil {
		t.Error("a wrong PKCE verifier was accepted")
	}
	// And the code is gone with it - spent once, whatever the outcome.
	if _, err := exchange(t, meta.TokenEndpoint, url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"client_id": {reg.ClientID}, "code_verifier": {verifier},
	}); err == nil {
		t.Error("an authorisation code was accepted twice")
	}

	// 6. So: approve again, and exchange properly.
	location = postForm(t, srv.URL+"/oauth/authorize", form, cookie)
	back, _ = url.Parse(location)
	tokens, err := exchange(t, meta.TokenEndpoint, url.Values{
		"grant_type": {"authorization_code"}, "code": {back.Query().Get("code")},
		"client_id": {reg.ClientID}, "code_verifier": {verifier},
	})
	if err != nil {
		t.Fatalf("exchanging the code: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != "Bearer" {
		t.Fatalf("the token response is incomplete: %+v", tokens)
	}

	// 7. What came out is an ordinary control-plane token, carrying the
	// perimeter that was approved on the page - which is what lets everything
	// already built work on it unchanged.
	list, err := st.ListAPITokens(ctx, "root", store.PlaneAdmin)
	if err != nil || len(list) != 1 {
		t.Fatalf("the connection must show as one token: %v %d", err, len(list))
	}
	got := list[0]
	if got.Name != "Claude Code" || got.ClientID != reg.ClientID {
		t.Errorf("the token must name the agent: %+v", got)
	}
	if got.Scope != store.ScopeFull || got.Domain != store.DomainGateway {
		t.Errorf("the approved perimeter was not carried: scope=%q domain=%q", got.Scope, got.Domain)
	}

	// 8. Refreshing keeps ONE connection with a new key, rather than piling up
	// a second one every twelve hours.
	renewed, err := exchange(t, meta.TokenEndpoint, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken},
		"client_id": {reg.ClientID},
	})
	if err != nil {
		t.Fatalf("refreshing: %v", err)
	}
	if renewed.AccessToken == tokens.AccessToken {
		t.Error("a refresh must hand over a new access token")
	}
	if list, _ := st.ListAPITokens(ctx, "root", store.PlaneAdmin); len(list) != 1 {
		t.Errorf("refreshing made %d connections, want 1", len(list))
	}
	// The old refresh token is spent: presented twice, it means one of the two
	// presenters stole it.
	if _, err := exchange(t, meta.TokenEndpoint, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken},
	}); err == nil {
		t.Error("a refresh token was accepted twice")
	}
}

// Approving hands an agent the approver's own powers over the whole gateway.
// That has always been a root decision here, and the flow does not become a
// way around it.
func TestConnectingAnAgentIsRootOnly(t *testing.T) {
	srv, st, _ := setupAgentFlow(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "bob", Username: "bob", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st, session.ForAdminPlane())
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "bob"); err != nil {
		t.Fatal(err)
	}
	client := store.OAuthClient{ID: "c1", Name: "An agent", RedirectURIs: []string{"http://127.0.0.1:1/cb"}}
	if err := st.SaveOAuthClient(ctx, client); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/oauth/authorize?response_type=code&client_id=c1&code_challenge=x&code_challenge_method=S256", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("a non-root approver answered %d, want 403", res.StatusCode)
	}
}

// A redirect nobody registered is refused ON THE PAGE and never by a redirect:
// sending anything to an unverified address is how an open redirector is made.
func TestAnUnregisteredRedirectIsRefusedInPlace(t *testing.T) {
	srv, st, cookie := setupAgentFlow(t)
	if err := st.SaveOAuthClient(context.Background(), store.OAuthClient{
		ID: "c1", Name: "An agent", RedirectURIs: []string{"http://127.0.0.1:5000/cb"},
	}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", srv.URL+
		"/oauth/authorize?response_type=code&client_id=c1&redirect_uri=https://evil.example/cb&code_challenge=x&code_challenge_method=S256", nil)
	req.AddCookie(cookie)
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("an unregistered redirect answered %d, want 400 in place", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); loc != "" {
		t.Errorf("nothing must be sent to an unverified address, got a redirect to %s", loc)
	}
}

// With the switch off there is nothing to authorise, and the flow says so
// rather than offering a door that does not open.
func TestTheFlowIsClosedWithTheEndpoint(t *testing.T) {
	srv, st, _ := setupAgentFlow(t)
	if err := st.SetSetting(context.Background(), store.SettingAgentEnabled, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-authorization-server",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("%s answered %d with the endpoint off, want 404", path, res.StatusCode)
		}
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func exchange(t *testing.T, endpoint string, form url.Values) (tokenResponse, error) {
	t.Helper()
	res, err := http.PostForm(endpoint, form)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		return out, &oauthFailure{out.Error}
	}
	return out, nil
}

type oauthFailure struct{ code string }

func (e *oauthFailure) Error() string { return "oauth: " + e.code }

func fetchPage(t *testing.T, url string, cookie *http.Cookie) string {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", url, res.StatusCode, body)
	}
	return string(body)
}

// postForm posts and returns the Location it was redirected to, without
// following it: the redirect target is a loopback port nobody is listening on.
func postForm(t *testing.T, url string, form url.Values, cookie *http.Cookie) string {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST %s: %d %s", url, res.StatusCode, body)
	}
	return res.Header.Get("Location")
}

// A chooser is a dead end for somebody who signed in as the wrong person: the
// only exit used to be clearing a cookie by hand. Both steps that ask a
// question now offer the way out.
func TestTheChoosersOfferAWayOut(t *testing.T) {
	mux, sm, st := setupFlow(t)
	ctx := context.Background()
	for _, id := range []string{"acme", "globex"} {
		if err := st.SaveTenant(ctx, store.Tenant{ID: id, Name: id, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveMembership(ctx, store.Membership{
			UserID: "u1", TenantID: id, Type: store.MemberUser, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Result().Cookies()[0]

	page := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/select-tenant", nil)
	req.AddCookie(cookie)
	mux.ServeHTTP(page, req)
	if page.Code != http.StatusOK {
		t.Fatalf("the chooser answered %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), `action="logout"`) {
		t.Errorf("the organisation chooser offers no way out:\n%s", page.Body.String())
	}

	// And it works from there: the session is gone and the way back is the
	// sign-in page.
	out := httptest.NewRecorder()
	logout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logout.AddCookie(cookie)
	mux.ServeHTTP(out, logout)
	if out.Code != http.StatusSeeOther || out.Header().Get("Location") != "/login" {
		t.Fatalf("signing out from the chooser answered %d %s", out.Code, out.Header().Get("Location"))
	}
	after := httptest.NewRecorder()
	again := httptest.NewRequest(http.MethodGet, "/select-tenant", nil)
	again.AddCookie(cookie)
	mux.ServeHTTP(after, again)
	if after.Code != http.StatusSeeOther {
		t.Errorf("the session survived the sign-out: %d", after.Code)
	}
}

// The apps menu lists PLACES, not routes. An installation commonly fronts one
// product with several routes - one per organisation, one per version - which
// differ in what they proxy and never in where you go. Listing them twice
// offered a choice that is not one, and ticked both, since the tick is matched
// on the entry path and both carried it.
func TestTheAppsMenuListsAPlaceOnce(t *testing.T) {
	_, sm, st := setupFlow(t)
	ctx := context.Background()
	h := New(st, sm)

	ui := func(id, link string, order int, access store.Access) store.Route {
		return store.Route{
			ID: id, Name: id, Order: order, Enabled: true, IsUI: true,
			Upstream:   "http://x.invalid",
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
			Access:     access,
			UI:         &store.RouteUI{Link: link},
		}
	}
	// The shape that produced the double entry: two routes on the same path,
	// same label, both reachable by this caller.
	for _, r := range []store.Route{
		ui("httpbin-acme", "httpbin", 5, store.Access{}),
		ui("httpbin-globex", "httpbin", 6, store.Access{}),
		// A different place is still its own entry.
		func() store.Route {
			r := ui("sales", "sales", 7, store.Access{})
			r.Predicates = []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/sales/**"}}}}
			return r
		}(),
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	links := h.reachableLinks(ctx, store.Session{UserID: "u1"})
	var places []string
	for _, l := range links {
		places = append(places, l.Name+" -> "+l.Href)
	}
	if len(links) != 2 {
		t.Fatalf("the menu offers %v, want one entry per place", places)
	}
	if links[0].Href != "/" || links[1].Href != "/sales" {
		t.Errorf("the menu offers %v", places)
	}
}
