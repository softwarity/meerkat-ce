package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Connecting an agent without anybody copying a secret (MCP-07).
//
// This is the flow Jira and Linear present, and the reason it is worth the
// code: an administrator adds one URL to their agent, a browser opens, they
// approve, and no credential is ever pasted into a file that will end up in a
// repository. What they approve is the PERIMETER - the perimeter did not
// disappear with the minting dialog, it moved to the moment somebody actually
// connects an agent, which is the moment they can judge it.
//
// Meerkat is both halves here: the resource server (the agent endpoint) and
// the authorisation server. That keeps the whole thing inside one port and one
// login - the console's - and it is also the groundwork for SAUTH-05, where
// this same machinery faces applications instead of agents.
//
// Public clients only: an agent is a command-line or desktop program, which
// cannot keep a secret. PKCE and a loopback redirect stand in for one.

const (
	// accessTokenLife is short because a refresh token renews it silently.
	accessTokenLife = 12 * time.Hour
	// refreshTokenLife is what an administrator would call "it keeps working":
	// long, but not forever, and revocable in one click.
	refreshTokenLife = 90 * 24 * time.Hour
	// mcpResourcePath is the endpoint these tokens are for.
	mcpResourcePath = "/mcp"
)

// registerOAuth mounts the authorisation server. Control plane only: the data
// plane serves applications, and an agent that reached this flow there would
// be authorising against the wrong door.
func (h *Handler) registerOAuth(mux *http.ServeMux) {
	// Where a 401 from the agent endpoint sends a client (RFC 9728). Both
	// shapes are served: the bare one, and the one carrying the resource's
	// path, which is what a client built for a path-scoped resource asks for.
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.protectedResource)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", h.protectedResource)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.authorizationServer)
	mux.HandleFunc("POST /oauth/register", h.oauthRegister)
	mux.HandleFunc("GET /oauth/authorize", h.oauthAuthorize)
	mux.HandleFunc("POST /oauth/authorize", h.oauthApprove)
	mux.HandleFunc("POST /oauth/token", h.oauthToken)
	mux.HandleFunc("POST /oauth/revoke", h.oauthRevoke)
}

// agentEndpointOpen mirrors the switch the agent endpoint itself reads: with
// it off there is nothing to authorise, and an authorisation server answering
// for a door that does not open would be a puzzle rather than a refusal.
func (h *Handler) agentEndpointOpen(r *http.Request) bool {
	var enabled bool
	_ = h.st.GetSetting(r.Context(), store.SettingAgentEnabled, &enabled)
	return enabled
}

// origin is this control plane as the caller reached it: whatever address an
// administrator's browser and their agent use is, by construction, the one
// every url in this flow must be rooted in.
//
// It is built from the REQUEST, host header included, which is what makes the
// flow work behind a proxy and on any address without a setting to keep in
// step. The price is that a caller can make these documents describe a
// gateway at an address of their choosing - in the answer to THEIR OWN
// request, which teaches them nothing, unless something between us caches it
// and serves that answer to somebody else. Hence no-store on both documents:
// they are cheap to compute and they must never be shared.
//
// X-Forwarded-Proto is trusted for the same reason: without it a control plane
// behind a TLS-terminating proxy would advertise http urls and no agent would
// connect. A proxy that forwards a header a client sent is a proxy that has to
// be fixed, here as everywhere else.
func origin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (h *Handler) protectedResource(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"resource":                 origin(r) + mcpResourcePath,
		"authorization_servers":    []string{origin(r)},
		"bearer_methods_supported": []string{"header"},
	})
}

func (h *Handler) authorizationServer(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	base := origin(r)
	w.Header().Set("Cache-Control", "no-store")
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"revocation_endpoint":                   base + "/oauth/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		// S256 only. "plain" exists in the specification and protects nothing.
		"code_challenge_methods_supported": []string{"S256"},
		"scopes_supported":                 []string{"mcp:read", "mcp:write"},
	})
}

// oauthRegister is dynamic client registration (RFC 7591). It is open, as it
// is everywhere this flow is used: registering yields nothing but a name to be
// asked about on the consent page, and what protects the gateway is the login
// and the approval that follow, not the registration.
func (h *Handler) oauthRegister(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	var body struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration could not be read")
		return
	}
	client := store.OAuthClient{
		ID: randomID(), Name: body.ClientName, RedirectURIs: body.RedirectURIs,
	}
	if err := h.st.SaveOAuthClient(r.Context(), client); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
		return
	}
	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"client_id":                  client.ID,
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

// consentData is the approval page.
type consentData struct {
	flowChrome
	ClientName string
	Query      template.URL // the request, carried through the form untouched
	Scopes     []consentChoice
	Domains    []consentChoice
}

type consentChoice struct {
	Value, Label, Detail string
	Checked              bool
}

var consentPage = flowPage("oauth-consent", consentBody)

// oauthAuthorize shows the approval page, after making the caller sign in.
//
// Root only, exactly as minting a control-plane token is: whoever approves is
// handing an agent their own powers, and only root should be able to do that.
func (h *Handler) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	// WHO, before WHAT. This ran fourth, so a stranger's parameters were
	// parsed and judged before anybody asked who was asking - and redirectError
	// below answers with a 302 to the client's own redirect_uri, which an
	// unauthenticated caller could therefore set going. The page hands an
	// agent the approver's powers over the whole gateway; nothing about it
	// should move before the approver is known.
	if _, _, ok := h.consentActor(w, r); !ok {
		return
	}
	q := r.URL.Query()
	client, redirect, ok := h.checkAuthorizeRequest(w, r)
	if !ok {
		return
	}
	// PKCE is mandatory: without it a stolen code is a working credential.
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		redirectError(w, r, redirect, q.Get("state"), "invalid_request",
			"this server requires PKCE with S256")
		return
	}
	if rt := q.Get("response_type"); rt != "code" {
		redirectError(w, r, redirect, q.Get("state"), "unsupported_response_type",
			"only the authorization code flow is supported")
		return
	}
	data := consentData{
		// The wide form: this page is two sets of choices with their
		// explanations, not a login box.
		flowChrome: h.flowData(r, "titleConnectAgent"),
		ClientName: client.Name,
		Query:      template.URL(r.URL.RawQuery), //nolint:gosec // re-parsed, never dereferenced
		Scopes: []consentChoice{
			{Value: store.ScopeReadOnly, Label: h.tr(r, "consentReadOnly"), Detail: h.tr(r, "consentReadOnlyDetail"), Checked: true},
			{Value: store.ScopeFull, Label: h.tr(r, "consentFull"), Detail: h.tr(r, "consentFullDetail")},
		},
		Domains: []consentChoice{
			{Value: store.DomainAll, Label: h.tr(r, "consentEverything"), Checked: true},
			{Value: store.DomainGateway, Label: h.tr(r, "consentGateway")},
			{Value: store.DomainApp, Label: h.tr(r, "consentApp")},
		},
	}
	data.List = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = consentPage.Execute(w, data)
}

// oauthApprove turns an approval into a code, or a refusal into a redirect
// that says so - a client left waiting on a page nobody answered is the worst
// of the three outcomes.
func (h *Handler) oauthApprove(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	// The original request travels through the form: this handler must judge
	// exactly what the page showed, not what a second visit would resolve to.
	asked, err := url.ParseQuery(r.PostFormValue("request"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r2 := r.Clone(r.Context())
	r2.URL = &url.URL{Path: "/oauth/authorize", RawQuery: asked.Encode()}
	client, redirect, ok := h.checkAuthorizeRequest(w, r2)
	if !ok {
		return
	}
	state := asked.Get("state")
	_, user, ok := h.consentActor(w, r)
	if !ok {
		return
	}
	if r.PostFormValue("approve") != "yes" {
		redirectError(w, r, redirect, state, "access_denied", "the request was refused")
		return
	}
	scope, err := store.SanitizeTokenScope(r.PostFormValue("scope"))
	if err != nil {
		redirectError(w, r, redirect, state, "invalid_scope", err.Error())
		return
	}
	domain, err := store.SanitizeTokenDomain(r.PostFormValue("domain"))
	if err != nil {
		redirectError(w, r, redirect, state, "invalid_scope", err.Error())
		return
	}
	code, err := randToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.st.SaveOAuthCode(r.Context(), store.OAuthCode{
		Hash: hashTrust(code), ClientID: client.ID, UserID: user.ID,
		RedirectURI: redirect, Challenge: asked.Get("code_challenge"),
		Scope: scope, Domain: domain, Resource: asked.Get("resource"),
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	back := *redirectURL(redirect)
	q := back.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	back.RawQuery = q.Encode()
	http.Redirect(w, r, back.String(), http.StatusSeeOther)
}

// checkAuthorizeRequest resolves the client and the redirect it may come back
// on. Anything wrong with EITHER is answered on this page and never by a
// redirect: sending an error to an address we have not verified is how an
// open redirector is built.
func (h *Handler) checkAuthorizeRequest(w http.ResponseWriter, r *http.Request) (store.OAuthClient, string, bool) {
	q := r.URL.Query()
	client, err := h.st.GetOAuthClient(r.Context(), q.Get("client_id"))
	if err != nil {
		http.Error(w, "unknown client: register first", http.StatusBadRequest)
		return client, "", false
	}
	asked := q.Get("redirect_uri")
	if asked == "" && len(client.RedirectURIs) == 1 {
		asked = client.RedirectURIs[0]
	}
	for _, registered := range client.RedirectURIs {
		if store.SameRedirect(registered, asked) {
			return client, asked, true
		}
	}
	http.Error(w, "this redirect uri is not one this client registered", http.StatusBadRequest)
	return client, "", false
}

// consentActor requires a signed-in root, sending anyone else through the
// console's own login first.
func (h *Handler) consentActor(w http.ResponseWriter, r *http.Request) (store.Session, store.User, bool) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		// Back here after the login, with the request intact.
		next := r.URL.RequestURI()
		if r.Method == http.MethodPost {
			next = "/oauth/authorize?" + r.PostFormValue("request")
		}
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
		return sess, store.User{}, false
	}
	user, err := h.st.GetUserByID(r.Context(), sess.UserID)
	if err != nil || !user.Enabled {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return sess, user, false
	}
	if !user.Root {
		// Approving hands an agent the approver's own powers over the whole
		// gateway. That is a root decision, as minting a control-plane token
		// has always been.
		http.Error(w, "connecting an agent requires the root account", http.StatusForbidden)
		return sess, user, false
	}
	return sess, user, true
}

func (h *Handler) oauthToken(w http.ResponseWriter, r *http.Request) {
	if !h.agentEndpointOpen(r) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the form could not be read")
		return
	}
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		h.exchangeCode(w, r)
	case "refresh_token":
		h.refreshToken(w, r)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type",
			"supported grants are authorization_code and refresh_token")
	}
}

func (h *Handler) exchangeCode(w http.ResponseWriter, r *http.Request) {
	code, err := h.st.TakeOAuthCode(r.Context(), hashTrust(r.PostFormValue("code")))
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	if code.ClientID != r.PostFormValue("client_id") {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "this code was issued to another client")
		return
	}
	if !pkceMatches(code.Challenge, r.PostFormValue("code_verifier")) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the PKCE verifier does not match")
		return
	}
	client, err := h.st.GetOAuthClient(r.Context(), code.ClientID)
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client", "unknown client")
		return
	}
	access, hash, prefix, err := mintAgentToken()
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	id := randomID()
	if err := h.st.AddAPIToken(r.Context(), store.NewToken{
		ID: id, UserID: code.UserID, Name: client.Name, TokenHash: hash, Prefix: prefix,
		Plane: store.PlaneAdmin, Scope: code.Scope, Domain: code.Domain, ClientID: client.ID,
		ExpiresAt: time.Now().Add(accessTokenLife).Unix(),
	}); err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	h.issueTokenResponse(w, r, id, access)
}

func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	id, err := h.st.TakeRefreshToken(r.Context(), hashTrust(r.PostFormValue("refresh_token")))
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	access, hash, prefix, err := mintAgentToken()
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	// The SAME row keeps its identity: its name, its perimeter, its place in
	// the list of connected agents. A refresh is one connection with a new
	// key, not a second connection.
	if err := h.st.RenewAPIToken(r.Context(), id, hash, prefix, time.Now().Add(accessTokenLife).Unix()); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	h.issueTokenResponse(w, r, id, access)
}

// issueTokenResponse pairs a fresh access token with a fresh refresh token.
func (h *Handler) issueTokenResponse(w http.ResponseWriter, r *http.Request, tokenID, access string) {
	refresh, err := randToken()
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	if err := h.st.SaveRefreshToken(r.Context(), hashTrust(refresh), tokenID,
		time.Now().Add(refreshTokenLife).Unix()); err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenLife.Seconds()),
		"refresh_token": refresh,
	})
}

// oauthRevoke lets a client hand its own credentials back (RFC 7009). It
// answers 200 whatever happens, as the specification asks: a caller learns
// nothing from being told a token it no longer holds was already unknown.
func (h *Handler) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		token := r.PostFormValue("token")
		if id, err := h.st.TakeRefreshToken(r.Context(), hashTrust(token)); err == nil {
			_ = h.st.DeleteAPIToken(r.Context(), id)
		} else if tok, err := h.st.ResolveAPIToken(r.Context(), hashTrust(token), time.Now().Unix()); err == nil {
			_ = h.st.DeleteAPIToken(r.Context(), tok.ID)
		}
	}
	w.WriteHeader(http.StatusOK)
}

// pkceMatches checks the S256 challenge against the verifier.
func pkceMatches(challenge, verifier string) bool {
	if challenge == "" || verifier == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}

// mintAgentToken produces an access token shaped exactly like a hand-minted
// one: it IS one, which is what lets the perimeter, the audit and the
// revocation work on it without a second machinery.
func mintAgentToken() (secret, hash, prefix string, err error) {
	value, err := randToken()
	if err != nil {
		return "", "", "", err
	}
	secret = apiTokenClearPrefix + value
	return secret, hashTrust(secret), secret[:12], nil
}

// redirectURL parses a redirect already validated against the registration.
func redirectURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return &url.URL{Path: "/"}
	}
	return u
}

// redirectError sends the failure to the client rather than showing it to a
// person: the agent is the one waiting, and it can say what happened.
func redirectError(w http.ResponseWriter, r *http.Request, redirect, state, code, detail string) {
	back := *redirectURL(redirect)
	q := back.Query()
	q.Set("error", code)
	q.Set("error_description", detail)
	if state != "" {
		q.Set("state", state)
	}
	back.RawQuery = q.Encode()
	http.Redirect(w, r, back.String(), http.StatusSeeOther)
}

func oauthError(w http.ResponseWriter, status int, code, detail string) {
	writeJSONResponse(w, status, map[string]any{"error": code, "error_description": detail})
}

func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// consentBody is the approval page: who is asking, and the two questions that
// decide what it may do.
const consentBody = `
<form method="post" action="/oauth/authorize" class="watch consent">
  <input type="hidden" name="request" value="{{.Query}}">
  <h1>{{.T.titleConnectAgent}}</h1>
  <p class="lead">{{.T.consentIntro}} <strong>{{.ClientName}}</strong></p>

  <fieldset>
    <legend>{{.T.consentWhatMay}}</legend>
    {{range .Scopes}}
    <label class="choice">
      <input type="radio" name="scope" value="{{.Value}}"{{if .Checked}} checked{{end}}>
      <span><strong>{{.Label}}</strong><small>{{.Detail}}</small></span>
    </label>
    {{end}}
  </fieldset>

  <fieldset>
    <legend>{{.T.consentOverWhat}}</legend>
    {{range .Domains}}
    <label class="choice">
      <input type="radio" name="domain" value="{{.Value}}"{{if .Checked}} checked{{end}}>
      <span><strong>{{.Label}}</strong></span>
    </label>
    {{end}}
  </fieldset>

  <div class="actions">
    <button type="submit" name="approve" value="no" class="ghost">{{.T.consentDeny}}</button>
    <button type="submit" name="approve" value="yes">{{.T.consentApprove}}</button>
  </div>
  <p class="note">{{.T.consentRevoke}}</p>
</form>
`
