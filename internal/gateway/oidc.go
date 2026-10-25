package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store"
)

// What the OpenID provider (SAUTH-05) asks of the router. The provider itself
// lives elsewhere - in the Enterprise tree - but WHO a person is and what a
// route says about them are the router's answers, and computing them a second
// time somewhere else is how the token an application receives would drift
// from the identity the same route forwards. So the provider asks here.

// OIDCRoute is a route that signs people into an OIDC client, as the router
// serves it: enabled, compiled, its vault references expanded - the same
// route a request meets, so the tokens an application receives say what the
// route says today.
type OIDCRoute struct {
	ID   string
	Name string
	// Access is the route's base rule: whom it lets sign in.
	Access   store.Access
	Identity store.IdentityForward
	// hosts are the route's host predicates (AND between the lists), nil
	// when it has none; paths the literal heads of its path predicates.
	// Where the route serves its application, which is where that
	// application may receive its sign-in (OIDCReturns).
	hosts [][]string
	paths [][]string
	// roleMap is the route's declared-role mapping (store.RouteAPI.RoleMap),
	// read by the roles expression's .Declared.
	roleMap map[string]string
}

func oidcRouteOf(r store.Route) *OIDCRoute {
	if r.Identity == nil || r.Identity.Mechanism != store.IdentityOIDC {
		return nil
	}
	id := *r.Identity
	id.Attributes = slices.Clone(id.Attributes)
	id.Claims = slices.Clone(id.Claims)
	return &OIDCRoute{ID: r.ID, Name: r.Name, Access: r.Access, Identity: id,
		hosts: routing.HostNames(r.Predicates), paths: routing.PathHeads(r.Predicates),
		roleMap: maps.Clone(r.RoleMap())}
}

// OIDCRoutes are the routes signing people into client, in routing order -
// the order the provider tries them in, since the first whose rule grants the
// person decides what the tokens say. Only what is served: a disabled route,
// or one that does not compile, signs nobody in.
func (rt *Router) OIDCRoutes(client string) []OIDCRoute {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var out []OIDCRoute
	for _, r := range rt.routes {
		if r.oidc != nil && r.oidc.Identity.Client == client {
			out = append(out, *r.oidc)
		}
	}
	return out
}

// Person is who a gateway session is, resolved exactly as a request through a
// route resolves it: the account, the organisation, the roles the session's
// group mode grants, the installation's own facts.
type Person struct {
	d identityData
}

// UserID is the account's id: the default subject of the tokens.
func (p Person) UserID() string { return p.d.UserID }

// Username is the account's name, for a log line.
func (p Person) Username() string { return p.d.Username }

// TenantID is the organisation the person is active in, "" without one.
func (p Person) TenantID() string { return p.d.TenantID }

// Caller is what an access rule is evaluated against, built as the routing
// builds it.
func (p Person) Caller() store.Caller {
	return store.Caller{Authenticated: true, Username: p.d.Username, TenantID: p.d.TenantID,
		Roles: p.d.Roles, Memberships: p.d.Memberships}
}

// PersonOf resolves a stored session the way a request carrying it would be
// resolved. ok is false for a session still in the login flow, or whose
// account is disabled or outside its validity window. It does not ask
// whether the session has expired: the caller holds the row and its clock.
func (rt *Router) PersonOf(req *http.Request, sess store.Session) (Person, bool) {
	if sess.Pending != "" || sess.UserID == "" {
		return Person{}, false
	}
	d, ok := rt.identityOfSession(req, sess)
	if !ok {
		return Person{}, false
	}
	return Person{d: d}, true
}

// PersonFor resolves an account as a session opened in tenantID would be: the
// same facts and roles a request carrying that session gets, read fresh. For
// an administrator trying a sign-in on someone's behalf, so the answer is
// what that person would get, not a sample. tenantID "" adopts the only
// membership, as sign-in does. The refusals say which part does not hold.
func (rt *Router) PersonFor(ctx context.Context, userID, tenantID string) (Person, error) {
	req := (&http.Request{}).WithContext(ctx)
	e, ok := rt.readIdentity(req, store.Session{UserID: userID, TenantID: tenantID})
	if !ok {
		return Person{}, fmt.Errorf("no account %q", userID)
	}
	if !e.owner.Enabled || !e.owner.ValidAt(time.Now()) {
		return Person{}, fmt.Errorf("account %q is disabled or outside its validity window: it signs nobody in", e.data.Username)
	}
	if tenantID != "" && !slices.Contains(e.data.Memberships, tenantID) {
		return Person{}, fmt.Errorf("%s is not an enabled member of tenant %q", e.data.Username, tenantID)
	}
	return Person{d: e.data}, nil
}

// Through is the person as route sees them: the same facts, with the route's
// declared-role mapping for its roles expression.
func (p Person) Through(route OIDCRoute) Person {
	p.d.Declared = route.roleMap
	return p
}

// OIDCClaims renders a route's oidc claims for a person: a fixed value as it
// is written, a field as the jwt mechanism renders that fact of the route's
// Identity section - the roles through its expression, an array staying an
// array. A fact the person does not have is left out rather than sent empty.
func (rt *Router) OIDCClaims(cfg store.IdentityForward, p Person) map[string]any {
	out := make(map[string]any, len(cfg.Claims))
	for _, c := range cfg.Claims {
		if c.Field == "" {
			out[c.Name] = c.Value
			continue
		}
		var attr *store.IdentityAttr
		for i := range cfg.Attributes {
			if cfg.Attributes[i].Field == c.Field {
				attr = &cfg.Attributes[i]
				break
			}
		}
		if attr == nil {
			continue
		}
		if c.Field == "roles" {
			v, err := renderRoles(attr.Expr, p.d)
			if err != nil || v == "" {
				continue
			}
			var arr []string
			if json.Unmarshal([]byte(v), &arr) == nil {
				out[c.Name] = arr
			} else {
				out[c.Name] = v
			}
			continue
		}
		if v, ok := fieldClaim(p.d, c.Field); ok {
			out[c.Name] = v
		}
	}
	return out
}

// errNoSigningKeys is what signing answers before any key exists: the keys
// are drawn at the first reload that finds a route needing them.
var errNoSigningKeys = errors.New("the gateway has no signing keys yet: they are drawn when an enabled route uses the signed-jwt or oidc mechanism")

// SignOIDC signs the provider's tokens: RS256, with the gateway's signing set,
// so an application verifies them against the same JWKS as every other token
// this gateway signs. RS256 because it is the one algorithm every OpenID
// relying party is required to accept.
func (rt *Router) SignOIDC(claims map[string]any) (string, error) {
	set := rt.currentSigning()
	if set == nil {
		return "", errNoSigningKeys
	}
	return set.SignJWT(signing.RS256, claims)
}

// VerifyOIDC checks the signature of a token SignOIDC produced and returns its
// claims. What they say is the caller's to judge.
func (rt *Router) VerifyOIDC(token string) (map[string]any, error) {
	set := rt.currentSigning()
	if set == nil {
		return nil, errNoSigningKeys
	}
	return set.VerifyRS256(token)
}

// RefuseSignIn answers a person none of rules lets in, with the same pages a
// browser refused by a route is sent to: the organisation chooser when
// another organisation would let them in under one of the rules, the waiting
// room, or the refusal page. Each carries this request as the way back, so a
// person who chooses another organisation resumes the sign-in.
func (rt *Router) RefuseSignIn(w http.ResponseWriter, req *http.Request, rules []store.Access, p Person) {
	if len(rules) == 0 {
		http.Error(w, "refused: nothing lets you in here", http.StatusForbidden)
		return
	}
	who := p.Caller()
	rule := rules[0]
	for _, a := range rules {
		if len(rt.switchWouldHelp(req.Context(), a, p.d.UserID, a.Switchable(who))) > 0 {
			rule = a
			break
		}
	}
	slog.Warn("access refused", "path", req.URL.Path, "method", req.Method,
		"user", who.Username, "tenant", who.TenantID, "why", refusalCode(rule, who))
	rt.refusePage(w, req, rule, who, p.d.UserID)
}
