package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// A Swarm listing carries only PUBLISHED ports, and the services a gateway is
// put in front of usually publish none: they are reached inside the overlay
// network. Those are the ones the console must still offer, so the reader
// falls back to what the image says it listens on.
func TestSwarmFallsBackToTheImagePorts(t *testing.T) {
	srv := (&daemon{services: `[
	  {"Spec":{"Name":"published","TaskTemplate":{"ContainerSpec":{"Image":"acme/web:1"}},
	    "EndpointSpec":{"Ports":[{"TargetPort":8080,"PublishedPort":80}]}},
	   "ServiceStatus":{"RunningTasks":2,"DesiredTasks":2}},
	  {"Spec":{"Name":"internal","TaskTemplate":{"ContainerSpec":{"Image":"acme/api:1"}}},
	   "ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"twin","TaskTemplate":{"ContainerSpec":{"Image":"acme/api:1"}}},
	   "ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"chatty","TaskTemplate":{"ContainerSpec":{"Image":"acme/dns:1"}}},
	   "ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"unknown","TaskTemplate":{"ContainerSpec":{"Image":"acme/gone:1"}}},
	   "ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
	]`, images: map[string]string{
		"acme/web:1": `{"Config":{"ExposedPorts":{"9999/tcp":{}}}}`,
		"acme/api:1": `{"Config":{"ExposedPorts":{"3000/tcp":{},"9229/tcp":{}}}}`,
		// udp only: nothing a route can ever point at.
		"acme/dns:1": `{"Config":{"ExposedPorts":{"53/udp":{}}}}`,
	}}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	by := map[string]Service{}
	for _, s := range res.Services {
		by[s.Name] = s
	}

	// A declared port wins: the image is only consulted when there is none,
	// so a published service is not second-guessed.
	if got := by["published"].Ports; len(got) != 1 || got[0].Target != 8080 || got[0].Published != 80 {
		t.Errorf("published: ports = %+v, want the declared 8080/80", got)
	}
	if got := by["internal"].Ports; len(got) != 2 || got[0].Target != 3000 || got[1].Target != 9229 {
		t.Errorf("internal: ports = %+v, want 3000 and 9229 from the image", got)
	}
	// Two services on one image get the same answer.
	if got := by["twin"].Ports; len(got) != 2 {
		t.Errorf("twin: ports = %+v, want the same two", got)
	}
	if got := by["chatty"].Ports; len(got) != 0 {
		t.Errorf("chatty: ports = %+v, want none - udp is not routable", got)
	}
	// An image the daemon does not hold answers 404, and the service simply
	// keeps no port rather than the whole listing failing.
	if got := by["unknown"].Ports; len(got) != 0 {
		t.Errorf("unknown: ports = %+v, want none", got)
	}
	// One port and only one becomes a suggestion; two stays a choice.
	if got := by["published"].Suggested; got != "http://published:8080" {
		t.Errorf("published: suggested = %q", got)
	}
	if got := by["internal"].Suggested; got != "" {
		t.Errorf("internal: suggested = %q, want none with two ports", got)
	}
}

// A distinct image is inspected once however many services run it.
func TestImagesAreInspectedOnce(t *testing.T) {
	d := &daemon{services: `[
	  {"Spec":{"Name":"a","TaskTemplate":{"ContainerSpec":{"Image":"acme/api:1"}}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"b","TaskTemplate":{"ContainerSpec":{"Image":"acme/api:1"}}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"c","TaskTemplate":{"ContainerSpec":{"Image":"acme/api:1"}}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
	]`, images: map[string]string{"acme/api:1": `{"Config":{"ExposedPorts":{"3000/tcp":{}}}}`}}
	srv := d.serve(t)

	if _, err := fromSwarm(t.Context(), srv.Client(), srv.URL); err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	if d.seen["acme/api:1"] != 1 {
		t.Errorf("image inspected %d times, want 1", d.seen["acme/api:1"])
	}
}

// daemon is a Docker API with only the three routes this package reads.
type daemon struct {
	services string
	// What /containers/<id>/json answers for the gateway's own container,
	// and the id the kernel would have handed it. Empty means the gateway is
	// not a container the daemon knows.
	selfID   string
	selfBody string
	// Keyed by image reference; a reference absent from it answers 404, the
	// way a daemon that never pulled it does.
	images   map[string]string
	networks string
	// How many times each image was inspected.
	seen map[string]int
}

func (d *daemon) serve(t *testing.T) *httptest.Server {
	t.Helper()
	if d.networks == "" {
		d.networks = "[]"
	}
	if d.seen == nil {
		d.seen = map[string]int{}
	}
	if d.selfID != "" {
		was := ownContainerID
		ownContainerID = func() string { return d.selfID }
		t.Cleanup(func() { ownContainerID = was })
	}
	// The reader inspects images in parallel, so the counter is shared.
	var mu sync.Mutex
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/services"):
			write(w, d.services)
		case strings.HasPrefix(r.URL.Path, "/networks"):
			write(w, d.networks)
		case d.selfID != "" && r.URL.Path == "/containers/"+d.selfID+"/json":
			write(w, d.selfBody)
		case strings.HasPrefix(r.URL.Path, "/images/"):
			ref, err := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(r.URL.EscapedPath(), "/images/"), "/json"))
			if err != nil {
				http.Error(w, "bad ref", http.StatusBadRequest)
				return
			}
			body, ok := d.images[ref]
			if !ok {
				http.Error(w, "no such image", http.StatusNotFound)
				return
			}
			mu.Lock()
			d.seen[ref]++
			mu.Unlock()
			write(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A route needs the number, so a port that cannot be one is dropped rather
// than offered as zero.
func TestExposedPortsRejectsWhatIsNotAPort(t *testing.T) {
	srv := (&daemon{services: `[]`, images: map[string]string{
		"acme/odd:1": `{"Config":{"ExposedPorts":{"http/tcp":{},"0/tcp":{},"8080":{},"7000/tcp":{}}}}`,
	}}).serve(t)
	got := exposedPorts(context.Background(), srv.Client(), srv.URL, "acme/odd:1")
	if len(got) != 2 || got[0].Target != 7000 || got[1].Target != 8080 {
		t.Errorf("ports = %+v, want 7000 and the protocol-less 8080", got)
	}
}

// A stack service answers to two names: the short alias the stack registers,
// and its full name. The short one is what an operator writes, so it leads -
// but only while it is unambiguous on the network that carries it.
func TestServicesOfferTheNameThatResolves(t *testing.T) {
	srv := (&daemon{services: `[
	  {"Spec":{"Name":"neo_mongodb","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["mongodb"]}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":27017}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"neo_cache","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["cache"]}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":6379}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"other_cache","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["cache"]}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":6379}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"far_cache","TaskTemplate":{"Networks":[{"Target":"n2","Aliases":["cache"]}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":6379}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
	  {"Spec":{"Name":"plain","TaskTemplate":{"Networks":[{"Target":"n1"}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
	]`}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	by := map[string]Service{}
	for _, s := range res.Services {
		by[s.Name] = s
	}

	if got := by["neo_mongodb"].Names; len(got) != 2 || got[0] != "mongodb" || got[1] != "neo_mongodb" {
		t.Errorf("neo_mongodb: names = %v, want the alias then the full name", got)
	}
	if got := by["neo_mongodb"].Suggested; got != "http://mongodb:27017" {
		t.Errorf("neo_mongodb: suggested = %q, want the alias", got)
	}
	// Two services claiming "cache" on the SAME network: nobody gets it, the
	// full name is all that can be promised.
	for _, n := range []string{"neo_cache", "other_cache"} {
		if got := by[n].Names; len(got) != 1 || got[0] != n {
			t.Errorf("%s: names = %v, want only the full name - the alias is contested", n, got)
		}
	}
	// The same alias on ANOTHER network is not a clash: DNS is per network.
	if got := by["far_cache"].Names; len(got) != 2 || got[0] != "cache" {
		t.Errorf("far_cache: names = %v, want its own uncontested alias", got)
	}
	// No alias at all: the full name, once, not twice.
	if got := by["plain"].Names; len(got) != 1 || got[0] != "plain" {
		t.Errorf("plain: names = %v", got)
	}
}

// The console shows networks to say whether this gateway can reach a service,
// which an id cannot do.
func TestNetworksAreNamed(t *testing.T) {
	srv := (&daemon{services: `[
	  {"Spec":{"Name":"web","TaskTemplate":{"Networks":[{"Target":"abc123"},{"Target":"unknown"}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
	]`, networks: `[{"Name":"neo_default","Id":"abc123"}]`}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	got := res.Services[0].Networks
	// An id nobody can name stays an id: still better than an empty field.
	if len(got) != 2 || got[0] != "neo_default" || got[1] != "unknown" {
		t.Errorf("networks = %v, want the name and the unresolved id", got)
	}
}

// A stack boundary is not a boundary: the NETWORK is. A service this gateway
// shares a network with resolves under both its names whatever stack it
// belongs to - and one it shares none with resolves under neither, so reaching
// for the full name buys nothing.
func TestReachIsByNetworkNotByStack(t *testing.T) {
	srv := (&daemon{
		services: `[
		  {"Spec":{"Name":"neo_api","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["api"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
		  {"Spec":{"Name":"other_api","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["shop"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
		  {"Spec":{"Name":"llm_far","TaskTemplate":{"Networks":[{"Target":"n2","Aliases":["far"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
		]`,
		networks: `[{"Name":"shared","Id":"n1"},{"Name":"elsewhere","Id":"n2"}]`,
		selfID:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		selfBody: `{"NetworkSettings":{"Networks":{"shared":{}}}}`,
	}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	by := map[string]Service{}
	for _, s := range res.Services {
		by[s.Name] = s
	}
	// Another stack entirely, but the same network: reachable, alias and all.
	if !by["other_api"].Reachable {
		t.Error("other_api: a different stack on the same network is reachable")
	}
	if !by["neo_api"].Reachable {
		t.Error("neo_api: same network, reachable")
	}
	// Same cluster, no shared network: no name resolves, long or short.
	if by["llm_far"].Reachable {
		t.Error("llm_far: shares no network, so nothing about it resolves from here")
	}
	if len(res.Reach) != 1 || res.Reach[0] != "shared" {
		t.Errorf("reach = %v, want the gateway's own network", res.Reach)
	}
}

// Outside a container there is nothing to ask, and a guess that hides a
// working upstream is worse than one that offers a doubtful one.
func TestUnknownReachOffersEverything(t *testing.T) {
	srv := (&daemon{services: `[
	  {"Spec":{"Name":"a","TaskTemplate":{"Networks":[{"Target":"n1"}]},
	    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
	]`}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	if res.Reach != nil {
		t.Errorf("reach = %v, want nothing claimed", res.Reach)
	}
	if !res.Services[0].Reachable {
		t.Error("a service must stay offered while reach is unknown")
	}
}

// Reach turns the alias question inside out. A gateway on TWO networks that
// each carry a "cache" resolves neither reliably, however unambiguous each
// network is when taken alone - so neither gets the short name. And a service
// outside that reach competes for nothing: only its full name is shown,
// because nothing about it resolves from here anyway.
func TestAnAliasCompetesAcrossEverythingTheGatewayJoins(t *testing.T) {
	srv := (&daemon{
		services: `[
		  {"Spec":{"Name":"neo_cache","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["cache"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":6379}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
		  {"Spec":{"Name":"llm_cache","TaskTemplate":{"Networks":[{"Target":"n2","Aliases":["cache"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":6379}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
		  {"Spec":{"Name":"neo_api","TaskTemplate":{"Networks":[{"Target":"n1","Aliases":["api"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},
		  {"Spec":{"Name":"far_api","TaskTemplate":{"Networks":[{"Target":"n3","Aliases":["api"]}]},
		    "EndpointSpec":{"Ports":[{"TargetPort":80}]}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}
		]`,
		networks: `[{"Name":"one","Id":"n1"},{"Name":"two","Id":"n2"},{"Name":"three","Id":"n3"}]`,
		selfID:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		// Attached to BOTH networks that carry a "cache".
		selfBody: `{"NetworkSettings":{"Networks":{"one":{},"two":{}}}}`,
	}).serve(t)

	res, err := fromSwarm(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fromSwarm: %v", err)
	}
	by := map[string]Service{}
	for _, s := range res.Services {
		by[s.Name] = s
	}
	for _, n := range []string{"neo_cache", "llm_cache"} {
		if got := by[n].Names; len(got) != 1 || got[0] != n {
			t.Errorf("%s: names = %v, want only the full name - two networks in reach claim the alias", n, got)
		}
	}
	// "api" is claimed twice too, but the second claim is on a network this
	// gateway never joined, so it does not contest anything.
	if got := by["neo_api"].Names; len(got) != 2 || got[0] != "api" {
		t.Errorf("neo_api: names = %v, want the alias - the rival is out of reach", got)
	}
	if got := by["far_api"].Names; len(got) != 1 || got[0] != "far_api" {
		t.Errorf("far_api: names = %v, want only the full name - nothing about it resolves from here", got)
	}
	if by["far_api"].Reachable {
		t.Error("far_api: shares no network with the gateway")
	}
}
