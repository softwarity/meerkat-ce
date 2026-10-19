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
// Nothing is switched on or off by it yet: the image is still what decides
// the edition. It is read, verified and shown, so that the day it removes the
// evaluation notice, the customers already hold it in the right place.

// licenseInfo is what the screen shows of a verified license. Never the file:
// it holds nothing secret, but there is no reason to send it back either.
type licenseInfo struct {
	ID           string    `json:"id,omitempty"`
	Licensee     string    `json:"licensee"`
	Plan         string    `json:"plan,omitempty"`
	IssuedAt     time.Time `json:"issuedAt"`
	UpdatesUntil time.Time `json:"updatesUntil"`
	// Covered says whether THIS build was released within the updates the
	// license paid for. Absent when the build carries no date (a dev build).
	// False switches nothing off: it is said, and that is all.
	Covered *bool `json:"covered,omitempty"`
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
	case errors.Is(err, license.ErrNotYet):
		return nil, errors.New("it is not valid yet: its issue date is in the future")
	case err != nil:
		return nil, fmt.Errorf("it is not a license file (%s)", strings.TrimPrefix(err.Error(), "license: "))
	}
	info := &licenseInfo{ID: lic.ID, Licensee: lic.Licensee, Plan: lic.Plan,
		IssuedAt: lic.IssuedAt, UpdatesUntil: lic.ExpiresAt}
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

// LogLicense says at start what license this node holds, or why the one it was
// given is refused. A refused license stops nothing: it is a line to read.
func (a *API) LogLicense(ctx context.Context) {
	if !edition.Enterprise {
		return
	}
	info, source, problem := a.currentLicense(ctx)
	switch {
	case problem != "":
		slog.Warn("license", "problem", problem)
	case info != nil:
		slog.Info("license", "licensee", info.Licensee, "id", info.ID, "from", source,
			"updatesUntil", info.UpdatesUntil.Format(time.DateOnly))
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
	a.getEdition(w, r, actor)
}
