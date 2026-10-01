package admin

import (
	"reflect"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// TestSaveRouteDeclaresEveryNestedObject: what save_route does not DECLARE, it
// cannot take. An object left to additionalProperties reaches the server as a
// string and the save is refused with "cannot unmarshal string into Go struct
// field Route.identity" - which is how this was found, by an agent that could
// read a route's identity forwarding with get_route and had no way to write it
// back. So the rule is mechanical: every field of a route that is not a plain
// scalar is named in the schema. Its SHAPE stays get_route's business; only the
// type has to be there, because the type is what makes the value survive.
func TestSaveRouteDeclaresEveryNestedObject(t *testing.T) {
	props, ok := routeSchema()["properties"].(map[string]any)
	if !ok {
		t.Fatal("the route schema has no properties")
	}
	rt := reflect.TypeOf(store.Route{})
	for i := range rt.NumField() {
		f := rt.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		kind := f.Type.Kind()
		if kind == reflect.Pointer {
			kind = f.Type.Elem().Kind()
		}
		if kind != reflect.Struct && kind != reflect.Slice && kind != reflect.Map {
			continue // a scalar survives undeclared
		}
		if _, declared := props[name]; !declared {
			t.Errorf("route field %q is an object and the save_route schema does not declare it: "+
				"it will reach the server as a string and be refused", name)
		}
	}
}
