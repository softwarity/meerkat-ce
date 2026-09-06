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
// AUTHORISATION IS NOT DONE HERE. The handler is mounted BEHIND the control
// plane's own funnel, so a subscription passes exactly the checks an API call
// passes - session, pending login, token perimeter, narrowing to the token's
// domain. Writing a second predicate here would be writing the security
// boundary twice, and the second copy is the one that drifts.
package live

import (
	"log/slog"
	"net/http"

	livewire "github.com/softwarity/livewire/go"
)

// Server is the console's live channel.
type Server struct {
	registry *livewire.Registry
	handler  http.Handler
}

// New returns a server with no sources yet.
func New() *Server {
	registry := livewire.NewRegistry(0)
	return &Server{
		registry: registry,
		// Authorize is nil ON PURPOSE: the library takes that to mean the
		// mount point has already authenticated, which is exactly the case -
		// see the note on the package.
		handler: livewire.NewServer(registry, livewire.Options{Logger: slog.Default()}),
	}
}

// Register puts a source on a topic. Called at wiring time, before serving.
func (s *Server) Register(topic string, source livewire.Source) {
	s.registry.Register(topic, source)
}

// Topics is what this server serves, for a status page and for a test that
// wants to know nothing was dropped.
func (s *Server) Topics() []string { return s.registry.Topics() }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
