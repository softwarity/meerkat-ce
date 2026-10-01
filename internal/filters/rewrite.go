package filters

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
)

// Rewriting a response body, once, for everything that needs it.
//
// This was the HTML injection's private plumbing until a second caller wanted
// it (removing fields from a JSON answer). Everything below is a refusal
// waiting to happen, and every refusal is the same shape: the response goes
// through UNTOUCHED. A body a gateway cannot rewrite safely is a body it must
// hand over as it is - the alternative is a page that arrives broken, and the
// cause is nowhere near the symptom.
//
// What it refuses, and why each one costs a production incident:
//
//   - a STREAM. Buffering an event stream holds it until the upstream closes,
//     which for SSE is never: the page hangs with nothing in the console. Same
//     for an upgraded connection, which is not an HTTP response any more.
//   - a codec it cannot read (zstd today). Rewriting compressed bytes as if
//     they were text produces garbage under a correct-looking header.
//   - a body over the ceiling. A gateway that buffers a 2 GB download to look
//     for a field is a gateway that dies of it.
//   - a RANGE answer (206). Rewriting a slice of a document changes lengths
//     the client is matching against offsets it asked for.
//
// And what it always does after a rewrite: recompute Content-Length, and drop
// the ETag. The upstream's ETag describes the upstream's bytes; keeping it on
// bytes we changed makes a cache serve one under the name of the other.

// DefaultMaxRewritableBody is the ceiling an installation runs with until an
// operator changes it - and what every installation ran with when it was a
// constant.
const DefaultMaxRewritableBody = 20 << 20 // 20 MiB

// ceiling is the live value. A package-level variable rather than a parameter
// threaded through every filter, and that is the modelling, not a shortcut:
// the bill is per REQUEST IN FLIGHT, so a per-route ceiling would make the
// memory cost "concurrent requests times the largest number anybody typed",
// with nothing bounding it. What this protects is the process, so it is one
// number for the process.
var ceiling atomic.Int64

// SetMaxRewritableBody installs the operator's ceiling, in bytes. Called from
// the route reload, so a change reaches every gateway through the same bus as
// the routes themselves. A value of zero restores the default.
func SetMaxRewritableBody(n int64) { ceiling.Store(n) }

// MaxRewritableBody caps what will be buffered to be rewritten. Bigger
// responses pass through - the ceiling is not a failure, it is the promise
// that a gateway's memory does not follow the size of what it proxies.
func MaxRewritableBody() int64 {
	if n := ceiling.Load(); n > 0 {
		return n
	}
	return DefaultMaxRewritableBody
}

// Rewritable reports whether this response can be rewritten at all, before
// anyone reads a byte of it. Exported because a filter may want to say "not
// applicable here" for its own reasons on top of these.
func Rewritable(res *http.Response) bool {
	if res == nil || res.Body == nil {
		return false
	}
	// 204/304 carry no body; 206 carries a slice of one, which is worse than
	// no body: rewriting it moves offsets the client already holds.
	switch res.StatusCode {
	case http.StatusNoContent, http.StatusNotModified, http.StatusPartialContent:
		return false
	}
	if res.Header.Get("Upgrade") != "" || res.StatusCode == http.StatusSwitchingProtocols {
		return false
	}
	if isStream(res) {
		return false
	}
	if res.ContentLength > MaxRewritableBody() {
		return false
	}
	return canRecode(strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Encoding"))))
}

// isStream reports whether this response is meant to arrive in pieces, over
// time. Buffering one is not slow, it is broken: the client sees nothing until
// the upstream closes, and an event stream never does.
func isStream(res *http.Response) bool {
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	return strings.HasPrefix(ct, "text/event-stream") ||
		strings.Contains(strings.ToLower(res.Header.Get("Cache-Control")), "no-transform")
}

// RewriteBody reads the body, hands the DECODED bytes to transform, and puts
// the result back in the codec it arrived in.
//
// transform returns the new body, or nil to say "leave it alone" - which a
// filter uses when it finds nothing to change, and which costs nothing: the
// bytes already read are handed back as they were.
func RewriteBody(res *http.Response, transform func([]byte) []byte) error {
	if !Rewritable(res) {
		return nil
	}
	encoding := strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Encoding")))
	body, err := io.ReadAll(io.LimitReader(res.Body, MaxRewritableBody()+1))
	if err != nil {
		_ = res.Body.Close()
		return fmt.Errorf("rewrite: read body: %w", err)
	}
	// Over the ceiling despite the Content-Length check: a chunked response
	// announces no length at all, so this is where most of them are caught.
	//
	// What goes back is the sample we read FOLLOWED BY the rest of the
	// upstream body, still open. Reading a sample, closing the upstream and
	// handing back the sample is how a 200 MB download used to arrive
	// truncated to the ceiling plus one byte - silently, with a correct
	// status, through any route carrying a body-rewriting filter. The ceiling
	// promises that a bigger answer passes through UNTOUCHED, and untouched
	// includes whole.
	if int64(len(body)) > MaxRewritableBody() {
		res.Body = joined{Reader: io.MultiReader(bytes.NewReader(body), res.Body), Closer: res.Body}
		return nil
	}
	// Under the ceiling the body is fully read, so this closes what we drained.
	if err := res.Body.Close(); err != nil {
		return fmt.Errorf("rewrite: close body: %w", err)
	}
	plain, err := decode(encoding, body)
	if err != nil {
		// Broken encoding: hand the original bytes back untouched.
		res.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	out := transform(plain)
	if out == nil {
		res.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	// Re-encoded with the SAME codec it arrived in: the response keeps the
	// Content-Encoding it announced, so nothing downstream has to be told
	// anything - and a body re-sent as plain text under a gzip header is the
	// blank page nobody finds by reading the HTML.
	if out, err = encode(encoding, out); err != nil {
		return fmt.Errorf("rewrite: %s: %w", encoding, err)
	}
	res.Body = io.NopCloser(bytes.NewReader(out))
	res.ContentLength = int64(len(out))
	res.Header.Set("Content-Length", strconv.Itoa(len(out)))
	// The upstream's ETag describes the upstream's bytes. Kept on bytes we
	// changed, a cache would serve one under the name of the other - and the
	// next conditional request would be answered 304 with the wrong body.
	res.Header.Del("ETag")
	// And the caching rules, for the same reason: they describe a document
	// that no longer exists. Without this, a configuration change in the
	// console lands only after the upstream's max-age has run out.
	Rewritten(res.Header)
	validate(res, out)
	return nil
}

// validate gives a rewritten page a validator of its own, and answers the
// revalidation itself (PERF-06).
//
// no-cache makes the browser ask before every reuse, which is right - the
// page may carry the PREVIOUS configuration - but without a validator every
// asking was a full download of a page that had not changed. So: an ETag over
// the bytes actually sent, and a 304 when the browser presents it. A change in
// the console changes the bytes, so the hash, so the next request gets the new
// page; nothing changed, and the answer is empty.
//
// STRONG, because it is computed over the exact octets of this representation
// - the encoded ones, so a gzip body and a plain body of the same page are two
// validators, as they must be. Several filters may rewrite one response in
// turn: each pass hashes what it produced, so only the last pass's tag is the
// one a browser can hold, and an earlier pass can only match when every later
// one left the bytes alone - when its tag IS the final one.
//
// Only on a 200, and never on a response kept from every cache (no-store): a
// page stamped for one person is not to be revalidated, it is not to be kept.
func validate(res *http.Response, out []byte) {
	if res.StatusCode != http.StatusOK ||
		strings.Contains(strings.ToLower(res.Header.Get("Cache-Control")), "no-store") {
		return
	}
	sum := sha256.Sum256(out)
	etag := `"mk-` + hex.EncodeToString(sum[:16]) + `"`
	res.Header.Set("ETag", etag)
	req := res.Request
	if req == nil || (req.Method != http.MethodGet && req.Method != http.MethodHead) {
		return
	}
	if !etagMatches(req.Header.Get("If-None-Match"), etag) {
		return
	}
	// The browser already holds these very bytes.
	_ = res.Body.Close()
	res.StatusCode = http.StatusNotModified
	res.Status = "304 Not Modified"
	res.Body = http.NoBody
	res.ContentLength = 0
	res.Header.Del("Content-Length")
}

// etagMatches is If-None-Match's comparison: WEAK (RFC 9110 13.1.2), so a W/
// on either side does not matter, over a list of tags or "*".
func etagMatches(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, t := range strings.Split(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(t), "W/") == want {
			return true
		}
	}
	return false
}

// joined is what a body handed back in two pieces has to be: the bytes already
// read in front, the upstream connection behind, and the ORIGINAL closer - so
// whoever finishes with this response still releases the connection it came
// on.
type joined struct {
	io.Reader
	io.Closer
}
