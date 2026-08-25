package events

import (
	"encoding/json"
	"testing"
)

// A message reaches the pages that were granted its topic, and only those.
// The grant is what makes a publisher free of permission questions, so it is
// the property worth pinning down.
func TestPublishReachesOnlyItsTopic(t *testing.T) {
	h := NewHub()
	everyone, err := h.Subscribe(TopicAll)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := h.Subscribe(TopicAll, TopicDev)
	if err != nil {
		t.Fatal(err)
	}
	mine, err := h.Subscribe(TopicAll, UserTopic("u1"))
	if err != nil {
		t.Fatal(err)
	}

	h.Publish(TopicDev, Message{Type: "tools", Data: map[string]string{"a": "b"}})
	if len(everyone.Events()) != 0 {
		t.Fatal("a page without the dev grant received a dev event")
	}
	if len(mine.Events()) != 0 {
		t.Fatal("a page without the dev grant received a dev event")
	}
	select {
	case raw := <-dev.Events():
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != "tools" {
			t.Fatalf("type = %q", msg.Type)
		}
	default:
		t.Fatal("the dev page received nothing")
	}

	h.Publish(UserTopic("u1"), Message{Type: "session"})
	if len(mine.Events()) != 1 {
		t.Fatal("the account's own room did not reach its page")
	}
	if len(dev.Events()) != 0 {
		t.Fatal("another account's room reached this page")
	}

	// The topic is an audience, not something the browser is told about.
	h.Publish(TopicAll, Message{Type: "served"})
	raw := <-everyone.Events()
	if got := string(raw); got != `{"type":"served"}` {
		t.Fatalf("envelope = %s", got)
	}
}

// A page that cannot keep up is dropped rather than buffered: its channel
// closes, the transport says goodbye, and the browser reconnects and fetches
// the state again. Anything else has the gateway holding messages for a page
// that may never read one.
func TestAPageThatFallsBehindIsDropped(t *testing.T) {
	h := NewHub()
	sub, err := h.Subscribe(TopicAll)
	if err != nil {
		t.Fatal(err)
	}
	for range outBuffer + 1 {
		h.Publish(TopicAll, Message{Type: "served"})
	}
	// Drain what fitted; the read after that must find a CLOSED channel.
	for range outBuffer {
		<-sub.Events()
	}
	if _, open := <-sub.Events(); open {
		t.Fatal("the subscription survived falling behind")
	}
	if h.Count() != 0 {
		t.Fatalf("the hub still holds %d subscriptions", h.Count())
	}
	// Closing twice is normal: both pumps of a connection race to do it.
	sub.Close()
}

// The ceiling is not a rationed feature, it is a bound on what one process
// holds open. It has to answer something the endpoint can turn into an HTTP
// status rather than a panic.
func TestTheHubRefusesBeyondItsCeiling(t *testing.T) {
	h := NewHub()
	for range MaxSubscribers {
		if _, err := h.Subscribe(TopicAll); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.Subscribe(TopicAll); err == nil {
		t.Fatal("the hub accepted one past its ceiling")
	}
}
