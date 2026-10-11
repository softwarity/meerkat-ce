package discovery

import (
	"context"
	"os"
	"strconv"
	"strings"
)

// Where the services of this gateway's own runtime reach one of its ports.
//
// The console names the internal plane's address beside the JWKS and the
// scheduler's API, and a name typed by hand ("meerkat") is a guess about
// somebody else's deployment. The runtime knows the real one: the Service in
// front of this pod, the Swarm service, the compose service, the container.

// Internal is that address, every name the runtime resolves to it, and which
// runtime gave it. An empty Origin with a reason means none could be asked.
//
// Names starts with the Origin's host, then the shorter forms a neighbour may
// use: what a certificate for this plane has to carry one of, and what the TLS
// screen checks a certificate against before it is placed.
type Internal struct {
	Origin string   `json:"origin,omitempty"`
	Names  []string `json:"names,omitempty"`
	Source string   `json:"source,omitempty"`
	Why    string   `json:"why,omitempty"`
}

// InternalOrigin asks the runtime under which name the given inside port is
// reached from within it.
func InternalOrigin(ctx context.Context, port int) Internal {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		self, err := readK8sSelf(ctx)
		if err != nil {
			return Internal{Why: "could not read this gateway's Services: " + err.Error()}
		}
		name, svcPort, ok := self.serviceFor(port)
		if !ok {
			return Internal{Why: "no Service of the namespace sends to port " + strconv.Itoa(port) + " of this pod"}
		}
		resolv, _ := os.ReadFile("/etc/resolv.conf")
		svc := name + "." + self.namespace + ".svc"
		host := svc + "." + clusterDomain(string(resolv), self.namespace)
		// OpenShift mounts its service CA beside the token in every pod: the
		// one sign that this cluster can issue a Service its certificate.
		source := "kubernetes"
		if _, err := os.Stat(saDir + "/service-ca.crt"); err == nil {
			source = "openshift"
		}
		return Internal{Origin: "http://" + host + ":" + strconv.Itoa(svcPort), Source: source,
			Names: []string{host, svc, name + "." + self.namespace, name}}
	}
	client, base, err := dockerClient()
	if err != nil {
		return Internal{Why: "no runtime to ask: no Kubernetes, no Docker socket"}
	}
	host, _ := os.Hostname()
	var c struct {
		Name   string `json:"Name"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := getJSON(ctx, client, base+"/containers/"+host+"/json", &c); err != nil {
		return Internal{Why: "the Docker socket does not know this container: " + err.Error()}
	}
	names, source := dockerNames(c.Config.Labels, c.Name)
	return Internal{Origin: "http://" + names[0] + ":" + strconv.Itoa(port), Names: names, Source: source}
}

// serviceFor picks the Service in front of this pod that sends to port: a
// ClusterIP before a published one, and among those the one named for the
// internal plane (the chart's "<release>-internal"), since that is the name a
// service is meant to call.
func (s k8sSelf) serviceFor(port int) (string, int, bool) {
	best, bestPort, bestScore := "", 0, -1
	for _, svc := range s.services {
		if len(svc.Spec.Selector) == 0 || !selects(svc.Spec.Selector, s.labels) {
			continue
		}
		for _, p := range svc.Spec.Ports {
			if s.inside(p.TargetPort, p.Port) != port {
				continue
			}
			score := 0
			if svc.Spec.Type == "" || svc.Spec.Type == "ClusterIP" {
				score += 2
			}
			if strings.HasSuffix(svc.Metadata.Name, "-internal") {
				score++
			}
			if score > bestScore {
				best, bestPort, bestScore = svc.Metadata.Name, p.Port, score
			}
		}
	}
	return best, bestPort, bestScore >= 0
}

// clusterDomain reads the cluster's DNS domain from the pod's resolv.conf,
// whose search list starts "<namespace>.svc.<domain>". cluster.local when it
// does not say: the default nearly every cluster keeps.
func clusterDomain(resolv, namespace string) string {
	prefix := namespace + ".svc."
	for _, line := range strings.Split(resolv, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "search" {
			continue
		}
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, prefix) && len(f) > len(prefix) {
				return f[len(prefix):]
			}
		}
	}
	return "cluster.local"
}

// dockerNames are the names the other containers of its network call this
// one by, the one to advertise first: its Swarm service (and the short name
// its stack resolves too), its compose service (and the container's own
// name), else its own name.
func dockerNames(labels map[string]string, containerName string) ([]string, string) {
	self := strings.TrimPrefix(containerName, "/")
	if s := labels["com.docker.swarm.service.name"]; s != "" {
		names := []string{s}
		if short, ok := strings.CutPrefix(s, labels["com.docker.stack.namespace"]+"_"); ok && short != "" {
			names = append(names, short)
		}
		return names, "swarm"
	}
	if s := labels["com.docker.compose.service"]; s != "" {
		names := []string{s}
		if self != "" && self != s {
			names = append(names, self)
		}
		return names, "compose"
	}
	return []string{self}, "docker"
}
