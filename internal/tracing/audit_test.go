package tracing

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// What leaves is an OTLP/JSON logs request the Collector routes by its
// resource: meerkat.stream=audit, with the action as the body and the trace id
// of the request that caused it (AUD-03).
func TestAnAuditBatchIsAnOTLPLogsRequestMarkedAudit(t *testing.T) {
	auditMu.Lock()
	auditConfig = Config{Service: "meerkat", Version: "1.2.3"}
	auditMu.Unlock()
	body, err := auditPayload([]AuditRecord{{
		Time: time.Unix(1700000000, 0), TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", Body: "route.update",
		Attrs: []Attr{String("user.id", "u1"), String("audit.detail", ""), Strings("meerkat.roles", []string{"admin"})},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		ResourceLogs []struct {
			Resource struct {
				Attributes []struct {
					Key   string
					Value map[string]any
				}
			}
			ScopeLogs []struct {
				LogRecords []map[string]any
			}
		}
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	res := map[string]any{}
	for _, a := range req.ResourceLogs[0].Resource.Attributes {
		res[a.Key] = a.Value["stringValue"]
	}
	if res["meerkat.stream"] != "audit" || res["service.name"] != "meerkat" || res["service.version"] != "1.2.3" {
		t.Errorf("resource: %v", res)
	}
	rec := req.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
	if rec["traceId"] != "4bf92f3577b34da6a3ce929d0e0e4736" || rec["timeUnixNano"] != "1700000000000000000" {
		t.Errorf("record: %v", rec)
	}
	if b, _ := rec["body"].(map[string]any); b["stringValue"] != "route.update" {
		t.Errorf("body: %v", rec["body"])
	}
	// An empty attribute is left out rather than written empty.
	if strings.Contains(string(body), "audit.detail") {
		t.Errorf("an empty attribute was written: %s", body)
	}
}

// Shipped once switched on; held and retried while the collector refuses;
// nothing at all while off.
func TestTheAuditIsSentAndRetriedUntilTaken(t *testing.T) {
	var mu sync.Mutex
	var got []string
	refuse := 1
	RegisterAuditStarter(func(Config) (func([]byte) error, error) {
		return func(b []byte) error {
			mu.Lock()
			defer mu.Unlock()
			if refuse > 0 {
				refuse--
				return errors.New("collector down")
			}
			got = append(got, string(b))
			return nil
		}, nil
	})
	defer func() { _ = ApplyAudit(Config{}, false); RegisterAuditStarter(nil) }()

	ShipAudit(AuditRecord{Time: time.Now(), Body: "before.on"})
	SetAuditScope(true, true)
	if err := ApplyAudit(Config{Endpoint: "http://collector:4318"}, true); err != nil {
		t.Fatal(err)
	}
	ShipAudit(AuditRecord{Time: time.Now(), Body: "route.update"})
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || !strings.Contains(got[0], "route.update") {
		t.Fatalf("after one refusal, want the batch delivered once, got %v", got)
	}
	if strings.Contains(got[0], "before.on") {
		t.Error("an event recorded while the audit was off was sent")
	}
}

func TestTheCommunityBinarySaysWhyItCannotSend(t *testing.T) {
	RegisterAuditStarter(nil)
	if err := ApplyAudit(Config{}, true); err == nil || !strings.Contains(err.Error(), "Enterprise") {
		t.Fatalf("want an Enterprise refusal, got %v", err)
	}
	if Shipping() {
		t.Error("shipping without a sender")
	}
}

// Two switches, two sides: the data plane's audit and the console's.
func TestEachAuditSwitchLetsOnlyItsSideThrough(t *testing.T) {
	sent := make(chan string, 8)
	RegisterAuditStarter(func(Config) (func([]byte) error, error) {
		return func(b []byte) error { sent <- string(b); return nil }, nil
	})
	defer func() { _ = ApplyAudit(Config{}, false); RegisterAuditStarter(nil); SetAuditScope(false, false) }()
	if err := ApplyAudit(Config{Endpoint: "http://collector:4318"}, true); err != nil {
		t.Fatal(err)
	}
	SetAuditScope(true, false)
	ShipAudit(AuditRecord{Time: time.Now(), Body: "route.update", Console: true})
	ShipAudit(AuditRecord{Time: time.Now(), Body: "signin"})
	select {
	case b := <-sent:
		if strings.Contains(b, "route.update") || !strings.Contains(b, "signin") {
			t.Fatalf("data plane only, got %s", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nothing sent")
	}
	if !ShippingData() {
		t.Error("the data plane's switch is on, yet ShippingData says no")
	}
	SetAuditScope(false, true)
	if ShippingData() {
		t.Error("an audited endpoint would read its body with the data plane's switch off")
	}
}
