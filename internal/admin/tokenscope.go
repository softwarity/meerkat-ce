package admin

import (
	"context"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// A control-plane token's perimeter, enforced (MCP-02).
//
// ONE axis, and it stayed one on purpose: a token opens a control plane, and
// what varies is how far into it - everything, reads only, the scheduled
// calls. Confining it to a PART of the administration used to be a
// second axis on the same token; what it confined is what the ACCOUNT already
// decides, so it was a second rights model to explain to whoever mints one.
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
	"POST /api/settings/telemetry/test":   true,
	// Proving a git location answers and its credential is accepted (CFG-07):
	// it reaches a repository and stores not one byte.
	"POST /api/config-remotes/{id}/check": true,
	// Asking a PostgreSQL server what it is before a copy: it reads, and
	// creates not even a schema.
	"POST /api/backup/check": true,
	// Asking a directory about one person: it reads, with the service account.
	"POST /api/auth-providers/{id}/lookup": true,
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
	Name  string
	Scope string
}

type actorTokenKey struct{}

// withActorToken remembers which token authenticated this request.
func withActorToken(ctx context.Context, sess store.Session) context.Context {
	if sess.TokenName == "" {
		return ctx
	}
	return context.WithValue(ctx, actorTokenKey{}, actingToken{
		Name: sess.TokenName, Scope: sess.TokenScope,
	})
}

// actorToken returns the token name recorded on the context, or "" for a human
// working from the console.
func actorToken(ctx context.Context) string {
	tok, _ := ctx.Value(actorTokenKey{}).(actingToken)
	return tok.Name
}

// tokenScope returns the perimeter of the acting token, or "" for a human
// working from the console - who carries no perimeter at all.
func tokenScope(ctx context.Context) string {
	tok, _ := ctx.Value(actorTokenKey{}).(actingToken)
	return tok.Scope
}

// tokenPerimeter describes the acting token in words an agent can repeat to
// the person it is talking to. A tool it cannot find is a dead end; a
// perimeter it can name is a sentence.
func tokenPerimeter(ctx context.Context) map[string]any {
	tok, ok := ctx.Value(actorTokenKey{}).(actingToken)
	if !ok {
		return map[string]any{"as": "a signed-in session", "mayChange": true}
	}
	return map[string]any{
		"token":     tok.Name,
		"mayChange": tok.Scope != store.ScopeReadOnly,
	}
}
