package admin

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/store"
)

// What passed through, over the last hour (OBS-01).
//
// The window is in memory and pooled across the cluster: every node broadcasts
// its running totals and every node aggregates, so this answers for the whole
// gateway rather than for whichever one the load balancer handed the screen.
// Each sample says how many nodes it covers, which is what lets the console
// name the scope instead of letting a partial curve pass for a total.
//
// A READ of a live window, not a query over stored history: nothing is written
// down, and a restart starts a new hour. An installation that wants a year
// scrapes /metrics into the time-series database it already runs, which is the
// Enterprise half of this feature. See internal/metrics for why a gateway
// should not grow into one badly.
// expositionPath is where a Prometheus scrapes (OBS-05). On the CONTROL plane,
// not the data plane: what it carries is how this gateway is running, which is
// an operator's business and not an application's traffic.
//
// The whole path, not "GET /metrics": the console's catch-all sits under "/",
// so a scraper reaching a mistyped method would be answered by the
// single-page app with a 200 and some HTML, which reads as a working target
// forever. Same reason the agent endpoint takes its own path.
const expositionPath = "/metrics"

// exposition renders the counters in Prometheus' text format. Nil in the
// community image, which is what makes /metrics an Enterprise endpoint: the
// code that knows the format is in ee/prometheus, and the community binary
// does not link it. Absent code refuses by itself; this only has to say so.
var exposition func(w io.Writer, reg *metrics.Registry)

// RegisterExposition is called from ee/prometheus' init().
func RegisterExposition(f func(w io.Writer, reg *metrics.Registry)) { exposition = f }

func (a *API) registerMetrics(mux Mux) {
	mux.Handle("GET /api/metrics", a.infraAdmin(a.readMetrics))
	mux.Handle("GET /api/settings/metrics", a.rootOnly(a.getMetricsSetting))
	mux.Handle("PUT /api/settings/metrics", a.rootOnly(a.putMetricsSetting))
	// Behind the same guard as the screen showing the same numbers, and NOT
	// merely behind a session: the counters name every route, every route's
	// name and every endpoint template, which is an operational map of the
	// installation. An application's user signs in through this gateway and
	// has no business reading it - the access matrix caught exactly that,
	// with a plain user answered 200.
	//
	// The token's perimeter is still checked where perimeters are checked
	// (admin.authed), and a control-plane token is root's, so a scraper's
	// passes this. Mounted unconditionally and answering WHY when it will not
	// serve: an endpoint that exists only in one build is one the coverage
	// test cannot see.
	mux.Handle(expositionPath, a.infraAdmin(a.serveExposition))
	// The live channel, BEHIND the same funnel as everything else: a
	// subscription passes exactly the checks an API call passes - session,
	// pending login, token perimeter, narrowing to the token's domain - and
	// the socket is only upgraded once they have. Writing a second predicate
	// for it would be writing the security boundary twice, and the second copy
	// is the one that drifts.
	//
	// Mounted UNCONDITIONALLY, and answering 503 when nothing feeds it. Behind
	// an `if` it would have been a route that exists only in production, and
	// the test that refuses an undecided section inspects the surface a bare
	// API registers - so an endpoint hidden behind a field would have slipped
	// past the one check written to stop exactly that.
	mux.Handle("GET /api/live", a.infraAdmin(func(w http.ResponseWriter, r *http.Request, _ store.User) {
		if a.Live == nil {
			writeErr(w, http.StatusServiceUnavailable, "this build runs no live channel")
			return
		}
		a.Live.ServeHTTP(w, r)
	}))
}

// metricsSetting is the switch, and it ships OFF. The token is the lock; this
// is the decision that the door exists at all, taken by a person rather than
// inherited from an upgrade - the same shape as the agent endpoint.
type metricsSetting struct {
	Enabled bool `json:"enabled"`
	// Enterprise says whether this build can serve it at all, so the console
	// shows the switch as locked rather than as off. Read-only: an edition is
	// decided by which image is running, never by a request.
	Enterprise bool `json:"enterprise"`
	// Path is where a scraper points, sent rather than assembled in the
	// console: one place decides it.
	Path string `json:"path"`
}

func (a *API) metricsEnabled(ctx context.Context) bool {
	var enabled bool
	_ = a.st.GetSetting(ctx, store.SettingMetricsEndpoint, &enabled)
	return enabled
}

func (a *API) getMetricsSetting(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, metricsSetting{
		Enabled: a.metricsEnabled(r.Context()), Enterprise: edition.Enterprise, Path: expositionPath,
	})
}

func (a *API) putMetricsSetting(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body metricsSetting
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	if body.Enabled {
		if err := edition.Require("exposing " + expositionPath + " to a monitoring stack"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	before := metricsSetting{Enabled: a.metricsEnabled(r.Context())}
	if err := a.st.SetSetting(r.Context(), store.SettingMetricsEndpoint, body.Enabled); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "metrics.expose", "settings", "", "", "",
		metricsSetting{Enabled: before.Enabled}, metricsSetting{Enabled: body.Enabled})
	writeJSON(w, http.StatusOK, metricsSetting{
		Enabled: body.Enabled, Enterprise: edition.Enterprise, Path: expositionPath,
	})
}

// serveExposition answers a scrape, or says which of the three things is
// missing: the edition, the switch, or something to count.
func (a *API) serveExposition(w http.ResponseWriter, r *http.Request, _ store.User) {
	if exposition == nil {
		writeErr(w, http.StatusForbidden,
			"exposing "+expositionPath+" is part of the Enterprise edition, and this is the community image: "+
				"the built-in metrics screen works here with nothing to install")
		return
	}
	if !a.metricsEnabled(r.Context()) {
		writeErr(w, http.StatusForbidden,
			"this gateway does not expose "+expositionPath+": turn it on in the console, under Metrics, Prometheus")
		return
	}
	reg := a.registry()
	if reg == nil {
		writeErr(w, http.StatusServiceUnavailable, "this build counts nothing to expose")
		return
	}
	// The version is part of the content type, not a courtesy: a scraper that
	// guesses the format is one that breaks on a body it half understands.
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	exposition(w, reg)
}

func (a *API) readMetrics(w http.ResponseWriter, r *http.Request, _ store.User) {
	answer := metricsAnswer{}
	if a.Metrics != nil {
		samples := a.Metrics.Samples()
		// `minutes` trims the window from the left, so a screen showing five
		// minutes does not carry an hour over the wire on every refresh. Zero
		// asks for none of it, which is what a caller after the endpoint
		// ranking alone wants.
		if n, err := strconv.Atoi(r.URL.Query().Get("minutes")); err == nil && n >= 0 {
			want := n * 60 / int(metrics.DefaultInterval.Seconds())
			if want < len(samples) {
				samples = samples[len(samples)-want:]
			}
		}
		answer.Interval = int(metrics.DefaultInterval.Seconds())
		answer.Buckets = metrics.Buckets
		answer.Samples = samples
	}
	if r.URL.Query().Get("endpoints") != "" {
		// The period the CALLER is looking at, so what a route's endpoints add
		// up to is what the route's own row says. Milliseconds, which is what
		// a browser has to hand from the sample it is drawing.
		var from time.Time
		if ms, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64); err == nil && ms > 0 {
			from = time.UnixMilli(ms)
		}
		answer.endpoints(a.registry(), from, 0)
	}
	writeJSON(w, http.StatusOK, answer)
}

// registry is this node's counters, or nil when there is no data plane to ask
// - which is how the admin API is built in half the tests.
func (a *API) registry() *metrics.Registry {
	if a.router == nil {
		return nil
	}
	return a.router.Metrics()
}

type metricsAnswer struct {
	// Interval is how many seconds one sample covers, so a reader turns counts
	// into rates without assuming the period.
	Interval int `json:"interval"`
	// Buckets are the histogram's boundaries in seconds, in order. The last
	// bucket of every sample is what fell past the final boundary.
	Buckets []float64        `json:"buckets"`
	Samples []metrics.Sample `json:"samples"`

	// ── which endpoint, over the SAME period ──────────────────────────────
	//
	// Asked for with `since`, and answered over it: what a route's endpoints
	// add up to has to be what the route's own row says, or the table is one
	// nobody can read. They are still THIS node's - the samples above are
	// summed over the cluster - which is the one thing left to say, and only
	// on a cluster.
	//
	// Not the sampled window, though: see internal/metrics/history.go for what
	// an endpoint keeps instead and why it is deliberately coarser. Since is
	// when this node started counting, so a caller asking for more than the
	// process has lived is told what it actually got.
	Since     *time.Time                 `json:"since,omitempty"`
	Endpoints []metrics.EndpointSnapshot `json:"endpoints,omitempty"`
}

// endpoints fills the per-endpoint half over the period starting at from,
// keeping at most top of them (0 for all).
func (m *metricsAnswer) endpoints(reg *metrics.Registry, from time.Time, top int) {
	if reg == nil {
		return
	}
	since := reg.Since()
	if from.After(since) {
		since = from
	}
	m.Since = &since
	m.Endpoints = reg.Operations(from)
	if top > 0 && len(m.Endpoints) > top {
		m.Endpoints = m.Endpoints[:top]
	}
}
