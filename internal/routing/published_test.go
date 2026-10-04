package routing

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

// strip runs the strip-prefix filter the way the proxy does and returns the
// request that would go upstream.
func strip(t *testing.T, path string, announce bool) *http.Request {
	t.Helper()
	def := filterRegistry["strip-prefix"]
	args, err := decodeArgs("strip-prefix", def.Params, map[string]any{"parts": 1, "announcePrefix": announce})
	if err != nil {
		t.Fatal(err)
	}
	f, err := def.compileRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	in := httptest.NewRequest("GET", path, nil)
	pr := &httputil.ProxyRequest{In: in, Out: in.Clone(in.Context())}
	f(pr)
	return pr.Out
}

// A static host answers "/fr" with a redirect to "/fr/": the slash the caller
// typed has to reach it, or every page of the site becomes that redirect.
func TestStripPrefixKeepsTheTrailingSlash(t *testing.T) {
	for in, want := range map[string]string{
		"/site/fr/": "/fr/", "/site/fr": "/fr", "/site/": "/", "/site": "/", "/site/a/b/": "/a/b/",
	} {
		out := strip(t, in, true)
		if out.URL.Path != want {
			t.Errorf("%s goes upstream as %q, want %q", in, out.URL.Path, want)
		}
		if got := StrippedPrefix(out); got != "/site" {
			t.Errorf("%s: stripped prefix %q, want /site", in, got)
		}
		if got := out.Header.Get(ForwardedPrefixHeader); got != "/site" {
			t.Errorf("%s: announced prefix %q, want /site", in, got)
		}
	}
}

func TestBringLocationHome(t *testing.T) {
	for _, c := range []struct{ name, prefix, loc, want string }{
		{"the upstream's own address", "/site", "https://www.example.com/fr/", "/site/fr/"},
		{"the same, over the other scheme", "/site", "http://www.example.com/fr/?a=1", "/site/fr/?a=1"},
		{"the upstream's bare origin", "/site", "https://www.example.com", "/site/"},
		{"a path from the service's root", "/site", "/login", "/site/login"},
		{"a service that knows its prefix", "/site", "/site/login", "/site/login"},
		{"its prefix alone", "/site", "/site", "/site"},
		{"another site", "/site", "https://accounts.example.org/login", "https://accounts.example.org/login"},
		{"a look-alike host", "/site", "https://www.example.com.evil.test/", "https://www.example.com.evil.test/"},
		{"a relative path", "/site", "next", "next"},
		{"a protocol-relative address", "/site", "//cdn.example.org/x", "//cdn.example.org/x"},
		{"no prefix: the origin still comes home", "", "https://www.example.com/fr/", "/fr/"},
		{"no prefix: a path is left alone", "", "/login", "/login"},
	} {
		req := httptest.NewRequest("GET", "http://gateway.test/x", nil)
		req.URL = &url.URL{Scheme: "https", Host: "www.example.com", Path: "/x"}
		if c.prefix != "" {
			req = withStripped(req, c.prefix)
		}
		res := &http.Response{Header: http.Header{"Location": {c.loc}}, Request: req}
		BringLocationHome(res)
		if got := res.Header.Get("Location"); got != c.want {
			t.Errorf("%s: %q became %q, want %q", c.name, c.loc, got, c.want)
		}
	}
}

func TestPublishBase(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"the root", `<head><base href="/"><title>x</title>`, `<head><base href="/site/"><title>x</title>`},
		{"single quotes, other attributes", `<BASE target="_top" href='/app/'>`, `<BASE target="_top" href='/site/app/'>`},
		{"already published", `<base href="/site/">`, ""},
		{"relative", `<base href="./">`, ""},
		{"another host", `<base href="//cdn.example.org/">`, ""},
		{"no base at all", `<head><title>x</title></head>`, ""},
	} {
		got := PublishBase([]byte(c.in), "/site")
		if c.want == "" {
			if got != nil {
				t.Errorf("%s: %q must be left alone, got %q", c.name, c.in, got)
			}
			continue
		}
		if string(got) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if PublishBase([]byte(`<base href="/">`), "") != nil {
		t.Error("no prefix, nothing to publish")
	}
}
