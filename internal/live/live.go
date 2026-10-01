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
// (admin.livePerimeter, read from the audit trail's own partition). This package
// turns that answer into a registry holding those sources and nothing else,
// because a source cannot do it: the library shares one read between the
// subscribers of a topic and hands it a context of its own, so by the time a
// source runs there is no caller left to check. Deciding it twice - once in the
// funnel, once in a predicate here - would be writing the boundary twice, and
// the second copy is the one that drifts.
package live

import (
	"log/slog"
	"net/http"
	"sort"
	"sync"

	livewire "github.com/softwarity/livewire/go"
)

// Server is the console's live channel.
//
// ONE REGISTRY PER PERIMETER, and the reason is in the library rather than in a
// taste for indirection: a registry shares one read between every subscriber of
// a topic and calls the source with a context of its own, so a source cannot
// tell its readers apart. Authorising a subscription therefore has to happen
// where the reader is still known - at the upgrade - and the way to say "this
// reader may watch these sources" is to hand them a registry that holds those.
//
// Built on demand and kept, keyed by the perimeter: the console has a handful
// of perimeters (root, one per capability, and the readers who administer an
// organisation), not one per screen, so this is a map with four or five entries
// in it and one read behind each.
type Server struct {
	// What every perimeter is built FROM, set once at wiring time.
	sources func(Perimeter) Sources

	mu       sync.Mutex
	handlers map[string]http.Handler
}

// Perimeter is one reader's view of the channel. It mirrors admin.LivePerimeter,
// which main translates: the control plane decides who sees what, this package
// decides which source serves it.
type Perimeter struct {
	// Key is what two readers must share to share one read.
	Key string
	// Named are the kinds whose last write may be described, Quiet the kinds the
	// reader is only told moved.
	Named []string
	Quiet []string
	// Whether this reader administers the routing plane (the traffic curves) and
	// the application's (the scheduled calls).
	Traffic   bool
	Schedules bool
}

// Sources is what one perimeter may watch, by topic. An alias, so the wiring
// names it without importing the library for one type.
type Sources = map[string]livewire.Source

// New returns a server whose perimeters are built by sources.
func New(sources func(Perimeter) Sources) *Server {
	return &Server{sources: sources, handlers: map[string]http.Handler{}}
}

// ServeFor upgrades a connection that may watch what the perimeter allows.
func (s *Server) ServeFor(p Perimeter, w http.ResponseWriter, r *http.Request) {
	s.handlerFor(p).ServeHTTP(w, r)
}

func (s *Server) handlerFor(p Perimeter) http.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, built := s.handlers[p.Key]; built {
		return h
	}
	registry := livewire.NewRegistry(0)
	for topic, source := range s.sources(p) {
		registry.Register(topic, source)
	}
	// Authorize is nil ON PURPOSE: the library takes that to mean the mount
	// point has already authenticated, which is exactly the case - and what it
	// authenticated is now also WHICH registry this is - see the note on the
	// package.
	h := livewire.NewServer(registry, livewire.Options{Logger: slog.Default()})
	s.handlers[p.Key] = h
	return h
}

// Topics is what one perimeter serves, sorted - for a status page and for a test
// that wants to know nothing was dropped.
func (s *Server) Topics(p Perimeter) []string {
	topics := make([]string, 0, 3)
	for topic := range s.sources(p) {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics
}
