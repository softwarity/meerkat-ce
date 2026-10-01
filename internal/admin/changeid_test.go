package admin

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// A write hands back the identifier of the event it produced, and that
// identifier is the one the live channel publishes: it is how the screen that
// saved tells its own write from somebody else's, instead of warning its user
// that "admin changed this somewhere else" about the save they just made.
func TestAWriteNamesItsOwnChange(t *testing.T) {
	f := setup(t)

	do := func(method, path, body string) (*http.Response, string) {
		t.Helper()
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, f.adminSrv.URL+path, rdr)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(f.rootC)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res, string(b)
	}

	res, out := do("PUT", "/api/settings/telemetry", `{"enabled":false,"endpoint":"","sample":0.5,"maxPerSecond":10}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", res.StatusCode, out)
	}
	id := res.Header.Get(changeIDHeader)
	if id == "" {
		t.Fatal("the write did not name the change it made: the screen cannot recognise its own save")
	}
	// It is the audit event's own identifier - the same string the live
	// channel carries as the row's id.
	_, trail := do("GET", "/api/audit", "")
	if !strings.Contains(trail, id) {
		t.Errorf("the identifier %q is not the audit event's: %s", id, trail)
	}

	// A read produces nothing and says nothing.
	res, _ = do("GET", "/api/settings/telemetry", "")
	if got := res.Header.Get(changeIDHeader); got != "" {
		t.Errorf("a read answered with a change identifier: %q", got)
	}
}
