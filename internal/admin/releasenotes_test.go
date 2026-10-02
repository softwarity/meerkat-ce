package admin

import (
	"strings"
	"testing"
)

const sampleNotes = `# Release Notes

## NEXT RELEASE

- **Coming.** Not yet stamped.

---

<!-- for the writers -->

## 1.0.1

- A fix.

---

## 1.0.0

First public release.

### Routing

- **Routes.**

---
`

// The version in the account menu opens the notes newest first, as the file
// has them: a release from its own section down to its minor, never NEXT
// RELEASE; a release stamped on an image built before its number was chosen
// reads its own notes under NEXT RELEASE; a development build is shown as the
// last release, with NEXT RELEASE on top. Empty sections are kept.
func TestTheNotesAreReadNewestFirstDownToTheMinor(t *testing.T) {
	titles := func(n releaseNotes) string {
		var out []string
		for _, p := range n.Parts {
			out = append(out, p.Title)
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct {
		version, shown string
		dev            bool
		parts          string
	}{
		{"1.0.0", "1.0.0", false, "1.0.0"},
		{"v1.0.1", "1.0.1", false, "1.0.1,1.0.0"},
		{"1.0.2", "1.0.2", false, "1.0.2,1.0.1,1.0.0"},
		{"1.1.0", "1.1.0", false, "1.1.0"},
		{"dev", "1.0.1", true, "NEXT RELEASE,1.0.1,1.0.0"},
		{"ee-1002-035328", "1.0.1", true, "NEXT RELEASE,1.0.1,1.0.0"},
	} {
		n := notesFor(c.version, sampleNotes)
		if n.Version != c.shown || n.Dev != c.dev || titles(n) != c.parts {
			t.Errorf("%s: shown %q dev %v parts %q, want %q %v %q", c.version, n.Version, n.Dev, titles(n), c.shown, c.dev, c.parts)
		}
	}
	// Stamped: 1.0.2 and 1.1.0 are not in the file, their notes are NEXT
	// RELEASE's - under their own number.
	if n := notesFor("1.1.0", sampleNotes); n.Parts[0].Markdown != "- **Coming.** Not yet stamped." {
		t.Errorf("a freshly stamped release reads NEXT RELEASE, got %q", n.Parts[0].Markdown)
	}
	n := notesFor("1.0.0", sampleNotes)
	if want := "First public release.\n\n### Routing\n\n- **Routes.**"; n.Parts[0].Markdown != want {
		t.Errorf("the section carried its separator or a comment: %q", n.Parts[0].Markdown)
	}
	empty := "# Release Notes\n\n## NEXT RELEASE\n\n---\n\n## 1.0.1\n\n---\n\n## 1.0.0\n\nFirst.\n"
	if n := notesFor("dev", empty); titles(n) != "NEXT RELEASE,1.0.1,1.0.0" || n.Parts[0].Markdown != "" || n.Parts[1].Markdown != "" {
		t.Errorf("an empty section is kept, for the console to say so: %+v", n)
	}
}
