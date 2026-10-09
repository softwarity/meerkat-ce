// Package fonts is the typefaces the gateway ships for its built-in pages
// (THEME-09), embedded so that it serves them itself: a visitor never
// contacts a font CDN, an offline installation has them, and a page's own
// Content-Security-Policy ('self') lets them through.
//
// The files are written once by tools/fonts/fetch.mjs and committed. Each
// family is the variable version on its weight axis, cut into the Unicode
// ranges Google cuts it into and kept only for the scripts Meerkat's twenty
// languages need. A family the integrator CHOOSES draws Latin (with Polish,
// Turkish and Vietnamese) and Cyrillic; four Noto COMPANIONS draw Arabic,
// Hebrew, Devanagari and Thai behind any choice; Chinese, Japanese and
// Korean are left to the system's own fonts. The browser assembles them
// character by character, and downloads a file only when a page holds a
// character of its range - a French page costs one Latin file.
package fonts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"strings"
)

//go:embed catalogue.json files/*.woff2 licenses/*.txt
var assets embed.FS

// Kinds: what a family may be chosen for. A companion is never chosen.
const (
	KindSans      = "sans"
	KindSerif     = "serif"
	KindMono      = "mono"
	KindCompanion = "companion"
)

// Face is one file of a family: the subset it draws and the characters.
type Face struct {
	Subset       string `json:"subset"`
	File         string `json:"file"`
	UnicodeRange string `json:"unicodeRange"`
}

// Family is a typeface the gateway serves.
type Family struct {
	Family   string     `json:"family"`
	Slug     string     `json:"slug"`
	Kind     string     `json:"kind"`
	Category string     `json:"category"`
	Weight   [2]float64 `json:"weight"`
	Faces    []Face     `json:"faces"`
}

var (
	catalogue []Family
	// version is the short content hash of each file, carried in its URL so a
	// file the fetch replaces is never served from a year-long cache.
	version = map[string]string{}
)

func init() {
	raw, err := assets.ReadFile("catalogue.json")
	if err != nil {
		panic("fonts: catalogue: " + err.Error())
	}
	if err := json.Unmarshal(raw, &catalogue); err != nil {
		panic("fonts: catalogue: " + err.Error())
	}
	for _, f := range catalogue {
		for _, face := range f.Faces {
			data, err := assets.ReadFile("files/" + face.File)
			if err != nil {
				panic("fonts: " + face.File + ": " + err.Error())
			}
			sum := sha256.Sum256(data)
			version[face.File] = hex.EncodeToString(sum[:])[:10]
		}
	}
}

// Families are the families an integrator may choose, in catalogue order -
// the companions are not among them.
func Families() []Family {
	out := make([]Family, 0, len(catalogue))
	for _, f := range catalogue {
		if f.Kind != KindCompanion {
			out = append(out, f)
		}
	}
	return out
}

// Lookup finds a family by its name.
func Lookup(name string) (Family, bool) {
	for _, f := range catalogue {
		if f.Family == name {
			return f, true
		}
	}
	return Family{}, false
}

// Check says whether name may fill a slot that takes these kinds, and names
// the families that may when it may not. An empty name is "the system's".
func Check(slot, name string, kinds ...string) error {
	if name == "" {
		return nil
	}
	if f, ok := Lookup(name); ok && slices.Contains(kinds, f.Kind) {
		return nil
	}
	var allowed []string
	for _, f := range Families() {
		if slices.Contains(kinds, f.Kind) {
			allowed = append(allowed, f.Family)
		}
	}
	return fmt.Errorf("font %s %q: allowed are %s, or none for the system's", slot, name, strings.Join(allowed, ", "))
}

// system is what follows the families in every stack: the platform's own
// font, which is also what draws Chinese, Japanese and Korean.
var system = map[string]string{
	KindSans:  `system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif`,
	KindSerif: `ui-serif, Georgia, 'Times New Roman', serif`,
	KindMono:  `ui-monospace, 'SF Mono', Menlo, Consolas, monospace`,
}

// Stack is the font-family value for a chosen family: it, then the
// companions for the scripts it does not draw, then the system's.
func Stack(name string) string {
	f, ok := Lookup(name)
	if !ok {
		return system[KindSans]
	}
	parts := []string{quote(f.Family)}
	if f.Kind != KindMono {
		for _, c := range catalogue {
			if c.Kind == KindCompanion {
				parts = append(parts, quote(c.Family))
			}
		}
	}
	return strings.Join(parts, ", ") + ", " + system[f.Kind]
}

func quote(s string) string { return "'" + s + "'" }

// FaceCSS is the @font-face rules for these families and, for any that is
// not monospace, the companions. Nothing is downloaded by declaring a face:
// a browser fetches a file only for a character of its range that a page
// draws in that family.
func FaceCSS(names ...string) string {
	want := map[string]bool{}
	companions := false
	for _, n := range names {
		if f, ok := Lookup(n); ok {
			want[f.Family] = true
			companions = companions || f.Kind != KindMono
		}
	}
	var b strings.Builder
	for _, f := range catalogue {
		if !want[f.Family] && (f.Kind != KindCompanion || !companions) {
			continue
		}
		for _, face := range f.Faces {
			fmt.Fprintf(&b, "@font-face { font-family: %s; font-style: normal; font-weight: %g %g; font-display: swap; "+
				"src: url(%s) format('woff2'); unicode-range: %s; }\n",
				quote(f.Family), f.Weight[0], f.Weight[1], URL(face.File), face.UnicodeRange)
		}
	}
	return b.String()
}

// Prefix is where the files are served, on both planes.
const Prefix = "/meerkat/fonts/"

// URL is where a file is served, its content hash in the query.
func URL(file string) string { return Prefix + file + "?v=" + version[file] }

// Handler serves the files and their licences. A file is immutable under its
// versioned URL, and readable from any origin: a page of a proxied
// application is not the gateway's origin when it is reached on another host.
func Handler() http.Handler {
	files, _ := fs.Sub(assets, ".")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, Prefix)
		var p, ctype string
		switch {
		case strings.HasPrefix(name, "licenses/") && path.Ext(name) == ".txt":
			p, ctype = name, "text/plain; charset=utf-8"
		case path.Ext(name) == ".woff2" && !strings.Contains(name, "/"):
			p, ctype = "files/"+name, "font/woff2"
		default:
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(files, p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(data)
	})
}
