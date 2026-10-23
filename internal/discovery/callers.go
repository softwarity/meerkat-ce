package discovery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Who is calling the internal plane, asked of the runtime (SCHED-01).
//
// Two questions, and the runtime answers both without the caller doing
// anything. Is this bearer a ServiceAccount token the cluster vouches for?
// The API server says, through a TokenReview: the token is checked where it
// was issued, and nothing about it has to be configured here. Which workload
// holds this address? The pods know their addresses on Kubernetes, and the
// Docker daemon knows its containers' and Swarm its tasks'.
//
// The second is a deduction, not a proof: an address says where a request
// came from, and anything able to take a pod's address could take its name.
// It is for tracing and filtering inside a cluster that trusts itself, and the
// console says so.
//
// Both are kept a minute. A busy service calls the scheduler many times a
// minute, and one API call per request would be the gateway asking the API
// server more often than the service asks the gateway.

// callerTTL is how long an answer is kept, a good one or a refusal.
const callerTTL = time.Minute

// callerCacheMost bounds each cache: a burst of distinct tokens or addresses
// costs memory once, and is swept at the next write past this.
const callerCacheMost = 4096

// Callers answers who a caller is. Nil-safe at none of its methods: build one
// with NewCallers, which knows what this gateway runs on.
type Callers struct {
	// api is the Kubernetes API, nil off Kubernetes.
	api  *k8sAPI
	root string // https://host:port
	// docker opens the daemon's socket, nil on Kubernetes.
	docker func() (*http.Client, string, error)

	now     func() time.Time
	mu      sync.Mutex
	reviews map[string]remembered
	addrs   map[string]remembered
	owners  map[string]remembered
}

type remembered struct {
	identity string
	via      string
	at       time.Time
}

// The ways an address was read, as the schedule records them.
const (
	viaWorkload  = "workload"
	viaContainer = "container"
)

// NewCallers asks the runtime this gateway runs in: Kubernetes when it is
// in a pod, the Docker socket otherwise.
func NewCallers() *Callers {
	c := newCallers()
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		if api, err := newK8sAPI(); err == nil {
			c.api = api
			c.root = strings.TrimSuffix(api.base, "/api/v1/namespaces/"+api.ns)
		}
		return c
	}
	c.docker = dockerClient
	return c
}

func newCallers() *Callers {
	return &Callers{now: time.Now, reviews: map[string]remembered{}, addrs: map[string]remembered{},
		owners: map[string]remembered{}}
}

// Kubernetes says whether a ServiceAccount token can be checked here.
func (c *Callers) Kubernetes() bool { return c.api != nil }

// Review checks token with the API server. It answers namespace/serviceaccount
// when the cluster vouches for it as a ServiceAccount's, "" when it does not -
// an expired token, a forged one, a person's - and an error only when the
// question could not be asked.
func (c *Callers) Review(ctx context.Context, token string) (string, error) {
	if c.api == nil {
		return "", fmt.Errorf("a ServiceAccount token is checked by the Kubernetes API, and this gateway does not run on Kubernetes")
	}
	sum := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(sum[:])
	if r, ok := c.recall(c.reviews, key); ok {
		return r.identity, nil
	}
	body := map[string]any{
		"apiVersion": "authentication.k8s.io/v1",
		"kind":       "TokenReview",
		"spec":       map[string]any{"token": token},
	}
	var review struct {
		Status struct {
			Authenticated bool `json:"authenticated"`
			User          struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"status"`
	}
	if err := c.send(ctx, http.MethodPost, c.root+"/apis/authentication.k8s.io/v1/tokenreviews", body, &review,
		"checking a ServiceAccount token (TokenReview)"); err != nil {
		return "", err
	}
	identity := ""
	if review.Status.Authenticated {
		identity = serviceAccountOf(review.Status.User.Username)
	}
	c.remember(c.reviews, key, remembered{identity: identity})
	return identity, nil
}

// serviceAccountOf turns system:serviceaccount:<ns>:<name> into <ns>/<name>,
// and anything else - a person, a node - into "": only a ServiceAccount is a
// service.
func serviceAccountOf(username string) string {
	rest, ok := strings.CutPrefix(username, "system:serviceaccount:")
	if !ok {
		return ""
	}
	ns, name, ok := strings.Cut(rest, ":")
	if !ok || ns == "" || name == "" {
		return ""
	}
	return ns + "/" + name
}

// ByAddress names what holds ip: on Kubernetes the workload owning the pod at
// that address (namespace/Kind/name), on Docker the Swarm service, the compose
// service or the container. Empty when nothing holds it, or nothing could be
// asked.
func (c *Callers) ByAddress(ctx context.Context, ip string) (identity, via string) {
	if ip == "" {
		return "", ""
	}
	if r, ok := c.recall(c.addrs, ip); ok {
		return r.identity, r.via
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	var r remembered
	switch {
	case c.api != nil:
		pods, err := c.podsAt(ctx, ip)
		if err != nil {
			return "", ""
		}
		for _, p := range pods {
			if id := c.workloadOf(ctx, p); id != "" {
				r = remembered{identity: id, via: viaWorkload}
				break
			}
		}
	case c.docker != nil:
		client, base, err := c.docker()
		if err != nil {
			return "", ""
		}
		if id := containerAt(ctx, client, base, ip); id != "" {
			r = remembered{identity: id, via: viaContainer}
		}
	}
	c.remember(c.addrs, ip, r)
	return r.identity, r.via
}

// ServiceAccounts lists the cluster's, as namespace/name: every namespace when
// the gateway may list them there, its own and the watched ones otherwise.
func (c *Callers) ServiceAccounts(ctx context.Context) ([]string, error) {
	if c.api == nil {
		return nil, fmt.Errorf("ServiceAccounts are Kubernetes', and this gateway does not run on Kubernetes")
	}
	var out []string
	err := c.eachNamespace(ctx, "serviceaccounts", func(raw json.RawMessage) error {
		var list struct {
			Items []struct {
				Metadata podMeta `json:"metadata"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return err
		}
		for _, it := range list.Items {
			out = append(out, it.Metadata.Namespace+"/"+it.Metadata.Name)
		}
		return nil
	})
	return sorted(out), err
}

// Workloads lists what a caller can be deduced as: on Kubernetes the workloads
// owning a running pod, on Docker the services and containers discovery
// already reads.
func (c *Callers) Workloads(ctx context.Context) ([]string, error) {
	if c.api == nil {
		r := Discover(ctx)
		if r.Unavailable != "" {
			return nil, fmt.Errorf("%s", r.Unavailable)
		}
		out := make([]string, 0, len(r.Services))
		for _, s := range r.Services {
			out = append(out, s.Name)
		}
		return sorted(out), nil
	}
	var pods []callerPod
	err := c.eachNamespace(ctx, "pods", func(raw json.RawMessage) error {
		var list struct {
			Items []callerPod `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return err
		}
		pods = append(pods, list.Items...)
		return nil
	})
	var out []string
	for _, p := range pods {
		if p.Status.Phase != "Running" || p.Spec.HostNetwork {
			continue
		}
		if id := c.workloadOf(ctx, p); id != "" {
			out = append(out, id)
		}
	}
	return sorted(out), err
}

// ---- Kubernetes ----

type podMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Labels          map[string]string `json:"labels"`
	OwnerReferences []struct {
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		Controller bool   `json:"controller"`
	} `json:"ownerReferences"`
}

type callerPod struct {
	Metadata podMeta `json:"metadata"`
	Spec     struct {
		HostNetwork bool `json:"hostNetwork"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
		PodIP string `json:"podIP"`
	} `json:"status"`
}

// podsAt are the running pods holding ip, a host-network pod last: it shares
// its node's address with every other one there, so it names a node rather
// than a workload.
func (c *Callers) podsAt(ctx context.Context, ip string) ([]callerPod, error) {
	selector := "?fieldSelector=" + url.QueryEscape("status.podIP="+ip)
	var pods []callerPod
	err := c.eachNamespaceQuery(ctx, "pods", selector, func(raw json.RawMessage) error {
		var list struct {
			Items []callerPod `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return err
		}
		pods = append(pods, list.Items...)
		return nil
	})
	if err != nil && len(pods) == 0 {
		return nil, err
	}
	var out, host []callerPod
	for _, p := range pods {
		if p.Status.Phase != "Running" || p.Status.PodIP != ip {
			continue
		}
		if p.Spec.HostNetwork {
			host = append(host, p)
			continue
		}
		out = append(out, p)
	}
	if len(out) > 0 {
		return out, nil
	}
	// Several workloads behind one node address cannot be told apart: only
	// a host-network pod alone at it is named.
	if len(host) == 1 {
		return host, nil
	}
	return nil, nil
}

// workloadOf names the workload owning p, qualified by its namespace:
// canopy/Deployment/reference-data. A ReplicaSet is followed to the
// Deployment that made it - by the template hash it carries in its name, and
// by asking for the ReplicaSet when the label does not say; a StatefulSet, a
// DaemonSet, a Job or anything else owning it is named as itself; a pod
// nothing owns is named as a pod.
func (c *Callers) workloadOf(ctx context.Context, p callerPod) string {
	ns := p.Metadata.Namespace
	if ns == "" {
		return ""
	}
	for _, o := range p.Metadata.OwnerReferences {
		if !o.Controller && len(p.Metadata.OwnerReferences) > 1 {
			continue
		}
		if o.Kind != "ReplicaSet" {
			return ns + "/" + o.Kind + "/" + o.Name
		}
		if hash := p.Metadata.Labels["pod-template-hash"]; hash != "" {
			if base, ok := strings.CutSuffix(o.Name, "-"+hash); ok {
				return ns + "/Deployment/" + base
			}
		}
		return ns + "/" + c.replicaSetOwner(ctx, ns, o.Name)
	}
	return ns + "/Pod/" + p.Metadata.Name
}

// replicaSetOwner is Kind/name of what owns a ReplicaSet - its Deployment, as
// a rule - and the ReplicaSet itself when nothing does or nobody may say.
func (c *Callers) replicaSetOwner(ctx context.Context, ns, name string) string {
	key := ns + "/" + name
	if r, ok := c.recall(c.owners, key); ok {
		return r.identity
	}
	owner := "ReplicaSet/" + name
	var rs struct {
		Metadata podMeta `json:"metadata"`
	}
	u := c.root + "/apis/apps/v1/namespaces/" + url.PathEscape(ns) + "/replicasets/" + url.PathEscape(name)
	if err := c.send(ctx, http.MethodGet, u, nil, &rs, "reading ReplicaSet "+key); err == nil {
		for _, o := range rs.Metadata.OwnerReferences {
			owner = o.Kind + "/" + o.Name
			break
		}
	}
	c.remember(c.owners, key, remembered{identity: owner})
	return owner
}

// eachNamespace reads one kind across the cluster, or namespace by namespace
// over the ones this gateway is granted when the cluster-wide list is refused.
func (c *Callers) eachNamespace(ctx context.Context, kind string, read func(json.RawMessage) error) error {
	return c.eachNamespaceQuery(ctx, kind, "", read)
}

// eachNamespaceQuery is eachNamespace with a query. The chart reads pods
// namespace by namespace on purpose - a pod's manifest carries its
// environment - so the cluster-wide list is refused there as a rule, and that
// refusal is remembered a minute rather than asked before every namespace.
func (c *Callers) eachNamespaceQuery(ctx context.Context, kind, query string, read func(json.RawMessage) error) error {
	if !c.clusterRefused(kind) {
		var raw json.RawMessage
		err := c.send(ctx, http.MethodGet, c.root+"/api/v1/"+kind+query, nil, &raw, "listing "+kind+" across the cluster")
		if err == nil {
			return read(raw)
		}
		if _, denied := err.(refused); !denied {
			return err
		}
		c.remember(c.owners, "\x00cluster/"+kind, remembered{identity: "refused"})
	}
	var last error
	for _, ns := range watchedNamespaces(c.api.ns) {
		var part json.RawMessage
		u := c.root + "/api/v1/namespaces/" + url.PathEscape(ns) + "/" + kind + query
		if err := c.send(ctx, http.MethodGet, u, nil, &part, "listing "+kind+" in namespace "+ns); err != nil {
			last = err
			continue
		}
		if err := read(part); err != nil {
			last = err
		}
	}
	return last
}

// clusterRefused says the cluster-wide list of kind was refused within the
// minute.
func (c *Callers) clusterRefused(kind string) bool {
	_, ok := c.recall(c.owners, "\x00cluster/"+kind)
	return ok
}

// send asks the API server, with the gateway's own ServiceAccount.
func (c *Callers) send(ctx context.Context, method, u string, body, out any, what string) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.api.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.api.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if err := answer(res, what, out); err != nil {
		if isCode(err, http.StatusForbidden) {
			return refused(what)
		}
		return err
	}
	return nil
}

// refused is the API server saying no to this gateway's ServiceAccount, in the
// words that name the grant: the ServiceAccounts and the TokenReviews are the
// cluster-wide half (rbac.internalCallers), the pods and their ReplicaSets the
// namespaces the gateway already reads (rbac.watch, rbac.watchNamespaces).
type refused string

func (r refused) Error() string {
	grant := "rbac.internalCallers"
	if strings.Contains(string(r), "pods") || strings.Contains(string(r), "ReplicaSet") {
		grant = "rbac.watch in its own namespace, rbac.watchNamespaces in another"
	}
	return string(r) + ": its service account may not - the chart grants it with " + grant
}

// ---- Docker and Swarm ----

// containerAt names what holds ip on Docker: a container of this daemon whose
// network address it is, and failing that a Swarm task on any node - a
// manager knows them all.
func containerAt(ctx context.Context, client *http.Client, base, ip string) string {
	var containers []struct {
		Names           []string          `json:"Names"`
		Labels          map[string]string `json:"Labels"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := getJSON(ctx, client, base+"/containers/json", &containers); err == nil {
		for _, ctr := range containers {
			for _, n := range ctr.NetworkSettings.Networks {
				if n.IPAddress != ip {
					continue
				}
				name := ""
				if len(ctr.Names) > 0 {
					name = strings.TrimPrefix(ctr.Names[0], "/")
				}
				return containerIdentity(ctr.Labels, name)
			}
		}
	}
	var tasks []struct {
		ServiceID           string `json:"ServiceID"`
		NetworksAttachments []struct {
			Addresses []string `json:"Addresses"`
		} `json:"NetworksAttachments"`
	}
	filters := url.QueryEscape(`{"desired-state":["running"]}`)
	if err := getJSON(ctx, client, base+"/tasks?filters="+filters, &tasks); err != nil {
		return ""
	}
	for _, t := range tasks {
		for _, n := range t.NetworksAttachments {
			for _, a := range n.Addresses {
				if host, _, _ := strings.Cut(a, "/"); host != ip {
					continue
				}
				var svc struct {
					Spec struct {
						Name string `json:"Name"`
					} `json:"Spec"`
				}
				if err := getJSON(ctx, client, base+"/services/"+url.PathEscape(t.ServiceID), &svc); err != nil {
					return ""
				}
				return svc.Spec.Name
			}
		}
	}
	return ""
}

// containerIdentity is the name a container is known by: its Swarm service,
// its compose service, or its own name - the same names discovery lists, so
// what the console offers is what a caller is matched as.
func containerIdentity(labels map[string]string, name string) string {
	if s := labels["com.docker.swarm.service.name"]; s != "" {
		return s
	}
	if s := labels["com.docker.compose.service"]; s != "" {
		return s
	}
	return name
}

// ---- the caches ----

func (c *Callers) recall(m map[string]remembered, key string) (remembered, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := m[key]
	if !ok || c.now().Sub(r.at) >= callerTTL {
		return remembered{}, false
	}
	return r, true
}

func (c *Callers) remember(m map[string]remembered, key string, r remembered) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(m) >= callerCacheMost {
		for k, v := range m {
			if now.Sub(v.at) >= callerTTL {
				delete(m, k)
			}
		}
		if len(m) >= callerCacheMost {
			clear(m)
		}
	}
	r.at = now
	m[key] = r
}

// sorted is the list deduplicated and in order: what a select offers.
func sorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
