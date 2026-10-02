package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// The version under the account button, and what it brought (CONSOLE-15).
//
// What is shown is the notes of the MINOR or MAJOR release this version
// belongs to - 1.0.0 for 1.0.1 - and never a patch's: a patch fixes, and the
// reader clicking the number wants to know what the release they run can do.

func (a *API) registerReleaseNotes(mux Mux) {
	mux.Handle("GET /api/release-notes", a.authed(a.getReleaseNotes))
}

type releaseNotes struct {
	// Version is the number to show: this binary's, or for a development
	// build the last release in its notes - a build tag says nothing to the
	// person reading it. Dev marks a binary that is not a release.
	Version string `json:"version"`
	Dev     bool   `json:"dev,omitempty"`
	// Parts are the sections to read, newest first as the file has them: NEXT
	// RELEASE (a development build only), the patches, then the minor or major
	// release. An empty one is kept, and the console says it is empty.
	Parts []notesPart `json:"parts,omitempty"`
}

type notesPart struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

func (a *API) getReleaseNotes(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, notesFor(version.Version, a.ReleaseNotes))
}

// notesFor reads the notes file for version v: X.Y.Z shows X.Y.Z down to
// X.Y.0, newest first. When "## X.Y.0" is not in the file yet, v IS that
// release, stamped on an image built before its number was chosen, and its
// notes are still under NEXT RELEASE. A version that is not a number is a
// development build: it is shown as the last release in the file, with NEXT
// RELEASE on top. A release never shows NEXT RELEASE.
func notesFor(v, file string) releaseNotes {
	sections, numbered := splitNotes(file)
	out := releaseNotes{Version: strings.TrimPrefix(v, "v")}
	next, hasNext := sections[nextRelease]

	major, minor, patch, ok := semver(out.Version)
	if !ok {
		out.Dev = true
		out.Version = ""
		if hasNext {
			out.Parts = append(out.Parts, notesPart{Title: nextRelease, Markdown: next})
		}
		if len(numbered) == 0 {
			return out
		}
		out.Version = numbered[0]
		major, minor, patch, _ = semver(numbered[0])
	}
	for z := patch; z >= 0; z-- {
		name := strconv.Itoa(major) + "." + strconv.Itoa(minor) + "." + strconv.Itoa(z)
		body, found := sections[name]
		if !found && !out.Dev && z == patch && hasNext {
			// Stamped after it was built: its own notes are NEXT RELEASE's.
			body, found = next, true
		}
		if found {
			out.Parts = append(out.Parts, notesPart{Title: name, Markdown: body})
		}
	}
	return out
}

const nextRelease = "NEXT RELEASE"

// splitNotes cuts the file at its "## " headings, without the separators and
// the comments kept for the writers, and lists the numbered ones newest first.
func splitNotes(file string) (map[string]string, []string) {
	sections := map[string]string{}
	var numbered []string
	var name string
	var buf []string
	flush := func() {
		if name == "" {
			return
		}
		body := strings.TrimSpace(stripComments(strings.Join(buf, "\n")))
		sections[name] = strings.TrimSpace(strings.TrimSuffix(body, "---"))
		if _, _, _, ok := semver(name); ok {
			numbered = append(numbered, name)
		}
	}
	for _, line := range strings.Split(file, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			name, buf = strings.TrimSpace(h), nil
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return sections, numbered
}

func semver(v string) (major, minor, patch int, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

func stripComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+3:]
	}
}
