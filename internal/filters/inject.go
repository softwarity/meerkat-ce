// Package filters holds Meerkat's response/request filters. The first one is
// the app-gateway signature move: injecting gateway-provided content into the
// HTML pages of proxied applications (UIF-02 in the requirements).
package filters

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"regexp"

	"github.com/andybalholm/brotli"
)

var headTag = regexp.MustCompile(`(?i)<head[^>]*>`)
var bodyTag = regexp.MustCompile(`(?i)<body[^>]*>`)
var headEnd = regexp.MustCompile(`(?i)</head\s*>`)

// InjectAfterHead returns a ReverseProxy ModifyResponse function that inserts
// fragment right after the opening <head> tag of HTML responses. Non-HTML
// responses and unsupported encodings pass through untouched. Gzip and brotli
// bodies are decoded and re-encoded in the codec they arrived in.
func InjectAfterHead(fragment string) func(*http.Response) error {
	return InjectAfterHeadFunc(func(*http.Response) string { return fragment })
}

// InjectAtBodyStart inserts fragment at the top of <body> instead of in the
// head, for anything that is NOT metadata.
//
// The distinction is not cosmetic. An element the parser does not allow in a
// head - a custom element, a div - closes the head where it stands, and
// everything written after it lands in the body: the charset, the title, and
// <base href>, which the browser then ignores. An application relying on its
// base (an Angular build served under a prefix) starts resolving everything
// from the root, its router stops recognising its own URL, and the page ends
// up on the application's own 404. That is what it did.
func InjectAtBodyStart(fragment string) func(*http.Response) error {
	return InjectAtBodyStartFunc(func(*http.Response) string { return fragment })
}

// InjectAtBodyStartFunc is InjectAtBodyStart with a per-response fragment.
func InjectAtBodyStartFunc(f func(*http.Response) string) func(*http.Response) error {
	return func(res *http.Response) error {
		if !isHTML(res) {
			return nil
		}
		frag := []byte(f(res))
		if len(frag) == 0 {
			return nil
		}
		return rewriteHTMLBody(res, func(body []byte) []byte { return injectAtBodyStart(body, frag) })
	}
}

// injectAtBodyStart inserts frag right after <body>, or after </head> when the
// document leaves the body tag implicit. A fragment is left alone, like
// everywhere else.
func injectAtBodyStart(body, frag []byte) []byte {
	at := -1
	if loc := bodyTag.FindIndex(body); loc != nil {
		at = loc[1]
	} else if loc := headEnd.FindIndex(body); loc != nil {
		at = loc[1]
	}
	if at < 0 {
		if !isDocument(body) {
			return body
		}
		return append(append([]byte{}, frag...), body...)
	}
	out := make([]byte, 0, len(body)+len(frag))
	out = append(out, body[:at]...)
	out = append(out, frag...)
	return append(out, body[at:]...)
}

// InjectAfterHeadFunc is InjectAfterHead with a PER-RESPONSE fragment: f runs
// on each HTML response (res.Request carries the caller's cookies/context, so
// the fragment can be session-specific). An empty fragment skips the rewrite
// WITHOUT buffering the body (the common anonymous case stays cheap).
func InjectAfterHeadFunc(f func(*http.Response) string) func(*http.Response) error {
	return func(res *http.Response) error {
		if !isHTML(res) {
			return nil
		}
		frag := []byte(f(res))
		if len(frag) == 0 {
			return nil
		}
		return rewriteHTMLBody(res, func(body []byte) []byte { return injectAfterHead(body, frag) })
	}
}

// RewriteHTMLFunc buffers each HTML response and hands the DECODED bytes to f,
// which returns the rewritten bytes (used for server-side stamping that must
// edit an existing tag, not just insert after <head>). gate, when non-nil,
// runs first and skips the whole thing (no body buffering) when it returns
// false - pass a cheap session check so anonymous requests stay untouched.
// Non-HTML and unsupported encodings pass through.
func RewriteHTMLFunc(gate func(*http.Response) bool, f func(res *http.Response, body []byte) []byte) func(*http.Response) error {
	return func(res *http.Response) error {
		if !isHTML(res) {
			return nil
		}
		if gate != nil && !gate(res) {
			return nil
		}
		return rewriteHTMLBody(res, func(body []byte) []byte { return f(res, body) })
	}
}

// rewriteHTMLBody is the HTML injection's use of the shared rewriter (see
// rewrite.go): the guards, the codecs and the recomputed headers are the same
// for every filter that touches a body.
func rewriteHTMLBody(res *http.Response, transform func([]byte) []byte) error {
	return RewriteBody(res, transform)
}

func isHTML(res *http.Response) bool {
	ct := res.Header.Get("Content-Type")
	return len(ct) >= 9 && ct[:9] == "text/html"
}

// injectAfterHead inserts frag after the first opening <head> tag, or at the
// top of a document that has none.
//
// A FRAGMENT is left alone. text/html is not only pages: an application asks
// for HTML partials over XHR, and prepending anything to one gives it a second
// root element - AngularJS answers [$compile:tplrt] and the screen never
// renders. Nothing useful was injected there anyway: a partial has no head to
// load a script from.
func injectAfterHead(body, frag []byte) []byte {
	loc := headTag.FindIndex(body)
	out := make([]byte, 0, len(body)+len(frag))
	if loc == nil {
		if !isDocument(body) {
			return body
		}
		out = append(out, frag...)
		return append(out, body...)
	}
	out = append(out, body[:loc[1]]...)
	out = append(out, frag...)
	return append(out, body[loc[1]:]...)
}

// docStart recognises a whole HTML document: a doctype or an <html> tag. A
// partial carries neither, whatever its content type says.
var docStart = regexp.MustCompile(`(?is)^\s*(<!doctype\s+html|<html[\s>])`)

func isDocument(body []byte) bool {
	return docStart.Match(body)
}

// canRecode reports whether a body in this Content-Encoding can be read AND
// written back. Identity and gzip have always been here; brotli is what
// browsers actually negotiate now, and a gateway that cannot read it stops
// injecting anything the day an upstream turns compression on (ROUTE-14).
//
// zstd is deliberately absent: it is negotiated by almost nothing yet, and an
// unread encoding costs an injection, not a page.
func canRecode(encoding string) bool {
	switch encoding {
	case "", "identity", "gzip", "br":
		return true
	}
	return false
}

func decode(encoding string, data []byte) ([]byte, error) {
	switch encoding {
	case "gzip":
		return gunzip(data)
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
	}
	return data, nil
}

func encode(encoding string, data []byte) ([]byte, error) {
	switch encoding {
	case "gzip":
		return gzipBytes(data)
	case "br":
		return brotliBytes(data)
	}
	return data, nil
}

func brotliBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	// Default quality: this runs on a response the visitor is waiting for, and
	// brotli's top levels cost far more time than they save bytes.
	bw := brotli.NewWriterLevel(&buf, brotli.DefaultCompression)
	if _, err := bw.Write(data); err != nil {
		return nil, err
	}
	if err := bw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	plain, err := io.ReadAll(zr)
	if cerr := zr.Close(); err == nil {
		err = cerr
	}
	return plain, err
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
