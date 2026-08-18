package admin

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The edition surface: what this installation IS, and the one setting that
// changes its shape.
//
// A single place answers it, because the console has to draw the same thing in
// several screens - a locked control here, a hidden menu entry there - and two
// sources would drift.
func (a *API) registerEdition(mux *http.ServeMux) {
	mux.Handle("GET /api/edition", a.authed(a.getEdition))
	mux.Handle("PUT /api/settings/tenancy", a.rootOnly(a.putTenancy))
}

// editionInfo is what the console needs to know before drawing anything.
type editionInfo struct {
	// Which IMAGE this is, and nothing finer. Meerkat is priced per production
	// instance, so there is no feature list to report: having the Enterprise
	// image is having bought it.
	Enterprise bool   `json:"enterprise"`
	Edition    string `json:"edition"`
	// Tenancy is the current mode, PrimaryTenant the organisation a single
	// installation serves (the console targets it without ever naming it), and
	// HiddenTenants how many are held back by single mode - zero in multi, and
	// what the banner counts otherwise.
	Tenancy        string `json:"tenancy"`
	PrimaryTenant  string `json:"primaryTenant"`
	HiddenTenants  int    `json:"hiddenTenants"`
	TenancyLocked  bool   `json:"tenancyLocked"`
	TenancyLockWhy string `json:"tenancyLockWhy,omitempty"`
}

func (a *API) getEdition(w http.ResponseWriter, r *http.Request, _ store.User) {
	ctx := r.Context()
	info := editionInfo{
		Enterprise: edition.Enterprise,
		Edition:    edition.Name,
		Tenancy:    a.st.Tenancy(ctx),
	}
	if primary, err := a.st.PrimaryTenant(ctx); err == nil {
		info.PrimaryTenant = primary.ID
	}
	if n, err := a.st.CountTenants(ctx); err == nil && info.Tenancy == store.TenancySingle && n > 1 {
		info.HiddenTenants = n - 1
	}
	if err := edition.Require("more than one organisation"); err != nil {
		info.TenancyLocked = true
		info.TenancyLockWhy = err.Error()
	}
	writeJSON(w, http.StatusOK, info)
}

type tenancyPayload struct {
	Tenancy string `json:"tenancy"`
}

// putTenancy switches the shape of the installation, live. Root only, and
// audited like any other mutation.
//
// Going to single with several organisations is ALLOWED and deletes nothing:
// the others stop being served until someone switches back. That is a real
// loss of access for the people in them, which is why the console confirms it
// by naming what disappears - but refusing it would be worse, since the only
// way out of a mistake would then be a restart.
func (a *API) putTenancy(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body tenancyPayload
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	if body.Tenancy == store.TenancyMulti {
		if err := edition.Require("more than one organisation"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	before := a.st.Tenancy(r.Context())
	if err := a.st.SetTenancy(r.Context(), body.Tenancy); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(r.Context(), actor, "tenancy.update", "settings", "", "", "",
		tenancyPayload{Tenancy: before}, body)
	a.getEdition(w, r, actor)
}
