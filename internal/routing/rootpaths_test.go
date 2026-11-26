package routing

import (
	"strings"
	"testing"
)

// The addresses a page names from the root get the prefix; everything else -
// relative links, other sites, the gateway's own, a script's text, an
// attribute that only looks like one - leaves as it came.
func TestARootAppGetsItsPrefixBack(t *testing.T) {
	in := `<!DOCTYPE html>
<html><head>
<link rel="stylesheet" href="/flasgger_static/swagger-ui.css">
<link rel=icon href=/static/favicon.ico>
<script src="/flasgger_static/swagger-ui-bundle.js"></script>
<script>var cfg = { url: "/spec.json", src: '/x.js' };</script>
<style>body { background: url("/img/bg.png") } @import '/theme.css';</style>
</head><body>
<a href="/forms/post">form</a> <a href="relative">rel</a> <a href="//cdn.example.com/x">cdn</a>
<a href="https://example.com/y">abs</a> <a href="/httpbin/already">done</a>
<a href="/meerkat/user-button.js">own</a> <a href="/logout">out</a>
<img srcset="/a.png 1x, b.png 2x, /c.png 3x" title='src="/not-an-attribute"'>
<div style="background-image: url(&quot;/bg.svg&quot;)"></div>
<form action="/post?x=1&amp;y=2"></form>
<!-- <a href="/in-a-comment"> -->
</body></html>`
	want := strings.NewReplacer(
		`"/flasgger_static/swagger-ui.css"`, `"/httpbin/flasgger_static/swagger-ui.css"`,
		`href=/static/favicon.ico`, `href=/httpbin/static/favicon.ico`,
		`"/flasgger_static/swagger-ui-bundle.js"`, `"/httpbin/flasgger_static/swagger-ui-bundle.js"`,
		`url("/img/bg.png")`, `url("/httpbin/img/bg.png")`,
		`@import '/theme.css'`, `@import '/httpbin/theme.css'`,
		`href="/forms/post"`, `href="/httpbin/forms/post"`,
		`srcset="/a.png 1x, b.png 2x, /c.png 3x"`, `srcset="/httpbin/a.png 1x, b.png 2x, /httpbin/c.png 3x"`,
		`url(&quot;/bg.svg&quot;)`, `url(&quot;/httpbin/bg.svg&quot;)`,
		`action="/post?x=1&amp;y=2"`, `action="/httpbin/post?x=1&amp;y=2"`,
	).Replace(in)
	got := string(PublishRootPaths([]byte(in), "/httpbin"))
	if got != want {
		t.Fatalf("rewritten page:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestAPageWithNothingFromTheRootIsLeftAlone(t *testing.T) {
	for _, page := range []string{
		`<html><head><script src="js/main.js"></script></head><body><a href="api/x">x</a></body></html>`,
		`<html><body><a href="/httpbin/x">x</a></body></html>`,
		``,
	} {
		if out := PublishRootPaths([]byte(page), "/httpbin"); out != nil {
			t.Errorf("%q rewritten to %q", page, out)
		}
	}
	if out := PublishRootPaths([]byte(`<a href="/x">`), ""); out != nil {
		t.Errorf("no prefix, yet rewritten: %q", out)
	}
}

func TestAStylesheetGetsItsPrefixBack(t *testing.T) {
	in := `@import url(/base.css); .a { background: url('/a.png') } .b { background: url(b.png) } .c { src: url(//cdn/x.woff) }`
	want := `@import url(/app/base.css); .a { background: url('/app/a.png') } .b { background: url(b.png) } .c { src: url(//cdn/x.woff) }`
	if got := string(PublishCSSPaths([]byte(in), "/app")); got != want {
		t.Fatalf("stylesheet:\n%s\nwant:\n%s", got, want)
	}
}

// The script carries the prefix and the gateway's own paths as JSON: a prefix
// cannot close the script it is written in.
func TestTheRootPathsScriptCarriesThePrefix(t *testing.T) {
	s := RootPathsScript("/app")
	if !strings.Contains(s, `var P = "/app", OWN = ["/meerkat","/logout","/select-tenant","/select-group"];`) {
		t.Fatalf("script head:\n%s", s[:300])
	}
	if strings.Contains(RootPathsScript("/a</script>"), "a</script>") {
		t.Fatal("a prefix closed the script")
	}
}
