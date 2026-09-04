// Package events is the gateway's live channel to the pages it serves: one
// WebSocket per page, carrying typed messages the browser could not have
// learned on its own.
//
// It is deliberately about NOTHING in particular. The first caller pushes the
// state of a developer's plug overrides, but the next ones are already
// visible - a session about to expire, a configuration reloaded, a message an
// operator wants on every screen - and a channel named after its first use is
// how each of them ends up opening a socket of its own. So: a hub, topics, and
// an envelope with a type the browser switches on.
//
// The channel is never REQUIRED. Every page that uses it already has its state
// from a fetch at load; this only keeps it fresh. A corporate proxy that eats
// WebSockets therefore costs the freshness, not the feature - which is also
// what let it be added to pages that were already working.
package events

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
)

// Message is what travels: a type the browser switches on, and whatever that
// type carries. The topic is not on the wire - it is how the server decides
// WHO gets this, and telling a client the name of a room it is not in buys
// nothing.
type Message struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// The topics that exist today. A topic is an audience, and the grant is made
// once per connection from the session: a subscriber cannot ask for a room it
// was not let into, so a publisher never has to think about permissions.
const (
	// TopicAll reaches every open page, signed in or not. What lands here is
	// what a visitor is entitled to know about the application in front of
	// them - a service being served by a developer's machine, for instance,
	// which changes what they are looking at whoever they are.
	TopicAll = "all"
	// TopicDev reaches the pages of people who hold the developer capability
	// (DEV-01), on an installation where the mode is on.
	TopicDev = "dev"
)

// UserTopic is the room of one account, across its open pages and its
// browsers. Sessions come and go; the account is the stable address.
func UserTopic(userID string) string { return "user:" + userID }

// ErrTooManySubscribers is what a hub at capacity answers. It is not a
// failure mode worth much ceremony: the page keeps the state it fetched, and
// tries again later.
var ErrTooManySubscribers = errors.New("too many open channels")

// MaxSubscribers caps what one gateway holds open. A connection is a goroutine
// and a small buffer, so the ceiling is high - it exists to keep an anonymous
// endpoint from becoming a way to spend a process, not to ration a feature.
const MaxSubscribers = 4096

// outBuffer is how far behind a page may fall before it is dropped. Small on
// purpose: see Publish.
const outBuffer = 16

// Hub fans messages out to the pages that are entitled to them.
type Hub struct {
	mu   sync.Mutex
	subs map[*Subscription]struct{}
	// relay carries a message to the OTHER gateways (STORE-03). A page is held
	// open by whichever node the load balancer gave it, so a message published
	// on one reaches a fraction of the audience unless it travels; with one
	// gateway there is nobody to tell and this stays nil.
	relay func(topic string, payload []byte)
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[*Subscription]struct{})}
}

// Subscription is one page's end of the channel.
type Subscription struct {
	hub    *Hub
	topics map[string]struct{}
	out    chan []byte
	once   sync.Once
}

// Subscribe opens a channel for a page that has been granted these topics.
func (h *Hub) Subscribe(topics ...string) (*Subscription, error) {
	sub := &Subscription{
		hub:    h,
		topics: make(map[string]struct{}, len(topics)),
		out:    make(chan []byte, outBuffer),
	}
	for _, t := range topics {
		sub.topics[t] = struct{}{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) >= MaxSubscribers {
		return nil, ErrTooManySubscribers
	}
	h.subs[sub] = struct{}{}
	return sub, nil
}

// Events is what the transport pumps. It closes when the subscription does.
func (s *Subscription) Events() <-chan []byte { return s.out }

// Close detaches the subscription. Safe to call twice, which matters: both
// pumps of a connection race to call it when the other side goes away.
func (s *Subscription) Close() {
	s.once.Do(func() {
		s.hub.mu.Lock()
		delete(s.hub.subs, s)
		s.hub.mu.Unlock()
		close(s.out)
	})
}

// Publish sends a message to everyone in a topic.
//
// A page that cannot keep up is DROPPED, not slowed down and not caught up
// later. This looks harsh and is the honest behaviour: the channel carries
// updates to a state the page fetched at load, so the cure for having missed
// one is to reconnect and fetch again - which the browser does on its own.
// The alternative, a growing buffer per page, trades a dead connection for a
// gateway holding messages nobody will ever read.
//
// Never blocks, whatever a publisher is in the middle of: Served is called
// from plug's accept path, and a status screen must not be able to hold up a
// tunnel.
func (h *Hub) Publish(topic string, msg Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("event could not be encoded", "type", msg.Type, "err", err)
		return
	}
	h.Deliver(topic, payload)
	h.mu.Lock()
	relay := h.relay
	h.mu.Unlock()
	if relay != nil {
		relay(topic, payload)
	}
}

// Relay installs the carrier to the other gateways. Wired by main; called once
// at wiring time.
func (h *Hub) Relay(fn func(topic string, payload []byte)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.relay = fn
}

// Deliver publishes an already-encoded message to THIS node's pages and tells
// nobody. It is what the carrier calls on the receiving side - relaying a
// relayed message is how one event becomes an infinite number.
func (h *Hub) Deliver(topic string, payload []byte) {
	var behind []*Subscription
	h.mu.Lock()
	for sub := range h.subs {
		if _, ok := sub.topics[topic]; !ok {
			continue
		}
		select {
		case sub.out <- payload:
		default:
			behind = append(behind, sub)
		}
	}
	h.mu.Unlock()
	// Outside the lock: Close takes it.
	for _, sub := range behind {
		slog.Debug("event channel fell behind and was dropped", "topic", topic)
		sub.Close()
	}
}

// Count is what an operator sees: how many pages are listening.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
