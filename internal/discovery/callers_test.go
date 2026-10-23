package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// clusterCallers is a Callers talking to a fake API server, the namespace
// "shop" its own.
func clusterCallers(t *testing.T, handle http.HandlerFunc) *Callers {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	c := newCallers()
	c.api = &k8sAPI{client: srv.Client(), base: srv.URL + "/api/v1/namespaces/shop", ns: "shop", token: "t"}
	c.root = srv.URL
	return c
}

// TestATokenIsReviewedOnceAMinute: the API server says whose token it is, a
// ServiceAccount is named namespace/name, anything else is nobody - and a
// busy service costs one review a minute, not one per request.
func TestATokenIsReviewedOnceAMinute(t *testing.T) {
	var asked atomic.Int32
	c := clusterCallers(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apis/authentication.k8s.io/v1/tokenreviews" {
			http.NotFound(w, r)
			return
		}
		asked.Add(1)
		var review struct {
			Spec struct {
				Token string `json:"token"`
			} `json:"spec"`
		}
		_ = json.NewDecoder(r.Body).Decode(&review)
		switch review.Spec.Token {
		case "sa":
			_, _ = io.WriteString(w, `{"status":{"authenticated":true,"user":{"username":"system:serviceaccount:canopy:reference-data"}}}`)
		case "person":
			_, _ = io.WriteString(w, `{"status":{"authenticated":true,"user":{"username":"alice@example.com"}}}`)
		default:
			_, _ = io.WriteString(w, `{"status":{"authenticated":false}}`)
		}
	})
	now := time.Unix(1_800_000_000, 0)
	c.now = func() time.Time { return now }
	ctx := context.Background()
	for _, want := range []struct{ token, id string }{{"sa", "canopy/reference-data"}, {"person", ""}, {"forged", ""}} {
		id, err := c.Review(ctx, want.token)
		if err != nil || id != want.id {
			t.Fatalf("%s: %q %v, want %q", want.token, id, err, want.id)
		}
	}
	if id, _ := c.Review(ctx, "sa"); id != "canopy/reference-data" || asked.Load() != 3 {
		t.Fatalf("asked %d times for three tokens and a repeat", asked.Load())
	}
	now = now.Add(callerTTL)
	if _, _ = c.Review(ctx, "sa"); asked.Load() != 4 {
		t.Fatalf("a minute later the answer was not asked again: %d", asked.Load())
	}
	if _, err := newCallers().Review(ctx, "sa"); err == nil || !strings.Contains(err.Error(), "Kubernetes") {
		t.Fatalf("off Kubernetes: %v", err)
	}
}

// callerPodJSON is one pod as the API server lists it.
func callerPodJSON(ns, name, ip, phase string, labels map[string]string, ownerKind, ownerName string) map[string]any {
	meta := map[string]any{"name": name, "namespace": ns, "labels": labels}
	if ownerKind != "" {
		meta["ownerReferences"] = []map[string]any{{"kind": ownerKind, "name": ownerName, "controller": true}}
	}
	return map[string]any{"metadata": meta, "status": map[string]any{"phase": phase, "podIP": ip}}
}

// TestAnAddressIsFollowedToItsWorkload: the pod at the address, then what
// owns it - a Deployment through its ReplicaSet (by the template hash, or by
// asking for the ReplicaSet), a StatefulSet as itself, a bare pod as a pod;
// nothing at an address is nobody. Kept a minute.
func TestAnAddressIsFollowedToItsWorkload(t *testing.T) {
	pods := map[string]map[string]any{
		"10.42.0.15": callerPodJSON("canopy", "reference-data-7d9f8b6c5-x2x9q", "10.42.0.15", "Running",
			map[string]string{"pod-template-hash": "7d9f8b6c5"}, "ReplicaSet", "reference-data-7d9f8b6c5"),
		"10.42.0.16": callerPodJSON("canopy", "legacy-abc12", "10.42.0.16", "Running", nil, "ReplicaSet", "legacy-rs"),
		"10.42.0.17": callerPodJSON("data", "postgres-0", "10.42.0.17", "Running", nil, "StatefulSet", "postgres"),
		"10.42.0.18": callerPodJSON("canopy", "debug", "10.42.0.18", "Running", nil, "", ""),
		"10.42.0.19": callerPodJSON("canopy", "done", "10.42.0.19", "Succeeded", nil, "Job", "migrate"),
	}
	var listed atomic.Int32
	c := clusterCallers(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/pods":
			listed.Add(1)
			ip := strings.TrimPrefix(r.URL.Query().Get("fieldSelector"), "status.podIP=")
			items := []map[string]any{}
			if p, ok := pods[ip]; ok {
				items = append(items, p)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case "/apis/apps/v1/namespaces/canopy/replicasets/legacy-rs":
			_, _ = io.WriteString(w, `{"metadata":{"name":"legacy-rs","ownerReferences":[{"kind":"Deployment","name":"legacy","controller":true}]}}`)
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	for ip, want := range map[string]string{
		"10.42.0.15": "canopy/Deployment/reference-data",
		"10.42.0.16": "canopy/Deployment/legacy",
		"10.42.0.17": "data/StatefulSet/postgres",
		"10.42.0.18": "canopy/Pod/debug",
		"10.42.0.19": "",
		"10.42.0.99": "",
	} {
		id, via := c.ByAddress(ctx, ip)
		if id != want {
			t.Errorf("%s: %q, want %q", ip, id, want)
		}
		if want != "" && via != viaWorkload {
			t.Errorf("%s: read as %q", ip, via)
		}
	}
	before := listed.Load()
	c.ByAddress(ctx, "10.42.0.15")
	if listed.Load() != before {
		t.Fatal("the same address was asked again within the minute")
	}
}

// TestAGatewayKeptToItsNamespacesStillAnswers: a cluster-wide list refused,
// the namespaces the chart grants are read one by one - the ServiceAccounts
// and the workloads the drawer offers, as the caller would be matched.
func TestAGatewayKeptToItsNamespacesStillAnswers(t *testing.T) {
	t.Setenv("MEERKAT_WATCH_NAMESPACES", "data")
	c := clusterCallers(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/pods", "/api/v1/serviceaccounts":
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
		case "/api/v1/namespaces/shop/serviceaccounts":
			_, _ = io.WriteString(w, `{"items":[{"metadata":{"name":"default","namespace":"shop"}},{"metadata":{"name":"orders","namespace":"shop"}}]}`)
		case "/api/v1/namespaces/data/serviceaccounts":
			_, _ = io.WriteString(w, `{"items":[{"metadata":{"name":"default","namespace":"data"}}]}`)
		case "/api/v1/namespaces/shop/pods":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
				callerPodJSON("shop", "orders-5f6d-aaaaa", "10.42.1.2", "Running", map[string]string{"pod-template-hash": "5f6d"}, "ReplicaSet", "orders-5f6d"),
				callerPodJSON("shop", "orders-5f6d-bbbbb", "10.42.1.3", "Running", map[string]string{"pod-template-hash": "5f6d"}, "ReplicaSet", "orders-5f6d"),
				callerPodJSON("shop", "old", "10.42.1.4", "Failed", nil, "", ""),
			}})
		case "/api/v1/namespaces/data/pods":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
				callerPodJSON("data", "postgres-0", "10.42.1.5", "Running", nil, "StatefulSet", "postgres"),
			}})
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	sas, err := c.ServiceAccounts(ctx)
	if err != nil || strings.Join(sas, ",") != "data/default,shop/default,shop/orders" {
		t.Fatalf("ServiceAccounts: %v %v", sas, err)
	}
	workloads, err := c.Workloads(ctx)
	if err != nil || strings.Join(workloads, ",") != "data/StatefulSet/postgres,shop/Deployment/orders" {
		t.Fatalf("workloads: %v %v", workloads, err)
	}
}

// TestADockerAddressIsItsServiceOrItsContainer: on Docker the daemon's own
// containers first - a Swarm task by its service, a compose one by its
// service, a plain one by its name - then a Swarm task on another node.
func TestADockerAddressIsItsServiceOrItsContainer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/containers/json":
			_, _ = io.WriteString(w, `[
				{"Names":["/shop_orders.1.abc"],"Labels":{"com.docker.swarm.service.name":"shop_orders"},
				 "NetworkSettings":{"Networks":{"shop_net":{"IPAddress":"10.0.1.7"}}}},
				{"Names":["/gw-billing-1"],"Labels":{"com.docker.compose.service":"billing"},
				 "NetworkSettings":{"Networks":{"gw_default":{"IPAddress":"172.18.0.4"}}}},
				{"Names":["/scratch"],"Labels":{},
				 "NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.9"}}}}]`)
		case "/tasks":
			_, _ = io.WriteString(w, `[{"ServiceID":"svc1","NetworksAttachments":[{"Addresses":["10.0.1.12/24"]}]}]`)
		case "/services/svc1":
			_, _ = io.WriteString(w, `{"Spec":{"Name":"shop_stock"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newCallers()
	c.docker = func() (*http.Client, string, error) { return srv.Client(), srv.URL, nil }
	ctx := context.Background()
	for ip, want := range map[string]string{
		"10.0.1.7":   "shop_orders",
		"172.18.0.4": "billing",
		"172.17.0.9": "scratch",
		"10.0.1.12":  "shop_stock",
		"10.0.1.99":  "",
	} {
		id, via := c.ByAddress(ctx, ip)
		if id != want || (want != "" && via != viaContainer) {
			t.Errorf("%s: %q by %q, want %q", ip, id, via, want)
		}
	}
}
