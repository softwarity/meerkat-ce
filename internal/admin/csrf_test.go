package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// callH is call with extra headers, so a test can play the browser signals a
// real request carries and a curl script does not.
func (f fixture) callH(t *testing.T, method, path, body string, cookie *http.Cookie, headers map[string]string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.adminSrv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

// wrote reports a PUT that went through: created (201) or updated (200).
func wrote(code int) bool { return code == http.StatusOK || code == http.StatusCreated }

// A cookie write from another site is refused (SEC-01): the cookie rides every
// request to this origin, so the console has to prove the request is its own.
// The two signals a page cannot forge are checked, and a non-browser client
// that sends neither is left alone.
func TestCrossSiteWriteIsRefused(t *testing.T) {
	f := setup(t)

	route := fmt.Sprintf(validRoute, "http://up.example")
	// The forged cases: a browser tells on itself, and the write dies before
	// the route is ever touched.
	for _, h := range []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "https://evil.example"},
	} {
		code, out := f.callH(t, "PUT", "/api/routes/r1", route, f.rootC, h)
		if code != http.StatusForbidden {
			t.Fatalf("headers %v: got %d %s, want 403", h, code, out)
		}
	}

	// The console's own request: same-origin by Fetch-Metadata, or an Origin
	// that matches the plane it reached. Both go through (201, the route is
	// created).
	if code, out := f.callH(t, "PUT", "/api/routes/r1", route, f.rootC,
		map[string]string{"Sec-Fetch-Site": "same-origin"}); !wrote(code) {
		t.Fatalf("same-origin write refused: %d %s", code, out)
	}
	if code, out := f.callH(t, "PUT", "/api/routes/r2", strings.Replace(route, `"api"`, `"api2"`, 1), f.rootC,
		map[string]string{"Origin": f.adminSrv.URL}); !wrote(code) {
		t.Fatalf("matching-origin write refused: %d %s", code, out)
	}

	// A bookmark or a typed URL is "none", never a forgery.
	if code, _ := f.callH(t, "DELETE", "/api/routes/r2", "", f.rootC,
		map[string]string{"Sec-Fetch-Site": "none"}); code != http.StatusNoContent {
		t.Fatalf("a same-origin delete was refused")
	}

	// A read is never a write: GET is out of scope whatever the site says.
	if code, _ := f.callH(t, "GET", "/api/routes", "", f.rootC,
		map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusOK {
		t.Fatalf("a cross-site READ must still be answered: %d", code)
	}

	// No headers at all: a script with a cookie, not a browser, and not a CSRF
	// vector - it keeps working.
	if code, out := f.callH(t, "DELETE", "/api/routes/r1", "", f.rootC, nil); code != http.StatusNoContent {
		t.Fatalf("a headerless cookie write was refused: %d %s", code, out)
	}
}

// A token is not a CSRF vector - a browser does not attach a Bearer header to a
// cross-site request - so the guard leaves token calls alone, cross-site header
// and all.
func TestATokenWriteSkipsTheCrossSiteGuard(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "POST", "/api/admin-tokens", `{"name":"cli","scope":"full"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("mint: %d %s", code, body)
	}
	var created struct{ Token string }
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("PUT", f.adminSrv.URL+"/api/routes/r1", strings.NewReader(fmt.Sprintf(validRoute, "http://up.example")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+created.Token)
	req.Header.Set("Sec-Fetch-Site", "cross-site") // a browser would never send this on a Bearer call
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	// 200 or 201: the write went through. What matters is that it was NOT the
	// cross-site refusal - a token is not a browser.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		t.Fatalf("a token write was refused as if it were a cookie: %d %s", res.StatusCode, out)
	}
}
