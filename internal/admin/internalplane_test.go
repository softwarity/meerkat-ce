package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/discovery"
)

// internalPlane serves the API's internal plane, as main mounts it on :7070.
func internalPlane(t *testing.T, f fixture) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	f.api.RegisterInternal(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// on calls srv with a token, a cookie, both or neither.
func on(t *testing.T, srv *httptest.Server, method, path, body, token string, cookie *http.Cookie) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

const stationSchedule = `{"name":"station 42","roles":["station_fetch"],"routeId":"r1","method":"POST",` +
	`"path":"/jobs/poll","every":"PT1H"}`

// TestTheInternalPlaneServesTheJWKSToAnybody: public keys give no power, and
// a service must be able to read them before it holds anything at all.
func TestTheInternalPlaneServesTheJWKSToAnybody(t *testing.T) {
	f := setupBare(t)
	srv := internalPlane(t, f)
	code, body := on(t, srv, "GET", "/.well-known/jwks.json", "", "", nil)
	if code != http.StatusOK || !strings.Contains(body, `"keys"`) {
		t.Fatalf("the JWKS: %d %s", code, body)
	}
}

// TestTheInternalPlaneAnswersTokensOnly: in token mode, a place is not a
// credential. No token is a 401, and so is the root's console session - a
// cookie has no business on this port.
func TestTheInternalPlaneAnswersTokensOnly(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	tokenMode(t, f)
	srv := internalPlane(t, f)
	if code, body := on(t, srv, "POST", "/api/schedules", stationSchedule, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d %s", code, body)
	}
	if code, body := on(t, srv, "GET", "/api/schedules", "", "", f.rootC); code != http.StatusUnauthorized {
		t.Fatalf("root's console session: %d %s", code, body)
	}

	token := serviceToken(t, f, "stations")
	code, body := on(t, srv, "POST", "/api/schedules", stationSchedule, token, nil)
	if code != http.StatusCreated {
		t.Fatalf("a service creates its schedule here: %d %s", code, body)
	}
	if code, body := on(t, srv, "GET", "/api/schedules", "", token, nil); code != http.StatusOK || !strings.Contains(body, "station 42") {
		t.Fatalf("and finds it again: %d %s", code, body)
	}
	// The token's perimeter still holds: it is the schedules, here as there.
	if code, _ := on(t, srv, "GET", "/api/me", "", token, nil); code != http.StatusNotFound {
		t.Fatalf("the internal plane serves nothing else: %d", code)
	}
}

// TestTheConsoleReadsAndActsButWritesNoSchedule: on the control plane the
// console keeps what it does - read, pause, run now, delete - and creating or
// changing a schedule is refused with the address where it is done.
func TestTheConsoleReadsAndActsButWritesNoSchedule(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	code, body := f.call(t, "POST", "/api/schedules", stationSchedule, f.rootC)
	if code != http.StatusForbidden || !strings.Contains(body, "internal plane") {
		t.Fatalf("root creating from the console: %d %s", code, body)
	}

	token := serviceToken(t, f, "stations")
	srv := internalPlane(t, f)
	code, body = on(t, srv, "POST", "/api/schedules", stationSchedule, token, nil)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	id := between(body, `"id":"`, `"`)
	if code, body := f.call(t, "PUT", "/api/schedules/"+id, stationSchedule, f.rootC); code != http.StatusForbidden {
		t.Fatalf("root changing it from the console: %d %s", code, body)
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/schedules"}, {"GET", "/api/schedules/" + id}, {"GET", "/api/schedules/" + id + "/runs"},
		{"POST", "/api/schedules/" + id + "/pause"}, {"POST", "/api/schedules/" + id + "/resume"},
		{"DELETE", "/api/schedules/" + id},
	} {
		code, body := f.call(t, c.method, c.path, "", f.rootC)
		if code >= 300 {
			t.Fatalf("the console's %s %s: %d %s", c.method, c.path, code, body)
		}
	}
}

// TestAServiceThatHasNotMovedYetStillWrites: a backend configured before the
// internal plane writes on the control plane, with its service token, until
// it is moved - told once in the journal where its door is now.
func TestAServiceThatHasNotMovedYetStillWrites(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")
	if code, body := f.bearerJSON(t, "POST", "/api/schedules", stationSchedule, token); code != http.StatusCreated {
		t.Fatalf("a service token on the old address: %d %s", code, body)
	}
}

func between(s, from, to string) string {
	i := strings.Index(s, from)
	if i < 0 {
		return ""
	}
	s = s[i+len(from):]
	if j := strings.Index(s, to); j >= 0 {
		return s[:j]
	}
	return s
}

// TestTheDiscoveryDocumentNamesTheIssuerAndItsKeys: the issuer the gateway
// signs with, and the JWKS at the address the caller used. The issuer is one
// of the application plane's public addresses: declaring a public name changes
// both the document and the tokens, and choosing among several too.
func TestTheDiscoveryDocumentNamesTheIssuerAndItsKeys(t *testing.T) {
	f := setupBare(t)
	srv := internalPlane(t, f)
	read := func() discoveryDocument {
		t.Helper()
		code, body := on(t, srv, "GET", DiscoveryPath, "", "", nil)
		if code != http.StatusOK {
			t.Fatalf("discovery: %d %s", code, body)
		}
		var d discoveryDocument
		if err := json.Unmarshal([]byte(body), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	d := read()
	if d.Issuer != "meerkat" || d.JWKSURI != srv.URL+"/.well-known/jwks.json" || len(d.Algorithms) != 3 {
		t.Fatalf("by default: %+v", d)
	}
	// Nothing to choose from yet: refused, saying where to declare a name.
	if code, body := f.call(t, "PUT", "/api/identity/issuer", `{"issuer":"https://gw.example.com"}`, f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(body, "Application tab") {
		t.Fatalf("no public name: %d %s", code, body)
	}

	// A public name declared: the issuer follows at once.
	declareAppNames(t, f, "gw.example.com", "id.example.com")
	if d := read(); d.Issuer != "https://gw.example.com" {
		t.Fatalf("after declaring the names: %+v", d)
	}

	if code, body := f.call(t, "PUT", "/api/identity/issuer", `{"issuer":"http://meerkat:7070"}`, f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(body, "https://gw.example.com, https://id.example.com") {
		t.Fatalf("an issuer outside the public addresses: %d %s", code, body)
	}
	if code, body := f.call(t, "PUT", "/api/identity/issuer", `{"issuer":"https://id.example.com"}`, f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account setting it: %d %s", code, body)
	}
	code, body := f.call(t, "PUT", "/api/identity/issuer", `{"issuer":"https://id.example.com"}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"issuer":"https://id.example.com"`) ||
		!strings.Contains(body, `"issuers":["https://gw.example.com","https://id.example.com"]`) {
		t.Fatalf("setting it: %d %s", code, body)
	}
	if d := read(); d.Issuer != "https://id.example.com" {
		t.Fatalf("after the change: %+v", d)
	}
	code, body = f.call(t, "POST", "/api/identity/preview",
		`{"routeName":"orders","identity":{"mechanism":"signed-jwt","algorithm":"ES256"}}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"iss":"https://id.example.com"`) {
		t.Fatalf("the token a service would receive: %d %s", code, body)
	}

	// The chosen name removed: the issuer follows to the one left.
	declareAppNames(t, f, "gw.example.com")
	if d := read(); d.Issuer != "https://gw.example.com" {
		t.Fatalf("after removing the chosen name: %+v", d)
	}
}

// declareAppNames declares the application plane's public names, served by a
// proxy in front so no certificate is needed, as the Domains & TLS screen does.
func declareAppNames(t *testing.T, f fixture, names ...string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"proxied": true, "names": names, "proxyAddresses": []string{}})
	if code, body := f.call(t, "PUT", "/api/settings/tls/doors/app", string(raw), f.rootC); code != http.StatusOK {
		t.Fatalf("declaring %v: %d %s", names, code, body)
	}
}

// TestTheConsoleIsGivenTheRuntimesNameForTheInternalPlane: the address beside
// the JWKS and the scheduler's API is the one the runtime gives - a Service's
// FQDN - and none when it cannot tell, for the console to fall back on.
func TestTheConsoleIsGivenTheRuntimesNameForTheInternalPlane(t *testing.T) {
	f := setupBare(t)
	f.api.InternalAddr = ":7070"
	asked := 0
	f.api.LookupInternal = func(_ context.Context, port int) discovery.Internal {
		asked++
		if port != 7070 {
			t.Errorf("asked for port %d", port)
		}
		return discovery.Internal{Origin: "http://meerkat-internal.shop.svc.cluster.local:7070", Source: "kubernetes"}
	}
	for _, path := range []string{"/api/identity/signing-keys", "/api/edition"} {
		code, body := f.call(t, "GET", path, "", f.rootC)
		if code != http.StatusOK || !strings.Contains(body, `"internalOrigin":"http://meerkat-internal.shop.svc.cluster.local:7070"`) {
			t.Fatalf("%s: %d %s", path, code, body)
		}
	}
	if asked != 1 {
		t.Fatalf("the runtime was asked %d times for two screens within a minute", asked)
	}
}
