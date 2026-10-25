package main

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/admin"
	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/notifier"
	"github.com/softwarity/meerkat/internal/oidcprovider"
)

// What each plane mounts of Meerkat's own, apart from what serves it.
//
// Three functions rather than lines spread through main, for one reason: the
// API reference documents each plane (internal/admin/apidocs), and a document
// written by hand is only true while something compares it with the code.
// apidocs_test.go calls these same functions on a recorder, so an endpoint
// added to a plane and not to its document fails the build, and so does a
// documented one nobody mounts.
//
// Not here: the routes on the data plane (the "/" fallback, which is the
// integrator's, documented by the routes' own specs) and the console on the
// control plane (pages, not an API).

// planeMux is what a plane is mounted on: a *http.ServeMux, or the test's
// recorder.
type planeMux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// mountProbes mounts what the orchestrator asks a plane: alive, and ready to
// serve. ready is nil on the internal plane, which only says alive.
func mountProbes(mux planeMux, ready http.HandlerFunc) {
	mux.HandleFunc("GET /healthz", healthz)
	if ready != nil {
		mux.HandleFunc("GET /readyz", ready)
	}
}

// mountData mounts the data plane's own endpoints: the sign-in flow and the
// profile, the developer docs and the UI test mode, the OpenID provider and
// the notifications.
func mountData(mux planeMux, ready http.HandlerFunc, pages *auth.Handler, router *gateway.Router,
	oidc oidcprovider.Provider, notify notifier.Notifier) {
	mountProbes(mux, ready)
	pages.Register(mux)
	router.RegisterDevDocs(mux) // /meerkat/apidocs - developer docs (dev capability)
	router.RegisterUISim(mux)   // /meerkat/dev-sim - UI test mode (dev capability)
	oidc.MountData(mux)
	notify.MountData(mux)
}

// mountAdmin mounts the control plane: its own sign-in, and the admin API.
func mountAdmin(mux planeMux, ready http.HandlerFunc, pages *auth.Handler, api *admin.API) {
	mountProbes(mux, ready)
	pages.Register(mux)
	api.Register(mux)
}

// mountInternal mounts the internal plane: the keys, the scheduler, the
// notifications the services publish.
func mountInternal(mux planeMux, api *admin.API) {
	mountProbes(mux, nil)
	api.RegisterInternal(mux)
}
