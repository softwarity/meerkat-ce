package admin

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/instances"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
)

// The Infrastructure screen: which gateways serve this installation, on which
// database, kept in step how, and what the license says of their number. Read
// only; it re-reads on the instances topic, which moves when a node comes,
// goes, or changes version or readiness.

func (a *API) registerInfrastructure(mux Mux) {
	mux.Handle("GET /api/infrastructure", a.infraAdmin(a.getInfrastructure))
}

type infrastructure struct {
	Instances []instances.Instance `json:"instances"`
	// Versions are the distinct versions running, oldest node's first: more
	// than one is an update in progress, or one that stopped half way.
	Versions []string `json:"versions"`
	Database struct {
		// Kind is "embedded" or "postgres".
		Kind string `json:"kind"`
		// Location is the file, or the server and database: never a password.
		Location string `json:"location"`
		Schema   int    `json:"schema,omitempty"`
	} `json:"database"`
	// Synced says the nodes are kept in step by the change bus: a shared
	// database. On the embedded one there is one process and nothing to sync.
	Synced     bool `json:"synced"`
	Enterprise bool `json:"enterprise"`
	// Plan is the license in force ("team", "enterprise"), absent for none;
	// Covers how many instances it covers, absent for no ceiling.
	Plan   string `json:"plan,omitempty"`
	Covers int    `json:"covers,omitempty"`
	// Services are what the routes point at, one per address, the ones that
	// want a look first. Only what a route declares: nothing is discovered
	// here, and a route that answers by itself has no service.
	Services []infraService `json:"services"`
	// Over is set while more instances run than the license covers: since
	// when, and when the evaluation notice comes back (or came back).
	Over *struct {
		Since    time.Time `json:"since"`
		NoticeAt time.Time `json:"noticeAt"`
	} `json:"over,omitempty"`
}

// infraService is one address behind the routes, as this node sees it.
type infraService struct {
	// Address is host:port, the port said even when the scheme implies it.
	Address string `json:"address"`
	// Routes are the routes that point at it, by id and name.
	Routes []infraRoute `json:"routes"`
	// UI says one of them serves pages to people: the console draws it as an
	// application rather than an API.
	UI bool `json:"ui"`
	// State is up, degraded, down or unknown (nothing found out yet), the
	// worst of its routes'; Why says it in a few words, At when (unix seconds).
	State string `json:"state"`
	Why   string `json:"why,omitempty"`
	At    int64  `json:"at,omitempty"`
	// Ready and Wanted are the runtime's replica count, when it counts them.
	Ready  int `json:"ready,omitempty"`
	Wanted int `json:"wanted,omitempty"`
}

type infraRoute struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// stateRank orders the states from the one that wants a look.
var stateRank = map[string]int{"down": 0, "degraded": 1, "unknown": 2, "up": 3}

// services groups the enabled routes by the address they point at, and gives
// each the worst health of its routes (gateway.Router.Health, this node's).
func (a *API) services(ctx context.Context) []infraService {
	out := []infraService{}
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return out
	}
	var health map[string]gateway.RouteHealth
	if a.router != nil {
		health = a.router.Health()
	}
	by := map[string]*infraService{}
	var order []string
	for _, r := range routes {
		addr := upstreamAddress(r.Upstream)
		if !r.Enabled || addr == "" {
			continue
		}
		svc, ok := by[addr]
		if !ok {
			svc = &infraService{Address: addr, State: "unknown"}
			by[addr] = svc
			order = append(order, addr)
		}
		svc.Routes = append(svc.Routes, infraRoute{ID: r.ID, Name: r.Name})
		svc.UI = svc.UI || r.IsUI
		h, known := health[r.ID]
		if !known {
			continue
		}
		state, why, at := "unknown", "", int64(0)
		switch {
		case h.State == "open":
			state, why, at = "down", h.LastError, h.LastAt
		case h.Target != "":
			state, why, at = h.Target, h.TargetWhy, h.TargetAt
		case h.LastOKAt > 0:
			state, at = "up", h.LastOKAt
		}
		if stateRank[state] < stateRank[svc.State] || svc.State == "unknown" && state != "unknown" {
			svc.State, svc.Why, svc.At = state, why, at
			svc.Ready, svc.Wanted = 0, 0
			if h.Replicas != nil {
				svc.Ready, svc.Wanted = h.Replicas.Ready, h.Replicas.Wanted
			}
		}
	}
	for _, addr := range order {
		out = append(out, *by[addr])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if stateRank[out[i].State] != stateRank[out[j].State] {
			return stateRank[out[i].State] < stateRank[out[j].State]
		}
		return out[i].Address < out[j].Address
	})
	return out
}

// upstreamAddress is an upstream URL's host:port, "" when it names none.
func upstreamAddress(upstream string) string {
	u, err := url.Parse(strings.TrimSpace(upstream))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https", "wss":
			port = "443"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func (a *API) getInfrastructure(w http.ResponseWriter, r *http.Request, _ store.User) {
	out := infrastructure{Enterprise: edition.Enterprise, Instances: []instances.Instance{}, Versions: []string{}}
	if a.Instances != nil {
		out.Instances = a.Instances.List()
	}
	seen := map[string]bool{}
	for _, n := range out.Instances {
		if !seen[n.Version] {
			seen[n.Version] = true
			out.Versions = append(out.Versions, n.Version)
		}
	}

	out.Database.Kind = "embedded"
	out.Database.Location = strings.TrimPrefix(strings.TrimPrefix(a.st.Location(), "sqlite:"), "postgres:")
	if a.st.Dialect() == "postgres" {
		out.Database.Kind = "postgres"
		out.Synced = true
	}
	if v, err := a.st.SchemaVersion(); err == nil {
		out.Database.Schema = v
	}

	if l := license.Current(); l != nil {
		out.Plan, out.Covers = l.Edition, l.Instances
		if since := license.OverSince(); !since.IsZero() {
			out.Over = &struct {
				Since    time.Time `json:"since"`
				NoticeAt time.Time `json:"noticeAt"`
			}{Since: since, NoticeAt: since.Add(license.InstanceGrace)}
		}
	}
	out.Services = a.services(r.Context())
	writeJSON(w, http.StatusOK, out)
}
