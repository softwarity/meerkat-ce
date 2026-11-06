package discovery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeSecrets(t *testing.T) *k8sAPI {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" || r.Method != http.MethodGet {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/namespaces/apps/secrets/app-oidc":
			// client-secret holds "s3cr3t-of-sixteen!"
			_, _ = io.WriteString(w, `{"data":{"client-secret":"czNjcjN0LW9mLXNpeHRlZW4h","other":"eA=="}}`)
		case "/api/v1/namespaces/apps/secrets/hidden":
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &k8sAPI{client: srv.Client(), base: srv.URL + "/api/v1/namespaces/shop", ns: "shop", token: "t"}
}

// TestASecretKeyIsReadAndEachRefusalSaysWhy: the value of the key, decoded;
// a missing key names the ones there are; a missing Secret, a refused read
// and a wrong shape each say what to change.
func TestASecretKeyIsReadAndEachRefusalSaysWhy(t *testing.T) {
	api := fakeSecrets(t)
	ctx := context.Background()
	v, err := readSecretKey(ctx, api, SecretKey{"apps", "app-oidc", "client-secret"})
	if err != nil || v != "s3cr3t-of-sixteen!" {
		t.Fatalf("%q %v", v, err)
	}
	if _, err := readSecretKey(ctx, api, SecretKey{"apps", "app-oidc", "nope"}); err == nil || !strings.Contains(err.Error(), "client-secret, other") {
		t.Fatalf("missing key: %v", err)
	}
	if _, err := readSecretKey(ctx, api, SecretKey{"apps", "gone", "k"}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing Secret: %v", err)
	}
	if _, err := readSecretKey(ctx, api, SecretKey{"apps", "hidden", "k"}); err == nil || !strings.Contains(err.Error(), "rbac.readSecrets") {
		t.Fatalf("refused: %v", err)
	}
	if _, err := ParseSecretRef("apps/app-oidc"); err == nil || !strings.Contains(err.Error(), "namespace/name/key") {
		t.Fatalf("shape: %v", err)
	}
}

// TestTheSecretsTheRoutesApplicationsReadAreOffered: a route's host is its
// Service, the Service's selector finds the pods, and the pods name their
// Secrets - a key by secretKeyRef, a whole Secret by envFrom (its keys read);
// a route that is not a Service offers nothing; a namespace whose pods cannot
// be read is a note naming the chart value.
func TestTheSecretsTheRoutesApplicationsReadAreOffered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/namespaces/shop/services/wiki":
			_, _ = io.WriteString(w, `{"spec":{"selector":{"app":"wiki"}}}`)
		case "/api/v1/namespaces/data/services/db":
			_, _ = io.WriteString(w, `{"spec":{"selector":{"app":"db"}}}`)
		case "/api/v1/namespaces/shop/pods":
			if r.URL.Query().Get("labelSelector") != "app=wiki" {
				http.Error(w, "selector", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"items":[{"spec":{"containers":[{"env":[
				{"name":"A","valueFrom":{"secretKeyRef":{"name":"wiki-oidc","key":"client-secret"}}},
				{"name":"B","value":"plain"}],
				"envFrom":[{"secretRef":{"name":"wiki-env"}}]}]}}]}`)
		case "/api/v1/namespaces/shop/secrets/wiki-env":
			_, _ = io.WriteString(w, `{"data":{"TOKEN":"eA==","DB_PASSWORD":"eA=="}}`)
		case "/api/v1/namespaces/data/pods":
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	api := &k8sAPI{client: srv.Client(), base: srv.URL + "/api/v1/namespaces/shop", ns: "shop", token: "t"}
	got, notes := offerSecrets(context.Background(), api, []RouteTarget{
		{Route: "Wiki", Host: "wiki"}, {Route: "Wiki admin", Host: "wiki.shop.svc"},
		{Route: "Site", Host: "www.example.com"}, {Route: "DB", Host: "db.data"},
	})
	if len(got) != 2 || got[0].Name != "wiki-env" || strings.Join(got[0].Keys, ",") != "DB_PASSWORD,TOKEN" ||
		got[1].Name != "wiki-oidc" || strings.Join(got[1].Keys, ",") != "client-secret" ||
		strings.Join(got[1].Routes, ",") != "Wiki,Wiki admin" {
		t.Fatalf("%+v", got)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "rbac.watchNamespaces") {
		t.Fatalf("notes: %v", notes)
	}
}
