package filters

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var nonceAttr = regexp.MustCompile(`nonce="([^"]+)"`)

const agent = `<script defer src="/meerkat/page.js"></script>`

// The policy an Angular build writes for itself: hashes and 'strict-dynamic',
// in a <meta>. Under it a host source is ignored, so the injected scripts are
// admitted by a nonce the policy is told - and the application's hashes stay.
func TestInjectedScriptsAreAdmittedByAStrictDynamicMetaPolicy(t *testing.T) {
	const policy = `script-src 'strict-dynamic' 'sha256-AAA=' https: 'unsafe-inline';object-src 'none';base-uri 'self';`
	page := `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="` + policy + `"></head><body><app-root></app-root></body></html>`
	res := htmlResponse(page, nil)
	if err := InjectAtBodyStart(agent)(res); err != nil {
		t.Fatal(err)
	}
	if err := InjectAtBodyStart(`<script defer src="/meerkat/portal.js"></script>`)(res); err != nil {
		t.Fatal(err)
	}
	out := readBody(t, res)
	nonces := nonceAttr.FindAllStringSubmatch(out, -1)
	if len(nonces) != 2 || nonces[0][1] != nonces[1][1] {
		t.Fatalf("the two injected scripts must share one nonce, got %v in:\n%s", nonces, out)
	}
	want := `'nonce-` + nonces[0][1] + `'`
	if strings.Count(out, want) != 1 {
		t.Fatalf("the policy must name the nonce once, got:\n%s", out)
	}
	for _, kept := range []string{`'strict-dynamic'`, `'sha256-AAA='`, `object-src 'none'`, `base-uri 'self'`} {
		if !strings.Contains(out, kept) {
			t.Errorf("the application's %s is gone from:\n%s", kept, out)
		}
	}
	if strings.Contains(out, `'self' 'nonce`) || strings.Contains(out, want+` 'self'`) {
		t.Errorf("'self' is ignored under 'strict-dynamic' and has no business being added:\n%s", out)
	}
	Sealed(res.Header)
	if res.Header.Get(nonceHeader) != "" {
		t.Error("the nonce's carrier must not leave with the response")
	}
}

// A policy sent as a header, without 'strict-dynamic': the nonce admits the
// tags, and what they load in turn comes from the page's origin.
func TestInjectedScriptsAreAdmittedByAHeaderPolicy(t *testing.T) {
	res := htmlResponse(`<html><head></head><body></body></html>`,
		http.Header{cspHeader: {"default-src 'none'; img-src https:"}})
	if err := InjectAfterHead(agent)(res); err != nil {
		t.Fatal(err)
	}
	out := readBody(t, res)
	m := nonceAttr.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no nonce on the injected script:\n%s", out)
	}
	got := res.Header.Get(cspHeader)
	// default-src is left alone: a nonce there would reach the styles too.
	if !strings.HasPrefix(got, "default-src 'none'; img-src https:") {
		t.Errorf("default-src must stay as written, got %q", got)
	}
	if !strings.Contains(got, "script-src 'nonce-"+m[1]+"' 'self'") {
		t.Errorf("a script-src with the nonce and 'self' was expected, got %q", got)
	}
	if !strings.Contains(got, "connect-src 'self'") {
		t.Errorf("the gateway's own endpoints must be reachable, got %q", got)
	}
}

// 'unsafe-inline' with no nonce and no hash lets every inline script run. A
// nonce added there would make the browser IGNORE 'unsafe-inline', and the
// application's own inline scripts would stop to admit ours.
func TestANonceIsNotAddedWhereItWouldSwitchOffUnsafeInline(t *testing.T) {
	const policy = "script-src 'self' 'unsafe-inline'"
	res := htmlResponse(`<html><head></head><body></body></html>`, http.Header{cspHeader: {policy}})
	if err := InjectAfterHead(agent)(res); err != nil {
		t.Fatal(err)
	}
	if got := res.Header.Get(cspHeader); got != policy {
		t.Errorf("the policy already admits them and must stay as written, got %q", got)
	}
}

// No policy, no change: the fragment goes in byte for byte.
func TestAPageWithoutAPolicyIsInjectedAsBefore(t *testing.T) {
	res := htmlResponse(`<html><head></head><body></body></html>`, nil)
	if err := InjectAfterHead(agent)(res); err != nil {
		t.Fatal(err)
	}
	if out := readBody(t, res); out != `<html><head>`+agent+`</head><body></body></html>` {
		t.Errorf("got %s", out)
	}
	if res.Header.Get(nonceHeader) != "" {
		t.Error("no nonce is drawn for a page that has no policy")
	}
}

func TestAdmitNonce(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"nothing about scripts", "img-src https:", "img-src https:"},
		{"none", "script-src 'none'", "script-src 'nonce-N' 'self'"},
		{"an existing nonce", "script-src 'nonce-abc'", "script-src 'nonce-abc' 'nonce-N' 'self'"},
		{"elements have their own directive", "script-src 'self'; script-src-elem 'self'", "script-src 'self'; script-src-elem 'self' 'nonce-N'"},
		{"calls restricted", "script-src 'self' 'unsafe-inline'; connect-src https://api.example.com", "script-src 'self' 'unsafe-inline'; connect-src https://api.example.com 'self'"},
		{"already told", "script-src 'nonce-N'", "script-src 'nonce-N'"},
	} {
		if got := admitNonce(c.in, "N"); got != c.want {
			t.Errorf("%s: admitNonce(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestAcceptReadable(t *testing.T) {
	for in, want := range map[string]string{
		"gzip, deflate, br, zstd": "gzip, br",
		"zstd":                    "",
		"br;q=1.0, zstd;q=0.9":    "br;q=1.0",
		"identity":                "identity",
		"":                        "",
	} {
		if got := AcceptReadable(in); got != want {
			t.Errorf("AcceptReadable(%q) = %q, want %q", in, got, want)
		}
	}
}
