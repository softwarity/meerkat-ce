package admin

import (
	"context"
	"net/http"

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
	before := a.userFields(r)
	if err := a.st.SetSetting(r.Context(), store.SettingUserFields, fields); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "model.user-fields", "settings", "", "", "",
		map[string]any{"fields": before}, map[string]any{"fields": fields})
	writeJSON(w, http.StatusOK, map[string]any{"fields": fields})
}

// userFields reads the definitions. An unreadable setting answers an empty
// list rather than an error: a model nobody defined and a model that could not
// be read look the same to a form, and neither is worth refusing a page over.
func (a *API) userFields(r *http.Request) []store.UserField {
	var fields []store.UserField
	_ = a.st.GetSetting(r.Context(), store.SettingUserFields, &fields)
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
