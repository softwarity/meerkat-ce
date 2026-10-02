package admin

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// Whether a route is actually answering (SVC-04, ROUTE-11).
//
// The console has always shown what is CONFIGURED and never what WORKS, which
// is the wrong half on the day somebody is on call: the question then is not
// "where does this route point" but "which of them are answering".
//
// Two sources, each answering what the other cannot. The circuit breaker
// (ROUTE-09) watches every real answer to decide whether to keep calling, so
// it says whether the traffic that WAS sent worked - but a route nobody has
// called yet has nothing to say. That was once the accepted price, on the
// argument that a prober forms a second opinion on traffic nobody sent. It
// turned out to be the wrong price for the case operators actually hit: a
// cluster service scaled to zero, an external host gone, both invisible until
// somebody's request fails. So the gateway now also checks each TARGET in the
// background (gateway/targets.go): the runtime's replica count when discovery
// knows the service, a bare TCP connect otherwise. It asks "is the target
// there at all", not "does the route work" - and when traffic says otherwise,
// the breaker's verdict wins.
//
// Per NODE, like the breaker and the check it reads. Two gateways may
// genuinely disagree about an upstream - a network path, a DNS answer, a
// sidecar - and this answers for the one that was asked.
func (a *API) registerRouteHealth(mux Mux) {
	mux.Handle("GET /api/routes/health", a.infraAdmin(a.routeHealth))
}

func (a *API) routeHealth(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.router.Health())
}
