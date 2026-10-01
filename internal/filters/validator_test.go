package filters

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func rewrittenPage(inm string, body string, passes ...func([]byte) []byte) *http.Response {
	req, _ := http.NewRequest(http.MethodGet, "http://app.example/", nil)
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	res := &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Request: req,
		Header:        http.Header{"Content-Type": {"text/html"}, "Etag": {`"upstream-1"`}, "Cache-Control": {"max-age=600"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	for _, p := range passes {
		if err := RewriteBody(res, p); err != nil {
			panic(err)
		}
	}
	return res
}

func appendText(s string) func([]byte) []byte {
	return func(b []byte) []byte { return append(append([]byte{}, b...), s...) }
}

// A rewritten page carries a validator of its own, and a browser presenting
// it gets an empty 304 (PERF-06); the same page after a configuration change
// is new bytes, a new tag, and a full answer.
func TestARewrittenPageRevalidatesToA304(t *testing.T) {
	first := rewrittenPage("", "<html></html>", appendText("<button/>"))
	etag := first.Header.Get("ETag")
	if first.StatusCode != http.StatusOK || !strings.HasPrefix(etag, `"mk-`) {
		t.Fatalf("first answer: %d, etag %q", first.StatusCode, etag)
	}
	if cc := first.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("cache-control %q", cc)
	}

	again := rewrittenPage(etag, "<html></html>", appendText("<button/>"))
	if again.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidation with the same tag: %d", again.StatusCode)
	}
	if b, _ := io.ReadAll(again.Body); len(b) != 0 {
		t.Fatalf("a 304 carried %d bytes", len(b))
	}

	// The console changed something: other bytes, another tag, a full page.
	changed := rewrittenPage(etag, "<html></html>", appendText("<portal/>"))
	if changed.StatusCode != http.StatusOK || changed.Header.Get("ETag") == etag {
		t.Fatalf("after a change: %d, etag %q", changed.StatusCode, changed.Header.Get("ETag"))
	}

	// The upstream's own tag never matches: it described other bytes.
	if up := rewrittenPage(`"upstream-1"`, "<html></html>", appendText("<button/>")); up.StatusCode != http.StatusOK {
		t.Fatalf("the upstream's tag was honoured: %d", up.StatusCode)
	}
}

// Two filters rewriting in turn: the tag a browser holds is the LAST pass's,
// and it revalidates; an earlier pass never answers for bytes it did not
// produce.
func TestTwoRewritesRevalidateOnTheFinalBytes(t *testing.T) {
	first := rewrittenPage("", "<html></html>", appendText("<a/>"), appendText("<b/>"))
	etag := first.Header.Get("ETag")
	again := rewrittenPage(etag, "<html></html>", appendText("<a/>"), appendText("<b/>"))
	if again.StatusCode != http.StatusNotModified {
		t.Fatalf("two passes, same bytes: %d", again.StatusCode)
	}
	// A later pass that changes nothing: the earlier pass's tag is the final one.
	noop := func([]byte) []byte { return nil }
	one := rewrittenPage("", "<html></html>", appendText("<a/>"), noop)
	if r := rewrittenPage(one.Header.Get("ETag"), "<html></html>", appendText("<a/>"), noop); r.StatusCode != http.StatusNotModified {
		t.Fatalf("a no-op last pass: %d", r.StatusCode)
	}
}

// A page kept from every cache is not given a validator.
func TestAPersonalPageGetsNoValidator(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://app.example/", nil)
	res := &http.Response{StatusCode: http.StatusOK, Request: req,
		Header: http.Header{"Content-Type": {"text/html"}, "Cache-Control": {"no-store"}},
		Body:   io.NopCloser(strings.NewReader("<html></html>"))}
	if err := RewriteBody(res, appendText("<x/>")); err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("ETag") != "" {
		t.Fatalf("a no-store page got a validator: %q", res.Header.Get("ETag"))
	}
}
