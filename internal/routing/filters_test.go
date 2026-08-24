package routing

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strconv"
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

// remove-json-fields is the first filter to touch a BODY, and what matters is
// as much what it refuses as what it removes.
func TestRemoveJSONFields(t *testing.T) {
	jsonResponse := func(body string) *http.Response {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}, "ETag": {`"upstream"`}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}
	read := func(res *http.Response) string {
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}

	f := responseFilter(t, Spec{Type: "remove-json-fields",
		Args: map[string]any{"fields": []string{"password", "user.email"}}})

	// Inside arrays too: a service answering a LIST of users would otherwise
	// keep every password it was asked to drop, which is the case this exists
	// for.
	res := jsonResponse(`[{"name":"a","password":"x","user":{"id":1,"email":"a@b"}},
	                      {"name":"b","password":"y","user":{"id":2,"email":"c@d"}}]`)
	if err := f(res); err != nil {
		t.Fatal(err)
	}
	out := read(res)
	if strings.Contains(out, "password") || strings.Contains(out, "email") {
		t.Fatalf("a field survived: %s", out)
	}
	if !strings.Contains(out, `"name":"a"`) || !strings.Contains(out, `"id":2`) {
		t.Fatalf("the rest of the document was lost: %s", out)
	}
	// A rewritten body is not the upstream's any more: its length is recomputed
	// and its ETag goes, or a cache serves one under the name of the other.
	if res.Header.Get("ETag") != "" {
		t.Fatal("the upstream ETag survived a rewrite")
	}
	if n, _ := strconv.Atoi(res.Header.Get("Content-Length")); n != len(out) {
		t.Fatalf("Content-Length %q does not match %d bytes", res.Header.Get("Content-Length"), len(out))
	}

	// Not JSON: forwarded as it is. A gateway that mangles what it did not
	// understand is worse than one that forwards it.
	html := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"text/html"}},
		Body:       io.NopCloser(strings.NewReader(`<p>password</p>`)),
	}
	if err := f(html); err != nil {
		t.Fatal(err)
	}
	if read(html) != `<p>password</p>` {
		t.Fatal("a non-JSON body was touched")
	}

	// Declared JSON but broken: same answer.
	broken := jsonResponse(`{"password": `)
	if err := f(broken); err != nil {
		t.Fatal(err)
	}
	if read(broken) != `{"password": ` {
		t.Fatal("an unparseable body was touched")
	}

	// A stream is never buffered: holding an event stream until the upstream
	// closes is a page that hangs, not a page that is slow.
	stream := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"password\":\"x\"}\n\n")),
	}
	if err := f(stream); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(stream), "password") {
		t.Fatal("an event stream was rewritten")
	}
}

// set-path replaces the path AND clears RawPath: left behind, the escaped form
// is what the proxy sends, so the upstream would receive the OLD path.
func TestSetPathClearsTheEscapedForm(t *testing.T) {
	f := requestFilter(t, Spec{Type: "set-path", Args: map[string]any{"path": "/health"}})
	pr := proxied(t, httptest.NewRequest("GET", "http://x/deep/path%20here", nil))
	f(pr)
	if pr.Out.URL.Path != "/health" || pr.Out.URL.RawPath != "" {
		t.Fatalf("path=%q rawPath=%q", pr.Out.URL.Path, pr.Out.URL.RawPath)
	}
	if _, err := CompileFilters([]Spec{{Type: "set-path", Args: map[string]any{"path": "health"}}}); err == nil {
		t.Fatal("a path without a leading slash must be refused")
	}
}

// Copy leaves the original where it is - that is the whole difference with
// rename, and the reason both exist: one service reads the new name while
// another still needs the old one.
func TestCopyRequestHeaderLeavesTheOriginal(t *testing.T) {
	f := requestFilter(t, Spec{Type: "copy-request-header",
		Args: map[string]any{"from": "X-Roles", "to": "X-Consumer-Groups"}})
	req := httptest.NewRequest("GET", "http://gateway.example/x", nil)
	req.Header.Add("X-Roles", "admin")
	req.Header.Add("X-Roles", "billing")
	pr := proxied(t, req)
	f(pr)
	if got := pr.Out.Header.Values("X-Roles"); len(got) != 2 {
		t.Fatalf("the original was disturbed: %v", got)
	}
	if got := pr.Out.Header.Values("X-Consumer-Groups"); len(got) != 2 || got[1] != "billing" {
		t.Fatalf("the copy carried %v, want both values", got)
	}
	// Applied twice (a retried request), the target must not pile up.
	f(pr)
	if got := pr.Out.Header.Values("X-Consumer-Groups"); len(got) != 2 {
		t.Fatalf("a second pass piled values up: %v", got)
	}
}

// preserve-host writes the header as well as the field, and that header is
// what the proxy rewrite reads to know a filter asked for a Host at all -
// without it SetURL hands the upstream's name over and the filter does
// nothing (see buildProxy).
func TestPreserveHostSignalsThroughTheHeader(t *testing.T) {
	f := requestFilter(t, Spec{Type: "preserve-host"})
	pr := proxied(t, httptest.NewRequest("GET", "http://shop.example/x", nil))
	f(pr)
	if pr.Out.Host != "shop.example" || pr.Out.Header.Get("Host") != "shop.example" {
		t.Fatalf("host=%q header=%q", pr.Out.Host, pr.Out.Header.Get("Host"))
	}
}

func TestRewriteQueryParamLeavesTheOthersAlone(t *testing.T) {
	f := requestFilter(t, Spec{Type: "rewrite-query-param",
		Args: map[string]any{"name": "redirect", "pattern": "^https?://[^/]+", "replacement": ""}})
	pr := proxied(t, httptest.NewRequest("GET", "http://gw.example/x?redirect=https://evil.test/back&page=2", nil))
	f(pr)
	q := pr.Out.URL.Query()
	if q.Get("redirect") != "/back" {
		t.Fatalf("redirect=%q", q.Get("redirect"))
	}
	if q.Get("page") != "2" {
		t.Fatalf("an untargeted parameter was touched: page=%q", q.Get("page"))
	}
	// A request that does not carry the parameter must come out unchanged -
	// not carrying an empty one that the service will then read as set.
	pr2 := proxied(t, httptest.NewRequest("GET", "http://gw.example/x?page=2", nil))
	f(pr2)
	if _, ok := pr2.Out.URL.Query()["redirect"]; ok {
		t.Fatal("the filter invented the parameter it was asked to rewrite")
	}
}

func TestDedupeResponseHeader(t *testing.T) {
	res := func() *http.Response {
		return &http.Response{Header: http.Header{
			"Access-Control-Allow-Origin": {"https://app.example", "*"},
			"Vary":                        {"Origin", "Accept", "Origin"},
		}}
	}
	first := responseFilter(t, Spec{Type: "dedupe-response-header",
		Args: map[string]any{"names": []string{"Access-Control-Allow-Origin"}}})
	r := res()
	if err := first(r); err != nil {
		t.Fatal(err)
	}
	if got := r.Header.Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != "https://app.example" {
		t.Fatalf("keep=first gave %v", got)
	}

	uniq := responseFilter(t, Spec{Type: "dedupe-response-header",
		Args: map[string]any{"names": []string{"Vary"}, "keep": "unique"}})
	r = res()
	if err := uniq(r); err != nil {
		t.Fatal(err)
	}
	// Order of first appearance, not a set: these lists are read positionally.
	if got := r.Header.Values("Vary"); len(got) != 2 || got[0] != "Origin" || got[1] != "Accept" {
		t.Fatalf("keep=unique gave %v", got)
	}

	if _, err := CompileFilters([]Spec{{Type: "dedupe-response-header",
		Args: map[string]any{"names": []string{"Vary"}, "keep": "middle"}}}); err == nil {
		t.Fatal("an unknown keep was accepted")
	}
}

func TestRewriteResponseHeaderHidesTheInternalHost(t *testing.T) {
	f := responseFilter(t, Spec{Type: "rewrite-response-header",
		Args: map[string]any{"name": "X-Debug-Backend", "pattern": "^[^.]+\\.internal", "replacement": "service"}})
	res := &http.Response{Header: http.Header{"X-Debug-Backend": {"billing-3.internal:8080"}}}
	if err := f(res); err != nil {
		t.Fatal(err)
	}
	if got := res.Header.Get("X-Debug-Backend"); got != "service:8080" {
		t.Fatalf("header=%q", got)
	}
}

// The case this filter exists for: a service behind a stripped prefix answers
// a redirect written in ITS OWN space, and the browser follows it straight out
// of the route.
func TestRewriteLocationBringsTheRedirectBack(t *testing.T) {
	f := responseFilter(t, Spec{Type: "rewrite-location"})
	out := httptest.NewRequest("GET", "http://billing.internal:8080/login", nil)
	out.Header.Set("X-Forwarded-Host", "shop.example")
	out.Header.Set("X-Forwarded-Proto", "https")
	res := &http.Response{
		StatusCode: 302,
		Header:     http.Header{"Location": {"http://billing.internal:8080/login?next=/cart"}},
		Request:    out,
	}
	if err := f(res); err != nil {
		t.Fatal(err)
	}
	if got := res.Header.Get("Location"); got != "https://shop.example/login?next=/cart" {
		t.Fatalf("Location=%q", got)
	}

	// Nothing reliable to point at: leave it alone rather than send the
	// browser to a host nobody asked for.
	bare := httptest.NewRequest("GET", "http://billing.internal:8080/login", nil)
	res2 := &http.Response{Header: http.Header{"Location": {"http://billing.internal:8080/x"}}, Request: bare}
	if err := f(res2); err != nil {
		t.Fatal(err)
	}
	if got := res2.Header.Get("Location"); got != "http://billing.internal:8080/x" {
		t.Fatalf("Location was rewritten without a public origin: %q", got)
	}

	// An explicit pair rewrites a PREFIX, path included - what a stripped
	// prefix needs and what the automatic mode alone cannot know.
	explicit := responseFilter(t, Spec{Type: "rewrite-location",
		Args: map[string]any{"from": "http://billing.internal:8080/api", "to": "/billing"}})
	res3 := &http.Response{
		Header:  http.Header{"Location": {"http://billing.internal:8080/api/invoices/7"}},
		Request: out,
	}
	if err := explicit(res3); err != nil {
		t.Fatal(err)
	}
	if got := res3.Header.Get("Location"); got != "/billing/invoices/7" {
		t.Fatalf("Location=%q", got)
	}
}
