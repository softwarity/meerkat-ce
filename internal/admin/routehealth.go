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
// It costs nothing to collect. The circuit breaker (ROUTE-09) already watches
// every answer to decide whether to keep calling, so this reports what it
// knows rather than probing on its own - and a separate prober would have been
// a second opinion, formed on traffic nobody sent, about a path the real
// requests may not even take. The cost is that a route nobody has called yet
// has nothing to say, which is honest: it is exactly what the gateway knows.
//
// Per NODE, like the breaker it reads. Two gateways may genuinely disagree
// about an upstream - a network path, a DNS answer, a sidecar - and this
// answers for the one that was asked.
func (a *API) registerRouteHealth(mux Mux) {
	mux.Handle("GET /api/routes/health", a.infraAdmin(a.routeHealth))
}

func (a *API) routeHealth(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.router.Health())
}
