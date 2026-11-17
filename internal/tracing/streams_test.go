package tracing

import (
	"encoding/json"
	"testing"
	"time"
)

// A route's own template wins over the installation's, which wins over the
// default; {servicename} is the service behind the route, or "meerkat" for
// what belongs to none.
func TestAnAuditSaysWhichServiceItAudits(t *testing.T) {
	defer SetStreams(Streams{})
	SetStreams(Streams{Routes: map[string]RouteStreams{
		"Bookings": {Audit: "bookings", Service: "booking-api"},
		"billing":  {Service: "billing-api"},
	}})
	cases := []struct{ got, want string }{
		{AuditStreamKey(), "audit"},
		{AuditStream("Bookings"), "bookings"},
		{AuditStream("billing"), "billing-api"},
		{AuditStream("unknown"), "unknown"},
		{AuditStream(""), "meerkat"},
	}
	SetStreams(Streams{AuditKey: "team.audit", Audit: "app.{servicename}", Routes: map[string]RouteStreams{
		"billing": {Service: "billing-api"},
	}})
	cases = append(cases,
		struct{ got, want string }{AuditStreamKey(), "team.audit"},
		struct{ got, want string }{AuditStream("billing"), "app.billing-api"},
	)
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// The service is the first label of the upstream's host.
func TestTheServiceIsTheUpstreamsHost(t *testing.T) {
	for upstream, want := range map[string]string{
		"https://orders-api.shop.svc.cluster.local:8443/api": "orders-api",
		"http://stack_orders:8080":                           "stack_orders",
		"https://meerkat.softwarity.dev":                     "meerkat",
		"http://10.0.0.5:9000":                               "10.0.0.5",
		"":                                                   "Orders REST",
		"http://$orders-host:8080":                           "Orders REST",
	} {
		if got := ServiceOf(upstream, "Orders REST"); got != want {
			t.Errorf("%q: %q, want %q", upstream, got, want)
		}
	}
}

type otlpBatch struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []struct {
				Key   string
				Value map[string]any
			}
		}
		ScopeLogs []struct{ LogRecords []map[string]any }
	}
}

// One batch, two services audited: two resources, each with its own records,
// in the order they first appeared.
func TestABatchOfTwoServicesLeavesUnderTwoResources(t *testing.T) {
	body, err := logsPayload(Config{Service: "meerkat"}, "audit", "meerkat", "meerkat.audit", []AuditRecord{
		{Time: time.Unix(1, 0), Body: "a", Stream: "orders"},
		{Time: time.Unix(2, 0), Body: "b"},
		{Time: time.Unix(3, 0), Body: "c", Stream: "orders"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var req otlpBatch
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.ResourceLogs) != 2 {
		t.Fatalf("%d resources, want 2: %s", len(req.ResourceLogs), body)
	}
	for i, want := range []struct {
		value string
		n     int
	}{{"orders", 2}, {"meerkat", 1}} {
		rl := req.ResourceLogs[i]
		var value any
		for _, a := range rl.Resource.Attributes {
			if a.Key == "audit" {
				value = a.Value["stringValue"]
			}
		}
		if value != want.value || len(rl.ScopeLogs[0].LogRecords) != want.n {
			t.Errorf("resource %d: audit=%v with %d records, want %s with %d", i, value, len(rl.ScopeLogs[0].LogRecords), want.value, want.n)
		}
	}
}

// The logs carry no such attribute: its absence is what makes a line a log.
func TestALogBatchSaysNothingItAudits(t *testing.T) {
	body, err := logsPayload(Config{Service: "meerkat"}, "", "", "meerkat.logs", []AuditRecord{{Time: time.Unix(1, 0), Body: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	var req otlpBatch
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	for _, a := range req.ResourceLogs[0].Resource.Attributes {
		if a.Key != "service.name" && a.Key != "service.version" {
			t.Errorf("a log line carries %s", a.Key)
		}
	}
}
