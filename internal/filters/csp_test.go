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
		// Styles, under the same rule: the injected scripts draw with a <style>.
		{"styles restricted", "style-src 'self'", "style-src 'self' 'nonce-N'"},
		{"styles inline already", "style-src 'self' 'unsafe-inline'", "style-src 'self' 'unsafe-inline'"},
		{"styles with a nonce", "style-src 'nonce-G'", "style-src 'nonce-G' 'nonce-N'"},
		{"style elements have their own", "style-src 'self'; style-src-elem 'self'", "style-src 'self'; style-src-elem 'self' 'nonce-N'"},
		// Only default-src: each kind gets a COPY of it plus the nonce, never
		// the nonce alone, and default-src itself stays as written.
		{"default-src alone", "default-src 'self' https://cdn.example.com",
			"default-src 'self' https://cdn.example.com; script-src 'self' https://cdn.example.com 'nonce-N'; style-src 'self' https://cdn.example.com 'nonce-N'"},
		{"default-src inline", "default-src 'self' 'unsafe-inline'", "default-src 'self' 'unsafe-inline'"},
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

// GeoServer's policy (2026-10-14): nonce-based for scripts AND styles, two
// policies in one header. The portal's bar ran and drew itself with a <style>
// the browser refused, spreading over the whole page. Every policy that
// restricts styles must be told the nonce; the others stay as written.
func TestTheNonceReachesStylesUnderEveryPolicy(t *testing.T) {
	const header = "script-src 'strict-dynamic' 'nonce-G'; style-src 'nonce-G'; frame-src 'self';, form-action 'self'"
	res := htmlResponse(`<html><head></head><body></body></html>`, http.Header{
		cspHeader:                             {header},
		"Content-Security-Policy-Report-Only": {"style-src 'self'"},
	})
	if err := InjectAfterHead(agent)(res); err != nil {
		t.Fatal(err)
	}
	m := nonceAttr.FindStringSubmatch(readBody(t, res))
	if m == nil {
		t.Fatal("no nonce on the injected script")
	}
	ours := "'nonce-" + m[1] + "'"
	policies := strings.Split(res.Header.Get(cspHeader), ",")
	if len(policies) != 2 {
		t.Fatalf("two policies in, %d out: %q", len(policies), res.Header.Get(cspHeader))
	}
	if !strings.Contains(policies[0], "style-src 'nonce-G' "+ours) {
		t.Errorf("the styles must admit our nonce beside GeoServer's: %q", policies[0])
	}
	if !strings.Contains(policies[0], "script-src 'strict-dynamic' 'nonce-G' "+ours) {
		t.Errorf("the scripts must admit our nonce: %q", policies[0])
	}
	if strings.TrimSpace(policies[1]) != "form-action 'self'" {
		t.Errorf("a policy restricting neither must stay as written: %q", policies[1])
	}
	if got := res.Header.Get("Content-Security-Policy-Report-Only"); !strings.Contains(got, ours) {
		t.Errorf("the report-only policy must be told too, or it reports our styles: %q", got)
	}
}

// A CSS block a route injects is a <style> of its own: under a policy that
// restricts styles it carries the nonce, without one it goes in untouched.
func TestAnInjectedStyleBlockCarriesTheNonce(t *testing.T) {
	const css = `<style>body{margin:0}</style>`
	res := htmlResponse(`<html><head></head><body></body></html>`, http.Header{cspHeader: {"style-src 'self'"}})
	if err := InjectAfterHead(css)(res); err != nil {
		t.Fatal(err)
	}
	out := readBody(t, res)
	m := nonceAttr.FindStringSubmatch(out)
	if m == nil || !strings.Contains(out, `<style nonce="`+m[1]+`">`) {
		t.Fatalf("the injected style block carries no nonce:\n%s", out)
	}
	if !strings.Contains(res.Header.Get(cspHeader), "'nonce-"+m[1]+"'") {
		t.Errorf("the policy was not told the style's nonce: %q", res.Header.Get(cspHeader))
	}

	plain := htmlResponse(`<html><head></head><body></body></html>`, nil)
	if err := InjectAfterHead(css)(plain); err != nil {
		t.Fatal(err)
	}
	if out := readBody(t, plain); out != `<html><head>`+css+`</head><body></body></html>` {
		t.Errorf("without a policy the block goes in byte for byte, got %s", out)
	}
}
