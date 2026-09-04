package config

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// A configuration with its media (CFG-05).
//
// Two kinds travel: the branding's pictures - the logo, the pages background,
// the tab icon - which live in a setting as data URIs, and the OpenAPI specs
// deposited on routes (SVC-06), which live in a table of their own. Left
// inline they are base64 lines, or megabytes of contract, in the middle of a
// file people read and diff - the surest way to make them stop opening it.
//
// So a document with a media travels as a ZIP: the YAML, and the files next
// to it. What matters is that BOTH forms stay self-contained - the plain YAML
// keeps its data URI, and the assets/ path only ever exists inside a package
// that carries the file it names. A dangling reference cannot be produced by
// either. Someone who unzips and imports the YAML alone gets the rule that
// governs everything else here: what the file does not carry, it does not
// destroy - the logo in place stays, and the import says so.

// BundleName is the document's name inside a package.
const BundleName = "meerkat.yaml"

// assetDir holds the pictures a package carries.
const assetDir = "assets"

// dataURIPrefixes are the image forms the branding accepts, mapped to the file
// extension they get inside a package.
var dataURIPrefixes = map[string]string{
	"data:image/png;base64,":     ".png",
	"data:image/jpeg;base64,":    ".jpg",
	"data:image/webp;base64,":    ".webp",
	"data:image/svg+xml;base64,": ".svg",
}

// imageFields are the branding entries that travel as files, with the name
// each takes inside a package. One list, walked by both directions: a picture
// added to the branding becomes a file here and nowhere else.
var imageFields = []struct {
	path []string // where it sits in the branding object
	name string   // its base name inside assets/
}{
	{[]string{"logo"}, "logo"},
	{[]string{"background", "image"}, "background"},
	{[]string{"favicon"}, "favicon"},
}

// HasImage reports whether doc carries a picture, which is what decides between
// a plain file and a package: a ZIP holding nothing but one YAML would be a
// wrapper nobody asked for.
func HasImage(doc *Document) bool { return imageBytes(doc) > 0 || len(doc.Specs) > 0 }

// specDir holds the OpenAPI files a package carries, one directory per route:
// two routes may both have deposited an "openapi.yaml", and the branding's
// three pictures could take fixed names only because there is one branding.
const specDir = "assets/specs"

// specAssets lists the deposited specs as files inside a package, and returns
// the routes they belong to. The route ID names the directory - a name can be
// edited between an export and the import that reads it, an ID cannot - and
// the file keeps the name it was deposited under, which is what makes an
// unzipped package readable.
func specAssets(doc *Document) map[string][]byte {
	out := map[string][]byte{}
	for _, r := range doc.Routes {
		content, ok := doc.Specs[r.ID]
		decl := r.Spec()
		if !ok || len(content) == 0 || decl.Type != store.SpecFile || decl.Filename == "" {
			continue
		}
		out[path.Join(specDir, r.ID, decl.Filename)] = content
	}
	return out
}

// MarshalBundle renders doc as a ZIP: the YAML with each picture replaced by a
// relative path, and the pictures as files beside it.
func MarshalBundle(doc *Document) ([]byte, error) {
	branding, err := brandingMap(doc)
	if err != nil {
		return nil, err
	}
	type asset struct {
		name string
		body []byte
	}
	var assets []asset
	for _, f := range imageFields {
		uri, _ := getPath(branding, f.path).(string)
		content, ext := decodeDataURI(uri)
		if content == nil {
			continue
		}
		name := path.Join(assetDir, f.name+ext)
		assets = append(assets, asset{name, content})
		setPath(branding, f.path, name)
	}
	for name, content := range specAssets(doc) {
		assets = append(assets, asset{name, content})
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].name < assets[j].name })
	if len(assets) == 0 {
		// Nothing to extract: a package would add a layer for no reason.
		return nil, fmt.Errorf("config: this configuration carries no image")
	}
	// The document is copied before being rewritten: the caller's own document
	// must not come back with paths where its images used to be.
	rewritten, err := withBranding(doc, branding)
	if err != nil {
		return nil, err
	}
	body, err := Marshal(rewritten)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range append([]asset{{BundleName, body}}, assets...) {
		w, err := zw.Create(entry.name)
		if err != nil {
			return nil, fmt.Errorf("config: package %s: %w", entry.name, err)
		}
		if _, err := w.Write(entry.body); err != nil {
			return nil, fmt.Errorf("config: package %s: %w", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("config: package: %w", err)
	}
	return buf.Bytes(), nil
}

// WithoutImages returns doc with its pictures taken OUT of the branding - what
// the console shows when a configuration is read or edited as text.
//
// A logo is a megabyte of base64 on one line, in the middle of a document
// someone is trying to read. Worse, it invites editing: a stray keystroke in
// that line is an image nobody can recover from a diff. So the rule the console
// states out loud is that MEDIA ARE NOT DEFINED BY EDITING - they are set on
// the Branding screen and travel in a package - and this is what makes the rule
// true rather than a sentence.
//
// The counterpart lives in the import: a document that carries no picture keeps
// the one in place (see importSettings). Without it, saving an edited document
// would silently erase the logo it never showed.
func WithoutImages(doc *Document) (*Document, error) {
	branding, err := brandingMap(doc)
	if err != nil {
		return nil, err
	}
	if branding == nil {
		return doc, nil
	}
	dropped := false
	for _, f := range imageFields {
		if uri, _ := getPath(branding, f.path).(string); strings.HasPrefix(uri, "data:") {
			setPath(branding, f.path, "")
			dropped = true
		}
	}
	if !dropped {
		return doc, nil
	}
	return withBranding(doc, branding)
}

// keepImages fills the picture fields of `in` from `stored` wherever `in` has
// none - the import-side half of WithoutImages.
func keepImages(in, stored map[string]any) map[string]any {
	if in == nil || stored == nil {
		return in
	}
	for _, f := range imageFields {
		if uri, _ := getPath(in, f.path).(string); uri != "" {
			continue
		}
		if was, _ := getPath(stored, f.path).(string); was != "" {
			setPath(in, f.path, was)
		}
	}
	return in
}

// IsBundle reports whether body is a ZIP, by its magic number rather than by
// its name: what an admin uploads has been through a browser, a chat and a
// download folder, and the extension is the first thing to be lost.
func IsBundle(body []byte) bool {
	return len(body) > 4 && body[0] == 'P' && body[1] == 'K' &&
		(body[2] == 3 || body[2] == 5 || body[2] == 7)
}

// UnmarshalBundle reads a package: the document, with the pictures it references
// put back inline.
func UnmarshalBundle(body []byte) (*Document, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("config: this package cannot be opened: %w", err)
	}
	files := map[string][]byte{}
	var doc []byte
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// A name climbing out of the archive is refused rather than cleaned:
		// nothing legitimate produces one, and the tidy version of a hostile
		// path is still a hostile intent.
		name := f.Name
		if path.IsAbs(name) || strings.Contains(name, "..") {
			return nil, fmt.Errorf("config: the package holds an unusable path %q", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("config: package %s: %w", name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxAsset))
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("config: package %s: %w", name, err)
		}
		if path.Base(name) == BundleName && path.Dir(name) == "." {
			doc = content
			continue
		}
		files[name] = content
	}
	if doc == nil {
		return nil, fmt.Errorf("config: this package holds no %s", BundleName)
	}
	parsed, err := Unmarshal(doc)
	if err != nil {
		return nil, err
	}
	// The deposited specs ride ALONGSIDE the document rather than inside it:
	// the route names the file, the package holds it, and an import that got
	// only the YAML simply has none (importRoutes says so and destroys
	// nothing).
	for _, r := range parsed.Routes {
		decl := r.Spec()
		if decl.Type != store.SpecFile || decl.Filename == "" {
			continue
		}
		if content, ok := files[path.Join(specDir, r.ID, decl.Filename)]; ok {
			if parsed.Specs == nil {
				parsed.Specs = map[string][]byte{}
			}
			parsed.Specs[r.ID] = content
		}
	}
	return inlineAssets(parsed, files)
}

// maxAsset bounds one file inside a package. It is sized on the biggest media a
// configuration may carry, which is no longer a picture but a deposited OpenAPI
// spec (the store refuses one over 4 MiB); the branding refuses a background
// over ~1 400 000 characters of its own. This stops a package from being read
// into memory before either refusal can happen.
const maxAsset = 4 << 20

// inlineAssets puts the pictures back where the document points at them.
func inlineAssets(doc *Document, files map[string][]byte) (*Document, error) {
	branding, err := brandingMap(doc)
	if err != nil || branding == nil {
		return doc, err
	}
	var touched bool
	for _, f := range imageFields {
		name, _ := getPath(branding, f.path).(string)
		if name == "" || strings.HasPrefix(name, "data:") {
			continue // inline already, or nothing there
		}
		content, ok := files[name]
		if !ok {
			// The document names a picture the package does not hold. Refused
			// rather than dropped: an import that silently loses the logo is one
			// nobody notices until a user does.
			return nil, fmt.Errorf("config: the configuration points at %s, which is not in the package", name)
		}
		prefix := ""
		for p, ext := range dataURIPrefixes {
			if strings.EqualFold(path.Ext(name), ext) {
				prefix = p
				break
			}
		}
		if prefix == "" {
			return nil, fmt.Errorf("config: %s is not an image Meerkat can wear (png, jpeg, webp, svg)", name)
		}
		setPath(branding, f.path, prefix+base64.StdEncoding.EncodeToString(content))
		touched = true
	}
	if !touched {
		return doc, nil
	}
	encoded, err := json.Marshal(branding)
	if err != nil {
		return nil, fmt.Errorf("config: branding: %w", err)
	}
	doc.Settings[store.SettingBranding] = encoded
	return doc, nil
}

// brandingMap decodes the branding setting, or returns nil when the document
// carries none.
func brandingMap(doc *Document) (map[string]any, error) {
	raw, ok := doc.Settings[store.SettingBranding]
	if !ok {
		return nil, nil
	}
	var branding map[string]any
	if err := json.Unmarshal(raw, &branding); err != nil {
		return nil, fmt.Errorf("config: branding: %w", err)
	}
	return branding, nil
}

// withBranding copies doc with a rewritten branding setting - the caller's own
// document keeps its images.
func withBranding(doc *Document, branding map[string]any) (*Document, error) {
	encoded, err := json.Marshal(branding)
	if err != nil {
		return nil, fmt.Errorf("config: branding: %w", err)
	}
	// A shallow copy is enough: only the settings map is rewritten, and it gets
	// a fresh one.
	out := *doc
	out.Settings = make(map[string]json.RawMessage, len(doc.Settings))
	for k, v := range doc.Settings {
		out.Settings[k] = v
	}
	out.Settings[store.SettingBranding] = encoded
	return &out, nil
}

// decodeDataURI returns the bytes an image data URI holds and the extension it
// takes inside a package - nil when the value is not one of the accepted forms.
func decodeDataURI(uri string) ([]byte, string) {
	for prefix, ext := range dataURIPrefixes {
		if !strings.HasPrefix(uri, prefix) {
			continue
		}
		content, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
		if err != nil {
			return nil, ""
		}
		return content, ext
	}
	return nil, ""
}

// getPath / setPath walk the branding object: the background is nested one
// level down, everything else sits at the top, and one list describes both.
func getPath(m map[string]any, keys []string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

func setPath(m map[string]any, keys []string, value string) {
	cur := m
	for _, k := range keys[:len(keys)-1] {
		next, ok := cur[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[k] = next
		}
		cur = next
	}
	cur[keys[len(keys)-1]] = value
}
