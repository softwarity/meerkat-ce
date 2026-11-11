// Package live keeps a console screen in step with what the gateway knows.
//
// One websocket for the whole control plane, and a REGISTRY of sources on it:
// a screen subscribes to a topic and receives its answer, then every answer
// after it. This is the transport, and it is deliberately not owned by any one
// screen - the traffic curves are its first user, the audit trail, the issue
// list and the routes' health are the obvious next ones, and a second socket
// per screen would be a second thing to authorise, keep alive and debug.
//
// WHY NOT THE ONE THE DATA PLANE ALREADY HAS. internal/events serves the
// pages this gateway injects into, and it is one-way by design: it pushes text
// frames and answers a ping, and its own comment says reading client data is
// deliberately absent. A console screen SPEAKS - it subscribes, unsubscribes
// and moves its window as somebody scrolls - which is the other half of RFC
// 6455, the half with the unmasking and the reassembly. Two mechanisms because
// they answer two different problems, not because nobody looked.
//
// AUTHORISATION IS NOT DECIDED HERE, IT IS APPLIED HERE. The handler is mounted
// BEHIND the control plane's own funnel, so a subscription passes exactly the
// checks an API call passes - session, pending login, token perimeter, narrowing
// to the token's domain - and the funnel also says WHAT this caller may watch
// (admin.livePerimeter, read from the audit trail's own partition). ServeFor
// puts that answer on the request that opens the socket, and the library hands
// that request to every subscription of the socket (SPEC 5.5): a topic this
// reader may not watch refuses the subscription, and the changes journal reads
// which kinds it may describe. Deciding it a second time here - in a predicate
// of our own over the caller - would be writing the boundary twice, and the
// second copy is the one that drifts.
package live

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"

	livewire "github.com/softwarity/livewire/go"
)

// Server is the console's live channel: one registry, one handler, for every
// reader. What a reader may watch travels with the socket, not in a registry of
// its own.
type Server struct {
	handler http.Handler
	topics  map[string]Topic
}

// Perimeter is one reader's view of the channel. It mirrors admin.LivePerimeter,
// which main translates: the control plane decides who sees what, this package
// decides which source serves it.
type Perimeter struct {
	// Named are the kinds whose last write may be described, Quiet the kinds the
	// reader is only told moved.
	Named []string
	Quiet []string
	// Whether this reader administers the routing plane (the traffic curves) and
	// the application's (the scheduled calls).
	Traffic   bool
	Schedules bool
}

// Topic is one source on the channel and who may watch it.
type Topic struct {
	Source livewire.Source
	// May answers whether a reader of this perimeter may subscribe. Nil is
	// everyone the funnel let in: a counter that names nothing, whose answer is
	// read through an endpoint that scopes it.
	May func(Perimeter) bool
}

// New returns the channel: the changes journal, whose rows depend on who is
// reading, and the other topics, each with who may watch it.
func New(changes *Changes, topics map[string]Topic) *Server {
	registry := livewire.NewRegistry(0)
	registry.RegisterWithContext(ChangesTopic, changes)
	for topic, t := range topics {
		if t.May == nil {
			registry.Register(topic, t.Source)
			continue
		}
		registry.RegisterWithContext(topic, gated{Source: t.Source, topic: topic, may: t.May})
	}
	// Authorize is nil ON PURPOSE: the library takes that to mean the mount
	// point has already authenticated, which is exactly the case - see the note
	// on the package.
	return &Server{
		handler: livewire.NewServer(registry, livewire.Options{Logger: slog.Default()}),
		topics:  topics,
	}
}

// ServeFor upgrades a connection that may watch what the perimeter allows.
func (s *Server) ServeFor(p Perimeter, w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), perimeterKey{}, p)))
}

// Topics is what one perimeter may watch, sorted - for a status page and for a
// test that wants to know nothing was dropped.
func (s *Server) Topics(p Perimeter) []string {
	out := []string{ChangesTopic}
	for topic, t := range s.topics {
		if t.May == nil || t.May(p) {
			out = append(out, topic)
		}
	}
	sort.Strings(out)
	return out
}

type perimeterKey struct{}

// perimeterOf is the perimeter ServeFor put on the request that opened the
// socket. A socket opened any other way has none, and is refused everything
// that depends on one: failing closed is the only answer for a boundary.
func perimeterOf(opening *http.Request) (Perimeter, error) {
	if opening != nil {
		if p, ok := opening.Context().Value(perimeterKey{}).(Perimeter); ok {
			return p, nil
		}
	}
	return Perimeter{}, errors.New("this socket was opened without a perimeter")
}

// gated is a source only some readers may watch. Its rows are the same for all
// of them, so they still share one read: nothing taken from the opening request
// is in the query, and the key stays the source's own.
type gated struct {
	livewire.Source
	topic string
	may   func(Perimeter) bool
}

// ReadQuery refuses a reader outside the topic's perimeter, in the words the
// library uses for a topic that does not exist: a reader learns nothing from a
// refusal that it would not learn from an absence.
func (g gated) ReadQuery(raw json.RawMessage, opening *http.Request) (any, error) {
	p, err := perimeterOf(opening)
	if err != nil || !g.may(p) {
		return nil, errors.New("No topic '" + g.topic + "'")
	}
	return g.Source.ReadQuery(raw)
}
