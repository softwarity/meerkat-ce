package admin

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// The license, as the License screen shows and takes it.
//
// Two places hold one, and only one of them speaks at a time. A deployment
// that mounts the file (MEERKAT_LICENSE_FILE, from a Secret) owns it: the
// console shows it and does not take another, since a pasted one would be
// undone by the next restart of a pod nobody looked at. Without the file, the
// license is the one pasted in the console, stored for every node.
//
// What it switches is held in internal/license, and put there from here: at
// start, on every look, on a save, and when another node saved one.

// watchInterval is how often WatchLicense looks. A mounted Secret has nobody to
// announce it, and the instances running are only known by counting them.
const watchInterval = 30 * time.Second

// licenseInfo is what the screen shows of a verified license. Never the file:
// it holds nothing secret, but there is no reason to send it back either.
type licenseInfo struct {
	ID       string `json:"id,omitempty"`
	Licensee string `json:"licensee"`
	Plan     string `json:"plan,omitempty"`
	// Tenants and Instances are its ceilings, absent for none.
	Tenants      int       `json:"tenants,omitempty"`
	Instances    int       `json:"instances,omitempty"`
	IssuedAt     time.Time `json:"issuedAt"`
	UpdatesUntil time.Time `json:"updatesUntil"`
	// Covered says whether THIS build was released within the updates the
	// license paid for. Absent when the build carries no date (a dev build).
	// False makes this build an evaluation again: see lapse.
	Covered *bool `json:"covered,omitempty"`
	// Ends says UpdatesUntil is a real end: a test license.
	Ends bool `json:"ends,omitempty"`
	// Lapsed says this gateway does not honour it, for the reason in lapse.
	Lapsed bool `json:"lapsed,omitempty"`
	lic    *license.License
}

// lapse says in a sentence why this build does not honour a license that
// verifies, "" while it does. It is worked out on every look and written
// nowhere: a build from the term, or a renewal, puts the license back by itself.
func (i *licenseInfo) lapse(now time.Time) string {
	until := i.UpdatesUntil.Format(time.DateOnly)
	switch {
	case i.lic.Ended(now):
		return fmt.Sprintf("this trial license ended on %s: this gateway is an evaluation again until it is replaced", until)
	case i.Covered != nil && !*i.Covered:
		return fmt.Sprintf("this version was built after the updates of this license ended on %s: "+
			"this gateway is an evaluation again until it runs a version built by then, or the license is renewed", until)
	}
	return ""
}

// licenseFileVar names the variable in what the console and the log say.
const licenseFileVar = "MEERKAT_LICENSE_FILE"

func (a *API) licenseKeys() []ed25519.PublicKey {
	if a.LicenseKeys != nil {
		return a.LicenseKeys
	}
	return license.ProductionKeys()
}

// verifyLicense checks a license file and says, in a sentence a person can act
// on, why it is refused.
func (a *API) verifyLicense(data []byte) (*licenseInfo, error) {
	keys := a.licenseKeys()
	if len(keys) == 0 {
		return nil, errors.New("this build carries no Softwarity signing key yet, so no license can be verified")
	}
	lic, err := license.Parse(data, keys, time.Now())
	switch {
	case errors.Is(err, license.ErrSignature):
		return nil, errors.New("its signature is not Softwarity's: the file was changed after it was issued, " +
			"or it is not a Meerkat license")
	case errors.Is(err, license.ErrProduct):
		return nil, errors.New("it was issued for another product")
	case errors.Is(err, license.ErrNotYet):
		return nil, errors.New("it is not valid yet: its issue date is in the future")
	case err != nil:
		return nil, fmt.Errorf("it is not a license file (%s)", strings.TrimPrefix(err.Error(), "license: "))
	}
	info := &licenseInfo{ID: lic.ID, Licensee: lic.Licensee, Plan: lic.Edition, Tenants: lic.Tenants,
		Instances: lic.Instances, IssuedAt: lic.IssuedAt, UpdatesUntil: lic.ExpiresAt, Ends: lic.Hard(), lic: lic}
	if built, err := time.Parse(time.RFC3339, version.Date); err == nil {
		covered := lic.Covered(built)
		info.Covered = &covered
	}
	return info, nil
}

// currentLicense is the license in force and where it comes from ("file",
// "console", or "" for none), or the reason the one held is refused. The file
// is read on every look: a Secret updated in place is seen without a restart.
func (a *API) currentLicense(ctx context.Context) (info *licenseInfo, source, problem string) {
	if a.LicenseFile != "" {
		data, err := os.ReadFile(a.LicenseFile)
		if err != nil {
			return nil, "file", fmt.Sprintf("%s (%s) cannot be read: %v", licenseFileVar, a.LicenseFile, err)
		}
		info, err := a.verifyLicense(data)
		if err != nil {
			return nil, "file", fmt.Sprintf("the license in %s is refused: %v", licenseFileVar, err)
		}
		return info, "file", ""
	}
	text, err := a.st.License(ctx)
	if err != nil || text == "" {
		return nil, "", ""
	}
	info, err = a.verifyLicense([]byte(text))
	if err != nil {
		return nil, "console", "the license pasted here is refused: " + err.Error()
	}
	return info, "console", ""
}

// ReloadLicense puts the license this node holds in force: read again, verified
// again. A refused one, or one this build does not honour, brings the evaluation
// notice back and stops nothing.
func (a *API) ReloadLicense(ctx context.Context) error {
	if !edition.Enterprise {
		return nil
	}
	info, _, _ := a.currentLicense(ctx)
	a.observeTenants(ctx)
	apply(info)
	return nil
}

// observeTenants tells the license how many tenants are served. Single-tenant
// mode serves the first and holds the others back, which is what a license for
// one tenant covers: it is the way down that deletes nothing.
func (a *API) observeTenants(ctx context.Context) {
	if a.st.Tenancy(ctx) != store.TenancyMulti {
		license.ObserveTenants(1)
		return
	}
	if n, err := a.st.CountTenants(ctx); err == nil {
		license.ObserveTenants(n)
	}
}

func apply(info *licenseInfo) {
	if info == nil || info.lapse(time.Now()) != "" {
		license.Apply(nil)
		return
	}
	// Applying again what is in force would restart the count of an overrun.
	if held := license.Current(); held == nil || *held != *info.lic {
		license.Apply(info.lic)
	}
}

// WatchLicense follows what no announcement reaches: a Secret updated in place,
// the end of a test license, how many instances are running, and how many
// tenants are served.
func (a *API) WatchLicense(ctx context.Context, running func() int) {
	if !edition.Enterprise {
		return
	}
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			_ = a.ReloadLicense(ctx)
			before := license.OverSince()
			license.Observe(running(), now)
			// The Infrastructure screen says when the notice comes back: it
			// hears that an overrun began, or ended, here.
			if !license.OverSince().Equal(before) && a.InstancesMoved != nil {
				a.InstancesMoved()
			}
		}
	}
}

// LogLicense puts the license in force at start and says what it is, or why
// the one this node was given is refused. A refused license stops nothing: it
// is a line to read.
func (a *API) LogLicense(ctx context.Context) {
	if !edition.Enterprise {
		return
	}
	info, source, problem := a.currentLicense(ctx)
	a.observeTenants(ctx)
	apply(info)
	switch {
	case problem != "":
		slog.Warn("license", "problem", problem)
	case info != nil:
		slog.Info("license", "licensee", info.Licensee, "id", info.ID, "from", source,
			"updatesUntil", info.UpdatesUntil.Format(time.DateOnly))
		if lapse := info.lapse(time.Now()); lapse != "" {
			slog.Warn("license", "problem", lapse)
		}
	}
}

type licensePayload struct {
	License string `json:"license"`
}

// licenseAudit is what the trail keeps of a license: who and which, never the
// file.
type licenseAudit struct {
	ID           string `json:"id"`
	Licensee     string `json:"licensee"`
	Plan         string `json:"plan"`
	UpdatesUntil string `json:"updatesUntil"`
}

func auditOfLicense(info *licenseInfo) licenseAudit {
	if info == nil {
		return licenseAudit{}
	}
	return licenseAudit{ID: info.ID, Licensee: info.Licensee, Plan: info.Plan,
		UpdatesUntil: info.UpdatesUntil.Format(time.DateOnly)}
}

// putLicense takes a license pasted, dropped or chosen in the console. Root's:
// it is the installation owner's contract.
func (a *API) putLicense(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require("a license"); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	if a.LicenseFile != "" {
		writeErr(w, http.StatusConflict, fmt.Sprintf("this gateway reads its license from %s (%s): "+
			"replace that file (its Secret) to change it - one pasted here would be undone at the next start",
			licenseFileVar, a.LicenseFile))
		return
	}
	var body licensePayload
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed license: "+err.Error())
		return
	}
	text := strings.TrimSpace(body.License)
	if text == "" {
		writeErr(w, http.StatusUnprocessableEntity, "no license given: paste the file Softwarity sent, or drop it")
		return
	}
	info, err := a.verifyLicense([]byte(text))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "the license is refused: "+err.Error())
		return
	}
	before, _, _ := a.currentLicense(r.Context())
	if err := a.st.SetLicense(r.Context(), text); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditUpdate(r.Context(), actor, "license.update", "license", info.ID, info.Licensee, "",
		auditOfLicense(before), auditOfLicense(info))
	a.announce(r.Context(), store.TopicLicense)
	a.getEdition(w, r, actor)
}

// deleteLicense takes the pasted license away, which is all it does: the image
// goes back to what it is without one, its notice included, and what the license
// allowed stays where it is. A license refused on reading can be taken away too:
// it is the one a person most wants gone. Root's, like the paste.
func (a *API) deleteLicense(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require("a license"); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	if a.LicenseFile != "" {
		writeErr(w, http.StatusConflict, fmt.Sprintf("this gateway reads its license from %s (%s): "+
			"remove that file (its Secret) from the deployment, there is nothing to remove here",
			licenseFileVar, a.LicenseFile))
		return
	}
	before, source, _ := a.currentLicense(r.Context())
	if source == "" {
		a.getEdition(w, r, actor)
		return
	}
	if err := a.st.DeleteLicense(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	held := auditOfLicense(before)
	a.auditEvent(r.Context(), actor, "license.delete", "license", held.ID, held.Licensee, "", "")
	a.announce(r.Context(), store.TopicLicense)
	a.getEdition(w, r, actor)
}
