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
