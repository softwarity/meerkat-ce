package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/idp"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// Literal is a secret field that was left OUT of the export because it held a
// value instead of a $name reference.
//
// The document stays valid - the field is simply absent from it - and the
// export stays public, which is the point. Saying so is what keeps the omission
// from being silent: the admin learns both that the field will be empty
// wherever this file lands, and that this install carries a secret their vault
// does not know about (VAULT-05 offers to move it in one click).
type Literal struct {
	Holder string `json:"holder"`
	ID     string `json:"id,omitempty"`
	Label  string `json:"label"`
	Field  string `json:"field"`
}

// Export builds the document from the current state of the store.
//
// The order of everything is fixed here rather than left to the database, so
// that exporting the same state twice produces the same bytes. Routes keep
// their own order, which is significant (first match wins); the rest is sorted
// by id, which is arbitrary but stable.
func Export(ctx context.Context, st *store.Store) (*Document, []Literal, error) {
	doc := &Document{Version: Version}

	routes, err := st.ListRoutes(ctx)
	if err != nil {
		return nil, nil, err
	}
	for i := range routes {
		trimRoute(&routes[i])
	}
	doc.Routes = routes

	roles, err := st.ListRoles(ctx)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	doc.Roles = roles

	// Organisations and their groups: who may do what is configuration, and it
	// is the part that hurts most to retype. The groups of every organisation
	// are carried in one flat list, each naming its own tenant.
	tenants, err := st.ListTenants(ctx)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(tenants, func(i, j int) bool { return tenants[i].ID < tenants[j].ID })
	doc.Tenants = tenants
	for _, t := range tenants {
		groups, err := st.ListGroups(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
		doc.Groups = append(doc.Groups, groups...)
	}

	providers, err := st.ListAuthProviders(ctx)
	if err != nil {
		return nil, nil, err
	}
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Order != providers[j].Order {
			return providers[i].Order < providers[j].Order
		}
		return providers[i].ID < providers[j].ID
	})
	doc.AuthProviders = providers

	// The ACTIVE theme, and it alone. The others are colour trials kept on the
	// side; carrying them made themes 71% of a typical export, most of it the
	// built-in palettes the receiving gateway already ships.
	themes, err := st.ListThemes(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range themes {
		if !t.Active {
			continue
		}
		// A preset the admin never touched travels as its NAME: the palettes
		// are in the binary on the other side too, and twenty colour tokens
		// that say "the ones you already have" are noise.
		if presetLike(t) {
			t.Dark, t.Light = nil, nil
		}
		doc.Themes = []store.Theme{t}
		break
	}

	// The relay travels only if there is one: an empty section in every file
	// would read as "no relay configured, on purpose", which is a different
	// statement from "this file says nothing about the relay".
	if relay := st.RawSMTP(ctx); relay.Host != "" {
		doc.MailRelay = &relay
	}

	doc.Settings = map[string]json.RawMessage{}
	for _, key := range ExportedSettings {
		var raw json.RawMessage
		if err := st.GetSetting(ctx, key, &raw); err != nil {
			continue // never set on this install: nothing to carry
		}
		doc.Settings[key] = raw
	}
	if len(doc.Settings) == 0 {
		doc.Settings = nil
	}

	return doc, stripSecrets(doc), nil
}

// trimRoute drops what a route says only because the store materialised it.
//
// Saving a route writes the whole shape, defaults included: a button that was
// never configured comes back as height 24, padX 12, shape "round", and every
// service route carries isUi false. None of that is a DECISION, and a
// configuration should read as the decisions someone made - a reader who sees
// `height: 24` has to know the default to know whether it means anything.
//
// Only values identical to the default are dropped, so nothing chosen is lost:
// height 24 set on purpose reads the same as height 24 never touched, and that
// is precisely why it does not matter which one it was.
func trimRoute(r *store.Route) {
	if r.UI != nil {
		trimUI(r.UI)
		if isZeroUI(r.UI) {
			r.UI = nil
		}
	}
	if r.Identity != nil {
		for i := range r.Identity.Attributes {
			a := &r.Identity.Attributes[i]
			if a.As == a.Field {
				a.As = ""
			}
		}
	}
	if r.API != nil && strings.TrimSpace(r.API.OpenapiURL) == "" && r.API.Security == nil {
		r.API = nil
	}
}

func trimUI(ui *store.RouteUI) {
	if b := &ui.UserButton; !b.Enabled {
		// A button that is off says nothing else worth carrying.
		*b = store.UserButton{}
	} else {
		if b.Height == 24 {
			b.Height = 0
		}
		if b.Position == "top-right" {
			b.Position = ""
		}
		if b.Shape == "round" {
			b.Shape = ""
		}
		if b.PadX == 12 {
			b.PadX = 0
		}
		if b.PadY == 12 {
			b.PadY = 0
		}
	}
	if ui.Scheme != nil && !ui.Scheme.Select && ui.Scheme.Mechanism == "" &&
		ui.Scheme.Light == "" && ui.Scheme.Dark == "" && ui.Scheme.Attribute == "" {
		ui.Scheme = nil
	}
	if ui.Roles != nil && !ui.Roles.Enabled {
		ui.Roles = nil
	}
	if ui.UserInfo != nil && !ui.UserInfo.Enabled {
		ui.UserInfo = nil
	}
}

// isZeroUI reports whether a ui block holds nothing anyone asked for.
func isZeroUI(ui *store.RouteUI) bool {
	return !ui.UserButton.Enabled && ui.Scheme == nil && ui.Roles == nil &&
		ui.UserInfo == nil && ui.CustomCSS == "" && ui.CustomJS == "" && ui.Link == ""
}

// stripSecrets empties every declared secret field that does not hold a
// reference, and reports what it emptied.
//
// The fields come from the SAME declarations the rest of Meerkat uses
// (idp.SecretFields, the relay's two credentials): sensitivity is declared once
// and never guessed from a field name, so a new provider kind cannot leak by
// forgetting to teach this file about itself.
func stripSecrets(doc *Document) []Literal {
	var found []Literal
	for i := range doc.AuthProviders {
		p := &doc.AuthProviders[i]
		for _, field := range idp.SecretFields(p.Kind) {
			value, _ := p.Config[field].(string)
			if value == "" || vault.IsRef(value) {
				continue
			}
			delete(p.Config, field)
			found = append(found, Literal{
				Holder: "authprovider", ID: p.ID, Label: p.Name, Field: field,
			})
		}
	}
	if r := doc.MailRelay; r != nil {
		for _, f := range []struct {
			name  string
			value *string
		}{
			{"password", &r.Password},
			{"oauth2ClientSecret", &r.OAuth2.ClientSecret},
		} {
			if *f.value == "" || vault.IsRef(*f.value) {
				continue
			}
			*f.value = ""
			found = append(found, Literal{Holder: "mailrelay", Label: "mail relay", Field: f.name})
		}
	}
	return found
}

// presetLike reports whether t is one of the built-in palettes, unmodified.
// Compared on what is actually seen - the two palettes and the flat switch -
// not on the name, which an admin may rename without changing a colour.
func presetLike(t store.Theme) bool {
	for _, p := range store.PresetThemes() {
		if p.ID != t.ID || p.Flat != t.Flat {
			continue
		}
		if sameMap(p.Dark, t.Dark) && sameMap(p.Light, t.Light) {
			return true
		}
	}
	return false
}

func sameMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Section is one KIND of thing a document carries, and how much of it. The
// console shows this before the download, and the question it answers is "what
// nature of information am I handing over" - not "which objects", which is what
// the file itself is for.
type Section struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	// Keys names the settings: "12 settings" says nothing, and the nature of
	// what is configured is exactly the question being asked.
	Keys []string `json:"keys,omitempty"`
	// Bytes is what an image weighs. An export is mostly text; a logo is not,
	// and it is the one thing that makes a file heavy without saying so.
	Bytes int `json:"bytes,omitempty"`
}

// Inventory lists the kinds doc holds, in the order they are written.
func Inventory(doc *Document) []Section {
	var out []Section
	add := func(kind string, n int) {
		if n > 0 {
			out = append(out, Section{Kind: kind, Count: n})
		}
	}
	add("route", len(doc.Routes))
	add("role", len(doc.Roles))
	add("tenant", len(doc.Tenants))
	add("group", len(doc.Groups))
	add("authProvider", len(doc.AuthProviders))
	add("theme", len(doc.Themes))
	if doc.MailRelay != nil {
		add("mailRelay", 1)
	}
	if n := len(doc.Settings); n > 0 {
		keys := make([]string, 0, n)
		for key := range doc.Settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out = append(out, Section{Kind: "setting", Count: n, Keys: keys})
	}
	// The branding's pictures ride inside its setting as data URIs, so they are
	// counted there too. Named separately because they are the only binaries in
	// the file and the only thing that can make it big.
	if n, count := imageBytes(doc), imageCount(doc); n > 0 {
		out = append(out, Section{Kind: "image", Count: count, Bytes: n})
	}
	return out
}

// imageBytes measures the pictures the branding setting carries; imageCount
// says how many there are.
func imageBytes(doc *Document) int { n, _ := brandingImages(doc); return n }

func imageCount(doc *Document) int { _, c := brandingImages(doc); return c }

func brandingImages(doc *Document) (bytes, count int) {
	branding, err := brandingMap(doc)
	if err != nil || branding == nil {
		return 0, 0
	}
	for _, f := range imageFields {
		if v, _ := getPath(branding, f.path).(string); v != "" {
			bytes += len(v)
			count++
		}
	}
	return bytes, count
}

// Refs returns every vault entry the document points at, sorted, without
// duplicates. Whole values and fragments alike ("http://${host}:8080" counts),
// because both have to resolve for the configuration to work.
//
// This is what the import preview turns into "here is what this configuration
// expects your vault to hold".
func Refs(doc *Document) ([]string, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("config: read references: %w", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("config: read references: %w", err)
	}
	// A literal argument is not scanned: a respond template writes $i and $r as
	// Go range variables, and reading them as vault references reserved two
	// empty entries nobody asked for. The gateway already leaves these alone
	// when it serves (restoreLiteralArgs); the reader has to agree with it.
	dropLiteralArgs(tree)

	seen := map[string]bool{}
	var walk func(any)
	walk = func(node any) {
		switch n := node.(type) {
		case string:
			for _, name := range vault.Refs(n) {
				seen[name] = true
			}
		case []any:
			for _, item := range n {
				walk(item)
			}
		case map[string]any:
			for _, item := range n {
				walk(item)
			}
		}
	}
	walk(tree)
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// dropLiteralArgs removes, from a document read as a JSON tree, the filter and
// predicate arguments a brick declared literal. They reach it verbatim, so
// whatever looks like a $reference in them is not one.
func dropLiteralArgs(tree any) {
	doc, ok := tree.(map[string]any)
	if !ok {
		return
	}
	routes, _ := doc["routes"].([]any)
	for _, r := range routes {
		route, ok := r.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"filters", "predicates"} {
			specs, _ := route[key].([]any)
			for _, s := range specs {
				spec, ok := s.(map[string]any)
				if !ok {
					continue
				}
				kind, _ := spec["type"].(string)
				args, _ := spec["args"].(map[string]any)
				for _, name := range routing.LiteralArgs(kind) {
					delete(args, name)
				}
			}
		}
	}
}

// SecretRefs returns the references that sit in a DECLARED secret field, which
// the vault has to hold as secrets rather than plain values. Everything else a
// document references (an upstream host, a header value) is an ordinary value.
func SecretRefs(doc *Document) map[string]bool {
	out := map[string]bool{}
	add := func(value string) {
		if name := vault.RefName(value); name != "" {
			out[name] = true
		}
	}
	for _, p := range doc.AuthProviders {
		for _, field := range idp.SecretFields(p.Kind) {
			value, _ := p.Config[field].(string)
			add(value)
		}
	}
	if r := doc.MailRelay; r != nil {
		add(r.Password)
		add(r.OAuth2.ClientSecret)
	}
	return out
}
