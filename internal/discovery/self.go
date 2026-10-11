package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Where the world reaches THIS gateway's ports.
//
// The gateway knows the port it listens on inside its container, and nothing
// in between: a Kubernetes Service publishing 9443 as 19443, a Docker port
// mapping, a Swarm ingress. A link written with the inside port reaches
// nothing. So the runtime is asked - with the same read rights the discovery
// uses - which port it publishes each inside port on.

// Published maps an inside port to the port the runtime publishes it on, and
// says which runtime answered. An empty map with a reason means it could not
// tell; an inside port absent from the map is not published.
type Published struct {
	Ports  map[int]int `json:"ports"`
	Source string      `json:"source,omitempty"`
	Why    string      `json:"why,omitempty"`
}

// Self asks the runtime where this gateway is published.
func Self(ctx context.Context) Published {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		p, err := selfKubernetes(ctx)
		if err != nil {
			return Published{Why: "could not read this gateway's Service: " + err.Error() +
				" - its service account needs get on pods and list on services in its namespace"}
		}
		return p
	}
	client, base, err := dockerClient()
	if err != nil {
		return Published{Why: "no runtime to ask: no Kubernetes, no Docker socket"}
	}
	host, _ := os.Hostname()
	var c struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := getJSON(ctx, client, base+"/containers/"+host+"/json", &c); err != nil {
		return Published{Why: "the Docker socket does not know this container: " + err.Error()}
	}
	// A Swarm task: the service publishes, not the container.
	if id := c.Config.Labels["com.docker.swarm.service.id"]; id != "" {
		var svc struct {
			Endpoint struct {
				Ports []struct {
					TargetPort    int `json:"TargetPort"`
					PublishedPort int `json:"PublishedPort"`
				} `json:"Ports"`
			} `json:"Endpoint"`
		}
		if err := getJSON(ctx, client, base+"/services/"+id, &svc); err != nil {
			return Published{Why: "the Swarm service of this task could not be read: " + err.Error()}
		}
		out := Published{Ports: map[int]int{}, Source: "swarm"}
		for _, p := range svc.Endpoint.Ports {
			if p.PublishedPort > 0 {
				out.Ports[p.TargetPort] = p.PublishedPort
			}
		}
		return out
	}
	out := Published{Ports: map[int]int{}, Source: "docker"}
	for key, binds := range c.NetworkSettings.Ports {
		inside, err := strconv.Atoi(strings.TrimSuffix(key, "/tcp"))
		if err != nil || len(binds) == 0 {
			continue
		}
		if outside, err := strconv.Atoi(binds[0].HostPort); err == nil {
			out.Ports[inside] = outside
		}
	}
	return out
}

// k8sSelf is this pod and the Services of its namespace, as the API server
// says them: what both Self and Internal read.
type k8sSelf struct {
	namespace string
	labels    map[string]string
	// named maps a container port's name to its number, for a Service whose
	// targetPort names it.
	named    map[string]int
	services []selfService
}

type selfService struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Type     string            `json:"type"`
		Selector map[string]string `json:"selector"`
		Ports    []struct {
			Port       int    `json:"port"`
			NodePort   int    `json:"nodePort"`
			TargetPort any    `json:"targetPort"`
			Name       string `json:"name"`
		} `json:"ports"`
	} `json:"spec"`
}

// readK8sSelf reads this pod, then every Service of the namespace.
func readK8sSelf(ctx context.Context) (k8sSelf, error) {
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return k8sSelf{}, fmt.Errorf("no service account token: %w", err)
	}
	ns, err := os.ReadFile(saDir + "/namespace")
	if err != nil {
		return k8sSelf{}, fmt.Errorf("no namespace: %w", err)
	}
	client, err := k8sClient()
	if err != nil {
		return k8sSelf{}, err
	}
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		port = "443"
	}
	self := k8sSelf{namespace: strings.TrimSpace(string(ns)), named: map[string]int{}}
	api := "https://" + net.JoinHostPort(os.Getenv("KUBERNETES_SERVICE_HOST"), port) +
		"/api/v1/namespaces/" + self.namespace
	auth := []string{"Authorization", "Bearer " + strings.TrimSpace(string(token))}
	host, _ := os.Hostname()
	var pod struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Ports []struct {
					Name          string `json:"name"`
					ContainerPort int    `json:"containerPort"`
				} `json:"ports"`
			} `json:"containers"`
		} `json:"spec"`
	}
	if err := getJSON(ctx, client, api+"/pods/"+host, &pod, auth...); err != nil {
		return k8sSelf{}, err
	}
	self.labels = pod.Metadata.Labels
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name != "" {
				self.named[p.Name] = p.ContainerPort
			}
		}
	}
	var list struct {
		Items []selfService `json:"items"`
	}
	if err := getJSON(ctx, client, api+"/services", &list, auth...); err != nil {
		return k8sSelf{}, err
	}
	self.services = list.Items
	return self, nil
}

// inside is the container port a Service port sends to.
func (s k8sSelf) inside(targetPort any, port int) int {
	switch t := targetPort.(type) {
	case float64:
		return int(t)
	case string:
		return s.named[t]
	case nil:
		return port
	}
	return 0
}

// selfKubernetes maps each port of this pod the way the world reaches it: a
// LoadBalancer on its port, a NodePort on its node port. A ClusterIP is only
// reachable from inside the cluster, so it publishes nothing here.
func selfKubernetes(ctx context.Context) (Published, error) {
	self, err := readK8sSelf(ctx)
	if err != nil {
		return Published{}, err
	}
	out := Published{Ports: map[int]int{}, Source: "kubernetes"}
	for _, svc := range self.services {
		if len(svc.Spec.Selector) == 0 || !selects(svc.Spec.Selector, self.labels) {
			continue
		}
		for _, p := range svc.Spec.Ports {
			inside := self.inside(p.TargetPort, p.Port)
			if inside == 0 {
				continue
			}
			switch svc.Spec.Type {
			case "LoadBalancer":
				out.Ports[inside] = p.Port
			case "NodePort":
				if _, set := out.Ports[inside]; !set {
					out.Ports[inside] = p.NodePort
				}
			}
		}
	}
	return out, nil
}

func selects(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
