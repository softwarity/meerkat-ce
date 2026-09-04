// Package discovery asks the runtime what services exist beside this gateway
// (SVC-02).
//
// The point is the moment a route is CREATED. An upstream typed by hand is
// where the typos live - a name that does not resolve, a port that belongs to
// the neighbouring service, a scheme forgotten - and none of them show up
// until traffic does. The runtime already knows the answer, so the console
// offers it.
//
// This is the DECLARED state, and it is not the same thing as the observed one
// the circuit breaker keeps (ROUTE-09). A service can be scheduled and ready
// and still be unreachable from here - a network policy, a wrong port, a
// sidecar not up yet - and it can be deliberately scaled to zero. The two
// disagreeing is exactly the case an operator is hunting, so the product keeps
// both rather than picking.
//
// No client library. The Docker API over its socket is plain HTTP, and the
// Kubernetes one is plain HTTPS with a token: two small readers here beat two
// dependencies whose transitive weight would dwarf this gateway.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service is one upstream the runtime knows about.
type Service struct {
	// Name is the identity: the full service name, unique cluster-wide.
	Name string `json:"name"`
	// Names is what actually RESOLVES to it from inside the network, best
	// first. A stack service answers to two names - the short alias the stack
	// registers ("mongodb") and the full service name ("neo_mongodb") - and
	// the short one is what people write. Offering only the long one is
	// technically correct and reads as wrong, which is worse than either.
	Names []string `json:"names,omitempty"`
	// Ports are what it listens on INSIDE the cluster - the number a route
	// should point at. A published port is where the outside world reaches it,
	// which is precisely what a gateway is replacing.
	Ports []Port `json:"ports,omitempty"`
	// Networks it is attached to, by name. A gateway that shares none of them
	// will not reach it whatever the console shows, so the console shows them.
	Networks []string `json:"networks,omitempty"`
	// Ready and Wanted are the DECLARED state: how many are running against
	// how many were asked for. Zero of zero is a service deliberately stopped,
	// which is not the same as a broken one.
	Ready  int `json:"ready"`
	Wanted int `json:"wanted"`
	// Reachable is false only when this gateway's own networks are KNOWN and
	// the service shares none of them. Then NO name resolves from here, short
	// or full - Swarm's DNS is per network, and a name outside it is not a
	// name at all. True whenever the answer is unknown: a guess that hides a
	// working upstream is worse than one that offers a doubtful one.
	Reachable bool `json:"reachable"`
	// Suggested is the url a route would use, when there is an obvious one.
	Suggested string `json:"suggested,omitempty"`
}

// Port is one listening port.
type Port struct {
	Target    int `json:"target"`
	Published int `json:"published,omitempty"`
}

// Result is what a lookup found, and where.
type Result struct {
	// Source is "swarm", "docker", "kubernetes" or "" when nothing answered.
	Source   string    `json:"source"`
	Services []Service `json:"services"`
	// Reach names the networks THIS gateway is on, when it could find out -
	// which is what decides whether a service is reachable, since a stack
	// boundary is not one. Empty when it could not: running outside a
	// container, typically, and then nothing is marked unreachable.
	Reach []string `json:"reach,omitempty"`
	// Why nothing answered, in a sentence naming what would make it work.
	// Empty when something did.
	Unavailable string `json:"unavailable,omitempty"`
}

// lookupTimeout bounds the whole thing. It runs from a console screen, and a
// runtime that does not answer must not hold that screen open.
const lookupTimeout = 5 * time.Second

// Discover asks whichever runtime is reachable.
//
// Kubernetes first, then Docker: an installation inside a Kubernetes cluster
// may well also have a Docker socket mounted for other reasons, and the
// orchestrator that schedules THIS gateway is the one whose answer is about
// the network it can actually reach.
func Discover(ctx context.Context) Result {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	if host := os.Getenv("KUBERNETES_SERVICE_HOST"); host != "" {
		r, err := fromKubernetes(ctx)
		if err == nil {
			return r
		}
		return Result{Unavailable: "this gateway runs in Kubernetes but could not read the services: " +
			err.Error() + " - its service account needs list on services in its own namespace"}
	}
	if r, err := fromDocker(ctx); err == nil {
		return r
	} else if !errIsNoSocket(err) {
		return Result{Unavailable: "the Docker socket answered, but not with services: " + err.Error()}
	}
	return Result{Unavailable: "no runtime to ask: mount the Docker socket at /var/run/docker.sock " +
		"(or set DOCKER_HOST), or run this gateway in Kubernetes with a service account that may list services"}
}

func errIsNoSocket(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), "connect: no such file") ||
		strings.Contains(err.Error(), "no such file or directory") ||
		strings.Contains(err.Error(), "connection refused"))
}

// ---- Docker, and Swarm on top of it ----

const defaultDockerSocket = "/var/run/docker.sock"

// dockerClient talks to the daemon over its socket. No API version in the
// path: the daemon answers with its own current one, which is what a client
// that only reads names and ports wants - pinning a version is how a gateway
// stops working the day somebody upgrades their engine.
func dockerClient() (*http.Client, string, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		if _, err := os.Stat(defaultDockerSocket); err != nil {
			return nil, "", err
		}
		host = "unix://" + defaultDockerSocket
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, "", fmt.Errorf("DOCKER_HOST %q: %w", host, err)
	}
	switch u.Scheme {
	case "unix", "":
		path := u.Path
		if path == "" {
			path = defaultDockerSocket
		}
		return &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", path)
			},
		}}, "http://docker", nil
	case "tcp", "http":
		return &http.Client{}, "http://" + u.Host, nil
	}
	return nil, "", fmt.Errorf("DOCKER_HOST %q: only unix:// and tcp:// are understood", host)
}

func fromDocker(ctx context.Context) (Result, error) {
	client, base, err := dockerClient()
	if err != nil {
		return Result{}, err
	}
	// Swarm first: a stack's services are what a route points at, and the
	// containers behind them come and go with every deployment.
	if r, err := fromSwarm(ctx, client, base); err == nil {
		return r, nil
	}
	return fromContainers(ctx, client, base)
}

type swarmService struct {
	Spec struct {
		Name         string            `json:"Name"`
		Labels       map[string]string `json:"Labels"`
		TaskTemplate struct {
			Networks []struct {
				Target  string   `json:"Target"`
				Aliases []string `json:"Aliases"`
			} `json:"Networks"`
			ContainerSpec struct {
				Image string `json:"Image"`
			} `json:"ContainerSpec"`
		} `json:"TaskTemplate"`
		EndpointSpec struct {
			Ports []struct {
				TargetPort    int `json:"TargetPort"`
				PublishedPort int `json:"PublishedPort"`
			} `json:"Ports"`
		} `json:"EndpointSpec"`
	} `json:"Spec"`
	Endpoint struct {
		Ports []struct {
			TargetPort    int `json:"TargetPort"`
			PublishedPort int `json:"PublishedPort"`
		} `json:"Ports"`
		VirtualIPs []struct {
			NetworkID string `json:"NetworkID"`
		} `json:"VirtualIPs"`
	} `json:"Endpoint"`
	ServiceStatus struct {
		RunningTasks int `json:"RunningTasks"`
		DesiredTasks int `json:"DesiredTasks"`
	} `json:"ServiceStatus"`
}

func fromSwarm(ctx context.Context, client *http.Client, base string) (Result, error) {
	var raw []swarmService
	if err := getJSON(ctx, client, base+"/services?status=true", &raw); err != nil {
		return Result{}, err
	}
	out := make([]Service, 0, len(raw))
	// The image each service runs, kept beside it for the fallback below.
	images := make([]string, 0, len(raw))
	for _, s := range raw {
		svc := Service{
			Name:   s.Spec.Name,
			Ready:  s.ServiceStatus.RunningTasks,
			Wanted: s.ServiceStatus.DesiredTasks,
		}
		ports := s.Spec.EndpointSpec.Ports
		if len(ports) == 0 {
			ports = s.Endpoint.Ports
		}
		for _, p := range ports {
			svc.Ports = append(svc.Ports, Port{Target: p.TargetPort, Published: p.PublishedPort})
		}
		for _, n := range s.Spec.TaskTemplate.Networks {
			svc.Networks = append(svc.Networks, n.Target)
		}
		images = append(images, s.Spec.TaskTemplate.ContainerSpec.Image)
		out = append(out, svc)
	}
	fillFromImages(ctx, client, base, out, images)
	// Order matters: the networks must be named before reach can be worked
	// out, and reach must be known before a short alias can be called safe.
	netName := nameTheNetworks(ctx, client, base, out)
	reach := markReach(ctx, client, base, out)
	nameTheServices(out, raw, netName, reach)
	for i := range out {
		out[i].Suggested = suggest(out[i])
	}
	sortByName(out)
	return Result{Source: "swarm", Services: out, Reach: reach}, nil
}

// markReach says which services this gateway can actually name.
//
// The stack is not the boundary - the NETWORK is. Swarm registers a service,
// and the short alias its stack asks for, in the DNS of each network it joins,
// so a gateway attached to that network resolves both whatever stack either of
// them belongs to. Cross-stack works, and the short name works across it.
//
// The other half of that sentence is the one that bites: a service on a
// network this gateway did NOT join resolves under no name at all. Reaching
// for the full name does not help - there is nothing to ask. So the answer is
// not "which stack" but "which network", and a list that cannot say so offers
// upstreams that provably cannot answer.
//
// Returns the networks this gateway is on, empty when it could not find out -
// running on a developer's host rather than in the cluster, typically. Nothing
// is marked unreachable then: a guess that hides a working upstream is worse
// than one that offers a doubtful one.
func markReach(ctx context.Context, client *http.Client, base string, out []Service) []string {
	mine := ownNetworks(ctx, client, base)
	if len(mine) == 0 {
		for i := range out {
			out[i].Reachable = true
		}
		return nil
	}
	for i := range out {
		for _, n := range out[i].Networks {
			if mine[n] {
				out[i].Reachable = true
				break
			}
		}
	}
	names := make([]string, 0, len(mine))
	for n := range mine {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ownNetworks asks the daemon what THIS container is attached to, by network
// name. Empty when the gateway is not a container the daemon knows.
func ownNetworks(ctx context.Context, client *http.Client, base string) map[string]bool {
	id := ownContainerID()
	if id == "" {
		return nil
	}
	var self struct {
		NetworkSettings struct {
			Networks map[string]struct{} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := getJSON(ctx, client, base+"/containers/"+id+"/json", &self); err != nil {
		return nil
	}
	mine := make(map[string]bool, len(self.NetworkSettings.Networks))
	for n := range self.NetworkSettings.Networks {
		mine[n] = true
	}
	return mine
}

var containerIDPattern = regexp.MustCompile(`[0-9a-f]{64}`)

// ownContainerID digs this process's container id out of the kernel.
//
// There is no API for it: a container is not told its own id. mountinfo
// carries it on both cgroup versions because the daemon bind-mounts
// /var/lib/docker/containers/<id>/... into every container; cgroup is the
// older fallback. Everything here is absent outside Linux, which is exactly
// the "cannot tell" answer the caller wants.
// A variable so a test can say what the kernel would have said: there is no
// /proc to write to, and the whole point of this function is to read one.
var ownContainerID = func() string {
	for _, f := range []string{"/proc/self/mountinfo", "/proc/self/cgroup"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := containerIDPattern.Find(b); m != nil {
			return string(m)
		}
	}
	return ""
}

// nameTheServices works out what actually resolves to each service.
//
// Swarm registers, per network, the aliases the stack asked for - and a stack
// asks for the SHORT name, so "neo_mongodb" answers to "mongodb" for anyone
// attached to the same network. Both work; the short one is what an operator
// writes, so it goes first.
//
// An alias is only offered when it is unambiguous ON THE NETWORK THAT CARRIES
// IT. DNS is per network, so the same short name under two stacks is fine as
// long as those stacks do not share a network - and when they do, the alias
// resolves to whichever Docker feels like, which is not something to put in
// front of somebody as a suggestion. The full service name is always last,
// because it is the one thing that can never be ambiguous.
func nameTheServices(out []Service, raw []swarmService, netName map[string]string, reach []string) {
	// The scope an alias competes in.
	//
	// When this gateway's own networks are known, every one of them is ONE
	// scope: a gateway attached to two networks that each carry a "cache"
	// resolves neither reliably, however unambiguous each network is on its
	// own. A service outside that reach competes nowhere - nothing about it
	// resolves from here, so only its full name is ever shown.
	//
	// When reach is unknown, per network is the best that can be said.
	mine := map[string]bool{}
	for _, n := range reach {
		mine[n] = true
	}
	scopeOf := func(id string) string {
		name := netName[id]
		if name == "" {
			name = id
		}
		if len(mine) == 0 {
			return name
		}
		if mine[name] {
			return "\x00reach"
		}
		return ""
	}

	claims := map[[2]string]int{}
	for _, s := range raw {
		for _, n := range s.Spec.TaskTemplate.Networks {
			scope := scopeOf(n.Target)
			if scope == "" {
				continue
			}
			for _, a := range n.Aliases {
				claims[[2]string{scope, a}]++
			}
		}
	}
	for i, s := range raw {
		seen := map[string]bool{out[i].Name: true}
		for _, n := range s.Spec.TaskTemplate.Networks {
			scope := scopeOf(n.Target)
			if scope == "" {
				continue
			}
			for _, a := range n.Aliases {
				if a == "" || seen[a] || claims[[2]string{scope, a}] > 1 {
					continue
				}
				seen[a] = true
				out[i].Names = append(out[i].Names, a)
			}
		}
		out[i].Names = append(out[i].Names, out[i].Name)
	}
}

// nameTheNetworks turns the ids a service listing carries into the names an
// operator reads. Best effort: an id nobody can name stays an id, which is
// still better than an empty field.
func nameTheNetworks(ctx context.Context, client *http.Client, base string, out []Service) map[string]string {
	var raw []struct {
		Name string `json:"Name"`
		ID   string `json:"Id"`
	}
	if err := getJSON(ctx, client, base+"/networks", &raw); err != nil {
		return nil
	}
	byID := make(map[string]string, len(raw))
	for _, n := range raw {
		byID[n.ID] = n.Name
	}
	for i := range out {
		for j, id := range out[i].Networks {
			if name := byID[id]; name != "" {
				out[i].Networks[j] = name
			}
		}
	}
	return byID
}

// fillFromImages gives a port to the services a Swarm listing leaves without
// one.
//
// A service's Endpoint carries only PUBLISHED ports, and a service a gateway
// exists to reach usually publishes nothing - it is reached inside the overlay
// network, which is the entire point of putting a gateway in front of it. On a
// real thirty-service stack that is a third of the list missing, and precisely
// the third that matters. The image's own EXPOSE is the next best source: it
// is the number the author of the service wrote down.
//
// It can be wrong - a base image that EXPOSEs 80 under an application
// listening on 3000 - which is why it only ever FILLS the field. An entry the
// operator corrects beats no entry at all, because no entry means typing the
// whole url from memory, which is the mistake this list exists to remove.
//
// Best effort throughout: the daemon knows only the images it holds, so a
// manager that never ran the task answers 404 and the service keeps no port.
func fillFromImages(ctx context.Context, client *http.Client, base string, out []Service, images []string) {
	want := map[string]bool{}
	for i := range out {
		if len(out[i].Ports) == 0 && images[i] != "" {
			want[images[i]] = true
		}
	}
	if len(want) == 0 {
		return
	}
	// One lookup per DISTINCT image: a stack routinely runs the same one under
	// several names, and this is a console screen waiting on the answer.
	refs := make([]string, 0, len(want))
	for ref := range want {
		refs = append(refs, ref)
	}
	found := imagePorts(ctx, client, base, refs)
	for i := range out {
		if len(out[i].Ports) == 0 {
			out[i].Ports = found[images[i]]
		}
	}
}

// imageLookups bounds how many inspections run at once - enough to keep a
// large stack under the lookup timeout, few enough not to flood a daemon that
// is also scheduling containers.
const imageLookups = 8

func imagePorts(ctx context.Context, client *http.Client, base string, refs []string) map[string][]Port {
	found := make(map[string][]Port, len(refs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	in := make(chan string)
	workers := min(imageLookups, len(refs))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ref := range in {
				ports := exposedPorts(ctx, client, base, ref)
				if len(ports) == 0 {
					continue
				}
				mu.Lock()
				found[ref] = ports
				mu.Unlock()
			}
		}()
	}
	for _, ref := range refs {
		in <- ref
	}
	close(in)
	wg.Wait()
	return found
}

func exposedPorts(ctx context.Context, client *http.Client, base, ref string) []Port {
	var img struct {
		Config struct {
			ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		} `json:"Config"`
	}
	if err := getJSON(ctx, client, base+"/images/"+url.PathEscape(ref)+"/json", &img); err != nil {
		return nil
	}
	ports := make([]Port, 0, len(img.Config.ExposedPorts))
	for spec := range img.Config.ExposedPorts {
		// "8080/tcp". A route speaks TCP; a udp port is not something it can
		// ever point at, so offering it would only be one more wrong answer.
		number, proto, _ := strings.Cut(spec, "/")
		if proto != "" && proto != "tcp" {
			continue
		}
		n, err := strconv.Atoi(number)
		if err != nil || n <= 0 {
			continue
		}
		ports = append(ports, Port{Target: n})
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Target < ports[j].Target })
	return ports
}

type dockerContainer struct {
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Labels map[string]string
	Ports  []struct {
		PrivatePort int `json:"PrivatePort"`
		PublicPort  int `json:"PublicPort"`
	} `json:"Ports"`
	NetworkSettings struct {
		Networks map[string]struct{} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// fromContainers is the plain-Docker answer: no swarm, so the services are
// containers, named by compose when there is one.
func fromContainers(ctx context.Context, client *http.Client, base string) (Result, error) {
	var raw []dockerContainer
	if err := getJSON(ctx, client, base+"/containers/json", &raw); err != nil {
		return Result{}, err
	}
	out := make([]Service, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		// The compose service name is what a sibling container resolves, and
		// the container name is a deployment detail that changes under it.
		if s := c.Labels["com.docker.compose.service"]; s != "" {
			name = s
		}
		if name == "" {
			continue
		}
		svc := Service{Name: name, Wanted: 1}
		if c.State == "running" {
			svc.Ready = 1
		}
		seen := map[int]bool{}
		for _, p := range c.Ports {
			if p.PrivatePort == 0 || seen[p.PrivatePort] {
				continue
			}
			seen[p.PrivatePort] = true
			svc.Ports = append(svc.Ports, Port{Target: p.PrivatePort, Published: p.PublicPort})
		}
		for n := range c.NetworkSettings.Networks {
			svc.Networks = append(svc.Networks, n)
		}
		sort.Strings(svc.Networks)
		// Compose already gives the short name, so there is no second one to
		// prefer - but the field is filled all the same, so the console has
		// one place to look whatever answered.
		svc.Names = []string{name}
		out = append(out, svc)
	}
	reach := markReach(ctx, client, base, out)
	for i := range out {
		out[i].Suggested = suggest(out[i])
	}
	sortByName(out)
	return Result{Source: "docker", Services: out, Reach: reach}, nil
}

// ---- Kubernetes, from inside the cluster ----

const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

type k8sList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Type      string `json:"type"`
			ClusterIP string `json:"clusterIP"`
			Ports     []struct {
				Port     int    `json:"port"`
				NodePort int    `json:"nodePort"`
				Name     string `json:"name"`
			} `json:"ports"`
		} `json:"spec"`
	} `json:"items"`
}

// fromKubernetes reads the services of THIS gateway's own namespace.
//
// Its own, and not the cluster's: a gateway that could list every namespace
// would be asking for a right nobody needs to answer "what can I route to",
// and the answer would be a list of things it cannot reach anyway.
//
// Only from INSIDE the cluster. A kubeconfig is a different job - contexts,
// client certificates, exec plugins to fetch a token from a cloud provider -
// and on a laptop the Docker socket already answers, which is what the
// developer loop needs.
func fromKubernetes(ctx context.Context) (Result, error) {
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return Result{}, fmt.Errorf("no service account token: %w", err)
	}
	ns, err := os.ReadFile(saDir + "/namespace")
	if err != nil {
		return Result{}, fmt.Errorf("no namespace: %w", err)
	}
	client, err := k8sClient()
	if err != nil {
		return Result{}, err
	}
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		port = "443"
	}
	endpoint := fmt.Sprintf("https://%s/api/v1/namespaces/%s/services", net.JoinHostPort(host, port), string(ns))

	var list k8sList
	if err := getJSON(ctx, client, endpoint, &list, "Authorization", "Bearer "+strings.TrimSpace(string(token))); err != nil {
		return Result{}, err
	}
	out := make([]Service, 0, len(list.Items))
	for _, it := range list.Items {
		// A headless service has no cluster IP and is resolved to the pods
		// themselves - still routable, so still offered.
		// A Kubernetes service resolves under its bare name inside its own
		// namespace, and under the fully qualified one from anywhere else.
		// Both are offered, the short one first: this gateway runs in that
		// namespace, which is where the short one works.
		svc := Service{
			Name:     it.Metadata.Name,
			Names:    []string{it.Metadata.Name, it.Metadata.Name + "." + string(ns) + ".svc"},
			Networks: []string{string(ns)},
		}
		for _, p := range it.Spec.Ports {
			svc.Ports = append(svc.Ports, Port{Target: p.Port, Published: p.NodePort})
		}
		// Kubernetes answers readiness through Endpoints, a second call per
		// service. Not made: this list is for choosing an upstream, and the
		// observed state (ROUTE-09) is what says whether it answers.
		//
		// Reachable without asking: this listing is the gateway's OWN
		// namespace, and a pod resolves every service in it.
		svc.Reachable = true
		svc.Suggested = suggest(svc)
		out = append(out, svc)
	}
	sortByName(out)
	return Result{Source: "kubernetes", Services: out, Reach: []string{string(ns)}}, nil
}

func k8sClient() (*http.Client, error) {
	pool, err := k8sCertPool()
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: pool}}, nil
}

// ---- shared ----

func getJSON(ctx context.Context, client *http.Client, endpoint string, into any, headers ...string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", endpoint, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(into)
}

// suggest builds the url a route would use: the service's own name and the
// port it listens on inside. Only when there is ONE obvious port - a service
// exposing three is a choice somebody has to make, and guessing it would be
// worse than leaving the field to them.
func suggest(s Service) string {
	if len(s.Ports) != 1 {
		return ""
	}
	if s.Ports[0].Target == 0 {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", s.best(), s.Ports[0].Target)
}

// best is the name to put in front of somebody: the first that resolves, and
// the full service name when nothing else was collected.
func (s Service) best() string {
	if len(s.Names) > 0 {
		return s.Names[0]
	}
	return s.Name
}

func sortByName(s []Service) {
	sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name })
}
