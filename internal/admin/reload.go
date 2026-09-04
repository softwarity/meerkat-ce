package admin

import (
	"context"

	"github.com/softwarity/meerkat/internal/store"
)

// The two things a write makes a node re-read, and the ONE place each is done.
//
// Not a wrapper for its own sake: reloading and announcing have to happen
// together, and they were fifteen call sites apart. A handler that reloaded
// without announcing left the other nodes serving the previous plan - the
// exact failure the change bus exists to close, reintroduced one endpoint at a
// time. So the pair lives here, the call sites say what they mean, and
// reload_test.go refuses any other route to it.

// reloadRouting recompiles this node's route plan and tells the others to do
// the same.
//
// The local reload is synchronous on purpose: the response to the request that
// caused it must already reflect it, or an operator saving a route and
// reloading the page would see their own change missing.
func (a *API) reloadRouting(ctx context.Context) error {
	if err := a.router.Reload(ctx); err != nil {
		return err
	}
	a.announce(ctx, store.TopicRouting)
	return nil
}

// announce is the nil-tolerant half. Most tests build an API with no bus -
// there is one node and nothing to tell - and a change that only matters in a
// cluster must not need a cluster to be tested.
func (a *API) announce(ctx context.Context, topic string) {
	if a.Bus == nil {
		return
	}
	a.Bus.Announce(ctx, topic)
}
