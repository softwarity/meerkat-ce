package admin

import (
	"net/http"
	"strings"
	"testing"
)

// A value the caller got wrong is a 422 that names what is allowed, wherever
// the store refuses it - never "internal error" with the sentence left in the
// log. Three writers that used to meet a store refusal without asking.
func TestAStoreRefusalIsNeverAnInternalError(t *testing.T) {
	f := setupBare(t)
	const base = `"name":"r","upstream":"http://example.test","predicates":[{"type":"path","args":{"patterns":["/r/**"]}}]`

	code, body := f.call(t, "PUT", "/api/routes/r1", `{`+base+`,"access":{"level":"authenticated"}}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "allowed are") {
		t.Fatalf("unknown access level: want 422 naming what is allowed, got %d %s", code, body)
	}

	code, body = f.call(t, "PUT", "/api/routes/r1", `{`+base+`,"limits":[{"per":"moon","requests":10,"window":"1m"}]}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "allowed are") {
		t.Fatalf("unknown rate limit key: want 422 naming what is allowed, got %d %s", code, body)
	}

	if code, body = f.call(t, "PUT", "/api/routes/r1", `{`+base+`}`, f.rootC); code != http.StatusOK {
		t.Fatalf("a valid route: %d %s", code, body)
	}
	code, body = f.call(t, "PUT", "/api/routes/r1/security",
		`{"endpoints":[{"method":"GET","path":"/r/x","level":"nobody"}]}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "allowed are") {
		t.Fatalf("unknown endpoint access level: want 422 naming what is allowed, got %d %s", code, body)
	}
}
