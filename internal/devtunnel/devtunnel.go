// Package devtunnel is the trunk's half of the developer tunnel (DEV-11):
// what is being served right now, and the seam the Enterprise agent plugs
// into.
//
// The split is deliberate. The STATE lives here, in the community trunk,
// because trunk code reads it - the user button, and the banner that tells
// every visitor a service in front of them is coming from someone's laptop.
// The AGENT lives in ee/devplug, because running one is the act that is sold.
//
// That is also why nothing here has to refuse anything: in the community image
// no agent is linked in, so nothing ever writes to the registry, so the
// registry is empty and the banner never appears. A feature that is absent
// declines by itself, without a guard that could be got wrong.
package devtunnel

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/store"
)

// The event types the registry publishes on the live channel. They ride
// TopicAll on purpose: an override changes what the APPLICATION in front of a
// visitor is, so it is news for everyone looking at it, developer or not.
const (
	EventServed   = "served"
	EventUnserved = "unserved"
)

// Served is one cluster name currently answered by somebody's machine.
type Served struct {
	Name string `json:"name"`
	// Who is the identity behind the key. Empty means the key authenticated
	// the SOFTWARE and not a person - what a shared key does, and what
	// standalone plug says about itself.
	Who    string    `json:"who,omitempty"`
	Ports  []string  `json:"ports,omitempty"`
	Parked bool      `json:"parked,omitempty"`
	Since  time.Time `json:"since"`
}

// Registry is what is served right now. In MEMORY, and that is a decision: a
// restart takes the gateway and the agent together, and the agent sweeps its
// leases on the way up, so both come back from an empty state that agrees.
// Persisting it would buy a reconciliation problem and nothing else.
type Registry struct {
	mu    sync.RWMutex
	names map[string]Served
	hub   *events.Hub
}

// NewRegistry builds an empty registry publishing on hub.
func NewRegistry(hub *events.Hub) *Registry {
	return &Registry{names: make(map[string]Served), hub: hub}
}

// Set records a name as served and tells the open pages.
//
// Called AFTER the name is actually being served, never before: announcing an
// intention that then fails would put a name on every screen that nothing
// answers to.
func (r *Registry) Set(s Served) {
	r.mu.Lock()
	r.names[s.Name] = s
	r.mu.Unlock()
	slog.Info("a name is served from a developer's machine", "name", s.Name, "who", s.Who, "parked", s.Parked)
	r.hub.Publish(events.TopicAll, events.Message{Type: EventServed, Data: s})
}

// Drop records that a name went back to the cluster.
//
// Not optional, and not best effort in the sense of "skippable": without it a
// screen shows names nobody is serving, which is worse than showing nothing.
func (r *Registry) Drop(name string) {
	r.mu.Lock()
	_, had := r.names[name]
	delete(r.names, name)
	r.mu.Unlock()
	if !had {
		return
	}
	slog.Info("a name went back to the cluster", "name", name)
	r.hub.Publish(events.TopicAll, events.Message{Type: EventUnserved, Data: Served{Name: name}})
}

// List is what a page asks for at load, before the channel has anything to
// say. Sorted by name so two reads of an unchanged state look unchanged.
func (r *Registry) List() []Served {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Served, 0, len(r.names))
	for _, s := range r.names {
		out = append(out, s)
	}
	sortByName(out)
	return out
}

func sortByName(s []Served) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Name < s[j-1].Name; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// DefaultAddr is where the tunnel listens unless the deployment says
// otherwise. It always has one: turning the tunnel off is done by closing the
// developer surface, not by withholding a port - a feature switched off by an
// empty string is a feature nobody can find the switch for.
const DefaultAddr = ":22222"

// ErrNoResources is what the agent answers when it cannot reach what it needs
// to do the job - no orchestrator, or a process that is not a container the
// orchestrator knows. It is not a failure to report as one: on a workstation
// it is the expected answer, and the gateway goes on serving.
var ErrNoResources = errors.New("the developer tunnel has no access to the resources it needs")

// Deps is what the agent needs from the gateway that hosts it.
type Deps struct {
	Store    *store.Store
	Registry *Registry
	// Addr is where the tunnel listens (MEERKAT_PLUG_ADDR). EMPTY MEANS OFF,
	// and that is the switch: the tunnel plugs a developer's machine into a
	// CLUSTER, so it only makes sense where a cluster is - and naming its port
	// is the deployment saying so.
	//
	// It matters most for the case that is not a deployment at all: a gateway
	// run from a working tree (make dev). There the agent cannot identify its
	// own container, so it cannot provision a single name whatever Docker is
	// doing on that laptop - it would open a port for nothing and log an
	// orchestrator failure at every reload.
	//
	// Above 1024 when it is named: binding a privileged port would want root
	// or CAP_NET_BIND_SERVICE, against the grain of the rest of the image.
	Addr string
}

// Hooks is what the Enterprise package registers from its init.
type Hooks struct {
	// Available answers whether the tunnel can do its job HERE, before
	// anything is announced or bound. Asked separately from Run so the log
	// never says "listening" in front of a line saying it could not start.
	Available func() error
	// Run serves until ctx is cancelled.
	Run func(ctx context.Context, d Deps) error
	// Verb runs ONE tunnel verb in this process and exits. The agent's verbs
	// call os.Exit, which is right for a process whose whole job is one
	// answer and unacceptable inside a gateway - hence a subprocess, and
	// hence this entry point, reached by re-exec'ing the gateway's own binary.
	Verb func()
}

var hooks Hooks

// Register is called from the Enterprise package's init.
func Register(h Hooks) { hooks = h }

// Linked reports whether this binary carries an agent at all.
func Linked() bool { return hooks.Run != nil }

// VerbArg is the hidden first argument that turns a gateway process into one
// verb answer. Hidden because nobody types it: the agent builds the command.
const VerbArg = "__plug-verb"

// RunVerb answers a verb and never returns. Called before anything else in
// main, from a process the agent started.
func RunVerb() {
	if hooks.Verb == nil {
		slog.Error("this image has no built-in developer tunnel, so it has no verbs to answer")
		return
	}
	hooks.Verb()
}

// Run starts the tunnel, or explains why this image will not.
func Run(ctx context.Context, d Deps) error {
	if !Linked() {
		if err := edition.Require("the built-in developer tunnel"); err != nil {
			return err
		}
		return errors.New("devtunnel: no agent was linked into this Enterprise build")
	}
	return hooks.Run(ctx, d)
}

// pollEvery is how often the developer-mode switch is re-read. Rarely flipped,
// and a few seconds late costs nothing: the switch gates a tunnel nobody is
// mid-handshake on.
// A var, not a const, so the tests can run the control path in milliseconds
// instead of paying the real cadence to watch one transition.
var pollEvery = 10 * time.Second

// retryCap bounds the wait between two attempts at a tunnel that will not
// start. Five minutes is short enough that freeing the port is noticed on its
// own, and long enough that the reason is readable in a log.
const retryCap = 5 * time.Minute

// Supervise ties the tunnel to developer mode (DEV-01): mode off, no agent, no
// port, nothing listening. The switch is the whole authorisation - an
// installation that has not asked for the developer surface does not get a
// second way in.
//
// It also means the deployment's extra rights (a Docker socket, a Kubernetes
// role) are only ever exercised by an installation that turned the mode on.
func Supervise(ctx context.Context, d Deps) {
	if !Linked() {
		return // community image: nothing to supervise, and nothing to say about it
	}
	if d.Addr == "" {
		// A port is not how the tunnel is turned off - the developer surface
		// is (DEV-01, and MEERKAT_PRODUCTION above it). So an unset address is
		// simply the default, not an instruction.
		d.Addr = DefaultAddr
	}
	var (
		cancel   context.CancelFunc
		finished chan struct{}
		// retryAfter is how long to wait before trying again once a start has
		// failed. A taken port is the common case, and it is usually a second
		// gateway on the way out - so retry, but back off: an operator who
		// cannot free the port should not have to read the same line every ten
		// seconds for a day.
		retryAfter time.Duration
		nextTry    time.Time
		// lastSaid is the reason already written to the log, so a tunnel that
		// cannot start says why once rather than at every attempt.
		lastSaid string
	)
	stop := func(why string) {
		if cancel != nil {
			cancel()
			cancel, finished = nil, nil
			slog.Info("developer tunnel stopped", "reason", why)
		}
	}
	defer stop("the gateway is shutting down")

	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	for {
		// Did the agent end on its own? A failed bind, a listener that died.
		// The tunnel degrades - it never takes the gateway with it - so the
		// only thing to decide is when to try again.
		if finished != nil {
			select {
			case <-finished:
				cancel, finished = nil, nil
				retryAfter = min(max(2*retryAfter, pollEvery), retryCap)
				nextTry = time.Now().Add(retryAfter)
			default:
			}
		}
		switch {
		case !d.Store.DevMode(ctx):
			stop("developer mode is off")
			retryAfter, nextTry, lastSaid = 0, time.Time{}, ""
		case cancel == nil && time.Now().After(nextTry):
			// Asked first, so the answer is known before anything is
			// announced. On a workstation this is where it stops, every time,
			// and it costs one probe every few minutes.
			if err := hooks.Available(); err != nil {
				retryAfter = min(max(2*retryAfter, pollEvery), retryCap)
				nextTry = time.Now().Add(retryAfter)
				// Said ONCE per reason, not once per attempt: a workstation
				// will never grow a cluster, and repeating the same paragraph
				// all day is how a log stops being read.
				if say := err.Error(); say != lastSaid {
					lastSaid = say
					if errors.Is(err, ErrNoResources) {
						slog.Info("developer tunnel off", "reason", err)
					} else {
						slog.Error("developer tunnel cannot start", "err", err)
					}
				}
				break
			}
			lastSaid = ""
			runCtx, done := context.WithCancel(ctx)
			ended := make(chan struct{})
			cancel, finished = done, ended
			go func() {
				defer done()
				defer close(ended)
				if err := Run(runCtx, d); err != nil && runCtx.Err() == nil {
					slog.Error("developer tunnel stopped", "err", err)
				}
			}()
			// Nothing announced here on purpose: the agent says "ready" on
			// its own port when it is actually bound, and a configured
			// address is not a listening one. Two lines saying the same thing
			// is one line too many, and the wrong one is the one that only
			// knows what was asked for.
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
