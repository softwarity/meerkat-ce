package admin

import (
	"context"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// A control-plane token's perimeter, enforced (MCP-02).
//
// The temptation is to bound an agent by the TOOLS it is offered. That is not
// a boundary: the same token opens the whole REST API on the same port, and an
// agent has curl. The boundary is here, in the one funnel every control-plane
// call goes through, so it covers the API and the agent endpoint with one rule.
//
// What counts as a read is decided by the ENDPOINT, not by the verb. Half a
// dozen POSTs here compute an answer and change nothing (the routing tester,
// the previews, the relay probe), and the agent endpoint is itself a POST that
// carries reads and writes alike. A verb-only rule would forbid exactly the
// things a read-only agent is for.

// readsNothing lists the non-GET endpoints that change nothing, by the pattern
// they were registered under (http.Request.Pattern). Everything absent from
// this map and not a safe method is a write.
//
// Adding an endpoint here is a DECISION, and the coverage test refuses a new
// non-GET pattern that nobody has classified - see api_surface_test.go.
var readsNothing = map[string]bool{
	// The testers and previews: they resolve, render or reach out, and store
	// nothing. Refusing these to a read-only token would forbid exactly what
	// such a token is for - reading a gateway includes asking it questions.
	"POST /api/routes/probe":              true,
	"POST /api/routes/respond-preview":    true,
	"POST /api/routes/version-preview":    true,
	"POST /api/identity/preview":          true,
	"POST /api/config/preview":            true,
	"POST /api/auth-providers/{id}/check": true,
	"POST /api/settings/mail-relay/test":  true,
	// The agent endpoint carries both kinds and sorts them per tool: an
	// annotated read-only tool answers, a mutating one is refused by name.
	"/mcp": true,
}

// readsOnly reports whether this request only reads.
func readsOnly(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return readsNothing[r.Pattern]
}

// actingToken carries what a token brings to a request, down to the places
// that cannot take it as a parameter: the audit (which is called from eighty
// sites with the request's context already in hand, and would lie by omission
// if any one of them forgot), and the tenant-administration check.
type actingToken struct {
	Name   string
	Scope  string
	Domain string
}

type actorTokenKey struct{}

// withActorToken remembers which token authenticated this request.
func withActorToken(ctx context.Context, sess store.Session) context.Context {
	if sess.TokenName == "" {
		return ctx
	}
	return context.WithValue(ctx, actorTokenKey{}, actingToken{
		Name: sess.TokenName, Scope: sess.TokenScope, Domain: sess.TokenDomain,
	})
}

// actorToken returns the token name recorded on the context, or "" for a human
// working from the console.
func actorToken(ctx context.Context) string {
	tok, _ := ctx.Value(actorTokenKey{}).(actingToken)
	return tok.Name
}

// tokenDomain returns the domain the acting token is confined to, or "" for a
// human and for a token that is confined to nothing.
func tokenDomain(ctx context.Context) string {
	tok, _ := ctx.Value(actorTokenKey{}).(actingToken)
	return tok.Domain
}

// tokenPerimeter describes the acting token in words an agent can repeat to
// the person it is talking to. A tool it cannot find is a dead end; a
// perimeter it can name is a sentence.
func tokenPerimeter(ctx context.Context) map[string]any {
	tok, ok := ctx.Value(actorTokenKey{}).(actingToken)
	if !ok {
		return map[string]any{"as": "a signed-in session", "mayChange": true}
	}
	out := map[string]any{
		"token":     tok.Name,
		"mayChange": tok.Scope != store.ScopeReadOnly,
	}
	switch tok.Domain {
	case store.DomainGateway:
		out["confinedTo"] = "the routing plane: routes, built-in pages, certificates"
	case store.DomainApp:
		out["confinedTo"] = "the application's identity: accounts, roles, organisations, settings"
	}
	return out
}

// narrowTo masks an actor's capabilities down to the token's domain (MCP-02).
//
// This is the whole trick, and it is why the perimeter needed no second rights
// model: every guard in this package already reads these booleans, so masking
// them once here confines the REST API, the agent's tools and anything added
// next month, with no list to keep up to date. Root is dropped rather than
// kept, because a domain that left root standing would confine nothing.
func narrowTo(actor store.User, domain string) store.User {
	switch domain {
	case store.DomainGateway:
		actor.InfraAdmin = actor.Root || actor.InfraAdmin
		actor.Root, actor.AppAdmin, actor.TenantCreator = false, false, false
	case store.DomainApp:
		actor.AppAdmin = actor.Root || actor.AppAdmin
		actor.Root, actor.InfraAdmin = false, false
	}
	return actor
}
