package admin

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/store"
)

// What the runtime says exists beside this gateway (SVC-02).
//
// It answers the moment a route is created: an upstream typed by hand is where
// the typos live, and none of them show up until traffic does. The runtime
// already knows, so the console offers the list.
//
// Always available, in both editions, with no switch in front of it. That is
// deliberate: the deployment should be the SAME whichever image is running, and
// a discovery that needs turning on is a discovery nobody has when they need
// it. What gates it is what the deployment grants - a mounted socket, a service
// account - which is a decision taken where the gateway is installed rather
// than one taken twice.
//
// Under the infra perimeter, like the routes it feeds.
func (a *API) registerServices(mux Mux) {
	mux.Handle("GET /api/services", a.infraAdmin(a.listServices))
}

func (a *API) listServices(w http.ResponseWriter, r *http.Request, _ store.User) {
	// A runtime that cannot be reached is not an error: it is an installation
	// without one, and the answer says so in a sentence naming what would make
	// it work. A 500 here would put a red box in front of a field that works
	// perfectly well typed by hand.
	writeJSON(w, http.StatusOK, discovery.Discover(r.Context()))
}
