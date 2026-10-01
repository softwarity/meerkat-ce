package config

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The two settings that TRAVEL and said their credential would not: the
// telemetry's auth header (OBS-04) and the ACME external-account key. Both
// carry a $name in the ordinary case, which is why nobody noticed that a
// literal - posted by the API, or left by a bootstrap file - went with the
// document.
func TestASettingsLiteralDoesNotTravel(t *testing.T) {
	doc := &Document{Settings: map[string]json.RawMessage{
		store.SettingTelemetry: json.RawMessage(
			`{"enabled":true,"endpoint":"http://jaeger:4318","headers":{"Authorization":"Bearer a-real-key","X-Scope":"${otlp-scope}"}}`),
		store.SettingTLS: json.RawMessage(`{"acme":{"enabled":true,"eabHmacKey":"a-real-hmac"}}`),
	}}
	found := stripSecrets(doc)

	body := string(doc.Settings[store.SettingTelemetry]) + string(doc.Settings[store.SettingTLS])
	for _, secret := range []string{"Bearer a-real-key", "a-real-hmac"} {
		if strings.Contains(body, secret) {
			t.Errorf("a literal left with the document: %s", body)
		}
	}
	// A reference is public by construction and stays: that is the whole point
	// of writing one.
	if !strings.Contains(body, "${otlp-scope}") {
		t.Errorf("a reference was emptied along with the literals: %s", body)
	}
	// And what was emptied is NAMED, so whoever imports it is told what is
	// missing rather than left with a silently unauthenticated export.
	var fields []string
	for _, l := range found {
		fields = append(fields, l.Holder+"/"+l.Field)
	}
	for _, want := range []string{"telemetry/Authorization", "tls/eabHmacKey"} {
		if !slices.Contains(fields, want) {
			t.Errorf("the emptied literal %q was not reported: %v", want, fields)
		}
	}

	// The reference travels as a SECRET, not as an ordinary value: a vault
	// that held it in clear would defeat the reference.
	if refs := SecretRefs(doc); !refs["otlp-scope"] {
		t.Errorf("the header's reference is not carried as a secret: %v", refs)
	}
}
