package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/oidcprovider"
	"github.com/softwarity/meerkat/internal/store"
)

// The OIDC clients (SAUTH-05): the applications behind the routes that sign
// their people in against this gateway as an OpenID provider. Declared here,
// in the drawer that already holds the issuer and the signing keys, because
// the three go together: a client is useless without an issuer its
// application can discover, and its tokens are signed with those keys.
//
// The provider itself is Enterprise and lives under ee/. What lives here is
// the declaration: on the community image the list can be read and a client
// deleted - a configuration or a database may have brought some over - but
// none can be declared or changed, and no secret drawn.

// oidcEdition is the community image's refusal, said once.
const oidcEdition = "The OIDC provider"

func (a *API) registerOIDC(mux Mux) {
	mux.Handle("GET /api/oidc/clients", a.infraAdmin(a.listOIDCClients))
	mux.Handle("POST /api/oidc/clients", a.infraAdmin(a.postOIDCClient))
	mux.Handle("PUT /api/oidc/clients/{id}", a.infraAdmin(a.putOIDCClient))
	mux.Handle("POST /api/oidc/clients/{id}/secret", a.infraAdmin(a.postOIDCSecret))
	mux.Handle("DELETE /api/oidc/clients/{id}", a.infraAdmin(a.deleteOIDCClient))
	// A sign-in tried on someone's behalf: what the application would get.
	mux.Handle("POST /api/oidc/clients/{id}/try", a.infraAdmin(a.postOIDCTry))
}

// oidcTryIn is who a sign-in is tried for: an account, by id or username,
// and the organisation the session would be opened in ("" adopts the only
// membership, as sign-in does).
type oidcTryIn struct {
	User   string `json:"user"`
	Tenant string `json:"tenant"`
}

func (a *API) postOIDCTry(w http.ResponseWriter, r *http.Request, _ store.User) {
	var in oidcTryIn
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed trial: "+err.Error()+` - send {"user": "...", "tenant": "..."}`)
		return
	}
	out, err := a.tryOIDCSignIn(r.Context(), r.PathValue("id"), in)
	if err != nil {
		a.writeOIDCErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// tryOIDCSignIn resolves the person as their session would be, then asks the
// provider what it would issue: the route that lets them in, those that do
// not, and the claims. Nothing is signed, stored or sent. Shared by the
// endpoint and the agent's tool.
func (a *API) tryOIDCSignIn(ctx context.Context, clientID string, in oidcTryIn) (oidcprovider.Trial, error) {
	if err := edition.Require(oidcEdition); err != nil {
		return oidcprovider.Trial{}, invalidErr(err)
	}
	client, err := a.st.GetOIDCClient(ctx, clientID)
	if err != nil {
		return oidcprovider.Trial{}, err
	}
	ref := strings.TrimSpace(in.User)
	if ref == "" {
		return oidcprovider.Trial{}, invalidErr(errors.New("whom to try it for: pass an account, by id or username"))
	}
	u, err := a.st.GetUserByID(ctx, ref)
	if err != nil {
		if u, err = a.st.GetUserByUsername(ctx, ref); err != nil {
			return oidcprovider.Trial{}, invalidErr(fmt.Errorf("no account %q, by id or by username", ref))
		}
	}
	tenantID := ""
	if t := strings.TrimSpace(in.Tenant); t != "" {
		if tenantID, err = a.tenantRef(ctx, t); err != nil {
			return oidcprovider.Trial{}, invalidErr(err)
		}
	}
	if a.router == nil || a.OIDC == nil {
		return oidcprovider.Trial{}, errors.New("the application plane is not reachable from this control plane")
	}
	person, err := a.router.PersonFor(ctx, u.ID, tenantID)
	if err != nil {
		return oidcprovider.Trial{}, invalidErr(err)
	}
	trial, err := a.OIDC.Try(ctx, client, person)
	if err != nil {
		return oidcprovider.Trial{}, invalidErr(err)
	}
	return trial, nil
}

// oidcRouteRef is a route that signs people into a client.
type oidcRouteRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// oidcClientView is a client as the console sees it: never its secret's hash,
// and the routes bound to it - what deleting it would break, and where its
// access rules are.
//
// AcceptedUnder is where the provider accepts a redirect_uri
// (https://host/prefix/), read from the enabled routes bound to it by the
// provider's own function: a client declares none. WholeHostRoutes names those of them
// covering every path of their host: any address there may receive a code,
// which the console says rather than leaves to be found out.
type oidcClientView struct {
	store.OIDCClient
	Routes          []oidcRouteRef `json:"routes"`
	AcceptedUnder   []string       `json:"acceptedUnder"`
	WholeHostRoutes []string       `json:"wholeHostRoutes"`
}

// oidcClientIssued is what declaring a client or drawing its secret answers:
// the secret in clear, this once. Never again - only its hash is kept. Empty
// when the secret was the application's: the caller has it already, and
// sending it back would only be one more place it is written.
type oidcClientIssued struct {
	Client oidcClientView `json:"client"`
	Secret string         `json:"secret,omitempty"`
}

// oidcClientIn is what a write accepts. Confidential is a pointer because its
// absence means something: true for a new client (the console's default), the
// value it had for an existing one. The read-only fields of a view are
// accepted and ignored, so a console may send back what it read.
type oidcClientIn struct {
	ID           string `json:"id"`
	Confidential *bool  `json:"confidential"`
	// RedirectURIs is refused when it names an address (checkNoRedirectURIs):
	// a client declares none any more, and saying so beats dropping it.
	RedirectURIs []string `json:"redirectUris"`
	Audience     string   `json:"audience"`
	TokenTTL     string   `json:"tokenTtl"`
	// Secret is the application's own, when it imposes one (its chart
	// generates it, its vendor fixed it): kept as a hash, never drawn. Empty
	// on a new confidential client: one is drawn. Only on a creation - a
	// client's secret changes through POST /api/oidc/clients/{id}/secret.
	Secret       string          `json:"secret"`
	SecretPrefix string          `json:"secretPrefix"`
	CreatedAt    int64           `json:"createdAt"`
	UpdatedAt    int64           `json:"updatedAt"`
	Routes       json.RawMessage `json:"routes"`
	// Read-only, derived from the routes.
	AcceptedUnder   json.RawMessage `json:"acceptedUnder"`
	WholeHostRoutes json.RawMessage `json:"wholeHostRoutes"`
}

// errOIDCClientNotFound and oidcInUse are the two refusals that are not the
// caller's typing: the client is not there (404), or routes still need it
// (409).
var errOIDCClientNotFound = errors.New("OIDC client not found")

type oidcInUse struct{ msg string }

func (e oidcInUse) Error() string { return e.msg }

// oidcViews binds each client to its routes, in routing order, reading the
// routes once.
func (a *API) oidcViews(ctx context.Context, clients []store.OIDCClient) ([]oidcClientView, error) {
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	bound := map[string][]oidcRouteRef{}
	for _, r := range routes {
		if r.Identity != nil && r.Identity.Mechanism == store.IdentityOIDC {
			bound[r.Identity.Client] = append(bound[r.Identity.Client], oidcRouteRef{ID: r.ID, Name: r.Name})
		}
	}
	out := make([]oidcClientView, 0, len(clients))
	for _, c := range clients {
		refs := bound[c.ID]
		if refs == nil {
			refs = []oidcRouteRef{}
		}
		view := oidcClientView{OIDCClient: c, Routes: refs, AcceptedUnder: []string{}, WholeHostRoutes: []string{}}
		for _, ret := range oidcprovider.Returns(a.router, c.ID) {
			if p := ret.String(); !slices.Contains(view.AcceptedUnder, p) {
				view.AcceptedUnder = append(view.AcceptedUnder, p)
			}
			if ret.WholeHost() && !slices.Contains(view.WholeHostRoutes, ret.Route) {
				view.WholeHostRoutes = append(view.WholeHostRoutes, ret.Route)
			}
		}
		out = append(out, view)
	}
	return out, nil
}

func (a *API) oidcView(ctx context.Context, c store.OIDCClient) (oidcClientView, error) {
	views, err := a.oidcViews(ctx, []store.OIDCClient{c})
	if err != nil {
		return oidcClientView{}, err
	}
	return views[0], nil
}

func (a *API) listOIDCClients(w http.ResponseWriter, r *http.Request, _ store.User) {
	clients, err := a.st.ListOIDCClients(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	views, err := a.oidcViews(r.Context(), clients)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

// writeOIDCErr answers a refusal with its status: what is missing, what is
// still in use, what the caller got wrong, or what broke.
func (a *API) writeOIDCErr(w http.ResponseWriter, err error) {
	var inUse oidcInUse
	switch {
	case errors.Is(err, errOIDCClientNotFound), errors.Is(err, store.ErrNoOIDCClient):
		writeErr(w, http.StatusNotFound, errOIDCClientNotFound.Error())
	case errors.As(err, &inUse):
		writeErr(w, http.StatusConflict, err.Error())
	case isInvalid(err):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	default:
		a.internal(w, err)
	}
}

func (a *API) postOIDCClient(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in oidcClientIn
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed OIDC client: "+err.Error())
		return
	}
	out, err := a.createOIDCClient(r.Context(), actor, in)
	if err != nil {
		a.writeOIDCErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) putOIDCClient(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in oidcClientIn
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed OIDC client: "+err.Error())
		return
	}
	id := r.PathValue("id")
	if in.ID != "" && in.ID != id {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"a client id cannot change: %q is written in the application's configuration. "+
				"Declare a new client and delete this one instead", id))
		return
	}
	in.ID = id
	out, err := a.updateOIDCClient(r.Context(), actor, in, nil)
	if err != nil {
		a.writeOIDCErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// oidcSecretIn is what POST /api/oidc/clients/{id}/secret accepts: nothing,
// or null, to draw a secret; the application's own to keep that one.
type oidcSecretIn struct {
	Secret string `json:"secret"`
}

func (a *API) postOIDCSecret(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in oidcSecretIn
	if err := decodeStrict(r, &in); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "malformed secret: "+err.Error()+` - send nothing to draw one, or {"secret": "..."} to keep the application's`)
		return
	}
	out, err := a.regenerateOIDCSecret(r.Context(), actor, r.PathValue("id"), in.Secret)
	if err != nil {
		a.writeOIDCErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) deleteOIDCClient(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := a.dropOIDCClient(r.Context(), actor, r.PathValue("id")); err != nil {
		a.writeOIDCErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createOIDCClient declares a client and, when it keeps a secret, draws it -
// or keeps the one the application brings.
// Shared by the endpoint and the agent's tool: one set of rules about what a
// client may be.
func (a *API) createOIDCClient(ctx context.Context, actor store.User, in oidcClientIn) (oidcClientIssued, error) {
	if err := edition.Require(oidcEdition); err != nil {
		return oidcClientIssued{}, invalidErr(err)
	}
	if err := checkNoRedirectURIs(in); err != nil {
		return oidcClientIssued{}, err
	}
	c := store.OIDCClient{
		ID: in.ID, Confidential: in.Confidential == nil || *in.Confidential,
		Audience: in.Audience, TokenTTL: in.TokenTTL,
	}
	var secret string
	switch {
	case in.Secret != "" && !c.Confidential:
		return oidcClientIssued{}, invalidErr(fmt.Errorf("client %q is public: it keeps no secret, and proves "+
			"itself with PKCE - make it confidential to give it the application's secret", c.ID))
	case in.Secret != "":
		var err error
		if c.SecretHash, c.SecretPrefix, err = store.OIDCSecretGiven(in.Secret); err != nil {
			return oidcClientIssued{}, err
		}
	case c.Confidential:
		var err error
		if secret, c.SecretHash, c.SecretPrefix, err = store.NewOIDCSecret(); err != nil {
			return oidcClientIssued{}, err
		}
	}
	saved, err := a.st.CreateOIDCClient(ctx, c)
	if err != nil {
		return oidcClientIssued{}, err
	}
	kind := "public"
	if saved.Confidential {
		kind = "confidential"
	}
	if in.Secret != "" {
		kind += ", secret given"
	}
	a.auditEvent(ctx, actor, "oidc.client.create", "oidc-client", saved.ID, saved.ID, "", kind)
	view, err := a.oidcView(ctx, saved)
	if err != nil {
		return oidcClientIssued{}, err
	}
	return oidcClientIssued{Client: view, Secret: secret}, nil
}

// checkNoRedirectURIs refuses a client that names return addresses. An
// application is served behind its routes, and a code goes back under the
// routes bound to its client, nowhere else: an address typed here would be
// taken for one the provider honours, and silently it would not be. An empty
// list is what an older console sends, and says nothing.
func checkNoRedirectURIs(in oidcClientIn) error {
	for _, u := range in.RedirectURIs {
		if strings.TrimSpace(u) != "" {
			return invalidErr(fmt.Errorf("redirectUris: a client declares no return address - the provider "+
				"accepts an address under the routes bound to it, so bind the application's route to the "+
				"client (Auth forward, mechanism oidc) instead of declaring %q", u))
		}
	}
	return nil
}

// updateOIDCClient changes what a client says about itself. given, when not
// nil, names the fields the caller passed (the agent's "what you pass is what
// changes"); nil means the body is the whole client, as the console sends it.
// The secret is never touched here.
func (a *API) updateOIDCClient(ctx context.Context, actor store.User, in oidcClientIn, given map[string]json.RawMessage) (oidcClientView, error) {
	if err := edition.Require(oidcEdition); err != nil {
		return oidcClientView{}, invalidErr(err)
	}
	if err := checkNoRedirectURIs(in); err != nil {
		return oidcClientView{}, err
	}
	if in.Secret != "" {
		return oidcClientView{}, invalidErr(fmt.Errorf("an edit does not change the secret of client %q: "+
			"set it with POST /api/oidc/clients/%s/secret, which says when the old one stops working", in.ID, in.ID))
	}
	old, err := a.st.GetOIDCClient(ctx, in.ID)
	if err != nil {
		return oidcClientView{}, err
	}
	passed := func(k string) bool {
		if given == nil {
			return true
		}
		_, ok := given[k]
		return ok
	}
	c := old
	if in.Confidential != nil {
		c.Confidential = *in.Confidential
	}
	if passed("audience") {
		c.Audience = in.Audience
	}
	if passed("tokenTtl") {
		c.TokenTTL = in.TokenTTL
	}
	saved, err := a.st.UpdateOIDCClient(ctx, c)
	if err != nil {
		return oidcClientView{}, err
	}
	a.auditUpdate(ctx, actor, "oidc.client.update", "oidc-client", saved.ID, saved.ID, "", old, saved)
	return a.oidcView(ctx, saved)
}

// regenerateOIDCSecret draws a new secret for a confidential client, or keeps
// the one given (the application's own): the old one stops working at once,
// which is the point - this is what is done when a secret leaked, when a
// client became confidential, or when the application's secret changed.
func (a *API) regenerateOIDCSecret(ctx context.Context, actor store.User, id, given string) (oidcClientIssued, error) {
	if err := edition.Require(oidcEdition); err != nil {
		return oidcClientIssued{}, invalidErr(err)
	}
	if _, err := a.st.GetOIDCClient(ctx, id); err != nil {
		return oidcClientIssued{}, err
	}
	var secret, hash, prefix string
	var err error
	what := "new secret "
	if given != "" {
		hash, prefix, err = store.OIDCSecretGiven(given)
		what = "secret given "
	} else {
		secret, hash, prefix, err = store.NewOIDCSecret()
	}
	if err != nil {
		return oidcClientIssued{}, err
	}
	if err := a.st.SetOIDCClientSecret(ctx, id, hash, prefix); err != nil {
		return oidcClientIssued{}, err
	}
	saved, err := a.st.GetOIDCClient(ctx, id)
	if err != nil {
		return oidcClientIssued{}, err
	}
	a.auditEvent(ctx, actor, "oidc.client.secret", "oidc-client", id, id, "", what+prefix+"...")
	view, err := a.oidcView(ctx, saved)
	if err != nil {
		return oidcClientIssued{}, err
	}
	return oidcClientIssued{Client: view, Secret: secret}, nil
}

// dropOIDCClient deletes a client, refused while a route still signs people
// into it: the route would keep sending its application to a provider that
// then refuses it, and nothing on the route's screen would say why. The routes
// are named, so the way out is in the refusal.
//
// Allowed on the community image: removing what a database brought over is
// cleaning up, not using the provider.
func (a *API) dropOIDCClient(ctx context.Context, actor store.User, id string) error {
	if _, err := a.st.GetOIDCClient(ctx, id); err != nil {
		return err
	}
	routes, err := a.st.RoutesForOIDCClient(ctx, id)
	if err != nil {
		return err
	}
	if len(routes) > 0 {
		names := make([]string, 0, len(routes))
		for _, r := range routes {
			names = append(names, r.Name)
		}
		return oidcInUse{fmt.Sprintf("OIDC client %q is still used by the route(s) %s: "+
			"give them another client or another mechanism first", id, strings.Join(names, ", "))}
	}
	if err := a.st.DeleteOIDCClient(ctx, id); err != nil {
		return err
	}
	a.auditEvent(ctx, actor, "oidc.client.delete", "oidc-client", id, id, "", "")
	return nil
}

// knownOIDCClient refuses a route naming a client this gateway has not
// declared. Asked where there is a store (validateRoute), since the engine's
// own validation is a pure function on a route.
func (a *API) knownOIDCClient(ctx context.Context, r store.Route) error {
	id := r.Identity
	if id == nil || id.Mechanism != store.IdentityOIDC {
		return nil
	}
	if _, err := a.st.GetOIDCClient(ctx, id.Client); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNoOIDCClient) {
		return err
	}
	clients, err := a.st.ListOIDCClients(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(clients))
	for _, c := range clients {
		ids = append(ids, c.ID)
	}
	declared := "none yet - declare one in OIDC / JWT"
	if len(ids) > 0 {
		declared = strings.Join(ids, ", ")
	}
	return fmt.Errorf("OIDC client %q is not declared on this gateway: declared clients are %s", id.Client, declared)
}
