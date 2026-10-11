package admin

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTrustedAuthoritiesAreImportedPEMs: an authority is a named PEM; one that
// holds no certificate, or holds a private key, is refused at save time; the
// list says what each one holds.
func TestTrustedAuthoritiesAreImportedPEMs(t *testing.T) {
	f := setupBare(t)
	upstream := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)
	authority := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))
	body := func(v map[string]string) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "company CA", "pem": authority}), f.rootC)
	if code != http.StatusCreated || !strings.Contains(out, `"fingerprint"`) {
		t.Fatalf("imported: %d %s", code, out)
	}
	key := authority + "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "oops", "pem": key}), f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "certificate pool") {
		t.Fatalf("a private key: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "junk", "pem": "not a pem"}), f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("no certificate: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/trusted-authorities", body(map[string]string{"name": "x", "pem": authority}), f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d %s", code, out)
	}
	code, out = f.call(t, "GET", "/api/trusted-authorities", "", f.rootC)
	var list []trustView
	if err := json.Unmarshal([]byte(out), &list); err != nil || code != http.StatusOK || len(list) != 1 || len(list[0].Certificates) != 1 {
		t.Fatalf("the list: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/trusted-authorities/"+list[0].ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, out)
	}
}
