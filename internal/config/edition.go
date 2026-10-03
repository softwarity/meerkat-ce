package config

import (
	"encoding/json"
	"fmt"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// What a configuration may NOT carry onto the community image.
//
// The screens and their endpoints refuse an Enterprise value on that image,
// one by one; an import writes many objects at once and went round all of
// them - the console's own YAML editor could set working hours, a layout, a
// hidden mark or a directory. So before anything is written, the Enterprise
// parts are taken out of the document, and the plan says which: a
// configuration exported from an Enterprise gateway still imports into a
// community one, without bringing what that image does not run.
//
// Taken out rather than refused, because the rest of such a document is
// exactly what somebody moving to the community image wants to keep.

// notOnCommunity strips the document in place and names what it took.
func notOnCommunity(doc *Document) []string {
	if edition.Enterprise {
		return nil
	}
	var out []string
	if _, ok := doc.Settings[store.SettingBusinessAccess]; ok {
		delete(doc.Settings, store.SettingBusinessAccess)
		out = append(out, "working hours (Enterprise)")
	}
	if raw, ok := doc.Settings[store.SettingPageLayout]; ok {
		var l store.PageLayout
		if json.Unmarshal(raw, &l) != nil || l.Name != store.DefaultPageLayout().Name {
			delete(doc.Settings, store.SettingPageLayout)
			out = append(out, "page layout (Enterprise)")
		}
	}
	if note := unsetFields(doc, store.SettingBranding, "hideMark"); note {
		out = append(out, "hiding the Meerkat mark (Enterprise)")
	}
	if note := unsetFields(doc, store.SettingTelemetry, "enabled", "logsPush", "audit", "auditConsole"); note {
		out = append(out, "OpenTelemetry export (Enterprise)")
	}
	hours := false
	for i := range doc.Tenants {
		if !doc.Tenants[i].BusinessAccess.Inherited {
			doc.Tenants[i].BusinessAccess = store.BusinessAccess{Inherited: true}
			hours = true
		}
	}
	if hours {
		out = append(out, "organisations' working hours (Enterprise)")
	}
	kept := doc.AuthProviders[:0]
	for _, p := range doc.AuthProviders {
		if p.Kind == store.ProviderLDAP || p.Kind == store.ProviderSAML {
			out = append(out, fmt.Sprintf("authority %q: %s is Enterprise", p.Name, p.Kind))
			continue
		}
		kept = append(kept, p)
	}
	doc.AuthProviders = kept
	return out
}

// unsetFields turns boolean fields of one setting off, and says whether any
// was on. The rest of the setting is left as written.
func unsetFields(doc *Document, key string, fields ...string) bool {
	raw, ok := doc.Settings[key]
	if !ok {
		return false
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	changed := false
	for _, f := range fields {
		if on, _ := m[f].(bool); on {
			m[f] = false
			changed = true
		}
	}
	if !changed {
		return false
	}
	if b, err := json.Marshal(m); err == nil {
		doc.Settings[key] = b
	}
	return true
}
