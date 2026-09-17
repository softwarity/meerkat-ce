// Package icons is the embedded Material Symbols catalogue (PORTAL-01): the
// SVGs the portal's icon picker searches, and the resolver that turns a chosen
// name into the SVG stored on a module. The data is generated from the
// @material-symbols/svg-400 npm package by console/scripts/build-icon-bank.mjs
// (`npm run icons:build`) into bank.json - nothing is fetched at runtime, and
// only viewBox + path survive normalization (no size, fill, script or handler),
// so a stored icon is safe to render as a CSS mask on any page.
package icons

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

//go:embed bank.json
var bankJSON []byte

// Icon is one catalogue entry: a Material Symbols name and its SVG.
type Icon struct {
	Name string `json:"name"`
	SVG  string `json:"svg"`
}

var (
	bank []Icon
	byNm map[string]string
)

func init() {
	_ = json.Unmarshal(bankJSON, &bank)
	byNm = make(map[string]string, len(bank))
	for _, i := range bank {
		byNm[i.Name] = i.SVG
	}
}

// Count is how many icons the catalogue holds (for tests and diagnostics).
func Count() int { return len(bank) }

// Lookup returns the SVG for a bare Material Symbols name.
func Lookup(name string) (string, bool) {
	svg, ok := byNm[name]
	return svg, ok
}

// Search returns up to limit icons whose name contains every whitespace- or
// underscore-separated token of the query (empty query returns the first
// `limit`). A name that STARTS with the query sorts first, so an exact-ish
// match leads the grid.
func Search(query string, limit int) []Icon {
	if limit <= 0 {
		limit = 120
	}
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.FieldsFunc(q, func(r rune) bool { return r == ' ' || r == '_' })

	out := make([]Icon, 0, limit)
	if len(tokens) == 0 {
		for i := 0; i < len(bank) && len(out) < limit; i++ {
			out = append(out, bank[i])
		}
		return out
	}

	type hit struct {
		icon   Icon
		prefix bool
	}
	var hits []hit
	for _, ic := range bank {
		name := ic.Name
		matched := true
		for _, t := range tokens {
			if !strings.Contains(name, t) {
				matched = false
				break
			}
		}
		if matched {
			hits = append(hits, hit{icon: ic, prefix: strings.HasPrefix(name, tokens[0])})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].prefix != hits[b].prefix {
			return hits[a].prefix // prefix matches first
		}
		return hits[a].icon.Name < hits[b].icon.Name
	})
	for i := 0; i < len(hits) && len(out) < limit; i++ {
		out = append(out, hits[i].icon)
	}
	return out
}

var (
	reViewBox = regexp.MustCompile(`viewBox="([^"]+)"`)
	rePathD   = regexp.MustCompile(`<path\b[^>]*\sd="([^"]+)"`)
	reName    = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// Resolve turns a module's stored icon field into a safe SVG to serve, or ""
// when it is neither a known name nor a usable SVG:
//
//   - a bare Material Symbols name -> its catalogue SVG (this is the
//     "resolve the name to an SVG when saved" path);
//   - an SVG (pasted by hand, or already resolved) -> sanitized to viewBox +
//     path only, so no script, handler or external reference survives;
//   - anything else -> "".
//
// It is idempotent: a value it produced passes through unchanged.
func Resolve(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	if reName.MatchString(field) {
		if svg, ok := byNm[field]; ok {
			return svg
		}
		return ""
	}
	return Sanitize(field)
}

// Sanitize keeps only the viewBox and the path geometry of an SVG, rebuilding a
// minimal <svg> - the one shape a mask needs and the one that carries no active
// content. It returns "" for markup with no viewBox or no path.
func Sanitize(svg string) string {
	vb := reViewBox.FindStringSubmatch(svg)
	if vb == nil {
		return ""
	}
	paths := rePathD.FindAllStringSubmatch(svg, -1)
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="`)
	b.WriteString(vb[1])
	b.WriteString(`">`)
	for _, p := range paths {
		b.WriteString(`<path d="`)
		b.WriteString(p[1])
		b.WriteString(`"/>`)
	}
	b.WriteString(`</svg>`)
	return b.String()
}
