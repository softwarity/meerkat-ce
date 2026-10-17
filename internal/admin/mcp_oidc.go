package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// The OIDC clients, as far as an agent reaches them (SAUTH-05).
//
// Putting an application behind the gateway end to end is a task an agent is
// asked to do: a route, a client, the claims its tokens carry. Leaving the
// client out would stop it one step short, with a person copying three fields
// from one screen to another. The secret is the delicate part, and it is
// handled the way the console handles it: shown once, in the answer of the
// call that drew it, and never readable afterwards.
func (a *API) oidcTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_oidc_clients", Allow: administersRouting, Title: "List the OIDC clients", ReadOnly: true,
			Description: "The applications that sign their people in against this gateway as an " +
				"OpenID provider: each client id, whether it keeps a secret (confidential) or proves " +
				"itself with PKCE alone (public), the addresses its routes serve where a redirect uri " +
				"is accepted - the only ones, a client declares none (acceptedUnder, and " +
				"wholeHostRoutes for a route covering a whole host), the audience and lifetime of its " +
				"tokens, the start of its secret, and the routes bound to it. A route binds to a " +
				"client with identity mechanism 'oidc'; its access rule decides who may sign in, " +
				"and the first route in routing order that grants the person wins. The secret " +
				"itself is never readable. The provider lives at the gateway's issuer, one of the " +
				"application plane's public names declared in Domains & TLS (https://<name>), and " +
				"answers nothing while none is declared.",
			Schema: noArgs(),
			Call:   a.toolListOIDCClients,
		},
		{
			Name: "save_oidc_client", Allow: administersRouting, Title: "Declare an OIDC client, or edit one",
			Description: "Declare an application as a client of this gateway's OpenID provider, or " +
				"change one. It works BY ID: an id this gateway does not hold is a new client, an " +
				"id it holds is that client, and on an existing one what you pass is what changes. " +
				"A NEW confidential client comes back with its secret in clear, ONCE: hand it to " +
				"the application's configuration, it cannot be read again (regenerate_oidc_client_secret " +
				"draws a new one). When the application imposes its own secret (its chart generates it, " +
				"its vendor fixed it), pass it as secret: it is kept as a hash and not sent back. A client declares no redirect uri: a code goes back to an address " +
				"under the paths of the enabled routes bound to it, on their host - bind the " +
				"application's route to the client with save_route. Enterprise edition only.",
			Schema: object(map[string]any{
				"id": str("The client_id, as the application's configuration will spell it: lower-case " +
					"letters, digits, . _ -. It cannot be changed afterwards."),
				"confidential": map[string]any{"type": "boolean",
					"description": "True (the default for a new client) when the application keeps a secret " +
						"on a server; false for code running in a browser or on a device, which proves " +
						"itself with PKCE alone."},
				"audience": str("The name the application expects in its access tokens. Empty: the client id."),
				"tokenTtl": str("How long the id and access tokens live, ISO-8601 from PT1M to PT24H. Empty: PT5M."),
				"secret": str("A NEW confidential client only: the application's own secret, 16 to 256 characters, " +
					"no spaces, when it imposes one. Omitted: one is drawn and shown once. An existing client's " +
					"secret changes with regenerate_oidc_client_secret."),
			}, "id"),
			Call: a.toolSaveOIDCClient,
		},
		{
			Name: "regenerate_oidc_client_secret", Allow: administersRouting, Title: "Replace the secret of an OIDC client",
			Description: "Replace a confidential client's secret. The old one stops working AT ONCE, " +
				"so the application is refused until its configuration carries the new one. Without " +
				"secret, one is drawn and this answer shows it in clear, once; with secret, the " +
				"application's own is kept, as a hash, and not sent back. Do this when a secret leaked, " +
				"after making a public client confidential, or when the application's secret changed. " +
				"Enterprise edition only.",
			Schema: object(map[string]any{
				"id": str("The client id, as list_oidc_clients spells it."),
				"secret": str("The application's own secret, 16 to 256 characters, no spaces, when it " +
					"imposes one. Omitted: one is drawn."),
			}, "id"),
			Call: a.toolRegenerateOIDCSecret,
		},
		{
			Name: "try_oidc_sign_in", Allow: administersRouting, Title: "Try a sign-in through an OIDC client", ReadOnly: true,
			Description: "What an application would get if this person signed in through this client now, " +
				"with nothing issued: the route that decides (the first bound to the client, in routing " +
				"order, whose access rule lets them in), the routes whose rule refuses them, and the claims " +
				"of the ID token, the access token and userinfo - built by the code that issues the real " +
				"ones, with the person's own roles and facts. Use it after save_route and save_oidc_client " +
				"to check a setup end to end, or to answer 'why does this person see nothing in the " +
				"application'. Enterprise edition only.",
			Schema: object(map[string]any{
				"id":   str("The client id, as list_oidc_clients spells it."),
				"user": str("The account, by id or username."),
				"tenant": str("Optional: the organisation the session would be opened in, by id or name. " +
					"Empty: the only one the person belongs to, as sign-in adopts it."),
			}, "id", "user"),
			Call: a.toolTryOIDCSignIn,
		},
		{
			Name: "delete_oidc_client", Allow: administersRouting, Title: "Delete an OIDC client",
			Description: "Remove a client, with the codes and refresh tokens it was handed. Refused " +
				"while a route still binds to it - the refusal names them: change their identity " +
				"mechanism or their client first, with get_route and save_route.",
			Schema: object(map[string]any{
				"id": str("The client id, as list_oidc_clients spells it."),
			}, "id"),
			Call: a.toolDeleteOIDCClient,
		},
	}
}

func (a *API) toolListOIDCClients(ctx context.Context, _ json.RawMessage) (any, error) {
	clients, err := a.st.ListOIDCClients(ctx)
	if err != nil {
		return nil, err
	}
	views, err := a.oidcViews(ctx, clients)
	if err != nil {
		return nil, err
	}
	return map[string]any{"clients": views}, nil
}

func (a *API) toolSaveOIDCClient(ctx context.Context, args json.RawMessage) (any, error) {
	var given map[string]json.RawMessage
	if err := json.Unmarshal(args, &given); err != nil {
		return nil, fmt.Errorf("this is not an OIDC client: %w", err)
	}
	var in struct {
		ID           string   `json:"id"`
		Confidential *bool    `json:"confidential"`
		RedirectURIs []string `json:"redirectUris"`
		Audience     string   `json:"audience"`
		TokenTTL     string   `json:"tokenTtl"`
		Secret       string   `json:"secret"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		return nil, fmt.Errorf("which client: pass its id, the client_id the application's configuration spells")
	}
	body := oidcClientIn{ID: in.ID, Confidential: in.Confidential, RedirectURIs: in.RedirectURIs,
		Audience: in.Audience, TokenTTL: in.TokenTTL, Secret: in.Secret}
	if _, err := a.st.GetOIDCClient(ctx, in.ID); err == nil {
		view, err := a.updateOIDCClient(ctx, mcpActor(ctx), body, given)
		if err != nil {
			return nil, err
		}
		return map[string]any{"saved": true, "created": false, "client": view}, nil
	}
	out, err := a.createOIDCClient(ctx, mcpActor(ctx), body)
	if err != nil {
		return nil, err
	}
	res := map[string]any{"saved": true, "created": true, "client": out.Client}
	if out.Secret != "" {
		res["secret"] = out.Secret
		res["note"] = "the secret is shown this once: put it in the application's configuration now"
	} else if in.Secret != "" {
		res["note"] = "the application's secret is kept, as a hash: nothing to copy"
	}
	return res, nil
}

func (a *API) toolRegenerateOIDCSecret(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, fmt.Errorf("which client: pass its id, as list_oidc_clients spells it")
	}
	out, err := a.regenerateOIDCSecret(ctx, mcpActor(ctx), id, in.Secret)
	if err != nil {
		return nil, oidcToolErr(id, err)
	}
	if in.Secret != "" {
		return map[string]any{
			"client": out.Client,
			"note":   "the old secret no longer works; the application's is kept, as a hash",
		}, nil
	}
	return map[string]any{
		"client": out.Client, "secret": out.Secret,
		"note": "the old secret no longer works; this one is shown this once",
	}, nil
}

func (a *API) toolTryOIDCSignIn(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		ID     string `json:"id"`
		User   string `json:"user"`
		Tenant string `json:"tenant"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(in.ID)
	trial, err := a.tryOIDCSignIn(ctx, id, oidcTryIn{User: in.User, Tenant: in.Tenant})
	if err != nil {
		return nil, oidcToolErr(id, err)
	}
	return trial, nil
}

func (a *API) toolDeleteOIDCClient(ctx context.Context, args json.RawMessage) (any, error) {
	id, err := oidcToolID(args)
	if err != nil {
		return nil, err
	}
	if err := a.dropOIDCClient(ctx, mcpActor(ctx), id); err != nil {
		return nil, oidcToolErr(id, err)
	}
	return map[string]any{"deleted": true, "id": id}, nil
}

func oidcToolID(args json.RawMessage) (string, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := decode(args, &in); err != nil {
		return "", err
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return "", fmt.Errorf("which client: pass its id, as list_oidc_clients spells it")
	}
	return id, nil
}

// oidcToolErr turns the store's "unknown client" into a sentence pointing at
// the tool that lists them.
func oidcToolErr(id string, err error) error {
	if errors.Is(err, store.ErrNoOIDCClient) {
		return fmt.Errorf("no OIDC client %q: call list_oidc_clients for the ids", id)
	}
	return err
}
