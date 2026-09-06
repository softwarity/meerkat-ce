package routing

import (
	"strings"
	"testing"
)

// The matcher was rewritten to stop allocating (see pathPattern.match), and
// this is what says the rewrite changed nothing else. The previous
// implementation is kept as the ORACLE: a table of expected answers would have
// been written by the same person who wrote the new code, and would have
// agreed with it for the same wrong reason. Two shapes DID diverge on the
// first attempt - "///" and "/a//" against "/{x}" - and neither was in the
// hand-written list above.
func matchBySplitting(p pathPattern, path string) bool {
	segs := splitPath(path)
	if p.tail {
		if len(segs) < len(p.segments) {
			return false
		}
	} else if len(segs) != len(p.segments) {
		return false
	}
	for i, want := range p.segments {
		if strings.HasPrefix(want, "{") {
			continue
		}
		if segs[i] != want {
			return false
		}
	}
	return true
}

func TestMatchingWithoutSplittingAgrees(t *testing.T) {
	patterns := []string{
		"/", "/**", "/a", "/a/", "/a/**", "/a/b", "/a/{id}", "/a/{id}/b",
		"/a/{id}/**", "/{x}", "/{x}/**", "/a/b/c",
	}
	paths := []string{
		"", "/", "//", "///", "/a", "/a/", "//a", "/a//", "/a/b", "/a//b",
		"/a/b/", "/a/b/c", "/a/b/c/d", "/ab", "/demolition", "/a/b//c",
		"/{id}", "/a/%20", "/A", "/a/B/c",
	}
	// And every path of up to four segments over a small alphabet, empty
	// segments included: the two divergences this caught were both there.
	alphabet := []string{"", "a", "b", "c"}
	for n := 0; n <= 4; n++ {
		var build func(prefix []string)
		build = func(prefix []string) {
			if len(prefix) == n {
				paths = append(paths, "/"+strings.Join(prefix, "/"), "/"+strings.Join(prefix, "/")+"/")
				return
			}
			for _, a := range alphabet {
				build(append(prefix, a))
			}
		}
		build(nil)
	}
	for _, raw := range patterns {
		p, err := compilePathPattern(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		for _, path := range paths {
			if got, want := p.match(path), matchBySplitting(p, path); got != want {
				t.Errorf("pattern %q, path %q: walking says %v, splitting says %v", raw, path, got, want)
			}
		}
	}
}

// stripBySplitting is what StripSegments used to be, kept as the ORACLE for
// the same reason matchBySplitting is: a table of expected answers would have
// been written by whoever wrote the new walk, and would have agreed with it
// wherever the walk was wrong. This function runs on the way to every upstream
// that carries a strip-prefix, so "it looks right" is not a standard.
func stripBySplitting(path string, n int) string {
	segs := splitPath(path)
	if n >= len(segs) {
		return "/"
	}
	return "/" + strings.Join(segs[n:], "/")
}

func TestStripSegmentsMatchesSplitting(t *testing.T) {
	paths := []string{
		"", "/", "//", "///", "/a", "/a/", "//a", "/a//", "/a/b", "/a//b",
		"/a/b/", "/a/b/c", "/a/b/c/d", "/ab", "/demolition", "/a/b//c",
		"a/b", "a", "/a/%20", "/A/b",
	}
	alphabet := []string{"", "a", "b"}
	for n := 0; n <= 4; n++ {
		var build func(prefix []string)
		build = func(prefix []string) {
			if len(prefix) == n {
				joined := strings.Join(prefix, "/")
				paths = append(paths, "/"+joined, "/"+joined+"/", joined)
				return
			}
			for _, a := range alphabet {
				build(append(prefix, a))
			}
		}
		build(nil)
	}
	// From zero: splitting PANICKED on a negative count (segs[n:]), so there
	// is no answer there to compare against. The walk simply strips nothing,
	// and no caller can reach it - stripPrefixCount never returns below zero.
	for _, path := range paths {
		for n := 0; n <= 5; n++ {
			if got, want := StripSegments(path, n), stripBySplitting(path, n); got != want {
				t.Errorf("path %q, n=%d: walking says %q, splitting says %q", path, n, got, want)
			}
		}
	}
}

// And that it costs nothing, which is why it was rewritten.
func TestStripSegmentsDoesNotAllocate(t *testing.T) {
	if n := testing.AllocsPerRun(200, func() {
		_ = StripSegments("/orders/12345/items", 1)
		_ = StripSegments("/anything/whatever", 0)
	}); n != 0 {
		t.Errorf("StripSegments allocated %v times per run, want 0", n)
	}
}
