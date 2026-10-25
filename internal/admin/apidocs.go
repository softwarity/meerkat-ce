package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/admin/apidocs"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// registerAPIDocs mounts the CONSOLE swagger-ui (DOCS-01): assets vendored in
// the binary (offline-first - nothing comes from a CDN) and Meerkat's OWN
// admin API, served and tried on this very origin. It stays here, on the
// control plane: the data plane never exposes Meerkat's contract. The routes'
// specs are a DEVELOPER matter and live there instead
// (gateway.RegisterDevDocs), where the dev capability is the whole gate.
func (a *API) registerAPIDocs(mux Mux) {
	mux.Handle("GET /apidocs", http.RedirectHandler("/apidocs/", http.StatusMovedPermanently))
	mux.HandleFunc("GET /apidocs/{$}", a.apidocsPage)
	mux.HandleFunc("GET /apidocs/assets/{file}", apidocsAsset)
	mux.Handle("GET /apidocs/specs.json", a.authed(a.apidocsSpecs))
	mux.Handle("GET /apidocs/specs/meerkat-admin.json", a.authed(a.apidocsAdminSpec))
	mux.Handle("GET /apidocs/specs/meerkat-internal.json", a.authed(a.apidocsInternalSpec))
	mux.Handle("GET /apidocs/specs/meerkat-data.json", a.authed(a.apidocsDataSpec))
	mux.Handle("POST /api/apidocs/token", a.devOrInfra(a.mintTestToken))
	mux.Handle("/apidocs/call/{plane}/{rest...}", a.authed(a.apidocsCall))
}

// apidocsPage serves the shell. A browser without a live session is sent to
// the login page (and comes back after) instead of staring at a JSON 401.
func (a *API) apidocsPage(w http.ResponseWriter, r *http.Request) {
	if sess, err := a.sm.Resolve(r.Context(), r); err != nil || sess.Pending != "" {
		http.Redirect(w, r, "/login?next="+url.QueryEscape("/apidocs/"), http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(apidocs.Page)
}

// apidocsAsset serves the vendored swagger-ui files and the Sentinel skin.
// They carry no data, so they are not session-gated; an hour of cache keeps
// the iframe snappy without pinning an old bundle after an upgrade.
func apidocsAsset(w http.ResponseWriter, r *http.Request) {
	var body []byte
	var contentType string
	switch r.PathValue("file") {
	case "swagger-ui-bundle.js":
		body, contentType = apidocs.BundleJS, "application/javascript; charset=utf-8"
	case "swagger-ui.css":
		body, contentType = apidocs.CSS, "text/css; charset=utf-8"
	case "skin.css":
		body, contentType = apidocs.Skin, "text/css; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(body)
}

// specRef is one entry of the spec picker.
type specRef struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	TryIt bool   `json:"tryIt"`
}

// apidocsSpecs: Meerkat's own three planes, the one this page is served on
// first. The routes' specs live on the data plane, for developers.
func (a *API) apidocsSpecs(w http.ResponseWriter, _ *http.Request, _ store.User) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, []specRef{
		{Name: "Control plane - Meerkat Admin API", URL: "specs/meerkat-admin.json", TryIt: true},
		{Name: "Internal plane - for the services of the cluster", URL: "specs/meerkat-internal.json", TryIt: true},
		{Name: "Data plane - what Meerkat answers beside the routes", URL: "specs/meerkat-data.json", TryIt: true},
	})
}

// apidocsAdminSpec serves the embedded admin spec, stamped with the build
// version so the page shows what this binary actually answers. Its servers
// are this origin: Try it out hits /api directly, no tunnel involved.
func (a *API) apidocsAdminSpec(w http.ResponseWriter, _ *http.Request, _ store.User) {
	a.serveSpec(w, apidocs.AdminSpec)
}

// apidocsInternalSpec offers two addresses: this page, which Try it out goes
// through, and the one the services are told (internalOriginOf). When the
// runtime cannot name the latter, the page says what the scheduler's how-to
// says: this gateway, on the internal port.
func (a *API) apidocsInternalSpec(w http.ResponseWriter, r *http.Request, _ store.User) {
	origin := a.internalOriginOf(r.Context())
	if origin == "" {
		origin = "http://<gateway>:" + portOf(a.InternalAddr)
	}
	a.serveSpec(w, apidocs.InternalSpec,
		apidocs.Server{URL: "/apidocs/call/internal", Description: "Through this console (Try it out)"},
		apidocs.Server{URL: origin, Description: "From inside the cluster"})
}

// apidocsDataSpec likewise: this page, then the address the console opens a
// route at (dataOrigin).
func (a *API) apidocsDataSpec(w http.ResponseWriter, r *http.Request, _ store.User) {
	a.serveSpec(w, apidocs.DataSpec,
		apidocs.Server{URL: "/apidocs/call/data", Description: "Through this console (Try it out)"},
		apidocs.Server{URL: a.dataOrigin(r), Description: "Direct"})
}

// apidocsCall runs a Try it out of the internal or the data plane's
// reference. A browser reaches neither from this page - the internal plane is
// inside the cluster, the data plane another origin with its own session - so
// the call is handed to that plane's handler in process, as the scheduler and
// the agent's call_route do.
//
// What crosses is what the page sent minus this plane's cookies: the plane
// called authenticates the request by its own means, the Authorization header
// the reader filled in. The console session only decides who may use this
// door: a person signed in here (never a token, which can call the plane
// itself), and for the internal plane root or an infra admin, whose plane it
// is.
func (a *API) apidocsCall(w http.ResponseWriter, r *http.Request, actor store.User) {
	var plane http.Handler
	switch r.PathValue("plane") {
	case "internal":
		if !actor.Root && !actor.InfraAdmin {
			writeErr(w, http.StatusForbidden, "calling the internal plane from here requires root or the infra-admin capability")
			return
		}
		plane = a.InternalPlane
	case "data":
		plane = a.DataPlane
	}
	if plane == nil {
		writeErr(w, http.StatusNotFound, "unknown plane: internal or data")
		return
	}
	if sess, err := a.sm.Resolve(r.Context(), r); err != nil || sess.TokenID != "" {
		writeErr(w, http.StatusForbidden, "Try it out is for a person signed in to the console: a token calls the plane itself")
		return
	}
	out := r.Clone(r.Context())
	out.URL.Path = "/" + r.PathValue("rest")
	out.URL.RawPath = ""
	out.RequestURI = ""
	out.Header.Del("Cookie")
	out.Header.Del("Origin")
	out.Header.Del("Sec-Fetch-Site")
	plane.ServeHTTP(w, out)
}

func (a *API) serveSpec(w http.ResponseWriter, spec []byte, servers ...apidocs.Server) {
	out, err := apidocs.Served(spec, version.Version, servers...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(out)
}

// testTokenRequest is the API screen's mint form: an identity to impersonate
// and a bounded lifetime.
type testTokenRequest struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Minutes  int      `json:"minutes"`
}

// mintTestToken issues an ephemeral test token (gateway/simulate.go): sent as
// `Authorization: Bearer mksim_...` to the data plane (swagger, curl, Postman),
// it carries the whole authorization - the same capabilities that may
// simulate by header may mint.
func (a *API) mintTestToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body testTokenRequest
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" {
		writeErr(w, http.StatusUnprocessableEntity, "a username is required (the identity the token impersonates)")
		return
	}
	if body.Minutes < 1 || body.Minutes > 60 {
		body.Minutes = 15
	}
	token, exp := a.router.MintSimulationToken(body.Username, body.Roles, time.Duration(body.Minutes)*time.Minute)
	a.auditEvent(r.Context(), actor, "token.simulate", "token", "", body.Username, "",
		fmt.Sprintf("test token as %q roles %v for %dm", body.Username, body.Roles, body.Minutes))
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "expiresAt": exp.Unix()})
}
