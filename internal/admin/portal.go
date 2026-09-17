package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/softwarity/meerkat/internal/icons"
	"github.com/softwarity/meerkat/internal/store"
)

// portalIcons answers the portal icon picker's search (PORTAL-01): the embedded
// Material Symbols catalogue, filtered by the query, each entry carrying its SVG
// so the console renders the result grid without an icon font. It stores
// nothing and reads no gateway state - the catalogue is a build-time constant.
func (a *API) portalIcons(w http.ResponseWriter, r *http.Request, _ store.User) {
	limit := 120
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 400 {
			limit = n
		}
	}
	result := icons.Search(r.URL.Query().Get("q"), limit)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(result)
}
