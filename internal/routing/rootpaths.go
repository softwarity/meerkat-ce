package routing

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// An application written for the root, published under a prefix.
//
// published.go puts the prefix back on what such an application writes from
// its root and the browser follows without asking: its redirects and the
// <base href> of its pages. That is enough for an application whose links are
// RELATIVE to the page - a base resolves those - and nothing for one that
// writes "/static/app.js": a path that starts with a slash ignores the base.
// The browser asks the gateway's root for it, where another route answers, or
// nobody.
//
// A route that says so (store.RouteUI.RootPaths) gets three more things, all
// adding the prefix the request lost to a path that starts with a slash:
//
//   - the attributes of its pages that name an address (href, src, action,
//     srcset...) and the url() of their inline styles, read as HTML - a tag is
//     a tag, a script's text is not touched;
//   - the url() and @import of its stylesheets;
//   - the calls its scripts make once the page runs (fetch, XMLHttpRequest,
//     WebSocket, EventSource, sendBeacon), through a script put first in the
//     page: an address built at run time exists nowhere in the bytes the
//     gateway sees.
//
// What stays out of reach is said where the switch is: an address a script
// assigns to an element or to location, a module imported by an absolute
// path, a worker. Only a host of its own covers those.

// GatewayPaths are the addresses the gateway's own scripts call from a page
// it serves (the user button, the portal bar, the tracing relay). They are
// the gateway's, under any route, and never get a prefix.
var GatewayPaths = []string{"/meerkat", "/logout", "/select-tenant", "/select-group"}

// rootPath returns u with prefix in front when u is a path from the root
// ("/x", not "//host/x") that is neither under prefix already nor one of the
// gateway's own. ok is false when u stays as it is.
func rootPath(u, prefix string) (string, bool) {
	if prefix == "" || !strings.HasPrefix(u, "/") || strings.HasPrefix(u, "//") {
		return u, false
	}
	p := u
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if under(p, prefix) {
		return u, false
	}
	for _, own := range GatewayPaths {
		if under(p, own) {
			return u, false
		}
	}
	return prefix + u, true
}

// urlAttrs are the attributes holding ONE address. srcset holds several and
// style holds CSS: both are read apart.
var urlAttrs = map[string]bool{
	"href": true, "src": true, "action": true, "formaction": true, "poster": true,
	"data": true, "background": true, "xlink:href": true, "manifest": true, "cite": true,
}

// PublishRootPaths puts prefix in front of the root paths a document's tags
// name, or returns nil when there is nothing to change. The page is read with
// an HTML tokenizer and written back byte for byte, only the values changed:
// a script's text, a comment or an attribute that names no address leave as
// they came.
func PublishRootPaths(body []byte, prefix string) []byte {
	if prefix == "" {
		return nil
	}
	z := html.NewTokenizer(bytes.NewReader(body))
	out := make([]byte, 0, len(body)+256)
	changed, inStyle := false, false
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() != io.EOF {
				return nil
			}
			break
		}
		raw := z.Raw()
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			inStyle = tt == html.StartTagToken && string(name) == "style"
			if t, ok := rewriteTag(raw, prefix); ok {
				out, changed = append(out, t...), true
				continue
			}
		case html.EndTagToken:
			inStyle = false
		case html.TextToken:
			if inStyle {
				if t, ok := rewriteCSS(raw, prefix); ok {
					out, changed = append(out, t...), true
					continue
				}
			}
		}
		out = append(out, raw...)
	}
	if !changed {
		return nil
	}
	return out
}

// PublishCSSPaths does the same to a stylesheet: its url() and @import.
func PublishCSSPaths(body []byte, prefix string) []byte {
	if out, ok := rewriteCSS(body, prefix); ok {
		return out
	}
	return nil
}

// rewriteTag reads a start tag's attributes one by one - a value is skipped
// as a whole, so "src=" inside a title is not an attribute - and rewrites the
// values that name addresses.
func rewriteTag(tag []byte, prefix string) ([]byte, bool) {
	var out []byte
	last, changed := 0, false
	i := 1
	// The tag's name.
	for i < len(tag) && !isSpace(tag[i]) && tag[i] != '>' && tag[i] != '/' {
		i++
	}
	for i < len(tag) {
		for i < len(tag) && (isSpace(tag[i]) || tag[i] == '/') {
			i++
		}
		if i >= len(tag) || tag[i] == '>' {
			break
		}
		start := i
		for i < len(tag) && !isSpace(tag[i]) && tag[i] != '=' && tag[i] != '>' && tag[i] != '/' {
			i++
		}
		name := strings.ToLower(string(tag[start:i]))
		for i < len(tag) && isSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			continue
		}
		i++
		for i < len(tag) && isSpace(tag[i]) {
			i++
		}
		var from, to int
		if i < len(tag) && (tag[i] == '"' || tag[i] == '\'') {
			q := tag[i]
			from = i + 1
			to = bytes.IndexByte(tag[from:], q)
			if to < 0 {
				return nil, false
			}
			to += from
			i = to + 1
		} else {
			from = i
			for i < len(tag) && !isSpace(tag[i]) && tag[i] != '>' {
				i++
			}
			to = i
		}
		value := tag[from:to]
		var nv []byte
		ok := false
		switch {
		case urlAttrs[name]:
			var s string
			if s, ok = rootPath(string(value), prefix); ok {
				nv = []byte(s)
			}
		case name == "srcset" || name == "imagesrcset":
			nv, ok = rewriteSrcset(value, prefix)
		case name == "style":
			nv, ok = rewriteCSS(value, prefix)
		}
		if ok {
			out = append(out, tag[last:from]...)
			out = append(out, nv...)
			last, changed = to, true
		}
	}
	if !changed {
		return nil, false
	}
	return append(out, tag[last:]...), true
}

// rewriteSrcset handles "a.png 1x, /b.png 2x": a comma-separated list of an
// address and its descriptor.
func rewriteSrcset(v []byte, prefix string) ([]byte, bool) {
	parts := strings.Split(string(v), ",")
	changed := false
	for i, part := range parts {
		lead := len(part) - len(strings.TrimLeft(part, " \t\n\r\f"))
		rest := part[lead:]
		end := strings.IndexAny(rest, " \t\n\r\f")
		if end < 0 {
			end = len(rest)
		}
		if s, ok := rootPath(rest[:end], prefix); ok {
			parts[i] = part[:lead] + s + rest[end:]
			changed = true
		}
	}
	if !changed {
		return nil, false
	}
	return []byte(strings.Join(parts, ",")), true
}

// cssRef finds where an address starts in CSS: after url( with or without a
// quote (an HTML-escaped one too, in a style attribute), and after @import's
// quote.
var cssRef = regexp.MustCompile(`(?i)(?:url\(\s*(?:&quot;|&#39;|["'])?|@import\s+["'])`)

func rewriteCSS(css []byte, prefix string) ([]byte, bool) {
	var out []byte
	last, changed := 0, false
	for _, m := range cssRef.FindAllIndex(css, -1) {
		at := m[1]
		end := at
		for end < len(css) && !strings.ContainsRune(`)"'&; `+"\t\n", rune(css[end])) {
			end++
		}
		if s, ok := rootPath(string(css[at:end]), prefix); ok {
			out = append(out, css[last:at]...)
			out = append(out, s...)
			last, changed = end, true
		}
	}
	if !changed {
		return nil, false
	}
	return append(out, css[last:]...), true
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// RootPathsScript is the script put first in a page of such a route: it puts
// prefix in front of the root paths the page's scripts call at run time. Only
// calls to this very origin, and never the gateway's own addresses.
func RootPathsScript(prefix string) string {
	p, _ := json.Marshal(prefix)
	own, _ := json.Marshal(GatewayPaths)
	return "<script>\n(function () {\n" +
		"  // Meerkat: this application writes its addresses from the root and is\n" +
		"  // published under P: its calls get the prefix back.\n" +
		"  var P = " + string(p) + ", OWN = " + string(own) + ";\n" +
		rootPathsJS +
		"})();\n</script>\n"
}

// Plain ES5 and no backtick: it lands in pages the gateway did not write, in
// browsers it does not choose.
const rootPathsJS = `  function under(p, b) { return p === b || p.indexOf(b + '/') === 0; }
  function fix(u) {
    if (typeof u !== 'string' && !(u instanceof URL)) return u;
    var a;
    try { a = new URL(String(u), location.href); } catch (e) { return u; }
    if (a.host !== location.host || under(a.pathname, P)) return u;
    for (var i = 0; i < OWN.length; i++) if (under(a.pathname, OWN[i])) return u;
    a.pathname = P + a.pathname;
    return a.href;
  }
  if (window.fetch) {
    var f = window.fetch;
    window.fetch = function (input, init) {
      if (typeof Request !== 'undefined' && input instanceof Request) {
        var g = fix(input.url);
        if (g !== input.url) input = new Request(g, input);
      } else {
        input = fix(input);
      }
      return f.call(this, input, init);
    };
  }
  if (window.XMLHttpRequest) {
    var open = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function (m, u) {
      var args = Array.prototype.slice.call(arguments);
      args[1] = fix(u);
      return open.apply(this, args);
    };
  }
  function wrap(name) {
    var C = window[name];
    if (!C) return;
    var W = function (u, o) { return arguments.length > 1 ? new C(fix(u), o) : new C(fix(u)); };
    W.prototype = C.prototype;
    for (var k in C) { try { W[k] = C[k]; } catch (e) {} }
    ['CONNECTING', 'OPEN', 'CLOSING', 'CLOSED'].forEach(function (k) { if (k in C) W[k] = C[k]; });
    window[name] = W;
  }
  wrap('WebSocket');
  wrap('EventSource');
  if (navigator.sendBeacon) {
    var b = navigator.sendBeacon.bind(navigator);
    navigator.sendBeacon = function (u, d) { return b(fix(u), d); };
  }
`
