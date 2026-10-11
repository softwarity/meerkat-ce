package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/discovery"
)

// fakeCertManager stands for cert-manager: what it was asked, and what it
// answers.
type fakeCertManager struct {
	issuers []discovery.Issuer
	asked   []string
	deleted int
}

func (f *fakeCertManager) SecretName() string { return "meerkat-internal-issued" }
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
