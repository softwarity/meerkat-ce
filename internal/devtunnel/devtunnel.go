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
	"fmt"
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

// Registry is what is served right now, ON EVERY NODE.
//
// In MEMORY, and that is a decision: a restart takes the gateway and the agent
// together, and the agent sweeps its leases on the way up, so both come back
// from an empty state that agrees. Persisting it would buy a reconciliation
// problem and nothing else.
//
// On a cluster the TRAFFIC needs nothing from this: plug plants a signpost
// service on the overlay and relays it to the task holding the session, so a
// plugged name answers from whichever gateway a request lands on. What was
// node-local was only the TELLING - and that is the worse half to lose, since
// the developer's code arrives either way and nothing on the other nodes said
// so. Hence a node's own list travels to the others (store.TopicServed) and
// each keeps one entry per node, the way the metrics fleet keeps totals: a
// node that stops talking expires, a node that starts late is correct within
// one interval, and nothing has to be reconciled.
type Registry struct {
	mu    sync.RWMutex
	names map[string]Served
	hub   *events.Hub
	// What the OTHER nodes say they serve, by node id, with when they last
	// said it. Never this node: its own answer is names above, so there is no
	// privileged member and one gateway takes the same path as five.
	remote map[string]remoteNames
	// StaleAfter is how long a silent node keeps its place. Four intervals,
	// for the metrics fleet's reason: enough that a lost message costs
	// nothing, short enough that a node that is gone stops being believed.
	StaleAfter time.Duration
	// publish carries this node's whole list to the others. nil until the
	// cluster wiring sets it, which is also the single-node case.
	publish func([]Served)
	now     func() time.Time
}

type remoteNames struct {
	names []Served
	at    time.Time
}

// DefaultStaleAfter is four report intervals.
const DefaultStaleAfter = 20 * time.Second

// NewRegistry builds an empty registry publishing on hub.
func NewRegistry(hub *events.Hub) *Registry {
	return &Registry{
		names: make(map[string]Served), hub: hub,
		remote: map[string]remoteNames{}, StaleAfter: DefaultStaleAfter,
		now: time.Now,
	}
}

// Relay wires the registry to the cluster: publish carries this node's list to
// the others, and Report brings theirs in. Called once, before anything is
// served. A registry with no relay is a single-node registry and behaves
// exactly as it did.
func (r *Registry) Relay(publish func([]Served)) {
	r.mu.Lock()
	r.publish = publish
	r.mu.Unlock()
}

// Report records what one OTHER node says it serves. The whole list every
// time, not a delta: a receiver replaces that node's set, so a message lost is
// a message the next one repairs and there is no order to preserve.
func (r *Registry) Report(node string, names []Served, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remote[node] = remoteNames{names: names, at: at}
}

// Mine is this node's own list, which is what travels.
func (r *Registry) Mine() []Served {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mineLocked()
}

func (r *Registry) mineLocked() []Served {
	out := make([]Served, 0, len(r.names))
	for _, s := range r.names {
		out = append(out, s)
	}
	sortByName(out)
	return out
}

// tell hands this node's list to the cluster, if there is one to hand it to.
// Called outside the lock: the publisher writes to the database.
func (r *Registry) tell() {
	r.mu.RLock()
	pub, mine := r.publish, r.mineLocked()
	r.mu.RUnlock()
	if pub != nil {
		pub(mine)
	}
}

// Tell publishes this node's list on a timer, so a node that starts after a
// session began learns about it, and one that stops talking is forgotten by
// the others. A node serving nothing stays quiet - its entries expire on their
// own, and a cluster where nobody plugs anything writes nothing at all.
func (r *Registry) Tell(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Forget()
			if len(r.Mine()) > 0 {
				r.tell()
			}
		}
	}
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
	r.tell()
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
	// Told at once, and told even when the list is now empty: waiting for it
	// to expire would leave a name on four intervals' worth of screens.
	r.tell()
}

// List is what a page asks for at load, before the channel has anything to
// say - THIS node's names and the other nodes'. Sorted by name so two reads of
// an unchanged state look unchanged.
//
// A name held by two nodes at once cannot happen (the agent refuses a name a
// live session already holds, whichever task it landed on), but a node on its
// way out and its replacement can overlap for an interval. This node's own
// answer wins then, and a remote one only fills a gap: the node answering the
// page is the one whose state a reader can check.
func (r *Registry) List() []Served {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool, len(r.names))
	out := make([]Served, 0, len(r.names))
	for _, s := range r.names {
		out = append(out, s)
		seen[s.Name] = true
	}
	cutoff := r.now().Add(-r.StaleAfter)
	for _, rn := range r.remote {
		if rn.at.Before(cutoff) {
			continue
		}
		for _, s := range rn.names {
			if !seen[s.Name] {
				out = append(out, s)
				seen[s.Name] = true
			}
		}
	}
	sortByName(out)
	return out
}

// Forget drops the nodes that have gone quiet. List already ignores them; this
// is what keeps the map from holding a name for every node that ever ran.
func (r *Registry) Forget() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := r.now().Add(-r.StaleAfter)
	for id, rn := range r.remote {
		if rn.at.Before(cutoff) {
			delete(r.remote, id)
		}
	}
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

// DownloadArg answers the ANONYMOUS `get` account, the one a developer reaches
// with `ssh get@gateway install | sh` before they have a client at all.
//
// Standalone plug hands back its own binaries from a script in its image. A
// gateway has none to hand back - it embeds the agent, not the client - and
// left to the default it fork/execs a path that does not exist and says
// "no such file or directory", which tells nobody anything.
//
// So it answers a script instead. It is piped straight into a shell, so what
// it prints IS the answer: a line naming where the client lives, and a
// non-zero exit so nothing downstream believes an install happened.
const DownloadArg = "__plug-download"

// PrintDownloadNotice writes that script. Kept in the trunk rather than beside
// the agent because the sentence is about this PRODUCT, not about the tunnel.
func PrintDownloadNotice() {
	fmt.Println(`echo "This gateway runs the plug agent, but it does not distribute the plug client." >&2`)
	fmt.Println(`echo "Install it from https://github.com/softwarity/plug (releases), then point it here." >&2`)
	fmt.Println(`exit 1`)
}

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
