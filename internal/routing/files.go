package routing

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Serving uploaded files instead of proxying (ROUTE-22).
//
// The case it answers: a UI that needs a resource nothing behind the gateway
// serves - a font, a stylesheet, a script, an image - typically offline,
// where the CDN it names is out of reach. A route takes that path and serves
// files uploaded on it. The files are not arguments of the brick: they live
// beside the route (store.RouteFile) and the router hands them over when it
// compiles it, so the brick only says how they are served.

// FilesOptions are the brick's arguments, read by the router.
type FilesOptions struct {
	// Index is the file answered at the route's own path, for a route that
	// stands for a single document.
	Index string
	// CORS lets any origin read the files: a font or a module loaded from a
	// page of another origin is refused without it.
	CORS bool
	// MaxAge is how long a browser may keep a file without asking again;
	// after that it revalidates against the ETag.
	MaxAge int
}

func init() {
	registerFilter(filterDef{
		Type: "files", Phase: phaseTerminal,
		Doc: "Serves files uploaded on the route instead of proxying: the font, stylesheet, " +
			"script or image a UI asks for when nothing behind the gateway serves it - offline, " +
			"where the CDN it names is out of reach. A file is answered at the route's path " +
			"followed by its name.",
		Params: []Param{
			{Name: "index", Kind: KindString, Doc: "The file answered at the route's own path, if any."},
			{Name: "cors", Kind: KindBool, Default: true,
				Doc: "Let pages of any origin read the files - a font or a module loaded cross-origin needs it."},
			{Name: "maxAge", Kind: KindInt, Default: 3600,
				Doc: "Seconds a browser keeps a file before revalidating it against its ETag."},
		},
		compileTerminal: func(a decoded) (http.Handler, error) {
			if a.num("maxAge") < 0 {
				return nil, fmt.Errorf("files: maxAge must not be negative, got %d", a.num("maxAge"))
			}
			// The router replaces this with the route's own files. Reached only
			// when it could not - a route checked without its files.
			return http.NotFoundHandler(), nil
		},
	})
}

// FilesOf returns the arguments of the route's files brick, and false when it
// has none.
func FilesOf(specs []Spec) (FilesOptions, bool, error) {
	for _, s := range specs {
		if s.Type != "files" {
			continue
		}
		a, err := decodeArgs("filter files", filterRegistry["files"].Params, s.Args)
		if err != nil {
			return FilesOptions{}, true, err
		}
		return FilesOptions{Index: strings.TrimLeft(a.str("index"), "/"), CORS: a.boolean("cors"), MaxAge: a.num("maxAge")}, true, nil
	}
	return FilesOptions{}, false, nil
}

// ServedFile is a file as the handler serves it.
type ServedFile struct {
	Name, ContentType, SHA string
	Data                   []byte
	Modified               time.Time
}

// FilesHandler answers prefix + "/" + name with the file of that name, and
// the bare prefix with the index when there is one. Anything else is 404:
// a files route has nothing behind it to fall back on.
func FilesHandler(prefix string, files []ServedFile, opt FilesOptions) http.Handler {
	byName := make(map[string]ServedFile, len(files))
	for _, f := range files {
		byName[f.Name] = f
	}
	prefix = strings.TrimRight(prefix, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if opt.CORS && r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "a files route only reads", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")
		if name == "" {
			name = opt.Index
		}
		f, ok := byName[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", f.ContentType)
		h.Set("ETag", `"`+f.SHA+`"`)
		h.Set("Cache-Control", "public, max-age="+strconv.Itoa(opt.MaxAge))
		h.Set("X-Content-Type-Options", "nosniff")
		if opt.CORS {
			h.Set("Access-Control-Allow-Origin", "*")
		}
		http.ServeContent(w, r, f.Name, f.Modified, bytes.NewReader(f.Data))
	})
}
