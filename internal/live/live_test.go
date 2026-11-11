package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// opened is the request a socket was opened with, carrying what ServeFor puts on
// it.
func opened(p Perimeter) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/live", nil)
	return r.WithContext(context.WithValue(r.Context(), perimeterKey{}, p))
}

// A topic only some readers may watch refuses the others, in the words of a
// topic that does not exist, and lets its own readers share one read.
func TestAGatedTopicRefusesAReaderOutsideItsPerimeter(t *testing.T) {
	counter := NewCounter("guarded")
	g := gated{Source: counter, topic: "guarded", may: func(p Perimeter) bool { return p.Traffic }}

	if _, err := g.ReadQuery(nil, opened(Perimeter{Schedules: true})); err == nil || err.Error() != "No topic 'guarded'" {
		t.Errorf("a reader outside the perimeter was answered %v, want the absent topic's refusal", err)
	}
	q, err := g.ReadQuery(nil, opened(Perimeter{Traffic: true}))
	if err != nil {
		t.Fatalf("a reader of the perimeter was refused: %v", err)
	}
	if g.Key(q) != counter.Key(q) {
		t.Errorf("the gate moved the key to %q: its readers no longer share one read", g.Key(q))
	}
}

// A socket that did not come through ServeFor carries no perimeter, and is
// refused everything that depends on one.
func TestASocketWithoutAPerimeterIsRefused(t *testing.T) {
	g := gated{Source: NewCounter("guarded"), topic: "guarded", may: func(Perimeter) bool { return true }}
	if _, err := g.ReadQuery(nil, httptest.NewRequest(http.MethodGet, "/api/live", nil)); err == nil {
		t.Error("a gated topic answered a socket with no perimeter")
	}
	if _, err := NewChanges().ReadQuery(nil, nil); err == nil {
		t.Error("the journal answered a socket with no perimeter")
	}
}

// The journal reads WHO is asking from the socket, never from the query: the
// perimeter decides what is described, and readers of one perimeter share a key.
func TestTheJournalReadsThePerimeterOfTheSocket(t *testing.T) {
	c := NewChanges()
	c.Record(store.AuditEvent{ID: "1", Target: "route", TargetID: "r1", Action: "update"})
	c.Record(store.AuditEvent{ID: "2", Target: "user", TargetID: "u1", Action: "update"})

	infra, err := c.ReadQuery([]byte(`{"named":["user"]}`), opened(Perimeter{Named: []string{"route"}}))
	if err != nil {
		t.Fatal(err)
	}
	window, err := c.Read(context.Background(), infra)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Rows) != 1 || window.Rows[0].Data["targetId"] != "r1" {
		t.Errorf("the routing plane's reader read %+v, want its route and nothing the query asked for", window.Rows)
	}

	again, _ := c.ReadQuery(nil, opened(Perimeter{Named: []string{"route"}}))
	app, _ := c.ReadQuery(nil, opened(Perimeter{Named: []string{"user"}}))
	if c.Key(infra) != c.Key(again) {
		t.Error("two readers of one perimeter do not share a read")
	}
	if c.Key(infra) == c.Key(app) {
		t.Error("two perimeters share one read")
	}
}

// What a perimeter may watch, as the status page lists it.
func TestTopicsFollowThePerimeter(t *testing.T) {
	s := New(NewChanges(), map[string]Topic{
		PauseTopic:   {Source: NewCounter(PauseTopic)},
		TrafficTopic: {Source: NewCounter(TrafficTopic), May: func(p Perimeter) bool { return p.Traffic }},
	})
	if got := s.Topics(Perimeter{}); !slices.Equal(got, []string{ChangesTopic, PauseTopic}) {
		t.Errorf("a reader of no plane watches %v", got)
	}
	if got := s.Topics(Perimeter{Traffic: true}); !slices.Contains(got, TrafficTopic) {
		t.Errorf("the routing plane's reader cannot watch the traffic: %v", got)
	}
}
