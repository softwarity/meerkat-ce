package routing

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// An application published under a prefix it does not know about answers as if
// it lived at the root: it redirects to its own address, and its page declares
// <base href="/">. Both send the browser OUT of the route - to the upstream's
// public name, or to the gateway's root, where its files are not. strip-prefix
// took the prefix off on the way in; these put it back on the way out, on the
// two things a browser follows without asking.

type strippedKey struct{}

// withStripped records, on the outgoing request, the prefix strip-prefix
// consumed: the response is read with that request in hand.
func withStripped(req *http.Request, prefix string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), strippedKey{}, prefix))
}

// StrippedPrefix is the public prefix this request lost on its way upstream:
// "/app" for /app/orders sent as /orders. Empty when nothing was stripped.
func StrippedPrefix(req *http.Request) string {
	if req == nil {
		return ""
	}
	p, _ := req.Context().Value(strippedKey{}).(string)
	return p
}

// under reports whether path is the prefix itself or something below it.
func under(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// BringLocationHome rewrites the Location of an upstream redirect so that the
// browser stays on the route it came through.
//
//   - A redirect to the upstream's own origin - "https://www.example.com/fr/"
//     from the upstream www.example.com - becomes a path on the gateway. Left
//     as it is, the browser goes to the application directly and everything
//     the gateway adds (the session, the portal, the access rule) is gone,
//     without an error anywhere.
//   - A path the application wrote from its own root - "/login" - gets the
//     prefix strip-prefix removed. An application that knows where it is
//     published (it read X-Forwarded-Prefix, or was configured) already writes
//     the prefix, and is left alone.
//
// Anything else - another site, a relative path - is the application's
// business.
func BringLocationHome(res *http.Response) {
	loc := res.Header.Get("Location")
	if loc == "" || res.Request == nil || res.Request.URL == nil {
		return
	}
	prefix := StrippedPrefix(res.Request)
	path, mine := "", false
	switch {
	case strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//"):
		// Only a stripped route has a prefix to give back.
		path, mine = loc, prefix != ""
	default:
		// The upstream's origin, under either scheme: a service behind TLS
		// termination redirects to https what was asked over http.
		host := res.Request.URL.Host
		for _, origin := range []string{"https://" + host, "http://" + host} {
			if rest, ok := strings.CutPrefix(loc, origin); ok && (rest == "" || rest[0] == '/' || rest[0] == '?' || rest[0] == '#') {
				path, mine = rest, true
				if path == "" || path[0] != '/' {
					path = "/" + path
				}
				break
			}
		}
	}
	if !mine {
		return
	}
	if prefix != "" {
		// Compared on the path alone: the query may carry anything.
		p := path
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		if !under(p, prefix) {
			path = prefix + path
		}
	}
	if path != loc {
		res.Header.Set("Location", path)
	}
}

// A <base> with a root-absolute href, the first of the document.
var baseHref = regexp.MustCompile(`(?i)(<base\b[^>]*?\bhref\s*=\s*)("(/[^"]*)"|'(/[^']*)')`)

// PublishBase puts the stripped prefix in front of a document's <base href>,
// or returns nil when there is nothing to change. <base href="/"> is what a
// single-page application is built with, and under a prefix it makes every
// script and stylesheet of the page resolve at the gateway's root: the page
// arrives and nothing it asks for does.
func PublishBase(body []byte, prefix string) []byte {
	if prefix == "" {
		return nil
	}
	m := baseHref.FindSubmatchIndex(body)
	if m == nil {
		return nil
	}
	// The href's value, without its quotes: group 3 or 4.
	from, to := m[6], m[7]
	if from < 0 {
		from, to = m[8], m[9]
	}
	href := string(body[from:to])
	if strings.HasPrefix(href, "//") || under(strings.TrimSuffix(href, "/"), prefix) || under(href, prefix) {
		return nil
	}
	out := make([]byte, 0, len(body)+len(prefix))
	out = append(out, body[:from]...)
	out = append(out, prefix...)
	return append(out, body[from:]...)
}
