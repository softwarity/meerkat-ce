package discovery

import (
	"strings"
	"testing"
)

// TestTheInternalServiceIsTheOneSentToThePort: of the Services in front of the
// pod, the one sending to the internal port - a ClusterIP before a published
// one, the "-internal" one before another ClusterIP.
func TestTheInternalServiceIsTheOneSentToThePort(t *testing.T) {
	svc := func(name, typ string, port int, target any) selfService {
		var s selfService
		s.Metadata.Name = name
		s.Spec.Type = typ
		s.Spec.Selector = map[string]string{"app": "meerkat"}
		s.Spec.Ports = append(s.Spec.Ports, struct {
			Port       int    `json:"port"`
			NodePort   int    `json:"nodePort"`
			TargetPort any    `json:"targetPort"`
			Name       string `json:"name"`
		}{Port: port, TargetPort: target})
		return s
	}
	self := k8sSelf{
		namespace: "shop", labels: map[string]string{"app": "meerkat"},
		named: map[string]int{"internal": 7070, "app": 8080},
		services: []selfService{
			svc("meerkat", "LoadBalancer", 8080, "app"),
			svc("meerkat-lb", "LoadBalancer", 7070, "internal"),
			svc("meerkat-other", "ClusterIP", 7070, float64(7070)),
			svc("meerkat-internal", "ClusterIP", 7070, "internal"),
		},
	}
	name, port, ok := self.serviceFor(7070)
	if !ok || name != "meerkat-internal" || port != 7070 {
		t.Fatalf("picked %q :%d %v", name, port, ok)
	}
	if _, _, ok := self.serviceFor(9999); ok {
		t.Fatal("a port no Service sends to was found")
	}
}

func TestTheClusterDomainIsReadFromResolvConf(t *testing.T) {
	if d := clusterDomain("nameserver 10.96.0.10\nsearch shop.svc.corp.internal svc.corp.internal corp.internal\n", "shop"); d != "corp.internal" {
		t.Fatalf("got %q", d)
	}
	if d := clusterDomain("nameserver 10.96.0.10\n", "shop"); d != "cluster.local" {
		t.Fatalf("no search line: %q", d)
	}
}

func TestADockerContainerIsCalledByItsService(t *testing.T) {
	for _, c := range []struct {
		labels     map[string]string
		name, want string
		wantSource string
	}{
		{map[string]string{"com.docker.swarm.service.name": "edge_meerkat", "com.docker.stack.namespace": "edge", "com.docker.compose.service": "x"}, "/c1", "edge_meerkat meerkat", "swarm"},
		{map[string]string{"com.docker.swarm.service.name": "meerkat"}, "/c1", "meerkat", "swarm"},
		{map[string]string{"com.docker.compose.service": "meerkat"}, "/proj-meerkat-1", "meerkat proj-meerkat-1", "compose"},
		{nil, "/gateway", "gateway", "docker"},
	} {
		if got, src := dockerNames(c.labels, c.name); strings.Join(got, " ") != c.want || src != c.wantSource {
			t.Errorf("%v: %q %q", c.labels, got, src)
		}
	}
}
