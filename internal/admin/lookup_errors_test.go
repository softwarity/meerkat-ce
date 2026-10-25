package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A lookup that FAILS is not a lookup that found nothing. Answering 404 to a
// database that does not answer tells the operator the id is wrong, and they
// go looking for a typo while the store is down: only "not found" is a 404,
// anything else is the server's own failure.
//
// The handlers are called directly, with the store closed under them: the
// guard in front reads the store too, and would answer before the handler
// could get it wrong.
func TestALookupThatFailsIsNotANotFound(t *testing.T) {
	f := setupBare(t)
	root := rootUser(t, f)
	type call struct {
		name    string
		handler userHandler
		method  string
		values  map[string]string
		body    string
	}
	calls := []call{
		{"withdraw a notification", f.api.withdrawNotification, "DELETE", map[string]string{"id": "n1"}, ""},
		{"compare configurations", f.api.compareConfigurations, "GET", map[string]string{"id": "c1", "other": "c2"}, ""},
		{"read an issue", f.api.getIssue, "GET", map[string]string{"id": "i1"}, ""},
		{"move a secret in", f.api.stashSecret, "POST", nil,
			`{"holder":"authprovider","id":"p1","field":"clientSecret","name":"idp-secret"}`},
	}
	run := func(c call) (int, string) {
		req := httptest.NewRequest(c.method, "/", strings.NewReader(c.body))
		for k, v := range c.values {
			req.SetPathValue(k, v)
		}
		rec := httptest.NewRecorder()
		c.handler(rec, req, root)
		return rec.Code, rec.Body.String()
	}
	// The store answers: an unknown id is a 404.
	for _, c := range calls {
		if code, body := run(c); code != http.StatusNotFound {
			t.Fatalf("%s, unknown id: %d %s, want 404", c.name, code, body)
		}
	}
	// The store fails: the same calls are the server's failure.
	if err := f.api.st.Close(); err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		if code, body := run(c); code != http.StatusInternalServerError {
			t.Fatalf("%s, store down: %d %s, want 500", c.name, code, body)
		}
	}
}
