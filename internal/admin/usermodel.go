package admin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The user MODEL: what an account carries beyond what this product invented.
//
// It sits in the infrastructure plane and not beside the accounts themselves,
// because defining a field and filling it are two different acts by two
// different people. One says "this installation records a cost centre"; the
// other says "Alice's is B200". The first is a decision about the shape of the
// product here, taken once; the second is daily administration.
//
// That split is also what makes the field safe to forward: a person cannot
// grant themselves an attribute an upstream trusts, and an application
// administrator cannot invent one either.
func (a *API) registerUserModel(mux Mux) {
	mux.Handle("GET /api/model/user-fields", a.authed(a.getUserFields))
	mux.Handle("PUT /api/model/user-fields", a.infraAdmin(a.putUserFields))
	// The organisation's own (MODEL-03): the same model one level up, defined
	// here and filled on the tenant's General screen.
	mux.Handle("GET /api/model/tenant-fields", a.authed(a.getTenantFields))
	mux.Handle("PUT /api/model/tenant-fields", a.infraAdmin(a.putTenantFields))
}

// getUserFields is readable by any signed-in caller, and deliberately so: the
// screen that FILLS these values is the accounts screen, run by application
// administrators who do not administer the infrastructure. A definition is not
// a secret - it is the shape of a form.
func (a *API) getUserFields(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, map[string]any{"fields": a.userFields(r)})
}

func (a *API) putUserFields(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body struct {
		Fields []store.UserField `json:"fields"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed model: "+err.Error())
		return
	}
	fields, err := store.SanitizeUserFields(body.Fields)
	if err != nil {
		// 422 and the sentence as written: this is read by somebody typing
		// into a form, and the refusal is the only teacher they get.
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := store.DisjointFields(fields, a.fieldDefs(r.Context(), store.SettingTenantFields)); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	before := a.userFields(r)
	if err := a.st.SetSetting(r.Context(), store.SettingUserFields, fields); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "model.user-fields", "settings", "", "", "",
		map[string]any{"fields": before}, map[string]any{"fields": fields})
	routes, err := a.followModel(r.Context(), actor, before, fields)
	if err != nil {
		a.internal(w, fmt.Errorf("model saved, but the routes were not brought along: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": fields, "routes": routes})
}

// getTenantFields is readable by any signed-in caller for the reason
// getUserFields gives: the organisation's screen draws its form from it, and
// the organisation's administrators read the values there.
func (a *API) getTenantFields(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, map[string]any{"fields": a.fieldDefs(r.Context(), store.SettingTenantFields)})
}

func (a *API) putTenantFields(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body struct {
		Fields []store.UserField `json:"fields"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed model: "+err.Error())
		return
	}
	// Several organisations are the Enterprise mode, and a field on the one a
	// community image serves would describe the installation, not an
	// organisation - which is what the gateway's own settings are for.
	// Emptying the list asks for nothing: a model is always allowed to shrink.
	if len(body.Fields) > 0 {
		if err := edition.Require("tenant fields"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	fields, err := store.SanitizeUserFields(body.Fields)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := store.DisjointFields(a.userFields(r), fields); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	before := a.fieldDefs(r.Context(), store.SettingTenantFields)
	if err := a.st.SetSetting(r.Context(), store.SettingTenantFields, fields); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "model.tenant-fields", "settings", "", "", "",
		map[string]any{"fields": before}, map[string]any{"fields": fields})
	routes, err := a.followModel(r.Context(), actor, before, fields)
	if err != nil {
		a.internal(w, fmt.Errorf("model saved, but the routes were not brought along: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": fields, "routes": routes})
}

// followModel brings the routes along when a list of fields changes, and
// answers the names of those it rewrote.
//
// A field ADDED is selected at once on every route that already declares
// what it knows about the caller: defining a fact is saying the services
// should have it, and leaving it unticked on every existing route made the
// definition look broken - the value was there and nothing received it. A
// route that declares nothing is left alone: it forwards nothing, and its
// editor offers every fact ticked the day somebody opens it.
//
// A field REMOVED leaves every route, its page stamp included: a name that no
// longer means anything would otherwise sit in the route, and the next save of
// that route would be refused for naming it.
//
// What lands on a PAGE is not ticked here: a page is read by the browser, and
// putting a fact in front of the person is a separate decision, made on the
// route. A rename is a removal and an addition, and reads as such.
func (a *API) followModel(ctx context.Context, actor store.User, before, after []store.UserField) ([]string, error) {
	had, has := map[string]bool{}, map[string]bool{}
	for _, f := range before {
		had[f.Name] = true
	}
	for _, f := range after {
		has[f.Name] = true
	}
	var added []string
	for _, f := range after {
		if !had[f.Name] {
			added = append(added, f.Name)
		}
	}
	gone := map[string]bool{}
	for n := range had {
		if !has[n] {
			gone[n] = true
		}
	}
	if len(added) == 0 && len(gone) == 0 {
		return []string{}, nil
	}
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	changed := []string{}
	for _, old := range routes {
		route, moved := followedRoute(old, added, gone)
		if !moved {
			continue
		}
		if err := a.st.SaveRoute(ctx, route); err != nil {
			return changed, fmt.Errorf("route %q: %w", route.Name, err)
		}
		a.auditUpdate(ctx, actor, "route.update", "route", route.ID, route.Name, "", old, route)
		changed = append(changed, route.Name)
	}
	if len(changed) > 0 {
		if err := a.reloadRouting(ctx); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// followedRoute is followModel on one route, without the store: the route as
// it should be, and whether that differs from what it is.
func followedRoute(r store.Route, added []string, gone map[string]bool) (store.Route, bool) {
	moved := false
	if r.Identity != nil && len(r.Identity.Attributes) > 0 {
		id := *r.Identity
		attrs := make([]store.IdentityAttr, 0, len(id.Attributes)+len(added))
		named := map[string]bool{}
		for _, at := range id.Attributes {
			if gone[at.Field] {
				moved = true
				continue
			}
			named[at.Field] = true
			attrs = append(attrs, at)
		}
		for _, n := range added {
			if !named[n] {
				attrs = append(attrs, store.IdentityAttr{Field: n})
				moved = true
			}
		}
		id.Attributes = attrs
		r.Identity = &id
	}
	if r.UI != nil && r.UI.UserInfo != nil {
		var kept map[string]string
		for field, name := range r.UI.UserInfo.Fields {
			if gone[field] {
				moved = true
				continue
			}
			if kept == nil {
				kept = map[string]string{}
			}
			kept[field] = name
		}
		if moved {
			ui := *r.UI
			info := *ui.UserInfo
			info.Fields = kept
			ui.UserInfo = &info
			r.UI = &ui
		}
	}
	return r, moved
}

// userFields reads the account definitions.
func (a *API) userFields(r *http.Request) []store.UserField {
	return a.fieldDefs(r.Context(), store.SettingUserFields)
}

// fieldDefs reads one list of definitions. An unreadable setting answers an
// empty list rather than an error: a model nobody defined and a model that
// could not be read look the same to a form, and neither is worth refusing a
// page over.
func (a *API) fieldDefs(ctx context.Context, setting string) []store.UserField {
	var fields []store.UserField
	_ = a.st.GetSetting(ctx, setting, &fields)
	if fields == nil {
		return []store.UserField{}
	}
	return fields
}

// checkedFields validates one account's custom values against the definitions
// and returns what to store - which is the definitions' fields and no other:
// a value whose field was removed stops travelling rather than lingering in
// the row, ready to reappear the day that name means something else.
func (a *API) checkedFields(ctx context.Context, values map[string]string) (map[string]string, error) {
	var defs []store.UserField
	_ = a.st.GetSetting(ctx, store.SettingUserFields, &defs)
	if len(defs) == 0 {
		return nil, nil
	}
	return store.ValidateUserValues(defs, values)
}

// checkedTenantFields is checkedFields for an organisation: its values against
// the organisation's definitions, and nothing a definition does not claim.
func (a *API) checkedTenantFields(ctx context.Context, values map[string]string) (map[string]string, error) {
	defs := a.fieldDefs(ctx, store.SettingTenantFields)
	if len(defs) == 0 {
		return nil, nil
	}
	return store.ValidateUserValues(defs, values)
}

// tenantFieldsWrite checks who changes an organisation's values. Root or an
// app-admin, and nobody else - the organisation's own administrators included:
// an upstream TRUSTS these, and `plan=premium` is not something an
// organisation should be able to grant itself. The same split as an account's
// fields, where the person never fills their own.
//
// It refuses a CHANGE, not a payload carrying the values: the General screen
// sends the whole organisation back, and an administrator saving its working
// hours sends its fields unchanged.
func tenantFieldsWrite(actor store.User, before, after map[string]string) error {
	if actor.Root || actor.AppAdmin || maps.Equal(before, after) {
		return nil
	}
	return errors.New("a tenant's fields are set by root or an app-admin: the services behind this gateway trust them, so a tenant does not fill its own")
}
