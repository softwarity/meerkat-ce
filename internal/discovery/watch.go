package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// The runtime, watched rather than asked (SVC-07).
//
// Discover answers a question once, which is right for a console screen that
// lists what a route could point at. The routes screen asks a different one,
// all the time: is the service behind each route there, with how many of its
// replicas, running which image? Asking that on a timer is a poll - late by
// half its period on the day it matters, and a cost paid every period on the
// days nothing happens. Both runtimes say when something changes, so this
// listens instead:
//
//   - Kubernetes: one LIST of the services and pods of this gateway's
//     namespace, then a WATCH of each from that list's resourceVersion. A pod
//     that starts, becomes ready, crashes or is replaced arrives as an event.
//   - Docker and Swarm: one inventory, then the daemon's /events stream. In a
//     Swarm a container's events are emitted only by the node it runs on, so
//     every node is listened to: through the socket proxy deployed on each of
//     them (deploy/stack.swarm.yml), found by Swarm's tasks.<name> DNS.
//
// An event does not carry the whole truth (a Swarm service's running count, a
// pod's place behind a Service), so it is applied (Kubernetes) or answered by
// a fresh inventory a moment later (Docker), several events folding into one.
// A stream that breaks is reopened after a short wait, starting with a fresh
// inventory: nothing that happened in between is missed.

// eventSettle is how long a burst of events is let finish before the state is
// read again: a deployment is a dozen events in a second, and one inventory
// answers them all.
const eventSettle = 750 * time.Millisecond

// retryMin and retryMax bound the wait before a broken stream is reopened.
const (
	retryMin = time.Second
	retryMax = 30 * time.Second
)

// Watcher keeps the runtime's current answer, and says when it changes.
type Watcher struct {
	mu    sync.RWMutex
	res   Result
	known bool

	changed chan struct{}
}

// NewWatcher returns a watcher that knows nothing until Run has read the
// runtime once.
func NewWatcher() *Watcher {
	return &Watcher{changed: make(chan struct{}, 1)}
}

// current is the watcher this process runs, which Discover answers from once
// it has read the runtime: one source for the routes screen and the upstream
// picker, and no second inventory for the picker to pay.
var (
	currentMu sync.RWMutex
	current   *Watcher
)

// Result is the last answer, and whether there is one yet.
func (w *Watcher) Result() (Result, bool) {
	if w == nil {
		return Result{}, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.res, w.known
}

// Changed receives after each change of the answer. One slot: a reader that
// was busy reads the latest state once, not every change it missed.
func (w *Watcher) Changed() <-chan struct{} { return w.changed }

func (w *Watcher) set(r Result) {
	w.mu.Lock()
	w.res, w.known = r, true
	w.mu.Unlock()
	select {
	case w.changed <- struct{}{}:
	default:
	}
}

// Run watches whichever runtime is reachable until ctx ends. Without one it
// records why, once, and returns: there is nothing to listen to, and the
// targets are then found out the way an external host is.
func (w *Watcher) Run(ctx context.Context) {
	currentMu.Lock()
	current = w
	currentMu.Unlock()
	defer func() {
		currentMu.Lock()
		if current == w {
			current = nil
		}
		currentMu.Unlock()
	}()

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		w.watchKubernetes(ctx)
		return
	}
	if _, _, err := dockerClient(); err != nil {
		w.set(Discover(ctx))
		return
	}
	w.watchDocker(ctx)
}

// backoff waits before a retry, doubling each time, and says false when ctx
// ended meanwhile.
func backoff(ctx context.Context, wait *time.Duration) bool {
	t := time.NewTimer(*wait)
	defer t.Stop()
	*wait = min(*wait*2, retryMax)
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// ---- Docker and Swarm ----

// dockerEvent is the part of an /events line this reads.
type dockerEvent struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
}

// matters says whether an event can change what a route is told: a container
// starting, stopping or changing health, a service or a node changing. The
// rest - an exec, an attach, a terminal resize - changes nothing anybody
// routes to.
func (e dockerEvent) matters() bool {
	switch e.Type {
	case "service", "node":
		return true
	case "container":
		a := e.Action
		return a == "start" || a == "die" || a == "stop" || a == "kill" || a == "destroy" ||
			a == "pause" || a == "unpause" || a == "restart" || strings.HasPrefix(a, "health_status")
	}
	return false
}

func (w *Watcher) watchDocker(ctx context.Context) {
	client, base, err := dockerClient()
	if err != nil {
		w.set(Discover(ctx))
		return
	}
	// The nodes to listen to: every task of the socket proxy when it runs as
	// a Swarm service (tasks.<name> resolves to one address per task), or the
	// one daemon otherwise.
	nodes := map[string]context.CancelFunc{}
	kick := make(chan struct{}, 1)
	poke := func() {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
	defer func() {
		for _, stop := range nodes {
			stop()
		}
	}()
	listen := func() {
		want := map[string]string{}
		for _, b := range dockerNodes(ctx, base) {
			want[b] = b
		}
		for b, stop := range nodes {
			if _, ok := want[b]; !ok {
				stop()
				delete(nodes, b)
			}
		}
		for b := range want {
			if _, ok := nodes[b]; ok {
				continue
			}
			nctx, stop := context.WithCancel(ctx)
			nodes[b] = stop
			go w.dockerEvents(nctx, client, b, poke)
		}
	}

	inventory := func() {
		r := dockerInventory(ctx, client, base)
		w.set(r)
	}
	inventory()
	listen()
	for {
		select {
		case <-ctx.Done():
			return
		case <-kick:
		}
		// Let the burst finish, then read once.
		settle := time.NewTimer(eventSettle)
		select {
		case <-ctx.Done():
			settle.Stop()
			return
		case <-settle.C:
		}
		select {
		case <-kick:
		default:
		}
		inventory()
		// A node that joined since shows up as a new proxy task.
		listen()
	}
}

// dockerInventory reads the runtime once. Through a socket proxy that runs
// on every node, the address answering may be a worker, which cannot list
// Swarm services: each node is asked in turn until a manager answers, and only
// when none does is the plain container list the answer.
func dockerInventory(ctx context.Context, client *http.Client, base string) Result {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	for _, b := range dockerNodes(ctx, base) {
		if r, err := fromSwarm(ctx, client, b); err == nil {
			return r
		}
	}
	r, err := fromContainers(ctx, client, base)
	if err != nil {
		return Result{Unavailable: "the Docker socket answered, but not with services: " + err.Error()}
	}
	return r
}

// dockerNodes is every daemon to listen to: the addresses of tasks.<host>
// when the Docker host is a Swarm service reached over TCP, else the one
// base. A unix socket is one daemon, the one this gateway's node runs.
func dockerNodes(ctx context.Context, base string) []string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "docker" {
		return []string{base}
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return []string{base}
	}
	if net.ParseIP(host) != nil {
		return []string{base}
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, "tasks."+host)
	if err != nil || len(ips) == 0 {
		return []string{base}
	}
	sort.Strings(ips)
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, u.Scheme+"://"+net.JoinHostPort(ip, port))
	}
	return out
}

// dockerEvents follows one daemon's /events until ctx ends, reopening it when
// it breaks, and pokes on every event that matters - and on every reopening,
// since what happened while it was down is unknown.
func (w *Watcher) dockerEvents(ctx context.Context, client *http.Client, base string, poke func()) {
	wait := retryMin
	first := true
	filters := url.QueryEscape(`{"type":["container","service","node"]}`)
	for {
		if !first {
			poke()
		}
		first = false
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/events?filters="+filters, nil)
		if err != nil {
			return
		}
		res, err := client.Do(req)
		if err == nil && res.StatusCode == http.StatusOK {
			wait = retryMin
			sc := bufio.NewScanner(res.Body)
			sc.Buffer(make([]byte, 64<<10), 1<<20)
			for sc.Scan() {
				var e dockerEvent
				if json.Unmarshal(sc.Bytes(), &e) == nil && e.matters() {
					poke()
				}
			}
		}
		if res != nil {
			_ = res.Body.Close()
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			slog.Debug("docker events stream broke, reopening", "daemon", base, "err", err)
		}
		if !backoff(ctx, &wait) {
			return
		}
	}
}

// ---- Kubernetes ----

type k8sMeta struct {
	Name              string            `json:"name"`
	Labels            map[string]string `json:"labels"`
	ResourceVersion   string            `json:"resourceVersion"`
	DeletionTimestamp *string           `json:"deletionTimestamp"`
}

type k8sService struct {
	Metadata k8sMeta `json:"metadata"`
	Spec     struct {
		Selector map[string]string `json:"selector"`
		Ports    []struct {
			Port     int `json:"port"`
			NodePort int `json:"nodePort"`
		} `json:"ports"`
	} `json:"spec"`
}

type k8sPod struct {
	Metadata k8sMeta `json:"metadata"`
	Spec     struct {
		Containers []struct {
			Name  string `json:"name"`
			Image string `json:"image"`
		} `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		ContainerStatuses []struct {
			Name    string `json:"name"`
			Image   string `json:"image"`
			ImageID string `json:"imageID"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

// k8sAPI is what talking to the API server needs.
type k8sAPI struct {
	client *http.Client
	base   string // https://host:port/api/v1/namespaces/<ns>
	ns     string
	token  string
}

func newK8sAPI() (*k8sAPI, error) {
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return nil, fmt.Errorf("no service account token: %w", err)
	}
	ns, err := os.ReadFile(saDir + "/namespace")
	if err != nil {
		return nil, fmt.Errorf("no namespace: %w", err)
	}
	client, err := k8sClient()
	if err != nil {
		return nil, err
	}
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		port = "443"
	}
	n := strings.TrimSpace(string(ns))
	return &k8sAPI{client: client, ns: n, token: strings.TrimSpace(string(token)),
		base: fmt.Sprintf("https://%s/api/v1/namespaces/%s", net.JoinHostPort(host, port), n)}, nil
}

// in is the same API, aimed at another namespace.
func (a *k8sAPI) in(ns string) *k8sAPI {
	b := *a
	b.ns = ns
	b.base = a.base[:strings.LastIndex(a.base, "/")+1] + ns
	return &b
}

// watchedNamespaces is this gateway's own namespace first, then the others
// MEERKAT_WATCH_NAMESPACES names - the ones the chart granted it a read in
// (rbac.watchNamespaces), for routes whose upstream lives there.
func watchedNamespaces(own string) []string {
	out := []string{own}
	seen := map[string]bool{own: true}
	for _, n := range strings.Split(os.Getenv("MEERKAT_WATCH_NAMESPACES"), ",") {
		if n = strings.TrimSpace(n); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// k8sState is the namespace as the watches last left it.
type k8sState struct {
	mu       sync.Mutex
	services map[string]k8sService
	pods     map[string]k8sPod
}

func (w *Watcher) watchKubernetes(ctx context.Context) {
	api, err := newK8sAPI()
	if err != nil {
		w.set(Result{Unavailable: "this gateway runs in Kubernetes but cannot talk to it: " + err.Error()})
		return
	}
	kick := make(chan struct{}, 1)
	poke := func() {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
	// One state per namespace, each with its two watches. Without the right
	// to read pods, the services are still worth listing: the picker offers
	// them, and their targets are dialled as before. Said once per namespace,
	// in a sentence naming the grant.
	type watched struct {
		api     *k8sAPI
		st      *k8sState
		counted bool
	}
	spaces := watchedNamespaces(api.ns)
	all := make([]*watched, 0, len(spaces))
	podsDenied := make(chan string, len(spaces)*2)
	for _, ns := range spaces {
		w := &watched{api: api.in(ns), st: &k8sState{services: map[string]k8sService{}, pods: map[string]k8sPod{}}, counted: true}
		all = append(all, w)
		svcDenied := podsDenied
		if ns == api.ns {
			svcDenied = nil // its own services: Discover says why when it cannot list them
		}
		go watchK8s(ctx, w.api, "services", w.st, poke, svcDenied)
		go watchK8s(ctx, w.api, "pods", w.st, poke, podsDenied)
	}
	denied := ""
	for {
		select {
		case <-ctx.Done():
			return
		case why := <-podsDenied:
			for _, sp := range all {
				if strings.Contains(why, "namespace "+sp.api.ns+" ") {
					sp.counted = false
				}
			}
			denied = why
			poke()
			continue
		case <-kick:
		}
		settle := time.NewTimer(eventSettle / 3)
		select {
		case <-ctx.Done():
			settle.Stop()
			return
		case <-settle.C:
		}
		select {
		case <-kick:
		default:
		}
		r := Result{Source: "kubernetes", Reach: []string{api.ns}}
		for _, sp := range all {
			part := sp.st.result(sp.api.ns, sp.counted, sp.api.ns == api.ns)
			r.Services = append(r.Services, part.Services...)
		}
		sortByName(r.Services)
		w.set(r)
		if denied != "" {
			slog.Warn("the routes screen shows no replicas: " + denied)
			denied = ""
		}
	}
}

// watchK8s keeps one kind of object in st: a LIST, then a WATCH from its
// resourceVersion, applied event by event; a watch that ends is resumed from
// the last version seen, and one the server says is too old ("410 Gone") is
// replaced by a fresh LIST.
func watchK8s(ctx context.Context, api *k8sAPI, kind string, st *k8sState, poke func(), denied chan<- string) {
	wait := retryMin
	for ctx.Err() == nil {
		rv, err := api.list(ctx, kind, st)
		if err != nil {
			if denied != nil && strings.Contains(err.Error(), "403") {
				denied <- "its service account may not list and watch " + kind + " in namespace " + api.ns +
					" (the Helm chart grants it with rbac.watch for its own namespace, rbac.watchNamespaces for another)"
				return
			}
			slog.Debug("kubernetes list failed, retrying", "kind", kind, "err", err)
			if !backoff(ctx, &wait) {
				return
			}
			continue
		}
		poke()
		wait = retryMin
		for ctx.Err() == nil {
			next, gone, err := api.watch(ctx, kind, rv, st, poke)
			if gone {
				break // too old: list again
			}
			if err != nil {
				if !backoff(ctx, &wait) {
					return
				}
				continue
			}
			wait = retryMin
			rv = next
		}
	}
}

func (a *k8sAPI) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	return a.client.Do(req)
}

// list replaces st's objects of that kind, and returns the version to watch
// from.
func (a *k8sAPI) list(ctx context.Context, kind string, st *k8sState) (string, error) {
	lctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	res, err := a.get(lctx, "/"+kind)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("listing %s answered %s", kind, res.Status)
	}
	var body struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	switch kind {
	case "services":
		var items []k8sService
		if err := json.Unmarshal(body.Items, &items); err != nil {
			return "", err
		}
		st.services = make(map[string]k8sService, len(items))
		for _, s := range items {
			st.services[s.Metadata.Name] = s
		}
	case "pods":
		var items []k8sPod
		if err := json.Unmarshal(body.Items, &items); err != nil {
			return "", err
		}
		st.pods = make(map[string]k8sPod, len(items))
		for _, p := range items {
			st.pods[p.Metadata.Name] = p
		}
	}
	return body.Metadata.ResourceVersion, nil
}

// watch applies events until the server ends the stream, returning the last
// version seen. gone says the version was too old to resume from.
func (a *k8sAPI) watch(ctx context.Context, kind, rv string, st *k8sState, poke func()) (string, bool, error) {
	res, err := a.get(ctx, "/"+kind+"?watch=1&allowWatchBookmarks=true&resourceVersion="+url.QueryEscape(rv))
	if err != nil {
		return rv, false, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusGone {
		return rv, true, nil
	}
	if res.StatusCode != http.StatusOK {
		return rv, false, fmt.Errorf("watching %s answered %s", kind, res.Status)
	}
	dec := json.NewDecoder(res.Body)
	for {
		var ev struct {
			Type   string          `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		if err := dec.Decode(&ev); err != nil {
			if ctx.Err() != nil {
				return rv, false, ctx.Err()
			}
			return rv, false, nil // the server closed it: resume from rv
		}
		var meta struct {
			Metadata k8sMeta `json:"metadata"`
			Code     int     `json:"code"`
		}
		_ = json.Unmarshal(ev.Object, &meta)
		switch ev.Type {
		case "ERROR":
			if meta.Code == http.StatusGone {
				return rv, true, nil
			}
			return rv, false, fmt.Errorf("watching %s: error %d", kind, meta.Code)
		case "BOOKMARK":
			rv = meta.Metadata.ResourceVersion
			continue
		}
		if meta.Metadata.ResourceVersion != "" {
			rv = meta.Metadata.ResourceVersion
		}
		st.apply(kind, ev.Type, ev.Object)
		poke()
	}
}

func (st *k8sState) apply(kind, typ string, raw json.RawMessage) {
	st.mu.Lock()
	defer st.mu.Unlock()
	switch kind {
	case "services":
		var s k8sService
		if json.Unmarshal(raw, &s) != nil {
			return
		}
		if typ == "DELETED" {
			delete(st.services, s.Metadata.Name)
		} else {
			st.services[s.Metadata.Name] = s
		}
	case "pods":
		var p k8sPod
		if json.Unmarshal(raw, &p) != nil {
			return
		}
		if typ == "DELETED" {
			delete(st.pods, p.Metadata.Name)
		} else {
			st.pods[p.Metadata.Name] = p
		}
	}
}

// result turns the namespace into services, each with the pods its selector
// picks: how many are meant to run (every pod not finished or being deleted)
// and how many are ready, and which image each runs. A service without a
// selector - its endpoints written by hand, or an ExternalName - has no pods
// to count, and is dialled like any host. Neither is anything when the pods
// could not be read (counted false).
func (st *k8sState) result(ns string, counted, own bool) Result {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]Service, 0, len(st.services))
	for _, s := range st.services {
		// A service of the gateway's own namespace resolves under its bare
		// name too; one of another namespace only under its qualified name,
		// which is also its identity, so two namespaces' "api" stay apart.
		svc := Service{
			Name:      s.Metadata.Name,
			Names:     []string{s.Metadata.Name, s.Metadata.Name + "." + ns + ".svc"},
			Networks:  []string{ns},
			Reachable: true,
		}
		if !own {
			svc.Name = s.Metadata.Name + "." + ns + ".svc"
			svc.Names = []string{svc.Name}
		}
		for _, p := range s.Spec.Ports {
			svc.Ports = append(svc.Ports, Port{Target: p.Port, Published: p.NodePort})
		}
		if counted && len(s.Spec.Selector) > 0 {
			svc.Counted = true
			tally := imageTally{}
			for _, p := range st.pods {
				if !selects(s.Spec.Selector, p.Metadata.Labels) || p.Metadata.DeletionTimestamp != nil ||
					p.Status.Phase == "Succeeded" || p.Status.Phase == "Failed" {
					continue
				}
				ready := podReady(p)
				svc.Wanted++
				if ready {
					svc.Ready++
				}
				tally.add(podImage(p), ready)
			}
			svc.Images = tally.list()
		}
		svc.Suggested = suggest(svc)
		out = append(out, svc)
	}
	sortByName(out)
	return Result{Source: "kubernetes", Services: out, Reach: []string{ns}}
}

func podReady(p k8sPod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// podImage is what the pod's FIRST container runs - the application, by the
// convention every chart follows; sidecars come after it. The status says
// what was actually pulled, digest included; the spec is the fallback while
// the container has not started.
func podImage(p k8sPod) Image {
	if len(p.Spec.Containers) == 0 {
		return Image{}
	}
	main := p.Spec.Containers[0]
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Name == main.Name {
			return parseImage(main.Image, cs.ImageID)
		}
	}
	return parseImage(main.Image, "")
}
