package gateway

import (
	"net/http"
	"strings"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/routing"
)

// rootPathsFilters are the response filters of a route whose application
// writes its addresses from the root (store.RouteUI.RootPaths). The prefix is
// the one THIS request lost to strip-prefix, never a value typed on the route:
// a route matching /geoserver/{ws}/** strips a different one on each request,
// and a typed copy would drift the day the predicate changes. A request that
// lost none leaves everything as it came.
func rootPathsFilters() []routing.ResponseFilter {
	return []routing.ResponseFilter{
		// The page: its links, then the script that fixes its calls, right
		// after <head> so it runs before the application's own scripts.
		filtering.RewriteHTMLFunc(
			func(res *http.Response) bool { return routing.StrippedPrefix(res.Request) != "" },
			func(res *http.Response, body []byte) []byte {
				prefix := routing.StrippedPrefix(res.Request)
				if out := routing.PublishRootPaths(body, prefix); out != nil {
					body = out
				}
				return filtering.AfterHead(res, body, []byte(routing.RootPathsScript(prefix)))
			}),
		// Its stylesheets.
		func(res *http.Response) error {
			prefix := routing.StrippedPrefix(res.Request)
			if prefix == "" || !strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")), "text/css") {
				return nil
			}
			return filtering.RewriteBody(res, func(body []byte) []byte {
				return routing.PublishCSSPaths(body, prefix)
			})
		},
	}
}
