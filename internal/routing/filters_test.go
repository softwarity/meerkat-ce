package routing

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"
)

// compile helpers: a filter is only ever reached through the catalogue, so the
// tests go through it too - a filter that compiles but is not registered would
// pass a direct test and be invisible in the console.
func requestFilter(t *testing.T, spec Spec) RequestFilter {
	t.Helper()
	out, err := CompileFilters([]Spec{spec})
	if err != nil {
		t.Fatalf("compile %s: %v", spec.Type, err)
	}
	if len(out.Request) != 1 {
		t.Fatalf("%s did not compile to a request filter", spec.Type)
	}
	return out.Request[0]
}

func responseFilter(t *testing.T, spec Spec) ResponseFilter {
	t.Helper()
	out, err := CompileFilters([]Spec{spec})
	if err != nil {
		t.Fatalf("compile %s: %v", spec.Type, err)
	}
	if len(out.Response) != 1 {
		t.Fatalf("%s did not compile to a response filter", spec.Type)
	}
	return out.Response[0]
}

func proxied(t *testing.T, req *http.Request) *httputil.ProxyRequest {
	t.Helper()
	out := req.Clone(req.Context())
	return &httputil.ProxyRequest{In: req, Out: out}
}

// set-host has to write BOTH the field and the header. Go puts Request.Host on
// the wire and ignores the header; everything reading the request afterwards
// reads the header. One without the other is a gateway that disagrees with
// itself, and the symptom is a virtual host answering the wrong site.
func TestSetHostWritesBoth(t *testing.T) {
	f := requestFilter(t, Spec{Type: "set-host", Args: map[string]any{"host": "app.internal"}})
	pr := proxied(t, httptest.NewRequest("GET", "http://gateway.example/x", nil))
	f(pr)
	if pr.Out.Host != "app.internal" || pr.Out.Header.Get("Host") != "app.internal" {
		t.Fatalf("host=%q header=%q", pr.Out.Host, pr.Out.Header.Get("Host"))
	}
}

func TestRenameRequestHeaderCarriesEveryValue(t *testing.T) {
	f := requestFilter(t, Spec{Type: "rename-request-header",
		Args: map[string]any{"from": "X-Old", "to": "X-New"}})
	req := httptest.NewRequest("GET", "http://x/", nil)
	req.Header.Add("X-Old", "one")
	req.Header.Add("X-Old", "two")
	req.Header.Set("X-New", "stale") // whatever was there is replaced, not joined
	pr := proxied(t, req)
	f(pr)
	if got := pr.Out.Header.Values("X-New"); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("X-New = %v", got)
	}
	if pr.Out.Header.Get("X-Old") != "" {
		t.Fatal("the old name survived the rename")
	}
}

// The Cookie header is ONE string carrying every cookie: removing one means
// writing the header again, and deleting it outright would take the session
// with it.
func TestRemoveRequestCookieKeepsTheOthers(t *testing.T) {
	f := requestFilter(t, Spec{Type: "remove-request-cookie", Args: map[string]any{"name": "tracking"}})
	req := httptest.NewRequest("GET", "http://x/", nil)
	req.Header.Set("Cookie", "session=abc; tracking=zzz; theme=dark")
	pr := proxied(t, req)
	f(pr)
	got := pr.Out.Header.Get("Cookie")
	if strings.Contains(got, "tracking") {
		t.Fatalf("the cookie is still there: %q", got)
	}
	if !strings.Contains(got, "session=abc") || !strings.Contains(got, "theme=dark") {
		t.Fatalf("the other cookies were lost: %q", got)
	}
}

func TestAddQueryParamKeepsWhatTheClientSent(t *testing.T) {
	f := requestFilter(t, Spec{Type: "add-query-param", Args: map[string]any{"name": "tag", "value": "b"}})
	pr := proxied(t, httptest.NewRequest("GET", "http://x/?tag=a", nil))
	f(pr)
	if got := pr.Out.URL.Query()["tag"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("tag = %v", got)
	}
}

// Cache-Control is a decision, and an Expires or Pragma from the upstream is
// the same decision made differently: leaving them is how a page gets cached
// that should not have been.
func TestCacheControlSilencesWhatContradictsIt(t *testing.T) {
	f := responseFilter(t, Spec{Type: "cache-control", Args: map[string]any{"value": "no-store"}})
	res := &http.Response{Header: http.Header{}}
	res.Header.Set("Expires", "Thu, 01 Jan 2099 00:00:00 GMT")
	res.Header.Set("Pragma", "cache")
	if err := f(res); err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("Cache-Control") != "no-store" ||
		res.Header.Get("Expires") != "" || res.Header.Get("Pragma") != "" {
		t.Fatalf("headers = %v", res.Header)
	}
}

// HSTS over TLS only: sent on a plain request it is ignored at best, and at
// worst remembered by a browser that then cannot reach a development gateway
// at all.
func TestSecurityHeadersHoldHSTSBackOnPlainHTTP(t *testing.T) {
	f := responseFilter(t, Spec{Type: "security-headers", Args: map[string]any{"hstsMaxAge": 3600}})
	plain := &http.Response{Header: http.Header{}, Request: httptest.NewRequest("GET", "http://x/", nil)}
	if err := f(plain); err != nil {
		t.Fatal(err)
	}
	if plain.Header.Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS was sent over plain HTTP")
	}
	if plain.Header.Get("X-Content-Type-Options") != "nosniff" ||
		plain.Header.Get("Referrer-Policy") == "" || plain.Header.Get("X-Frame-Options") == "" {
		t.Fatalf("the rest must apply anyway: %v", plain.Header)
	}

	secure := &http.Response{Header: http.Header{}, Request: httptest.NewRequest("GET", "https://x/", nil)}
	secure.Request.TLS = &tls.ConnectionState{}
	if err := f(secure); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secure.Header.Get("Strict-Transport-Security"), "max-age=3600") {
		t.Fatalf("HSTS over TLS: %q", secure.Header.Get("Strict-Transport-Security"))
	}
}

// An application that does not know it sits behind TLS sets cookies without
// Secure. This is the filter that fixes it without touching the application -
// and SameSite=None implies Secure, because a browser drops the pair otherwise.
func TestCookieAttributesRewriteEveryCookie(t *testing.T) {
	f := responseFilter(t, Spec{Type: "cookie-attributes",
		Args: map[string]any{"httpOnly": true, "sameSite": "None"}})
	res := &http.Response{Header: http.Header{}}
	res.Header.Add("Set-Cookie", "a=1; Path=/")
	res.Header.Add("Set-Cookie", "b=2; Path=/app")
	if err := f(res); err != nil {
		t.Fatal(err)
	}
	got := res.Header.Values("Set-Cookie")
	if len(got) != 2 {
		t.Fatalf("cookies lost: %v", got)
	}
	for _, c := range got {
		if !strings.Contains(c, "Secure") || !strings.Contains(c, "HttpOnly") ||
			!strings.Contains(c, "SameSite=None") {
			t.Fatalf("cookie not hardened: %q", c)
		}
	}
	if !strings.Contains(got[1], "Path=/app") {
		t.Fatalf("the cookie's own attributes were lost: %q", got[1])
	}
}

func TestCookieAttributesRefusesAnUnknownSameSite(t *testing.T) {
	_, err := CompileFilters([]Spec{{Type: "cookie-attributes", Args: map[string]any{"sameSite": "maybe"}}})
	if err == nil || !strings.Contains(err.Error(), "Lax") {
		t.Fatalf("err = %v, want a refusal naming what is allowed", err)
	}
}

// Everything registered is in the catalogue with a phase and a doc: the
// console builds its forms from this, so a filter that forgets to describe
// itself is a filter nobody can configure.
func TestEveryFilterDescribesItself(t *testing.T) {
	for _, e := range Catalog() {
		if e.Kind != "filter" {
			continue
		}
		if e.Phase == "" || e.Doc == "" {
			t.Fatalf("%s: phase=%q doc=%q", e.Type, e.Phase, e.Doc)
		}
		for _, p := range e.Params {
			if p.Name == "" || p.Kind == "" {
				t.Fatalf("%s: a parameter without a name or a kind: %+v", e.Type, p)
			}
		}
	}
}
