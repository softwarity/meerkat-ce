package admin

import (
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
)

// Endpoint audit is Enterprise (AUD-04): its events leave through the
// collector export. The community image refuses to set rules, by the screen
// or with the route, and still lets a configuration drop the ones it carries.
func TestEndpointAuditIsRefusedOnTheCommunityImage(t *testing.T) {
	if edition.Enterprise {
		t.Skip("the refusal is the community image's")
	}
	f := setupBare(t)
	const route = `"name":"r","upstream":"http://example.test","predicates":[{"type":"path","args":{"patterns":["/r/**"]}}]`
	if code, body := f.call(t, "PUT", "/api/routes/r1", `{`+route+`}`, f.rootC); code != http.StatusOK {
		t.Fatalf("a plain route: %d %s", code, body)
	}
	code, body := f.call(t, "PUT", "/api/routes/r1/audit", `{"endpoints":[{"method":"GET","path":"/r/x"}]}`, f.rootC)
	if code != http.StatusForbidden || !strings.Contains(body, "Enterprise") {
		t.Fatalf("audit rules on the community image: want 403 naming the edition, got %d %s", code, body)
	}
	if code, body = f.call(t, "PUT", "/api/routes/r1/audit", `{"endpoints":[]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("dropping the rules must stay possible: %d %s", code, body)
	}
	code, body = f.call(t, "PUT", "/api/routes/r1", `{`+route+`,"api":{"audit":[{"method":"GET","path":"/r/x"}]}}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "Enterprise") {
		t.Fatalf("audit rules carried by a route save: want 422 naming the edition, got %d %s", code, body)
	}
}
