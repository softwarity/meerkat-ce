package cluster_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/store"
)

// A page is held open by whichever gateway the load balancer gave it. So a
// message published on one node reaches the fraction of the audience that
// happens to be connected to it, and the rest see nothing - which for the
// first user of this channel means a developer takes over a service and half
// the people looking at it are never told.
func TestALiveMessageReachesThePagesOnTheOtherGateway(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hubA, hubB := events.NewHub(), events.NewHub()
	busA, busB := cluster.New(sa), cluster.New(sb)
	wire := func(hub *events.Hub, bus *cluster.Bus) {
		hub.Relay(func(topic string, payload []byte) {
			bus.Signal(ctx, store.TopicEvent, topic+" "+string(payload))
		})
		bus.OnSignal(store.TopicEvent, func(arg string) {
			topic, payload, ok := cut(arg)
			if ok {
				hub.Deliver(topic, []byte(payload))
			}
		})
	}
	wire(hubA, busA)
	wire(hubB, busB)
	go busA.Run(ctx)
	go busB.Run(ctx)
	time.Sleep(500 * time.Millisecond)

	// One page on each gateway.
	subA, err := hubA.Subscribe(events.TopicAll)
	if err != nil {
		t.Fatal(err)
	}
	defer subA.Close()
	subB, err := hubB.Subscribe(events.TopicAll)
	if err != nil {
		t.Fatal(err)
	}
	defer subB.Close()

	hubA.Publish(events.TopicAll, events.Message{Type: "probe", Data: "hello"})

	// The page on the OTHER gateway hears it.
	select {
	case raw := <-subB.Events():
		var got events.Message
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("what arrived was not a message: %v", err)
		}
		if got.Type != "probe" {
			t.Fatalf("the wrong message arrived: %+v", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the page on the second gateway heard nothing: half an audience is not an audience")
	}

	// And the page on the node that published hears it ONCE. PostgreSQL
	// delivers a notification to the sender too, so without the node stamp on
	// every signal this is where one event becomes two.
	<-subA.Events()
	select {
	case raw := <-subA.Events():
		t.Fatalf("the publishing gateway delivered the message twice: %s", raw)
	case <-time.After(2 * time.Second):
	}
}

// cut splits "topic payload" the way main does.
func cut(arg string) (topic, payload string, ok bool) {
	for i := range len(arg) {
		if arg[i] == ' ' {
			return arg[:i], arg[i+1:], true
		}
	}
	return "", "", false
}
