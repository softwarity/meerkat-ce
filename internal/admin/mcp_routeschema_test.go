package admin

import (
	"reflect"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// TestEveryRouteFieldIsDeclared: save_route's schema names every field a
// route carries. An undeclared one reaches the server as a STRING - the
// agent's client knows no better - and the save is refused ("cannot unmarshal
// string into Go struct field Route.telemetry"), so a route read with
// get_route could not be sent back whole. A field added to store.Route must
// be added to routeSchema too.
func TestEveryRouteFieldIsDeclared(t *testing.T) {
	props, _ := routeSchema()["properties"].(map[string]any)
	rt := reflect.TypeOf(store.Route{})
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		if _, ok := props[name]; !ok {
			t.Errorf("save_route does not declare %q: an agent's value for it would arrive as a string", name)
		}
	}
}
