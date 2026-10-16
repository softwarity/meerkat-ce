// Package oidcprovider is the community side of the OpenID provider
// (SAUTH-05): the socket the Enterprise implementation plugs into, and nothing
// else.
//
// The provider - the endpoints an application signs its people in through -
// is Enterprise code (ee/oidcprovider), linked by cmd/meerkat/link_ee.go. The
// community binary does not contain it, and this package is what lets the
// trunk mount it without knowing it exists: the Enterprise package registers a
// factory at init, main asks for the provider, and in the community image the
// answer is a provider that mounts nothing. Every path it would have served
// then falls through to the routes, which answer what they always answered.
//
// What stays here is what the trunk itself has to name: the paths (the
// discovery document of the internal plane lists them), and the question "is
// the provider answering" that document asks.
package oidcprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The provider's own paths. Under /meerkat/, the namespace the gateway keeps
// for itself on the data plane: no route can be shadowed by them. The
// discovery document is the exception - it lives where the standard puts it,
// and is only claimed while the provider is in use (see Provider.WrapData).
const (
	PathPrefix    = "/meerkat/oidc/"
	AuthorizePath = PathPrefix + "authorize"
	TokenPath     = PathPrefix + "token"
	UserinfoPath  = PathPrefix + "userinfo"
	JWKSPath      = PathPrefix + "jwks"
	DiscoveryPath = "/.well-known/openid-configuration"
)

// Deps is what the provider is built on: the store holding its clients, codes
// and refresh tokens, the data plane's sessions people sign in with, and the
// router that knows who a person is and what a route says about them.
type Deps struct {
	Store    *store.Store
	Sessions *session.Manager
	Router   *gateway.Router
	// Refuse draws the page a person lands on when the request that brought
	// them cannot be answered to the application: the data plane's own page,
	// themed and in their language (auth's ServeAppRefused, whose closed list
	// of reasons why is). Nil draws a bare line of text.
	Refuse func(w http.ResponseWriter, r *http.Request, why string)
}

// Mux is where the endpoints are mounted: a plane's ServeMux.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Provider is what an implementation offers the planes.
type Provider interface {
	// MountData mounts the data plane's endpoints: authorize, token,
	// userinfo, jwks.
	MountData(mux Mux)
	// WrapData wraps the data plane's catch-all - the router - to answer the
	// discovery document at the root and under the issuer's path, which no
	// static pattern can name: the issuer is a setting.
	WrapData(next http.Handler) http.Handler
	// MountInternal mounts what a server inside the cluster calls: token,
	// userinfo, jwks. Never authorize - that is a browser's door, and a
	// browser has no business on the internal plane.
	MountInternal(mux Mux)
	// Active says whether the provider answers: an issuer that is an
	// absolute URL, and at least one client declared.
	Active(ctx context.Context) bool
	// Endpoints names the provider's endpoints for a discovery document:
	// authorize always on the public address (the issuer's origin), the
	// server-to-server ones on serverBase - the address the caller used,
	// which on the internal plane is the gateway's name inside the cluster.
	// ok is false while the provider is not active.
	Endpoints(ctx context.Context, serverBase string) (e Endpoints, ok bool)
}

// Endpoints are the provider's addresses, as a discovery document lists them.
type Endpoints struct {
	Authorization string
	Token         string
	Userinfo      string
	JWKS          string
	EndSession    string
}

// What the provider supports, named once for both discovery documents.
var (
	ResponseTypes        = []string{"code"}
	GrantTypes           = []string{"authorization_code", "refresh_token"}
	CodeChallengeMethods = []string{"S256"}
	TokenAuthMethods     = []string{"client_secret_basic", "client_secret_post", "none"}
	SigningAlgorithms    = []string{"RS256"}
	Scopes               = []string{"openid", "profile", "email"}
)

// ErrIssuerNotURL is what every endpoint answers while the issuer is a name
// rather than the gateway's public address: while the application plane has
// no public name, the issuer falls back to one (store.DefaultJWTIssuer).
var ErrIssuerNotURL = errors.New("the OpenID provider needs this gateway's public address as its issuer: " +
	"declare a public name in Domains & TLS, Application tab")

// IssuerOrigin is the public origin the provider lives at, read from the
// issuer: an application is sent to <origin>/meerkat/oidc/authorize, and it
// compares the issuer of its tokens with the one discovery gave it, so the two
// must be one address. The issuer is one of the application plane's public
// addresses (store.IssuerAddress): https, or http on a loopback name. A name
// ("meerkat"), or plain http anywhere else, cannot be one.
func IssuerOrigin(issuer string) (string, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || u.Opaque != "" {
		return "", ErrIssuerNotURL
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && store.LoopbackName(u.Hostname()):
	default:
		return "", fmt.Errorf("issuer %q: %w", issuer, ErrIssuerNotURL)
	}
	return u.Scheme + "://" + u.Host, nil
}

// Returns are the places the enabled routes bound to client serve, read
// against the gateway's public address: where the provider accepts a
// redirect_uri the client did not declare. One function for the provider
// that decides and the console that shows it, so what an administrator reads
// under a client is what an application is held to.
func Returns(rt *gateway.Router, client string) []gateway.OIDCReturn {
	if rt == nil {
		return nil
	}
	origin, _ := IssuerOrigin(rt.Issuer())
	return gateway.OIDCReturns(origin, rt.OIDCRoutes(client))
}

// Factory builds the provider over its dependencies.
type Factory func(Deps) Provider

var factory Factory

// Register is called by the Enterprise implementation, at init. Twice is a
// linking mistake, and said so at start rather than discovered as whichever
// registration happened to win.
func Register(f Factory) {
	if factory != nil {
		panic("oidcprovider: a provider is already registered")
	}
	factory = f
}

// Linked says whether this binary carries a provider.
func Linked() bool { return factory != nil }

// New builds the provider, or the one that mounts nothing when this binary
// carries none.
func New(d Deps) Provider {
	if factory == nil {
		return absent{}
	}
	return factory(d)
}

// absent is the community image's provider: no path, no discovery, never
// active.
type absent struct{}

func (absent) MountData(Mux)                           {}
func (absent) WrapData(next http.Handler) http.Handler { return next }
func (absent) MountInternal(Mux)                       {}
func (absent) Active(context.Context) bool             { return false }
func (absent) Endpoints(context.Context, string) (Endpoints, bool) {
	return Endpoints{}, false
}
