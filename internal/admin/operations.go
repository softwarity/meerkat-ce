package admin

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/store"
)

// How often the operations a route exposes are resolved again.
//
// Slow on purpose. A spec changes when somebody deploys a service, not when a
// request arrives, and the only cost of being ten minutes late is that a brand
// new endpoint is counted on its route rather than on itself for ten minutes.
// Against that: one HTTP call per route per period, made to every service this
// gateway fronts.
const refreshOperationsEvery = 10 * time.Minute

// opsClient is deliberately not specClient: this runs in the background,
// unasked, and a service that hangs must not hold the refresher for twenty
// seconds - the next round will catch it.
var opsClient = &http.Client{Timeout: 5 * time.Second}

// RefreshOperations keeps the data plane's per-endpoint counters named.
//
// WHY THE CONTROL PLANE DOES THIS. The router can name an operation from what
// the database holds - a deposited spec, the per-endpoint rules somebody wrote
// - because reading those costs nothing on the reload path. A spec the route
// FETCHES from its upstream is the common case and cannot be read there: a
// network call per route on every reload is a reload that hangs because one
// service is slow, and a gateway that stops taking configuration when a
// service is unwell has the dependency exactly backwards.
//
// So it is resolved here, on a schedule of its own, and pushed in. A failure
// is a route counted whole for another period - never an error anybody has to
// act on, and never something that touches traffic.
//
// Runs until ctx is done. Started by main; a build with no router does nothing.
func (a *API) RefreshOperations(ctx context.Context) {
	if a.router == nil {
		return
	}
	t := time.NewTicker(refreshOperationsEvery)
	defer t.Stop()
	for {
		a.refreshOperationsOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *API) refreshOperationsOnce(ctx context.Context) {
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		slog.Debug("operations refresh: routes unavailable", "err", err)
		return
	}
	for _, r := range routes {
		if ctx.Err() != nil {
			return
		}
		// A disabled route serves nothing, and a deposited spec is already
		// read by the router itself while it compiles - asking twice would be
		// a second reader of the same bytes, free to drift from the first.
		if !r.Enabled || r.Spec().Type != store.SpecUpstream {
			continue
		}
		ops, err := a.upstreamOperations(ctx, r)
		if err != nil {
			// Expected, and often: a service that is down, a spec url that
			// moved, a document this parser will not take. The route keeps
			// being counted whole, which is the honest answer.
			slog.Debug("operations refresh: spec unreadable", "route", r.Name, "err", err)
			continue
		}
		a.router.SetOperations(r.ID, ops)
	}
}

// upstreamOperations fetches a route's spec and projects it down to the pairs
// the counters are keyed on.
func (a *API) upstreamOperations(ctx context.Context, route store.Route) ([]gateway.Operation, error) {
	specURL, err := resolveSpecURL(route)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, opsClient.Timeout)
	defer cancel()
	spec, _, err := openapi.Fetch(ctx, opsClient, specURL)
	if err != nil {
		return nil, err
	}
	// The spec's own coordinates, unchanged. That is the same convention the
	// endpoint guard reads paths in (it strips what strip-prefix removes and
	// compares against spec paths), and the console's endpoint editor writes
	// in - one set of coordinates for the three, or the screens disagree about
	// what "/orders/{id}" means.
	ops := make([]gateway.Operation, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		ops = append(ops, gateway.Operation{Method: op.Method, Path: op.Path})
	}
	return ops, nil
}
