package admin

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/store"
)

// The edition surface: what this installation IS, and the one setting that
// changes its shape.
//
// A single place answers it, because the console has to draw the same thing in
// several screens - a locked control here, a hidden menu entry there - and two
// sources would drift.
func (a *API) registerEdition(mux Mux) {
	mux.Handle("GET /api/edition", a.authed(a.getEdition))
	// App-admin: single or multi organisation is the SHAPE of the identity -
	// who belongs where - and the capability that holds users, roles and
	// organisations is the one that decides it.
	mux.Handle("PUT /api/settings/tenancy", a.appAdmin(a.putTenancy))
	// Root: the license is the installation owner's contract.
	mux.Handle("PUT /api/license", a.rootOnly(a.putLicense))
}

// editionInfo is what the console needs to know before drawing anything.
type editionInfo struct {
	// Which IMAGE this is, and nothing finer. Meerkat is priced per production
	// instance, so nothing is unlocked one by one: having the Enterprise image
	// is having bought it. Features below only NAMES what that image carries.
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
	// DataOrigin is where the APPLICATIONS answer, scheme and host. The console
	// is served by the other plane and cannot work it out from its own address:
	// a published port or an ingress sits between the two, and the browser only
	// ever sees the admin one. It is the same answer the auth authorities are
	// given for their callbacks, from the same place.
	DataOrigin string `json:"dataOrigin"`
	// InternalPort is the internal plane's (:7070): where the services behind
	// the gateway reach the JWKS and the scheduler's API. A port and not an
	// address - the gateway's name inside a cluster is the deployment's.
	InternalPort string `json:"internalPort"`
	// InternalOrigin is the internal plane as the runtime names it -
	// http://<release>-internal.<namespace>.svc.cluster.local:7070, a Swarm
	// or compose service - empty when no runtime could be asked.
	InternalOrigin string `json:"internalOrigin,omitempty"`
	// InternalNames are every name the runtime resolves to that plane, the
	// advertised one first: what the TLS screen checks a certificate against
	// before it is placed there.
	InternalNames []string `json:"internalNames,omitempty"`
	// InternalSource is which runtime named it: kubernetes, openshift, swarm,
	// compose, docker - what the TLS screen's how-to opens on.
	InternalSource string `json:"internalSource,omitempty"`
	// ConfigurationCap is how many saved configurations this image keeps at
	// once, 0 for no limit (CFG-01). Said up front - "2 of 3" - because a cap
	// discovered by being refused is a trap, and a cap announced is a price.
	ConfigurationCap int `json:"configurationCap"`
	// Features are the Enterprise rows of FEATURES.md, as the binary carries
	// it (CONSOLE-14): the License screen names each and says where it lives.
	Features []EditionFeature `json:"features"`
	// Notice is the evaluation image's sentence, empty in both shipped images.
	// It is not a flag and nothing reads it as one: it is the text itself,
	// filled by ee/eval the same way the pages and the console's watermark are
	// (EVAL-01), and the License screen shows it INSTEAD of the perpetual
	// licence wording - which would be a lie on a build that is licensed for
	// evaluation only.
	Notice string `json:"notice,omitempty"`
	// License is the verified license in force, LicenseSource where it comes
	// from ("file" when MEERKAT_LICENSE_FILE is set, "console" when pasted),
	// and LicenseProblem why the one held is refused. See license.go.
	License        *licenseInfo `json:"license,omitempty"`
	LicenseSource  string       `json:"licenseSource,omitempty"`
	LicenseProblem string       `json:"licenseProblem,omitempty"`
	// MailReady says an administrator can mail a person their way in - an
	// invitation, a reset link (AUTH-25) - and MailWhy, when not, why: the
	// sentence beside the option the console greys out.
	MailReady bool   `json:"mailReady"`
	MailWhy   string `json:"mailWhy,omitempty"`
}

// EditionFeature is one Enterprise row of the contract.
type EditionFeature struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Edition string `json:"edition"`
}

func (a *API) getEdition(w http.ResponseWriter, r *http.Request, _ store.User) {
	ctx := r.Context()
	info := editionInfo{
		Enterprise:   edition.Enterprise,
		Edition:      edition.Name,
		Tenancy:      a.st.Tenancy(ctx),
		DataOrigin:   a.dataOrigin(r),
		InternalPort: portOf(a.InternalAddr),
		Features:     a.EnterpriseFeatures,
		Notice:       evalmark.MailNote,
	}
	if edition.Enterprise {
		info.License, info.LicenseSource, info.LicenseProblem = a.currentLicense(ctx)
	}
	_, info.MailWhy = a.accessMailOrigin(ctx)
	info.MailReady = info.MailWhy == ""
	in := a.internalOf(ctx)
	info.InternalOrigin, info.InternalNames, info.InternalSource = in.Origin, in.Names, in.Source
	if info.Features == nil {
		info.Features = []EditionFeature{}
	}
	if !edition.Enterprise {
		info.ConfigurationCap = FreeConfigurations
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

// putTenancy switches the shape of the installation, live. The app-admin's
// decision (see the registration), and audited like any other mutation.
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
