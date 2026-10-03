package config

import (
	"context"
	"slices"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// On the community image an import applies a configuration without its
// Enterprise parts, and says which it left out: working hours, a layout, a
// hidden mark, the OpenTelemetry export, a directory.
func TestTheCommunityImageImportsWithoutTheEnterpriseParts(t *testing.T) {
	if edition.Enterprise {
		t.Skip("the community image's behaviour")
	}
	st := openTemp(t)
	ctx := context.Background()
	doc, err := Unmarshal([]byte(`{"version":1,
		"settings":{
			"business_access":{"timezone":"UTC","days":[{"day":1,"from":"09:00","to":"17:00"}]},
			"page_layout":{"name":"split"},
			"branding":{"appName":"Acme","hideMark":true},
			"telemetry":{"enabled":true,"endpoint":"http://otel:4318","logs":true}
		},
		"authProviders":[{"id":"ldap1","name":"Corp","kind":"ldap"}],
		"roles":[{"name":"A"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Apply(ctx, st, doc, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(plan.NotApplied) != 5 {
		t.Fatalf("want five parts left out, got %q", plan.NotApplied)
	}
	var ba store.BusinessAccess
	_ = st.GetSetting(ctx, store.SettingBusinessAccess, &ba)
	if ba.Timezone == "UTC" && len(ba.Days) == 1 {
		t.Errorf("the imported working hours were written: %+v", ba)
	}
	var b map[string]any
	_ = st.GetSetting(ctx, store.SettingBranding, &b)
	if b["hideMark"] == true || b["appName"] != "Acme" {
		t.Errorf("branding: want the name kept and the mark shown, got %v", b)
	}
	tel := st.RawTelemetry(ctx)
	if tel.Enabled || !tel.Logs {
		t.Errorf("telemetry: want the export off and the log format kept, got %+v", tel)
	}
	providers, _ := st.ListAuthProviders(ctx)
	if slices.ContainsFunc(providers, func(p store.AuthProvider) bool { return p.Kind == store.ProviderLDAP }) {
		t.Error("a directory was imported")
	}
	if _, err := st.GetRole(ctx, "A"); err != nil {
		t.Errorf("the rest of the document was not applied: %v", err)
	}
}
