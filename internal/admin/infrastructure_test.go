package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// The services behind the routes are one per address: two routes to the same
// host and port are one service carrying both, a UI route makes it an
// application, a disabled route brings none, and the port is said even when
// the scheme implies it.
func TestTheServicesAreTheAddressesTheRoutesPointAt(t *testing.T) {
	f := setup(t)
	put := func(id, name, upstream string, ui, enabled bool, pattern string) {
		body := fmt.Sprintf(`{"name": %q, "order": 1, "enabled": %t, "isUi": %t, "upstream": %q,
			"predicates": [{"type": "path", "args": {"patterns": [%q]}}]}`, name, enabled, ui, upstream, pattern)
		if code, out := f.call(t, "PUT", "/api/routes/"+id, body, f.rootC); code != http.StatusOK {
			t.Fatalf("route %s: %d %s", id, code, out)
		}
	}
	put("orders-api", "Orders API", "http://orders.shop.svc:8080", false, true, "/orders/**")
	put("orders-admin", "Orders admin", "http://orders.shop.svc:8080/admin", true, true, "/orders-admin/**")
	put("erp", "ERP", "https://erp.corp.local", true, true, "/erp/**")
	put("old", "Old", "http://old.corp.local:9000", false, false, "/old/**")

	code, out := f.call(t, "GET", "/api/infrastructure", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("infrastructure: %d %s", code, out)
	}
	var got infrastructure
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	by := map[string]infraService{}
	for _, s := range got.Services {
		by[s.Address] = s
	}
	if _, ok := by["old.corp.local:9000"]; ok {
		t.Fatal("a disabled route brought its service")
	}
	orders, ok := by["orders.shop.svc:8080"]
	if !ok || len(orders.Routes) != 2 || !orders.UI {
		t.Fatalf("two routes to one address are one application: %+v", got.Services)
	}
	if erp, ok := by["erp.corp.local:443"]; !ok || !erp.UI || len(erp.Routes) != 1 || erp.Routes[0].Name != "ERP" {
		t.Fatalf("https names its port: %+v", got.Services)
	}
	for _, s := range got.Services {
		if stateRank[s.State] == 0 && s.State != "down" {
			t.Fatalf("a state the console cannot draw: %+v", s)
		}
	}
}
