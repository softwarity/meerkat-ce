// Package mcp is the Model Context Protocol server Meerkat exposes to an
// administrator's agent (MCP-01). It knows the PROTOCOL and nothing about
// gateways: the tools are built by internal/admin, which owns the store and
// the router. One direction, admin -> mcp, so the protocol can be read and
// tested on its own.
//
// Why no SDK: the server half of MCP, for a server that offers tools, is
// initialize / tools/list / tools/call / ping. That subset is frozen by the
// specification, and the official Go SDK brings eight direct dependencies for
// it - against a house rule (pure Go, dependencies counted) that already
// turned down the official Kubernetes client. If sampling or elicitation ever
// becomes useful, the SDK is still there.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// Tool is one thing an agent can do. The name is a PUBLIC identifier: people
// put it in their prompts and their saved workflows, so renaming one breaks
// them - which is why the set is snapshotted in a golden file.
type Tool struct {
	// Name is what tools/call names, in snake_case: list_routes, get_route.
	Name string
	// Title is the human label a client shows when it asks for consent.
	Title string
	// Description is read by a MODEL, not by a person. It says what the tool
	// answers and when to reach for it, because that is what decides whether
	// the agent picks the right one out of a dozen.
	Description string
	// Schema is the JSON Schema of the arguments, written literally. An object
	// schema even when there are no arguments - clients differ on what they do
	// with a missing one.
	Schema map[string]any
	// ReadOnly says the tool changes nothing. It is an annotation for the
	// client AND the rule this server enforces: a read-only caller is offered
	// read-only tools and no others.
	ReadOnly bool
	// Allow reports whether the current caller may run this tool at all, read
	// or not. Nil means anyone the transport let in.
	//
	// It takes a context and not a user because this package knows nothing
	// about accounts: the host puts the caller on the context and answers the
	// question. A tool WITHOUT one is a tool that reads the store with no
	// regard for who is asking - which is how an agent endpoint becomes the
	// way around the capabilities the API spent a product enforcing.
	Allow func(ctx context.Context) bool
	// Call runs the tool. Returning an error is not a protocol failure: it
	// comes back as a tool result the model can READ and act on, which is how
	// an agent corrects an argument instead of giving up.
	Call func(ctx context.Context, args json.RawMessage) (any, error)
}

// Server holds the tool set. It is immutable once built and safe for
// concurrent use: one is built at startup and serves every agent.
type Server struct {
	// Name and Version identify this server to the client.
	Name, Version string
	tools         []Tool
	byName        map[string]Tool
}

// NewServer builds a server over a tool set, refusing a set that would confuse
// an agent: a duplicate name, a tool with no description, or one with no
// argument schema. This is a programming error, so it panics at startup rather
// than answering badly for months.
func NewServer(name, version string, tools []Tool) *Server {
	s := &Server{Name: name, Version: version, byName: map[string]Tool{}}
	for _, t := range tools {
		if err := validate(t); err != nil {
			panic("mcp: " + err.Error())
		}
		if _, dup := s.byName[t.Name]; dup {
			panic("mcp: two tools named " + t.Name)
		}
		s.byName[t.Name] = t
		s.tools = append(s.tools, t)
	}
	slices.SortFunc(s.tools, func(a, b Tool) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		}
		return 0
	})
	return s
}

func validate(t Tool) error {
	switch {
	case t.Name == "":
		return fmt.Errorf("a tool with no name")
	case t.Description == "":
		return fmt.Errorf("tool %q has no description: a model chooses on the description", t.Name)
	case t.Schema == nil:
		return fmt.Errorf("tool %q has no argument schema", t.Name)
	case t.Call == nil:
		return fmt.Errorf("tool %q does nothing", t.Name)
	}
	return nil
}

// Tools returns the whole set, sorted by name. Used by the golden-file test
// and by the coverage test that compares tools to endpoints.
func (s *Server) Tools() []Tool { return slices.Clone(s.tools) }

// Caller is what the transport knows about who is asking. The perimeter comes
// from the token (MCP-02) and decides what the agent is even shown.
type Caller struct {
	// ReadOnly hides every mutating tool. Hiding rather than refusing: an
	// agent does not attempt what it cannot see, and an attempt refused three
	// turns into a plan is the frustration this avoids. What the agent needs
	// in order to EXPLAIN itself is in describe_gateway, which names the
	// perimeter.
	ReadOnly bool
}

// offered is the tool list this caller may see: what their perimeter allows,
// and what their capabilities allow. Both filter the LIST, so an agent plans
// with the tools it actually has.
func (s *Server) offered(ctx context.Context, c Caller) []Tool {
	var out []Tool
	for _, t := range s.tools {
		if s.mayCall(ctx, c, t) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) mayCall(ctx context.Context, c Caller, t Tool) bool {
	if c.ReadOnly && !t.ReadOnly {
		return false
	}
	return t.Allow == nil || t.Allow(ctx)
}
