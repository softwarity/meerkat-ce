package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/store"
)

// fakeCertManager stands for cert-manager: what it was asked, and what it
// answers.
type fakeCertManager struct {
	issuers []discovery.Issuer
	asked   []string
	deleted int
	// secret is its Certificate's name, the internal plane's when empty.
	secret string
}

func (f *fakeCertManager) SecretName() string {
	if f.secret != "" {
		return f.secret
	}
	return "meerkat-internal-issued"
}
func (f *fakeCertManager) Issuers(context.Context) ([]discovery.Issuer, error) {
	return f.issuers, nil
}
func (f *fakeCertManager) Ensure(_ context.Context, kind, name string, names []string) error {
	f.asked = append(f.asked, kind+"/"+name+" "+strings.Join(names, ","))
	return nil
}
func (f *fakeCertManager) Status(context.Context) (discovery.IssuedStatus, error) {
	return discovery.IssuedStatus{Exists: true, IssuerName: "cluster-ca"}, nil
}
func (f *fakeCertManager) Delete(context.Context) error { f.deleted++; return nil }

// TestTheInternalCertificateIsAskedOfCertManager: the screen says which issuer,
// the gateway decides the rest - the names the runtime resolves to the
// internal plane, its own Certificate - and the row waits on the internal
// door for the Secret. An issuer it did not offer is refused; asking again
// re-aims the same row; removing it removes what cert-manager holds.
func TestTheInternalCertificateIsAskedOfCertManager(t *testing.T) {
	f := setupBare(t)
	f.api.InternalAddr = ":7070"
	f.api.LookupInternal = func(context.Context, int) discovery.Internal {
		return discovery.Internal{Origin: "http://meerkat-internal.shop.svc.cluster.local:7070", Source: "kubernetes",
			Names: []string{"meerkat-internal.shop.svc.cluster.local", "meerkat-internal"}}
	}

	// Not on Kubernetes: shown, and why not.
	f.api.CertManagerWhy = "this gateway does not run on Kubernetes"
	if code, out := f.call(t, "GET", "/api/certificates/cert-manager", "", f.rootC); code != http.StatusOK ||
		!strings.Contains(out, `"available":false`) || !strings.Contains(out, "does not run on Kubernetes") {
		t.Fatalf("no cert-manager: %d %s", code, out)
	}

	cm := &fakeCertManager{issuers: []discovery.Issuer{
		{Kind: "ClusterIssuer", Name: "cluster-ca", Type: "ca", Ready: true},
		{Kind: "ClusterIssuer", Name: "broken", Ready: false, Why: "secret not found"},
	}}
	f.api.CertManager = cm
	if code, out := f.call(t, "GET", "/api/certificates/cert-manager", "", f.rootC); code != http.StatusOK ||
		!strings.Contains(out, `"available":true`) || !strings.Contains(out, `"names":["meerkat-internal.shop.svc.cluster.local","meerkat-internal"]`) {
		t.Fatalf("the view: %d %s", code, out)
	}
	for _, body := range []string{`{"kind":"ClusterIssuer","name":"broken"}`, `{"kind":"ClusterIssuer","name":"made-up"}`} {
		if code, out := f.call(t, "POST", "/api/certificates/cert-manager", body, f.rootC); code != http.StatusUnprocessableEntity ||
			!strings.Contains(out, "ClusterIssuer/cluster-ca") {
			t.Fatalf("%s: %d %s", body, code, out)
		}
	}
	code, out := f.call(t, "POST", "/api/certificates/cert-manager", `{"kind":"ClusterIssuer","name":"cluster-ca"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("asked: %d %s", code, out)
	}
	var row struct {
		ID, Source, SecretName string
		Internal, Waiting      bool
	}
	_ = json.Unmarshal([]byte(out), &row)
	if row.Source != "cert-manager" || !row.Internal || !row.Waiting || row.SecretName != "meerkat-internal-issued" {
		t.Fatalf("the row: %s", out)
	}
	if len(cm.asked) != 1 || cm.asked[0] != "ClusterIssuer/cluster-ca meerkat-internal.shop.svc.cluster.local,meerkat-internal" {
		t.Fatalf("what cert-manager was asked: %v", cm.asked)
	}
	if code, out := f.call(t, "POST", "/api/certificates/cert-manager", `{"kind":"ClusterIssuer","name":"cluster-ca"}`, f.rootC); code != http.StatusOK ||
		!strings.Contains(out, row.ID) {
		t.Fatalf("asked again: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/certificates/"+row.ID, "", f.rootC); code != http.StatusNoContent || cm.deleted != 1 {
		t.Fatalf("removed: %d %s, cluster deletes %d", code, out, cm.deleted)
	}
}

// Behind a proxy in front, the console or the application asks cert-manager
// for the hop it is re-encrypted on (SSL-14): the names are that plane's in
// the runtime, the Certificate that plane's, and the row lands on that plane.
// Served by the gateway itself, the plane's public names are an authority's
// business, and the screen says so instead of offering it.
func TestAProxiedPlaneAsksCertManagerForItsHop(t *testing.T) {
	f := setupBare(t)
	tlsSupervisor(t, f)
	f.api.TLS.Ports("127.0.0.1:18080", "127.0.0.1:19090", "127.0.0.1:17070")
	f.api.LookupInternal = func(_ context.Context, port int) discovery.Internal {
		if port == 18080 {
			return discovery.Internal{Source: "kubernetes", Names: []string{"meerkat.shop.svc.cluster.local", "meerkat.shop.svc", "meerkat"}}
		}
		return discovery.Internal{}
	}
	issuers := []discovery.Issuer{{Kind: "ClusterIssuer", Name: "cluster-ca", Type: "ca", Ready: true}}
	internal, app := &fakeCertManager{issuers: issuers}, &fakeCertManager{issuers: issuers, secret: "meerkat-app-issued"}
	f.api.CertManager = internal
	f.api.CertManagers = map[string]CertManager{store.PlaneApp: app}

	_, out := f.call(t, "GET", "/api/certificates/cert-manager?plane=app", "", f.rootC)
	if !strings.Contains(out, "serves its public names itself") {
		t.Fatalf("the application serves its own names: %s", out)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["shop.example.org"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("proxied: %d %s", code, out)
	}
	_, out = f.call(t, "GET", "/api/certificates/cert-manager?plane=app", "", f.rootC)
	var v certManagerView
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Available || v.Secret != "meerkat-app-issued" || v.Names[0] != "meerkat.shop.svc.cluster.local" {
		t.Fatalf("the application's hop: %s", out)
	}
	code, out := f.call(t, "POST", "/api/certificates/cert-manager", `{"kind":"ClusterIssuer","name":"cluster-ca","plane":"app"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("ask: %d %s", code, out)
	}
	var row certView
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		t.Fatal(err)
	}
	if !row.App || row.Internal || row.SecretName != "meerkat-app-issued" {
		t.Fatalf("the row is the application's, following its own Secret: %s", out)
	}
	if len(app.asked) != 1 || !strings.Contains(app.asked[0], "meerkat.shop.svc") || len(internal.asked) != 0 {
		t.Fatalf("asked of the application's Certificate only: app %v internal %v", app.asked, internal.asked)
	}
	// Taken off the plane, then asked again: it goes back on, not a second row.
	if code, out := f.call(t, "PUT", "/api/certificates/"+row.ID+"/placement", `{"app":false}`, f.rootC); code != http.StatusOK {
		t.Fatalf("take off: %d %s", code, out)
	}
	code, out = f.call(t, "POST", "/api/certificates/cert-manager", `{"kind":"ClusterIssuer","name":"cluster-ca","plane":"app"}`, f.rootC)
	var again certView
	if err := json.Unmarshal([]byte(out), &again); err != nil || code != http.StatusOK || again.ID != row.ID || !again.App {
		t.Fatalf("asked again after taking it off: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/certificates/"+row.ID, "", f.rootC); code != http.StatusNoContent || app.deleted != 1 || internal.deleted != 0 {
		t.Fatalf("delete removes the application's Certificate: %d %s (app %d, internal %d)", code, out, app.deleted, internal.deleted)
	}
}
