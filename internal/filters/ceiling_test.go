package filters

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A response over the ceiling is FORWARDED, not refused: the ceiling is the
// promise that the gateway's memory does not follow the size of what it
// proxies, and a filter that cannot do its job on a 200 MB download must hand
// the download over rather than break it.
func answer(t *testing.T, size int) *http.Response {
	t.Helper()
	body := bytes.Repeat([]byte("a"), size)
	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"text/plain"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(size),
	}
	return res
}

func TestTheCeilingMovesWithTheSetting(t *testing.T) {
	t.Cleanup(func() { SetMaxRewritableBody(0) })

	// Default first: nothing wired, the value this product always had.
	SetMaxRewritableBody(0)
	if got := MaxRewritableBody(); got != DefaultMaxRewritableBody {
		t.Fatalf("unset ceiling = %d, want the default %d", got, DefaultMaxRewritableBody)
	}

	SetMaxRewritableBody(1 << 10) // 1 KiB
	if Rewritable(answer(t, 2<<10)) {
		t.Error("a 2 KiB body was rewritable under a 1 KiB ceiling")
	}
	if !Rewritable(answer(t, 512)) {
		t.Error("a 512 byte body was refused under a 1 KiB ceiling")
	}

	// Raise it and the same body becomes rewritable - which is the whole point
	// of the setting: it was a constant, and an installation answering big
	// documents had no way to say so.
	SetMaxRewritableBody(4 << 10)
	if !Rewritable(answer(t, 2<<10)) {
		t.Error("a 2 KiB body is still refused after the ceiling was raised to 4 KiB")
	}
}

// The ceiling is also enforced on a body whose length was NOT declared -
// chunked answers arrive with ContentLength -1, so the guard before the read
// cannot see them and the one after the read is what stops them.
func TestAnUndeclaredBodyIsStoppedByTheReadItself(t *testing.T) {
	t.Cleanup(func() { SetMaxRewritableBody(0) })
	SetMaxRewritableBody(1 << 10)

	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"text/plain"}},
		Body:          io.NopCloser(strings.NewReader(strings.Repeat("a", 4<<10))),
		ContentLength: -1,
	}
	touched := false
	if err := RewriteBody(res, func(b []byte) []byte { touched = true; return b }); err != nil {
		t.Fatalf("an oversized body must pass through, not fail: %v", err)
	}
	if touched {
		t.Error("a body four times the ceiling was buffered and rewritten")
	}
	// And it is still the whole body that comes out.
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4<<10 {
		t.Errorf("the forwarded body is %d bytes, want %d: the guard ate part of it", len(out), 4<<10)
	}
}
