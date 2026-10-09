package gateway

import (
	"bytes"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A UI route's own CSS and JavaScript (store.Injection), turned into what the
// page receives: one fragment per place, the blocks in the route's order, and
// a file linked from RouteAssetPath rather than pasted in - the browser
// caches it, and a page does not carry the same 200 KB on every load.

// injectionFilters are the response filters writing a route's blocks. A block
// naming a file the route does not have is left out: the route still serves,
// and the console lists the file as missing.
func injectionFilters(routeID string, list []store.Injection, files []store.RouteFile) []routing.ResponseFilter {
	byName := make(map[string]store.RouteFile, len(files))
	for _, f := range files {
		byName[f.Name] = f
	}
	frags := map[string]*bytes.Buffer{}
	for _, in := range list {
		tag := injectionTag(routeID, in, byName)
		if tag == "" {
			continue
		}
		b := frags[in.Position]
		if b == nil {
			b = &bytes.Buffer{}
			frags[in.Position] = b
		}
		b.WriteString(tag)
	}
	var out []routing.ResponseFilter
	if b := frags[store.InjectHeadStart]; b != nil {
		out = append(out, filtering.InjectAfterHead(b.String()))
	}
	if b := frags[store.InjectHeadEnd]; b != nil {
		out = append(out, filtering.InjectBeforeHeadEnd(b.String()))
	}
	if b := frags[store.InjectBodyEnd]; b != nil {
		out = append(out, filtering.InjectAtBodyEnd(b.String()))
	}
	return out
}

func injectionTag(routeID string, in store.Injection, files map[string]store.RouteFile) string {
	if in.File == "" {
		if strings.TrimSpace(in.Code) == "" {
			return ""
		}
		if in.Kind == store.InjectCSS {
			return "<style>\n" + in.Code + "\n</style>\n"
		}
		open := "<script>"
		if in.Load == store.LoadModule {
			open = `<script type="module">`
		}
		return open + "\n" + in.Code + "\n</script>\n"
	}
	f, ok := files[in.File]
	if !ok {
		return ""
	}
	src := html.EscapeString(routeAssetURL(routeID, f))
	if in.Kind == store.InjectCSS {
		return `<link rel="stylesheet" href="` + src + `">` + "\n"
	}
	attr := ""
	switch in.Load {
	case store.LoadDefer:
		attr = " defer"
	case store.LoadAsync:
		attr = " async"
	case store.LoadModule:
		attr = ` type="module"`
	}
	return `<script src="` + src + `"` + attr + "></script>\n"
}

// routeAssetURL names a file by its content as well as its name: the version
// changes with the bytes, so the file can be cached for good.
func routeAssetURL(routeID string, f store.RouteFile) string {
	segs := strings.Split(f.Name, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	v := f.SHA
	if len(v) > 12 {
		v = v[:12]
	}
	return store.RouteAssetPath + url.PathEscape(routeID) + "/" + strings.Join(segs, "/") + "?v=" + v
}

// injectedFiles is what RouteAssetPath may serve for a route: the files its
// blocks name, and only those - a file uploaded on a route is not public
// because it exists.
func injectedFiles(r store.Route, files []store.RouteFile) map[string]store.RouteFile {
	if !r.IsUI || r.UI == nil {
		return nil
	}
	named := map[string]bool{}
	for _, in := range r.UI.Injections {
		if in.File != "" {
			named[in.File] = true
		}
	}
	var out map[string]store.RouteFile
	for _, f := range files {
		if named[f.Name] {
			if out == nil {
				out = map[string]store.RouteFile{}
			}
			out[f.Name] = f
		}
	}
	return out
}

// serveRouteAsset answers RouteAssetPath<route id>/<name>. Public like the
// page that loads it, cached for a year when the version matches - the URL
// changes with the content - and revalidated by its ETag otherwise.
func (rt *Router) serveRouteAsset(w http.ResponseWriter, req *http.Request) {
	rest := strings.TrimPrefix(req.URL.Path, store.RouteAssetPath)
	routeID, name, _ := strings.Cut(rest, "/")
	rt.mu.RLock()
	f, ok := rt.assets[routeID][name]
	rt.mu.RUnlock()
	if !ok {
		http.NotFound(w, req)
		return
	}
	h := w.Header()
	h.Set("Content-Type", f.ContentType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("ETag", `"`+f.SHA+`"`)
	if v := req.URL.Query().Get("v"); v != "" && strings.HasPrefix(f.SHA, v) {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "public, max-age=0, must-revalidate")
	}
	http.ServeContent(w, req, "", time.Unix(f.UpdatedAt, 0), bytes.NewReader(f.Data))
}
