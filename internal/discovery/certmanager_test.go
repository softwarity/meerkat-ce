package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is the slice of an API server cert-manager's client talks to.
func fakeAPI(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *CertManager {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	return newCertManager(&k8sAPI{client: srv.Client(), base: srv.URL + "/api/v1/namespaces/shop", ns: "shop", token: "t"}, "meerkat-internal-issued")
}

// TestTheIssuersAreListedAndAbsentCertManagerIsSaid: the ClusterIssuers and
// the namespace's Issuers that sign with a lasting authority, each with
// whether it is ready - an ACME or selfSigned one is not offered; a cluster
// without cert-manager says so in a sentence.
func TestTheIssuersAreListedAndAbsentCertManagerIsSaid(t *testing.T) {
	m := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/cert-manager.io/v1/clusterissuers":
			_, _ = io.WriteString(w, `{"items":[{"metadata":{"name":"cluster-ca"},"spec":{"ca":{"secretName":"x"}},
				"status":{"conditions":[{"type":"Ready","status":"True"}]}},
				{"metadata":{"name":"vault-pki"},"spec":{"vault":{}},"status":{"conditions":[{"type":"Ready","status":"False","message":"no token"}]}},
				{"metadata":{"name":"le"},"spec":{"acme":{}},"status":{"conditions":[{"type":"Ready","status":"True"}]}},
				{"metadata":{"name":"bootstrap"},"spec":{"selfSigned":{}},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`)
		case "/apis/cert-manager.io/v1/namespaces/shop/issuers":
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	})
	got, err := m.Issuers(context.Background())
	if err != nil || len(got) != 2 || !got[0].Ready || got[0].Type != "ca" || got[1].Ready || got[1].Why != "no token" {
		t.Fatalf("%+v %v", got, err)
	}
	none := fakeAPI(t, http.NotFound)
	if _, err := none.Issuers(context.Background()); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("no cert-manager: %v", err)
	}
}

// TestTheCertificateIsCreatedThenReaimed: a POST, and on a conflict the same
// spec as a merge patch - the names and the issuer the gateway decided.
func TestTheCertificateIsCreatedThenReaimed(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	exists := false
	m := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Content-Type"))
		mu.Unlock()
		if r.Method == http.MethodPost {
			if exists {
				w.WriteHeader(http.StatusConflict)
				return
			}
			var c struct {
				Spec struct {
					SecretName string   `json:"secretName"`
					DNSNames   []string `json:"dnsNames"`
					IssuerRef  struct{ Name, Kind string }
				} `json:"spec"`
			}
			_ = json.Unmarshal(b, &c)
			if c.Spec.SecretName != "meerkat-internal-issued" || len(c.Spec.DNSNames) != 2 || c.Spec.IssuerRef.Name != "cluster-ca" {
				t.Errorf("created: %s", b)
			}
			exists = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	names := []string{"meerkat-internal.shop.svc.cluster.local", "meerkat-internal"}
	if err := m.Ensure(context.Background(), "ClusterIssuer", "cluster-ca", names); err != nil {
		t.Fatal(err)
	}
	if err := m.Ensure(context.Background(), "ClusterIssuer", "cluster-ca", names); err != nil {
		t.Fatal(err)
	}
	want := "PATCH /apis/cert-manager.io/v1/namespaces/shop/certificates/meerkat-internal-issued application/merge-patch+json"
	if len(seen) != 3 || seen[2] != want {
		t.Fatalf("requests: %v", seen)
	}
}

// TestTheSecretIsWatchedAndARenewalIsHeard: the pair is read from the Secret
// as listed, then from each event - a renewal is a MODIFIED - and issued is
// told each time it changed.
func TestTheSecretIsWatchedAndARenewalIsHeard(t *testing.T) {
	events := make(chan string, 4)
	m := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fieldSelector") != "metadata.name=meerkat-internal-issued" {
			t.Errorf("not narrowed to the one name: %s", r.URL.RawQuery)
		}
		secret := strings.HasSuffix(r.URL.Path, "/secrets")
		if r.URL.Query().Get("watch") == "" {
			if secret {
				_, _ = io.WriteString(w, `{"metadata":{"resourceVersion":"1"},"items":[{"metadata":{"name":"meerkat-internal-issued"},
					"data":{"tls.crt":"Y2VydDE=","tls.key":"a2V5MQ=="}}]}`)
			} else {
				_, _ = io.WriteString(w, `{"metadata":{"resourceVersion":"1"},"items":[]}`)
			}
			return
		}
		if !secret {
			<-r.Context().Done()
			return
		}
		w.(http.Flusher).Flush()
		for ev := range events {
			_, _ = io.WriteString(w, ev+"\n")
			w.(http.Flusher).Flush()
		}
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	issued := make(chan struct{}, 4)
	go m.Watch(ctx, func() { issued <- struct{}{} }, func() {})
	wait := func(what string) {
		select {
		case <-issued:
		case <-time.After(5 * time.Second):
			t.Fatalf("not told: %s", what)
		}
	}
	wait("the first read")
	if c, k, ok := m.Pair(); !ok || c != "cert1" || k != "key1" {
		t.Fatalf("first pair: %q %q", c, k)
	}
	events <- `{"type":"MODIFIED","object":{"metadata":{"name":"meerkat-internal-issued","resourceVersion":"2"},"data":{"tls.crt":"Y2VydDI=","tls.key":"a2V5Mg=="}}}`
	wait("the renewal")
	if c, _, _ := m.Pair(); c != "cert2" {
		t.Fatalf("renewed pair: %q", c)
	}
	close(events)
}
