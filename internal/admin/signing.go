package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/oidcprovider"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store"
)

// registerSigning mounts the identity signing-key surface: read the public keys
// (PEM + JWKS location) for backends verifying signed-jwt, and rotate them.
// Routing plane (GATEWAY scope): read for infra-admin, rotate for infra-admin.
func (a *API) registerSigning(mux Mux) {
	mux.Handle("GET /api/identity/signing-keys", a.gw(a.getSigningKeys))
	mux.Handle("POST /api/identity/signing-keys/renew", a.infraAdmin(a.renewSigningKeys))
	// Which of the application plane's public addresses is the "iss" of every
	// token (AUTH-06), the issuer both discovery documents name (SAUTH-04) and
	// the OpenID provider's address (SAUTH-05). The router reloads on it.
	mux.Handle("PUT /api/identity/issuer", a.infraAdmin(a.putIssuer))
	mux.Handle("POST /api/identity/preview", a.infraAdmin(a.previewIdentity))
}

// identityPreviewRequest previews an identity config WITHOUT saving it, so the
// editor can show what an upstream would receive while it is still a draft.
// The claim VALUES are never taken from here: they are the server's fixed
// sample caller (see gateway.PreviewHeaders/PreviewClaims).
type identityPreviewRequest struct {
	RouteName string                `json:"routeName"`
	Identity  store.IdentityForward `json:"identity"`
}

// identityPreviewResponse is what the upstream would receive: the headers, or
// the bearer token with its decoded claims (plus the verifying key material
// for signed-jwt).
type identityPreviewResponse struct {
	Mechanism string                   `json:"mechanism"`
	Headers   []gateway.IdentityHeader `json:"headers,omitempty"`
	Token     string                   `json:"token,omitempty"`
	Claims    map[string]any           `json:"claims,omitempty"`
	Algorithm string                   `json:"algorithm,omitempty"`
	Kid       string                   `json:"kid,omitempty"`
	PublicPem string                   `json:"publicPem,omitempty"`
}

func (a *API) previewIdentity(w http.ResponseWriter, r *http.Request, _ store.User) {
	var req identityPreviewRequest
	if err := decodeStrict(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed identity: "+err.Error())
		return
	}
	res, err := a.identityPreview(r.Context(), req)
	if err != nil {
		if isInvalid(err) {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// identityPreview is what an upstream would receive from a draft identity,
// for the fixed sample caller. Shared by the endpoint and the agent's tool;
// a refusal of the draft is an invalidErr.
func (a *API) identityPreview(ctx context.Context, req identityPreviewRequest) (identityPreviewResponse, error) {
	cfg := req.Identity
	// Validate through the engine: a preview must refuse exactly what a save
	// would refuse (unknown mechanism, bad attribute, bad claim name...).
	draft := store.Route{
		Name: req.RouteName, Upstream: "http://preview.invalid",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Identity:   &cfg,
	}
	if err := gateway.Validate(draft); err != nil {
		return identityPreviewResponse{}, invalidErr(err)
	}
	// And through the settings: since a custom field is a name this
	// installation defines (MODEL-01), the engine accepts any name that COULD
	// be one, and only the store knows which ones ARE. Asked here too, or a
	// preview would happily draw a header for a field the save then refuses.
	if err := a.knownFields(ctx, draft); err != nil {
		return identityPreviewResponse{}, invalidErr(err)
	}
	res := identityPreviewResponse{Mechanism: cfg.Mechanism}
	switch cfg.Mechanism {
	case "headers":
		res.Headers = gateway.PreviewHeaders(cfg)
	case "jwt", "signed-jwt":
		claims := gateway.PreviewClaims(cfg, req.RouteName, a.st.JWTIssuer(ctx, a.DataAddr))
		res.Claims = claims
		if cfg.Mechanism == "jwt" {
			tok, err := gateway.MintUnsignedJWT(claims)
			if err != nil {
				return identityPreviewResponse{}, err
			}
			res.Token = tok
			break
		}
		alg := cfg.Algorithm
		if alg == "" {
			alg = signing.ES256
		}
		set, err := a.st.EnsureSigningSet(ctx)
		if err != nil {
			return identityPreviewResponse{}, err
		}
		tok, err := set.SignJWT(alg, claims)
		if err != nil {
			return identityPreviewResponse{}, err
		}
		pemStr, err := set.PublicPEM(alg)
		if err != nil {
			return identityPreviewResponse{}, err
		}
		res.Token, res.Algorithm, res.Kid, res.PublicPem = tok, alg, set.Kid(alg), pemStr
	default:
		return identityPreviewResponse{}, invalidErr(fmt.Errorf("identity mechanism %s has nothing to preview "+
			"with a sample caller: an oidc route is tried for a real person with try_oidc_sign_in", cfg.Mechanism))
	}
	return res, nil
}

// signingKey is one algorithm's public half as the console shows it. The private
// key never leaves the server.
type signingKey struct {
	Algorithm string `json:"algorithm"`
	Kid       string `json:"kid"`
	PublicPem string `json:"publicPem"`
}

// signingKeysView is the whole picture: where the JWKS is served, and the public
// key per algorithm to paste into a backend that prefers static keys.
//
// WHERE is a port and a path, not an address: the gateway's name inside a
// cluster is the deployment's (a Service, a compose service), and this
// console is reached under yet another one. InternalPort is the internal
// plane's, the only one that serves it - and its discovery document, which
// names Issuer.
type signingKeysView struct {
	JwksPath      string `json:"jwksPath"`
	DiscoveryPath string `json:"discoveryPath"`
	InternalPort  string `json:"internalPort"`
	// InternalOrigin is the internal plane as the runtime names it, "" when
	// none could be asked (see discovery.InternalOrigin).
	InternalOrigin string `json:"internalOrigin,omitempty"`
	// Issuer is the one in effect (store.JWTIssuer), Issuers what it may be:
	// the application plane's public addresses, none while it has no public
	// name - the console then says where to declare one.
	Issuer  string       `json:"issuer"`
	Issuers []string     `json:"issuers"`
	Keys    []signingKey `json:"keys"`
	// PublicDiscovery is the OpenID provider's discovery document under the
	// issuer, the one address an application is told; absent while the
	// issuer is not an address the provider can live at
	// (oidcprovider.IssuerOrigin). Whether this image carries the provider is
	// not its question: the community console shows the section dimmed.
	PublicDiscovery string `json:"publicDiscovery,omitempty"`
}

func (a *API) signingView(ctx context.Context) (signingKeysView, error) {
	set, err := a.st.EnsureSigningSet(ctx)
	if err != nil {
		return signingKeysView{}, err
	}
	view := signingKeysView{JwksPath: gateway.JWKSPath, DiscoveryPath: DiscoveryPath,
		InternalPort: portOf(a.InternalAddr), InternalOrigin: a.internalOriginOf(ctx),
		Issuer: a.st.JWTIssuer(ctx, a.DataAddr), Issuers: a.st.IssuerChoices(ctx, a.DataAddr)}
	if _, err := oidcprovider.IssuerOrigin(view.Issuer); err == nil {
		view.PublicDiscovery = view.Issuer + oidcprovider.DiscoveryPath
	}
	for _, alg := range signing.Algorithms {
		pemStr, err := set.PublicPEM(alg)
		if err != nil {
			return signingKeysView{}, err
		}
		view.Keys = append(view.Keys, signingKey{Algorithm: alg, Kid: set.Kid(alg), PublicPem: pemStr})
	}
	return view, nil
}

func (a *API) getSigningKeys(w http.ResponseWriter, r *http.Request) {
	view, err := a.signingView(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *API) renewSigningKeys(w http.ResponseWriter, r *http.Request, actor store.User) {
	view, err := a.renewKeys(r.Context(), actor)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// renewKeys rotates the signing keys. Shared by the endpoint and the agent's
// tool.
func (a *API) renewKeys(ctx context.Context, actor store.User) (signingKeysView, error) {
	if _, err := a.st.RenewSigningKeys(ctx); err != nil {
		return signingKeysView{}, err
	}
	// Reload so the gateway signs with the new keys and publishes the new JWKS
	// (the previous public keys linger there through their grace window).
	if err := a.reloadRouting(ctx); err != nil {
		return signingKeysView{}, fmt.Errorf("renewed, but reload failed: %w", err)
	}
	a.auditEvent(ctx, actor, "identity.signing_renew", "settings", "", "", "",
		"identity signing keys rotated")
	return a.signingView(ctx)
}

// issuerChange is the issuer as the console sends it: one of the public
// addresses the view lists.
type issuerChange struct {
	Issuer string `json:"issuer"`
}

func (a *API) putIssuer(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p issuerChange
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed issuer: "+err.Error())
		return
	}
	view, err := a.setIssuer(r.Context(), actor, p.Issuer)
	if err != nil {
		if isInvalid(err) {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// setIssuer makes one of the public addresses the issuer. Shared by the
// endpoint and the agent's tool; a refusal names the addresses allowed.
func (a *API) setIssuer(ctx context.Context, actor store.User, issuer string) (signingKeysView, error) {
	before := a.st.JWTIssuer(ctx, a.DataAddr)
	if err := a.st.SetJWTIssuer(ctx, a.DataAddr, issuer); err != nil {
		return signingKeysView{}, invalidErr(err)
	}
	// The router signs with what it read at reload, on every node.
	if err := a.reloadRouting(ctx); err != nil {
		return signingKeysView{}, fmt.Errorf("stored, but reload failed: %w", err)
	}
	a.auditUpdate(ctx, actor, "identity.issuer", "settings", "", "", "",
		issuerChange{Issuer: before}, issuerChange{Issuer: issuer})
	return a.signingView(ctx)
}

// portOf is the port of a listen address, ":7070" or "0.0.0.0:7070" alike.
func portOf(addr string) string {
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		return addr[i+1:]
	}
	return addr
}
