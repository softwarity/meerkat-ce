package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestImagesAreReadFromTheirReference(t *testing.T) {
	cases := []struct{ ref, id, wantRef, tag, digest string }{
		{"registry.example.com/team/orders:1.4.0", "", "registry.example.com/team/orders:1.4.0", "1.4.0", ""},
		{"nginx:1.27@sha256:0123456789abcdef0123", "", "nginx:1.27", "1.27", "0123456789ab"},
		{"localhost:5000/app", "", "localhost:5000/app", "", ""},
		{"localhost:5000/app:2", "docker.io/library/app@sha256:fedcba9876543210", "localhost:5000/app:2", "2", "fedcba987654"},
		{"busybox", "sha256:aaaabbbbccccdddd", "busybox", "", "aaaabbbbcccc"},
	}
	for _, c := range cases {
		got := parseImage(c.ref, c.id)
		if got.Ref != c.wantRef || got.Tag != c.tag || got.Digest != c.digest {
			t.Errorf("parseImage(%q, %q) = %+v", c.ref, c.id, got)
		}
	}
}

// Only what can change a route's answer wakes anyone.
func TestOnlyEventsThatMatterAreFollowed(t *testing.T) {
	for _, e := range []dockerEvent{{"container", "start"}, {"container", "die"}, {"container", "health_status: unhealthy"},
		{"service", "update"}, {"node", "update"}} {
		if !e.matters() {
			t.Errorf("%+v ignored", e)
		}
	}
	for _, e := range []dockerEvent{{"container", "exec_start: sh"}, {"container", "attach"}, {"image", "pull"}} {
		if e.matters() {
			t.Errorf("%+v followed", e)
		}
	}
}

func pod(name, app, image string, ready bool) k8sPod {
	var p k8sPod
	p.Metadata.Name = name
	p.Metadata.Labels = map[string]string{"app": app}
	p.Spec.Containers = append(p.Spec.Containers, struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	}{"main", image})
	p.Status.Phase = "Running"
	st := "False"
	if ready {
		st = "True"
	}
	p.Status.Conditions = append(p.Status.Conditions, struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}{"Ready", st})
	return p
}

func service(name string, selector map[string]string) k8sService {
	var s k8sService
	s.Metadata.Name = name
	s.Spec.Selector = selector
	return s
}

// A Service counts the pods its selector picks: ready ones, the ones meant to
// run, and what they run. One without a selector is not counted at all.
func TestAServiceCountsThePodsItSelects(t *testing.T) {
	gone := "2026-10-08T10:00:00Z"
	leaving := pod("orders-old", "orders", "orders:1.3.9", true)
	leaving.Metadata.DeletionTimestamp = &gone
	done := pod("migrate", "orders", "orders:1.4.0", false)
	done.Status.Phase = "Succeeded"
	st := &k8sState{
		services: map[string]k8sService{
			"orders":   service("orders", map[string]string{"app": "orders"}),
			"external": service("external", nil),
		},
		pods: map[string]k8sPod{
			"a": pod("a", "orders", "orders:1.4.0", true),
			"b": pod("b", "orders", "orders:1.4.0", true),
			"c": pod("c", "orders", "orders:1.3.9", false),
			"x": pod("x", "billing", "billing:2", true),
			"o": leaving, "m": done,
		},
	}
	r := st.result("shop", true, true)
	by := map[string]Service{}
	for _, s := range r.Services {
		by[s.Name] = s
	}
	o := by["orders"]
	if !o.Counted || o.Ready != 2 || o.Wanted != 3 {
		t.Fatalf("orders: %+v", o)
	}
	if len(o.Images) != 2 || o.Images[0].Tag != "1.4.0" || o.Images[0].Count != 2 || o.Images[1].Tag != "1.3.9" || o.Images[1].Ready != 0 {
		t.Errorf("orders images: %+v", o.Images)
	}
	// A replica that has not pulled yet runs the same reference: not a rollout.
	starting := pod("d", "orders", "orders:1.4.0", false)
	pulled := pod("e", "orders", "orders:1.4.0", true)
	pulled.Status.ContainerStatuses = append(pulled.Status.ContainerStatuses, struct {
		Name    string `json:"name"`
		Image   string `json:"image"`
		ImageID string `json:"imageID"`
	}{"main", "orders:1.4.0", "orders@sha256:aaaabbbbccccdddd"})
	st2 := &k8sState{services: map[string]k8sService{"orders": service("orders", map[string]string{"app": "orders"})},
		pods: map[string]k8sPod{"d": starting, "e": pulled}}
	if imgs := st2.result("shop", true, true).Services[0].Images; len(imgs) != 1 || imgs[0].Count != 2 || imgs[0].Ready != 1 || imgs[0].Digest != "aaaabbbbcccc" {
		t.Errorf("a starting replica read as another image: %+v", imgs)
	}
	if by["external"].Counted || len(by["external"].Images) != 0 {
		t.Errorf("a service without a selector was counted: %+v", by["external"])
	}
	if r := st.result("shop", false, true); r.Services[1].Counted {
		t.Error("pods that could not be read were counted")
	}
}

// fakeK8s serves a LIST and then a WATCH whose events the test pushes.
type fakeK8s struct {
	mu     sync.Mutex
	pods   []k8sPod
	events chan string
}

func (f *fakeK8s) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if r.URL.Query().Get("watch") == "" {
		f.mu.Lock()
		defer f.mu.Unlock()
		var items any = []k8sService{service("orders", map[string]string{"app": "orders"})}
		if kind == "pods" {
			items = f.pods
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"metadata": map[string]string{"resourceVersion": "10"}, "items": items})
		return
	}
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	if kind != "pods" {
		<-r.Context().Done()
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-f.events:
			_, _ = fmt.Fprintln(w, e)
			w.(http.Flusher).Flush()
		}
	}
}

// The state follows the watch: a pod becoming ready is an event, and the
// answer changes without anybody asking again.
func TestTheNamespaceIsFollowedByItsEvents(t *testing.T) {
	f := &fakeK8s{pods: []k8sPod{pod("a", "orders", "orders:1.4.0", true), pod("b", "orders", "orders:1.4.0", false)},
		events: make(chan string, 4)}
	srv := httptest.NewServer(f)
	defer srv.Close()
	api := &k8sAPI{client: srv.Client(), base: srv.URL + "/api/v1/namespaces/shop", ns: "shop"}
	st := &k8sState{services: map[string]k8sService{}, pods: map[string]k8sPod{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poked := make(chan struct{}, 16)
	poke := func() { poked <- struct{}{} }
	go watchK8s(ctx, api, "services", st, poke, nil)
	go watchK8s(ctx, api, "pods", st, poke, nil)

	waitFor := func(what string, ok func(Service) bool) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			if s := st.result("shop", true, true).Services; len(s) == 1 && ok(s[0]) {
				return
			}
			select {
			case <-poked:
			case <-deadline:
				t.Fatalf("%s never came: %+v", what, st.result("shop", true, true).Services)
			}
		}
	}
	waitFor("the listed state", func(s Service) bool { return s.Ready == 1 && s.Wanted == 2 })

	b := pod("b", "orders", "orders:1.4.0", true)
	b.Metadata.ResourceVersion = "11"
	raw, _ := json.Marshal(map[string]any{"type": "MODIFIED", "object": b})
	f.events <- string(raw)
	waitFor("b ready", func(s Service) bool { return s.Ready == 2 && s.Wanted == 2 })

	raw, _ = json.Marshal(map[string]any{"type": "DELETED", "object": b})
	f.events <- string(raw)
	waitFor("b gone", func(s Service) bool { return s.Ready == 1 && s.Wanted == 1 })
}

// A Swarm daemon whose answers the test changes, with an /events stream the
// test writes to.
type swarmDaemon struct {
	mu      sync.Mutex
	running int
	image   string
	events  chan string
	asked   int
}

func (d *swarmDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	running, image := d.running, d.image
	d.mu.Unlock()
	switch {
	case strings.HasPrefix(r.URL.Path, "/services"):
		d.mu.Lock()
		d.asked++
		d.mu.Unlock()
		_, _ = fmt.Fprintf(w, `[{"ID":"s1","Spec":{"Name":"shop_orders","TaskTemplate":{"ContainerSpec":{"Image":%q}},
			"EndpointSpec":{"Ports":[{"TargetPort":8080}]}},"ServiceStatus":{"RunningTasks":%d,"DesiredTasks":3}}]`, image, running)
	case strings.HasPrefix(r.URL.Path, "/tasks"):
		var tasks []string
		for i := 0; i < 3; i++ {
			state := "running"
			if i >= running {
				state = "starting"
			}
			tasks = append(tasks, fmt.Sprintf(`{"ServiceID":"s1","Spec":{"ContainerSpec":{"Image":%q}},"Status":{"State":%q}}`, image, state))
		}
		_, _ = fmt.Fprint(w, "["+strings.Join(tasks, ",")+"]")
	case strings.HasPrefix(r.URL.Path, "/networks"):
		_, _ = fmt.Fprint(w, "[]")
	case strings.HasPrefix(r.URL.Path, "/events"):
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case e := <-d.events:
				_, _ = fmt.Fprintln(w, e)
				w.(http.Flusher).Flush()
			}
		}
	default:
		http.NotFound(w, r)
	}
}

// An event is answered by one fresh inventory: the replica that came back
// and the image it runs are seen without a timer.
func TestASwarmIsFollowedByItsEvents(t *testing.T) {
	d := &swarmDaemon{running: 2, image: "orders:1.4.0@sha256:0123456789abcdef", events: make(chan string, 4)}
	srv := httptest.NewServer(d)
	defer srv.Close()
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	w := NewWatcher()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	read := func(what string, ok func(Service) bool) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			if r, known := w.Result(); known && len(r.Services) == 1 && ok(r.Services[0]) {
				return
			}
			select {
			case <-w.Changed():
			case <-deadline:
				r, _ := w.Result()
				t.Fatalf("%s never came: %+v", what, r)
			}
		}
	}
	read("the inventory", func(s Service) bool {
		return s.Counted && s.Ready == 2 && s.Wanted == 3 && len(s.Images) == 1 && s.Images[0].Ready == 2 && s.Images[0].Digest == "0123456789ab"
	})
	if r := Discover(ctx); len(r.Services) != 1 || r.Services[0].Ready != 2 {
		t.Errorf("Discover did not answer from the watcher: %+v", r)
	}

	d.mu.Lock()
	d.running = 3
	d.mu.Unlock()
	d.events <- `{"Type":"container","Action":"exec_start: sh"}`
	d.events <- `{"Type":"container","Action":"start"}`
	read("the third replica", func(s Service) bool { return s.Ready == 3 })
}

// Another namespace is read the same way, under its qualified names only:
// the bare name resolves in the gateway's own namespace, not in that one.
func TestAnotherNamespaceIsNamedInFull(t *testing.T) {
	t.Setenv("MEERKAT_WATCH_NAMESPACES", " monitoring, shop ,,monitoring")
	if got := watchedNamespaces("shop"); len(got) != 2 || got[0] != "shop" || got[1] != "monitoring" {
		t.Fatalf("watched namespaces: %v", got)
	}
	api := &k8sAPI{base: "https://10.0.0.1:443/api/v1/namespaces/shop", ns: "shop"}
	if other := api.in("monitoring"); other.base != "https://10.0.0.1:443/api/v1/namespaces/monitoring" || other.ns != "monitoring" {
		t.Errorf("aimed at %s (%s)", other.base, other.ns)
	}
	st := &k8sState{services: map[string]k8sService{"grafana": service("grafana", map[string]string{"app": "grafana"})},
		pods: map[string]k8sPod{"g": pod("g", "grafana", "grafana/grafana:12.1.0", true)}}
	s := st.result("monitoring", true, false).Services[0]
	if s.Name != "grafana.monitoring.svc" || len(s.Names) != 1 || s.Ready != 1 || s.Images[0].Tag != "12.1.0" {
		t.Errorf("a service of another namespace: %+v", s)
	}
}
