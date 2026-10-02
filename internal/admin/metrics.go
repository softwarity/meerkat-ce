package admin

import (
	"net/http"
	"strconv"
	"time"

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
// pushes the counters over OTLP to a collector (Infra, OpenTelemetry), which
// writes them into the time-series database it already runs. See
// internal/metrics for why a gateway should not grow into one badly.
func (a *API) registerMetrics(mux Mux) {
	mux.Handle("GET /api/metrics", a.infraAdmin(a.readMetrics))
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
	// It answers every ADMINISTRATOR and not only the routing plane's, because
	// what is on it is no longer only the traffic curves: the screens of both
	// planes watch it to learn that what they are showing moved (CONSOLE-13).
	// It also closed a hole rather than opening one - the scheduled calls answer
	// an application administrator (/api/schedules) while their screen draws
	// from this socket, so that screen stayed empty for exactly the person the
	// API was opened to.
	//
	// What a subscriber may then WATCH is decided per caller, here, and handed
	// to the channel: the library shares one read between the subscribers of a
	// topic and cannot tell them apart, so a perimeter chosen anywhere but at
	// the upgrade would not be a perimeter at all.
	mux.Handle("GET /api/live", a.authed(func(w http.ResponseWriter, r *http.Request, actor store.User) {
		if a.Live == nil {
			writeErr(w, http.StatusServiceUnavailable, "this build runs no live channel")
			return
		}
		p, ok := a.livePerimeter(r.Context(), actor)
		if !ok {
			writeErr(w, http.StatusForbidden,
				"the live channel answers an administrator: root, the gateway-admin or app-admin capability, "+
					"or the administration of an organisation")
			return
		}
		a.Live(p, w, r)
	}))
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
