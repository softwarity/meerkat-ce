package admin

import (
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// The gateway's own bounds (PERF-02): what it will hold in memory while it
// proxies. Its own endpoint under infraAdmin rather than a field in the
// application settings blob, for the reason the TLS, relay and issues settings
// are here too - this is a property of the INSTALLATION. An application
// administrator decides who may reach what; how much memory the proxy is
// allowed to spend is not theirs to move.
func (a *API) registerProxyLimits(mux Mux) {
	mux.Handle("GET /api/settings/proxy", a.infraAdmin(a.getProxyLimits))
	mux.Handle("PUT /api/settings/proxy", a.infraAdmin(a.putProxyLimits))
	mux.Handle("GET /api/settings/maintenance", a.infraAdmin(a.getMaintenance))
	mux.Handle("PUT /api/settings/maintenance", a.infraAdmin(a.putMaintenance))
}

// The global maintenance switch (LIFE-05). Same perimeter as the limits above
// and for the same reason: taking every application down is an operator's act,
// not an application administrator's.
func (a *API) getMaintenance(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.st.GetMaintenance(r.Context()))
}

func (a *API) putMaintenance(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body store.Maintenance
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	before := a.st.GetMaintenance(r.Context())
	if err := store.SanitizeMaintenance(&body, before, time.Now()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SetSetting(r.Context(), store.SettingMaintenance, body); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	// Loudly in the audit trail, because this is the one setting whose effect
	// every visitor sees at once, and the question afterwards is always who
	// and when.
	a.auditUpdate(r.Context(), actor, "maintenance", "settings", "", "", "", before, body)
	writeJSON(w, http.StatusOK, body)
}

func (a *API) getProxyLimits(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.st.GetProxyLimits(r.Context()))
}

func (a *API) putProxyLimits(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body store.ProxyLimits
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	if err := store.SanitizeProxyLimits(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	before := a.st.GetProxyLimits(r.Context())
	if err := a.st.SetSetting(r.Context(), store.SettingProxyLimits, body); err != nil {
		a.internal(w, err)
		return
	}
	// The compiled routes carry the ceiling, so the reload is what makes the
	// new number the one in force - here, and on every other node, since
	// reloadRouting announces it.
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "proxy.limits", "settings", "", "", "", before, body)
	writeJSON(w, http.StatusOK, body)
}
