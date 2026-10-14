package filters

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
)

// A page that carries a Content-Security-Policy decides which scripts run, and
// the scripts the gateway injects are not on its list: the application wrote
// its policy before anybody put a gateway in front of it. Under
// "script-src 'strict-dynamic' 'sha256-...'" - what an Angular build writes for
// itself, in a <meta> - the browser refuses /meerkat/page.js, the user button
// and the portal, and the page comes out without any of them, silently: the
// only trace is in the browser's console.
//
// The gateway is what serves the page, so it is in a position to say that its
// own scripts are part of it. Each injected <script> gets a nonce, drawn per
// response, and each policy the page carries - header or <meta> - is told that
// nonce. Nothing else is opened: the application's own sources stay as they
// are, and a script that is neither the application's nor the gateway's is
// refused exactly as before.
//
// The same goes for STYLES: the user button and the portal draw themselves in
// a shadow root with a <style> of their own, and a route may inject a CSS
// block. Under a policy restricting style-src (GeoServer's, nonce-based) those
// were refused and the portal's bar spread over the whole page, unstyled. The
// nonce goes into the style directive too, the injected <style> blocks carry
// it, and the scripts set it on the styles they create.
//
// A nonce rather than a hash or a host: under 'strict-dynamic' a host source
// ('self') is ignored, and a hash names one inline script, not a file. A nonce
// works under every shape of policy, and lets a script it admits load what it
// needs.

// nonceHeader carries the response's nonce from one injection to the next: a
// page receives several fragments, and they have to share one. It never
// leaves: Sealed removes it once the route's filters have run.
const nonceHeader = "Meerkat-Csp-Nonce"

const cspHeader = "Content-Security-Policy"

// cspHeaders are the policies a response may carry: the enforced one, and the
// one only reported - told too, or its reports name the gateway's scripts as
// violations every time a page loads.
var cspHeaders = []string{cspHeader, "Content-Security-Policy-Report-Only"}

var (
	scriptOpen = regexp.MustCompile(`(?i)<script\b`)
	styleOpen  = regexp.MustCompile(`(?i)<style\b`)
	// A policy in the document: <meta http-equiv="Content-Security-Policy" content="...">.
	metaCSP     = regexp.MustCompile(`(?is)<meta\b[^>]*\bhttp-equiv\s*=\s*["']?content-security-policy["']?[^>]*>`)
	metaContent = regexp.MustCompile(`(?is)(\bcontent\s*=\s*)("([^"]*)"|'([^']*)')`)
)

// Sealed removes what the injections kept on the response for each other. The
// router calls it once a route's response filters have run.
func Sealed(h http.Header) {
	h.Del(nonceHeader)
}

// admitScripts makes the page's policies accept the scripts in frag, and
// returns the fragment and the body to use. Both come back untouched when the
// fragment carries no script or the page no policy - the common case, which
// must stay byte for byte what it was.
func admitScripts(res *http.Response, body, frag []byte) ([]byte, []byte) {
	if !scriptOpen.Match(frag) && !styleOpen.Match(frag) {
		return body, frag
	}
	inHeader := false
	for _, name := range cspHeaders {
		inHeader = inHeader || len(res.Header.Values(name)) > 0
	}
	inBody := metaCSP.Match(body)
	if !inHeader && !inBody {
		return body, frag
	}
	nonce := res.Header.Get(nonceHeader)
	if nonce == "" {
		raw := make([]byte, 18)
		if _, err := rand.Read(raw); err != nil {
			// No randomness, no nonce: the page keeps its policy and loses
			// our scripts, which is the lesser of the two failures.
			return body, frag
		}
		nonce = base64.StdEncoding.EncodeToString(raw)
		res.Header.Set(nonceHeader, nonce)
	}
	withNonce := func(tag []byte) []byte {
		// A new slice: tag is a window on frag, and appending to it would
		// write over the bytes that follow.
		return []byte(string(tag) + ` nonce="` + nonce + `"`)
	}
	frag = scriptOpen.ReplaceAllFunc(frag, withNonce)
	frag = styleOpen.ReplaceAllFunc(frag, withNonce)
	for _, name := range cspHeaders {
		values := res.Header.Values(name)
		if len(values) == 0 {
			continue
		}
		res.Header.Del(name)
		for _, v := range values {
			// One header may carry several policies, separated by commas:
			// each is enforced on its own, so each has to be told.
			policies := strings.Split(v, ",")
			for i, p := range policies {
				policies[i] = admitNonce(p, nonce)
			}
			res.Header.Add(name, strings.Join(policies, ","))
		}
	}
	if inBody {
		body = metaCSP.ReplaceAllFunc(body, func(tag []byte) []byte {
			return metaContent.ReplaceAllFunc(tag, func(attr []byte) []byte {
				m := metaContent.FindSubmatch(attr)
				quote, policy := `"`, string(m[3])
				if len(m[2]) > 0 && m[2][0] == '\'' {
					quote, policy = `'`, string(m[4])
				}
				return []byte(string(m[1]) + quote + admitNonce(policy, nonce) + quote)
			})
		})
	}
	return body, frag
}

// admitNonce returns policy with nonce admitted for script and style elements,
// and the page's own origin admitted for what those scripts fetch. A policy that says
// nothing about scripts is returned as it is: it refuses none.
func admitNonce(policy, nonce string) string {
	source := "'nonce-" + nonce + "'"
	if strings.Contains(policy, source) {
		return policy // a second fragment on the same page
	}
	changed := false
	directives := splitPolicy(policy)

	// What governs <script> elements, then <style> elements: the most
	// specific directive present. When only default-src speaks, a script-src
	// (a style-src) is written from it rather than touching default-src
	// itself - a nonce there would switch off 'unsafe-inline' for everything
	// else that falls back on it.
	if admit(&directives, source, "script-src", true, "script-src-elem", "script-src", "default-src") {
		changed = true
	}
	// The injected scripts' own <style> elements (the user button, the
	// portal's bar) and the CSS blocks a route injects. Inline, so no host is
	// added: a nonce is all they need.
	if admit(&directives, source, "style-src", false, "style-src-elem", "style-src", "default-src") {
		changed = true
	}

	// What the admitted scripts call: the gateway's own endpoints, on the
	// page's origin (the account, the portal's menu).
	if at, from := governing(directives, "connect-src", "default-src"); at >= 0 {
		sources := directives[at].sources
		if !has(sources, "'self'") && !has(sources, "*") {
			sources = append(without(sources, "'none'"), "'self'")
			if from == "default-src" {
				directives = append(directives, directive{name: "connect-src", sources: sources})
			} else {
				directives[at].sources = sources
			}
			changed = true
		}
	}

	if !changed {
		return policy
	}
	parts := make([]string, len(directives))
	for i, d := range directives {
		parts[i] = strings.TrimSpace(d.name + " " + strings.Join(d.sources, " "))
	}
	out := strings.Join(parts, "; ")
	// Kept as the application wrote it where it can be: its trailing
	// semicolon, and the space it put (or did not put) in front of the policy.
	if strings.HasSuffix(strings.TrimSpace(policy), ";") {
		out += ";"
	}
	if strings.HasPrefix(policy, " ") {
		out = " " + out
	}
	return out
}

// admit puts the nonce into the directive governing one kind of element, and
// says whether it changed anything. Written from default-src, it becomes a
// directive of its own named created: a copy of default-src plus the nonce,
// never the nonce alone, or the page would lose its own sources for it.
func admit(directives *[]directive, source, created string, scripts bool, names ...string) bool {
	at, from := governing(*directives, names...)
	if at < 0 {
		return false // nothing restricts this kind of element
	}
	d := *directives
	sources := without(d[at].sources, "'none'")
	// The nonce goes in unless the policy lets everything inline run: a
	// browser that sees a nonce or a hash IGNORES 'unsafe-inline', so adding
	// ours to "script-src 'self' 'unsafe-inline'" would switch off the
	// application's own inline code to admit ours - which already runs there.
	if !has(sources, "'unsafe-inline'") || namesAScript(sources) {
		sources = append(sources, source)
	}
	// Without 'strict-dynamic', a script from a file is judged by its host,
	// and so is what an admitted script loads in turn: the gateway's scripts
	// come from the page's own origin.
	if scripts && !has(sources, "'strict-dynamic'") && !has(sources, "'self'") && !has(sources, "*") {
		sources = append(sources, "'self'")
	}
	// Nothing added: this policy admits them already, and stays as written.
	if len(sources) == len(d[at].sources) {
		return false
	}
	if from == "default-src" {
		*directives = append(d, directive{name: created, sources: sources})
	} else {
		d[at].sources = sources
	}
	return true
}

type directive struct {
	name    string
	sources []string
}

func splitPolicy(policy string) []directive {
	var out []directive
	for _, part := range strings.Split(policy, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		out = append(out, directive{name: strings.ToLower(fields[0]), sources: fields[1:]})
	}
	return out
}

// governing returns the first of names the policy carries, and its position.
func governing(directives []directive, names ...string) (int, string) {
	for _, name := range names {
		for i, d := range directives {
			if d.name == name {
				return i, name
			}
		}
	}
	return -1, ""
}

func has(sources []string, want string) bool {
	for _, s := range sources {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func without(sources []string, drop string) []string {
	out := make([]string, 0, len(sources)+2)
	for _, s := range sources {
		if !strings.EqualFold(s, drop) {
			out = append(out, s)
		}
	}
	return out
}

// namesAScript reports whether the sources carry a nonce or a hash - the
// presence of either is what makes a browser ignore 'unsafe-inline'.
func namesAScript(sources []string) bool {
	for _, s := range sources {
		l := strings.ToLower(s)
		if strings.HasPrefix(l, "'nonce-") || strings.HasPrefix(l, "'sha256-") ||
			strings.HasPrefix(l, "'sha384-") || strings.HasPrefix(l, "'sha512-") {
			return true
		}
	}
	return false
}
