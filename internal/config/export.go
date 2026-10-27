package config

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
	// The deposited specs. An upstream spec has nothing to carry - the url
	// alone re-fetches it - but a deposited file has no other home than this
	// gateway, so an export that dropped it would produce an import whose
	// per-endpoint policies point at operations nothing can list any more.
	if specs, err := st.RouteSpecContents(ctx); err == nil && len(specs) > 0 {
		doc.Specs = specs
	} else if err != nil {
		return nil, nil, err
	}
	// The files uploaded on routes: they have no other home than this gateway
	// either, and a files route imported without them answers 404 to all.
	if files, err := st.RouteFileContents(ctx); err == nil && len(files) > 0 {
		doc.Files = files
	} else if err != nil {
		return nil, nil, err
	}

	roles, err := st.ListRoles(ctx)
	if err != nil {
		return nil, nil, err
	}
	// The revision is this installation's bookkeeping - which write each row is
	// on - and it means nothing in a document that travels. Carried, it would
	// also turn every import into a STALE write against rows that do not exist
	// on the other side. Zeroed here, once per kind, rather than trusted to
	// each writer downstream.
	for i := range roles {
		roles[i].Rev = 0
	}
	doc.Roles = roles

	// Organisations and their groups: who may do what is configuration, and it
	// is the part that hurts most to retype. The groups of every organisation
	// are carried in one flat list, each naming its own tenant.
	tenants, err := st.ListTenants(ctx)
	if err != nil {
		return nil, nil, err
	}
	for i := range tenants {
		tenants[i].Rev = 0
	}
	doc.Tenants = tenants
	for _, t := range tenants {
		groups, err := st.ListGroups(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		for i := range groups {
			groups[i].Rev = 0
		}
		doc.Groups = append(doc.Groups, groups...)
	}

	providers, err := st.ListAuthProviders(ctx)
	if err != nil {
		return nil, nil, err
	}
	for i := range providers {
		providers[i].Rev = 0
	}
	doc.AuthProviders = providers

	// The OIDC clients, hash of their secret included: the one place it leaves
	// the database, so that the application keeps its secret wherever this
	// configuration lands (see OIDCClient). Already sorted by id.
	clients, err := st.ListOIDCClients(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range clients {
		doc.OIDCClients = append(doc.OIDCClients, oidcClientOf(c))
	}

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
		// A theme travels as its colours: the palettes are generated from them
		// on the other side, by the same generator, and a hundred tokens that
		// say "what those six make" are noise.
		t.Dark, t.Light = nil, nil
		t.Rev = 0
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

	order(doc)
	return doc, stripSecrets(doc), nil
}

// order puts every list whose order means nothing in one order, the same for
// an export and for a comparison. Routes keep theirs, which is significant
// (first match wins); the rest is sorted by what identifies it, which is
// arbitrary but stable.
//
// A group's roles are a SET, read from the database in whatever order it
// hands them - no ORDER BY, so two gateways, or two engines, wrote the same
// group two ways, and a file edited by hand a third. Sorted here, the export
// is the same bytes for the same state, and a role list somebody reordered by
// hand is not a change.
func order(doc *Document) {
	sort.Slice(doc.Roles, func(i, j int) bool { return doc.Roles[i].Name < doc.Roles[j].Name })
	sort.Slice(doc.Tenants, func(i, j int) bool { return doc.Tenants[i].ID < doc.Tenants[j].ID })
	// By organisation, then by id: the groups of one organisation together,
	// as they were listed tenant by tenant.
	sort.Slice(doc.Groups, func(i, j int) bool {
		a, b := doc.Groups[i], doc.Groups[j]
		if a.TenantID != b.TenantID {
			return a.TenantID < b.TenantID
		}
		return a.ID < b.ID
	})
	for i := range doc.Groups {
		sort.Strings(doc.Groups[i].Roles)
	}
	sort.SliceStable(doc.AuthProviders, func(i, j int) bool {
		a, b := doc.AuthProviders[i], doc.AuthProviders[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	sort.Slice(doc.OIDCClients, func(i, j int) bool { return doc.OIDCClients[i].ID < doc.OIDCClients[j].ID })
}

// exportedRoute is a route as a document writes it: a COPY, trimmed the way
// Export trims. Every comparison of a route goes through it, on both sides.
//
// The export drops what the store materialised (see trimRoute), and the
// console saves a route with everything its form holds - a user button that is
// off still has its height, a roles block that is off still has its tag. So
// the row behind a route and the document it was exported to differ by exactly
// what the trim took out, and comparing one raw against the other reported
// such a route as "update" on every plan: a capture pushed to git and pulled
// back named nine routes that had not moved. Trimming both sides with the SAME
// function is what keeps it true for a field added tomorrow: whatever the
// export learns to leave out, the comparison stops seeing.
//
// A copy, never the caller's route: the trim rewrites the ui and identity
// blocks in place, and the route an import is about to SAVE must keep what
// its document said.
func exportedRoute(r store.Route) store.Route {
	raw, err := json.Marshal(r)
	if err != nil {
		return r
	}
	var out store.Route
	if err := json.Unmarshal(raw, &out); err != nil {
		return r
	}
	// What the store does to a route on its way in, so that a document
	// written by hand compares as the row it would become. A rule it would
	// refuse is left as written: the save says so, not the comparison.
	_ = store.SanitizeAccess(&out.Access)
	_ = store.SanitizeRateLimits(out.Limits)
	trimRoute(&out)
	return out
}

// exported is doc with every object the way an export writes it - what a
// comparison of two documents, or of a document and what runs, must look at.
// A copy: only the lists it rewrites are new.
func exported(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	out := *doc
	if doc.Routes != nil {
		out.Routes = make([]store.Route, len(doc.Routes))
		for i, r := range doc.Routes {
			out.Routes[i] = exportedRoute(r)
		}
	}
	if doc.Groups != nil {
		out.Groups = make([]store.Group, len(doc.Groups))
		for i, g := range doc.Groups {
			out.Groups[i] = exportedGroup(g)
		}
	}
	if doc.Tenants != nil {
		out.Tenants = make([]store.Tenant, len(doc.Tenants))
		for i, t := range doc.Tenants {
			out.Tenants[i] = exportedTenant(t)
		}
	}
	out.Roles = slices.Clone(doc.Roles)
	out.AuthProviders = slices.Clone(doc.AuthProviders)
	out.OIDCClients = slices.Clone(doc.OIDCClients)
	order(&out)
	return &out
}

// exportedGroup is a group as the store keeps it once saved: its window
// sanitised (a group that never mentioned hours inherits them - the store
// writes inherited true for it), its roles in order. A copy.
func exportedGroup(g store.Group) store.Group {
	g.Name = strings.TrimSpace(g.Name)
	g.Roles = slices.Clone(g.Roles)
	sort.Strings(g.Roles)
	g.BusinessAccess.Days = slices.Clone(g.BusinessAccess.Days)
	store.SanitizeBusinessAccess(&g.BusinessAccess)
	return g
}

// exportedTenant is a tenant as the store keeps it once saved. A copy.
func exportedTenant(t store.Tenant) store.Tenant {
	t.BusinessAccess.Days = slices.Clone(t.BusinessAccess.Days)
	store.SanitizeBusinessAccess(&t.BusinessAccess)
	return t
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
	// The revision is this installation's own bookkeeping - which write this
	// row is on - and it means nothing in a document that travels to another
	// one. Carried, it would also make an import a STALE write against a route
	// that does not exist there yet.
	r.Rev = 0
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
	// The role map counts: a route may map its declared roles without a spec
	// of its own, and an export that dropped the block lost them - which the
	// comparison, trimming both sides the same way, would then not even see.
	if r.API != nil && r.Spec().Empty() && r.API.Security == nil && len(r.API.Audit) == 0 &&
		len(r.API.RoleMap) == 0 {
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
		ui.Scheme.Light == "" && ui.Scheme.Dark == "" && ui.Scheme.Attribute == "" &&
		ui.Scheme.Tag == "" && ui.Scheme.Button == "" {
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
		ui.UserInfo == nil && len(ui.Injections) == 0
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
	found = append(found, stripSettingSecrets(doc)...)
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

// settingSecrets are the secret fields of the settings a document carries, by
// the path they sit at inside the setting's own JSON.
//
// TWO SETTINGS SAID THEIR CREDENTIAL WOULD NOT TRAVEL and nothing made it so:
// the telemetry's auth header (OBS-04) and the ACME external-account key
// (TLS). Both carry a $name in the ordinary case, which is exactly why the
// hole stayed invisible - a literal only arrives through the API or a
// bootstrap file, and then it left with the document.
var settingSecrets = []struct {
	setting string
	// path walks the setting's decoded JSON to the object holding the field;
	// empty means the setting itself.
	path []string
	// field is the key to empty, or "" to empty EVERY string value of the
	// object at path (the telemetry's headers are named by whoever set them).
	field string
	label string
	// each, when set, names a LIST under path whose every object holds the
	// field - the ACME authorities - and idKey the key naming each object.
	each, idKey string
}{
	{setting: store.SettingTelemetry, path: []string{"headers"}, label: "OpenTelemetry"},
	{setting: store.SettingTLS, path: []string{"acme"}, each: "authorities", idKey: "id", field: "eabHmacKey", label: "ACME authority"},
	// The one account of the settings before authorities (v73).
	{setting: store.SettingTLS, path: []string{"acme"}, field: "eabHmacKey", label: "TLS"},
}

// secretHolder is one object a declared secret field sits in, and the id it
// is reported under.
type secretHolder struct {
	id  string
	obj map[string]any
}

// holdersOf walks a setting's JSON to the objects a declaration names.
func holdersOf(whole map[string]any, path []string, each, idKey string) []secretHolder {
	holder := whole
	for _, step := range path {
		next, ok := holder[step].(map[string]any)
		if !ok {
			return nil
		}
		holder = next
	}
	if each == "" {
		return []secretHolder{{obj: holder}}
	}
	list, _ := holder[each].([]any)
	var out []secretHolder
	for _, item := range list {
		if obj, ok := item.(map[string]any); ok {
			id, _ := obj[idKey].(string)
			out = append(out, secretHolder{id: id, obj: obj})
		}
	}
	return out
}

// stripSettingSecrets empties a literal wherever settingSecrets says one can
// sit, and reports it like any other - so the import side offers to put it
// back, and the export side stays true to what it promises.
func stripSettingSecrets(doc *Document) []Literal {
	var found []Literal
	for _, decl := range settingSecrets {
		raw, ok := doc.Settings[decl.setting]
		if !ok {
			continue
		}
		var whole map[string]any
		if err := json.Unmarshal(raw, &whole); err != nil {
			continue
		}
		changed := false
		for _, h := range holdersOf(whole, decl.path, decl.each, decl.idKey) {
			for key, value := range h.obj {
				if decl.field != "" && key != decl.field {
					continue
				}
				text, ok := value.(string)
				if !ok || text == "" || vault.IsRef(text) {
					continue
				}
				h.obj[key] = ""
				changed = true
				found = append(found, Literal{Holder: decl.setting, ID: h.id, Label: decl.label, Field: key})
			}
		}
		if !changed {
			continue
		}
		if encoded, err := json.Marshal(whole); err == nil {
			doc.Settings[decl.setting] = encoded
		}
	}
	return found
}

// settingSecretRefs collects the references sitting in those same fields, so
// they are carried as SECRETS rather than as plain values.
func settingSecretRefs(doc *Document, add func(string)) {
	for _, decl := range settingSecrets {
		raw, ok := doc.Settings[decl.setting]
		if !ok {
			continue
		}
		var whole map[string]any
		if err := json.Unmarshal(raw, &whole); err != nil {
			continue
		}
		for _, h := range holdersOf(whole, decl.path, decl.each, decl.idKey) {
			for key, value := range h.obj {
				if decl.field != "" && key != decl.field {
					continue
				}
				if text, ok := value.(string); ok {
					add(text)
				}
			}
		}
	}
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
	add("oidcClient", len(doc.OIDCClients))
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
	settingSecretRefs(doc, add)
	return out
}
