package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
)

// The identity a route forwards, as far as an agent reaches it (AUTH-06,
// AUTH-07, SAUTH-04): the keys the gateway signs with and where they are
// published, the issuer every token names, and what a draft identity would
// send.
//
// This section was out of reach as "JWT signing keys", and the reason held
// for the private halves, which still never leave the server: what an agent
// gets is what the console shows, the public keys. It did not hold for the
// rest. Choosing the issuer is the step between declaring a public name and
// an application signing in through the gateway, and an agent asked to put an
// application behind the gateway end to end stopped there.
func (a *API) identityTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "get_signing_keys", Allow: administersRouting, Title: "Read the signing keys and the issuer", ReadOnly: true,
			Description: "What a service verifying the gateway's tokens needs: the issuer every token names " +
				"(iss) and the public addresses it may be (issuers - the application plane's public names; " +
				"none while none is declared), the internal plane's port and paths of the discovery document " +
				"and the JWKS, the public OIDC discovery address when the issuer can carry one, and the PUBLIC " +
				"key of each algorithm (ES256, EdDSA, RS256) with its kid. Private keys never leave the gateway.",
			Schema: noArgs(),
			Call: func(ctx context.Context, _ json.RawMessage) (any, error) {
				return a.signingView(ctx)
			},
		},
		{
			Name: "set_issuer", Allow: administersRouting, Title: "Choose the issuer",
			Description: "Make one of the application plane's public addresses the issuer: the iss of every " +
				"JWT a route sends, the issuer of both discovery documents and the address of the OpenID " +
				"provider. Only an address get_signing_keys lists under issuers is accepted, and the refusal " +
				"names them. Every route reloads on every node: services checking iss must expect the new one.",
			Schema: object(map[string]any{
				"issuer": str("One of the issuers get_signing_keys lists, e.g. https://apps.example.com."),
			}, "issuer"),
			Call: func(ctx context.Context, args json.RawMessage) (any, error) {
				var in struct {
					Issuer string `json:"issuer"`
				}
				if err := decode(args, &in); err != nil {
					return nil, err
				}
				return a.setIssuer(ctx, mcpActor(ctx), strings.TrimSpace(in.Issuer))
			},
		},
		{
			Name: "renew_signing_keys", Allow: administersRouting, Title: "Renew the signing keys",
			Description: "Draw a new key pair per algorithm. The gateway signs with the new ones at once; the " +
				"previous public keys stay in the JWKS through their grace window, so a service reading the " +
				"JWKS follows on its own. A service holding a PEM copied from get_signing_keys must be given " +
				"the new one. Do this when a key may have leaked, or on a rotation schedule.",
			Schema: noArgs(),
			Call: func(ctx context.Context, _ json.RawMessage) (any, error) {
				return a.renewKeys(ctx, mcpActor(ctx))
			},
		},
		{
			Name: "preview_identity", Allow: administersRouting, Title: "Preview what a route forwards", ReadOnly: true,
			Description: "What a service would receive from an identity section, for a fixed sample caller, " +
				"WITHOUT saving it: the headers, or the JWT with its decoded claims (and the public key that " +
				"verifies a signed one). Refuses exactly what save_route would refuse, so it is the way to " +
				"check an identity before writing it. The identity is the shape get_route returns under " +
				"identity. An oidc identity has no sample: try_oidc_sign_in tries it for a real person.",
			Schema: object(map[string]any{
				"routeName": str("The route's name, as the tokens' audience shows it."),
				"identity": map[string]any{"type": "object",
					"description": "The identity section: mechanism (headers, jwt, signed-jwt), attributes, algorithm, ttl..."},
			}, "identity"),
			Call: func(ctx context.Context, args json.RawMessage) (any, error) {
				var in identityPreviewRequest
				if err := decode(args, &in); err != nil {
					return nil, fmt.Errorf("this is not an identity to preview: %w", err)
				}
				return a.identityPreview(ctx, in)
			},
		},
	}
}
