// Package instances keeps the list of gateways serving one installation: each
// says, every few seconds, who it is, which version it runs and whether it can
// take a request, and every node keeps what the others said. The
// Infrastructure screen draws it, and the license counts it.
//
// Nothing is written to the database for this. A node that stops speaking is
// dropped after StaleAfter, which is what "it is gone" means here: a crashed
// pod says nothing on its way out.
package instances

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// Every is how often a node speaks; StaleAfter how long its silence is
// forgiven - four missed words, so one lost message never drops a node.
const (
	Every      = 5 * time.Second
	StaleAfter = 4 * Every
)

// Instance is one gateway as it describes itself.
type Instance struct {
	// ID is the node's identity for the life of its process (cluster.NodeID).
	ID string `json:"id"`
	// Host is its host name: the pod's name on Kubernetes, the container's
	// on Docker - what an operator finds it by.
	Host    string    `json:"host"`
	Version string    `json:"version"`
	Started time.Time `json:"started"`
	// Ready is what its /readyz would answer, and Reason why not.
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
	// Self marks the node that answered: filled on the way out, never sent.
	Self bool `json:"self,omitempty"`
	// Leaving is a node's last word: it is stopping, drop it now rather than
	// after its silence. Never shown.
	Leaving bool `json:"leaving,omitempty"`
	seen    time.Time
}

// Registry is every node this one has heard from, itself included.
type Registry struct {
	mu    sync.Mutex
	self  string
	nodes map[string]Instance
	// Moved is told when the list changes - a node came, went, or changed
	// version or readiness - and not on every word: the screen re-reads on it.
	Moved func()
	// now is the clock, a test's to set.
	now func() time.Time
}

// New returns a registry for the node self.
func New(self string) *Registry {
	return &Registry{self: self, nodes: map[string]Instance{}, now: time.Now}
}

// live says a node counts at this instant: this one always, another while its
// last word is younger than StaleAfter. Asked at READ time and not only at the
// next Check: behind a load balancer the screen is told by one node and reads
// from another, and the two must agree on who is gone - they heard the same
// words at the same time, so judged at the same instant they do.
func (r *Registry) live(id string, n Instance, now time.Time) bool {
	return id == r.self || now.Sub(n.seen) <= StaleAfter
}

// Report records what one node says of itself, at the time it was heard.
func (r *Registry) Report(in Instance, at time.Time) {
	if in.Leaving {
		r.forget(in.ID)
		return
	}
	r.mu.Lock()
	before, known := r.nodes[in.ID]
	in.Self, in.seen = false, at
	r.nodes[in.ID] = in
	changed := !known || before.Host != in.Host || before.Version != in.Version ||
		before.Ready != in.Ready || before.Reason != in.Reason || !before.Started.Equal(in.Started)
	moved := r.Moved
	r.mu.Unlock()
	if changed && moved != nil {
		moved()
	}
}

// forget drops a node that said it was leaving.
func (r *Registry) forget(id string) {
	r.mu.Lock()
	_, known := r.nodes[id]
	delete(r.nodes, id)
	moved := r.Moved
	r.mu.Unlock()
	if known && moved != nil {
		moved()
	}
}

// Check drops the nodes silent for longer than StaleAfter, and says so.
func (r *Registry) Check(now time.Time) {
	r.mu.Lock()
	dropped := false
	for id, n := range r.nodes {
		if !r.live(id, n, now) {
			delete(r.nodes, id)
			dropped = true
		}
	}
	moved := r.Moved
	r.mu.Unlock()
	if dropped && moved != nil {
		moved()
	}
}

// List is the nodes, oldest first: the order they joined in, which does not
// reshuffle while a rolling update brings new ones.
func (r *Registry) List() []Instance {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	out := make([]Instance, 0, len(r.nodes))
	for id, n := range r.nodes {
		if !r.live(id, n, now) {
			continue
		}
		n.Self = n.ID == r.self
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Started.Equal(out[j].Started) {
			return out[i].Started.Before(out[j].Started)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Count is how many nodes run, at least this one.
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	now, n := r.now(), 0
	for id, in := range r.nodes {
		if r.live(id, in, now) {
			n++
		}
	}
	return max(n, 1)
}

// Encode is the word a node sends of itself.
func Encode(in Instance) string {
	in.Self = false
	b, _ := json.Marshal(in)
	return string(b)
}

// Decode reads a word, the sender's identity taken from the bus rather than
// from what it says.
func Decode(from, msg string) (Instance, error) {
	var in Instance
	if err := json.Unmarshal([]byte(msg), &in); err != nil {
		return Instance{}, err
	}
	in.ID = from
	return in, nil
}
