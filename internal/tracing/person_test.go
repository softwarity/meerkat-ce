package tracing

import (
	"strings"
	"testing"
)

func TestTheCallerIsOnTheSpansOnlyWhenSwitchedOn(t *testing.T) {
	defer SetCaller(false)
	who := Who{UserID: "u1", Username: "alice", TenantID: "t1", Tenant: "Acme", Group: "ops", Roles: []string{"admin", "viewer"}}
	if a := PersonAttrs(who); a != nil {
		t.Errorf("off by default, yet the person left: %v", a)
	}
	SetCaller(true)
	var k []string
	for _, a := range PersonAttrs(who) {
		k = append(k, a.Key+"="+a.Str+strings.Join(a.Strs, "+"))
	}
	if got, want := strings.Join(k, ","),
		"user.id=u1,user.name=alice,meerkat.tenant.id=t1,meerkat.tenant.name=Acme,meerkat.group=ops,meerkat.roles=admin+viewer"; got != want {
		t.Errorf("on: %q, want %q", got, want)
	}
	if a := PersonAttrs(Who{}); a != nil {
		t.Errorf("an anonymous call says nothing about a person, got %v", a)
	}
}

func TestARelayedBatchCarriesThePersonAndNothingElseChanges(t *testing.T) {
	// The timestamp is beyond what a float64 holds exactly: decoding it as a
	// number and writing it back would move it.
	in := `{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"scope":{"name":"fetch"},` +
		`"spans":[{"name":"GET /x","startTimeUnixNano":1727771234567891234,"unknownField":true,` +
		`"attributes":[{"key":"http.method","value":{"stringValue":"GET"}},` +
		`{"key":"user.id","value":{"stringValue":"somebody-else"}}]}]}]}]}`
	out, err := StampSpans([]byte(in), []Attr{String("user.id", "u1"), String("meerkat.tenant.id", "t1"),
		Strings("meerkat.roles", []string{"admin", "viewer"})})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`1727771234567891234`,
		`"unknownField":true`,
		`{"key":"http.method","value":{"stringValue":"GET"}}`,
		`{"key":"user.id","value":{"stringValue":"u1"}}`,
		`{"key":"meerkat.tenant.id","value":{"stringValue":"t1"}}`,
		`{"key":"meerkat.roles","value":{"arrayValue":{"values":[{"stringValue":"admin"},{"stringValue":"viewer"}]}}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	// The gateway knows who holds the session; the page does not get to say.
	if strings.Contains(got, "somebody-else") {
		t.Errorf("the page's own user.id survived the stamp: %s", got)
	}
}

func TestABatchThatIsNotJSONIsRefusedByTheStamp(t *testing.T) {
	if _, err := StampSpans([]byte("\x0a\x02protobuf"), []Attr{String("user.id", "u1")}); err == nil {
		t.Fatal("a protobuf batch was taken for JSON")
	}
	body := []byte("anything")
	if out, err := StampSpans(body, nil); err != nil || string(out) != "anything" {
		t.Fatalf("nothing to stamp must hand the batch back untouched, got %q %v", out, err)
	}
}

func TestOnlyOurBundlesMarkSaysTheJourneyStartedInAPage(t *testing.T) {
	for in, want := range map[string]bool{
		"meerkat=b":           true,
		"vendor=x, meerkat=b": true,
		"meerkat=c":           false,
		"":                    false,
		"somebody-meerkat=b":  false,
	} {
		if got := FromPage(in); got != want {
			t.Errorf("FromPage(%q) = %v, want %v", in, got, want)
		}
	}
}
