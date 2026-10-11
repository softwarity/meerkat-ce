package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
)

// TestACertificateMountedInTheContainerIsTakenByItsPaths: the mount table is
// read, what a mount holds is offered without its content, a pair the table
// offers is taken into the pool and placed on the internal plane, and a path
// the table does not offer is refused - the endpoint opens no file because
// somebody typed its name.
func TestACertificateMountedInTheContainerIsTakenByItsPaths(t *testing.T) {
	f := setupBare(t)
	mount := t.TempDir()
	certPEM, keyPEM := pemPair(t, "meerkat-internal.shop.svc.cluster.local")
	if err := os.WriteFile(filepath.Join(mount, "tls.crt"), []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mount, "tls.key"), []byte(keyPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(table, []byte("1 0 0:1 / / rw - overlay overlay rw\n2 1 0:2 / "+mount+" ro - tmpfs tmpfs rw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	was := certs.MountInfoPath
	certs.MountInfoPath = table
	t.Cleanup(func() { certs.MountInfoPath = was })

	code, out := f.call(t, "GET", "/api/certificates/mounted", "", f.rootC)
	if code != http.StatusOK || strings.Contains(out, "PRIVATE KEY") || !strings.Contains(out, "tls.key") {
		t.Fatalf("mounted: %d %s", code, out)
	}
	pick := func(certPath, keyPath string) (int, string) {
		b, _ := json.Marshal(map[string]any{"certPath": certPath, "keyPath": keyPath, "internal": true})
		return f.call(t, "POST", "/api/certificates/file", string(b), f.rootC)
	}
	if code, out := pick("/etc/passwd", filepath.Join(mount, "tls.key")); code != http.StatusUnprocessableEntity {
		t.Fatalf("a path the table does not offer: %d %s", code, out)
	}
	code, out = pick(filepath.Join(mount, "tls.crt"), filepath.Join(mount, "tls.key"))
	if code != http.StatusCreated || !strings.Contains(out, `"source":"file"`) || !strings.Contains(out, `"internal":true`) {
		t.Fatalf("taken: %d %s", code, out)
	}
	if code, out := f.call(t, "GET", "/api/certificates", "", f.rootC); code != http.StatusOK || !strings.Contains(out, `"certPath"`) {
		t.Fatalf("the pool: %d %s", code, out)
	}
}

// TestTheInternalPlaneRefusesACertificateForAnotherName: it knows the name the
// services call it by, and a certificate that does not carry it is refused
// there - and still placed anywhere else.
func TestTheInternalPlaneRefusesACertificateForAnotherName(t *testing.T) {
	f := setupBare(t)
	f.api.InternalAddr = ":7070"
	f.api.LookupInternal = func(context.Context, int) discovery.Internal {
		return discovery.Internal{Origin: "http://meerkat-internal.shop.svc.cluster.local:7070", Source: "kubernetes",
			Names: []string{"meerkat-internal.shop.svc.cluster.local", "meerkat-internal.shop.svc", "meerkat-internal.shop", "meerkat-internal"}}
	}
	importOne := func(host string) string {
		certPEM, keyPEM := pemPair(t, host)
		code, out := f.call(t, "POST", "/api/certificates/import", `{"certPem":`+jsonString(certPEM)+`,"keyPem":`+jsonString(keyPEM)+`}`, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("import %s: %d %s", host, code, out)
		}
		var c struct{ ID string }
		_ = json.Unmarshal([]byte(out), &c)
		return c.ID
	}
	other := importOne("apps.acme.example")
	code, out := f.call(t, "PUT", "/api/certificates/"+other+"/placement", `{"internal":true}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "meerkat-internal.shop.svc.cluster.local") {
		t.Fatalf("another name on the internal plane: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/certificates/"+other+"/placement", `{"app":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("the same one on the application: %d %s", code, out)
	}
	wild := importOne("*.shop.svc.cluster.local")
	if code, out := f.call(t, "PUT", "/api/certificates/"+wild+"/placement", `{"internal":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("a wildcard that covers it: %d %s", code, out)
	}
	// Any of the names the runtime resolves to it will do: a certificate
	// written for the Service's short form is one its neighbours accept.
	short := importOne("meerkat-internal.shop.svc")
	if code, out := f.call(t, "PUT", "/api/certificates/"+short+"/placement", `{"internal":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("the short form of the Service: %d %s", code, out)
	}
	// The mirror: only the internal names, nobody reaches the console or the
	// application by them.
	if code, out := f.call(t, "PUT", "/api/certificates/"+short+"/placement", `{"internal":true,"app":true}`, f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(out, "only the internal plane's names") {
		t.Fatalf("internal names on the application: %d %s", code, out)
	}
	if code, body := f.call(t, "GET", "/api/edition", "", f.rootC); code != http.StatusOK ||
		!strings.Contains(body, `"internalNames":["meerkat-internal.shop.svc.cluster.local","meerkat-internal.shop.svc"`) {
		t.Fatalf("the names the console checks against: %d %s", code, body)
	}
}
