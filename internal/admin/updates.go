package admin

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// Whether a newer release exists (CONSOLE-16).
//
// OFF UNTIL ROOT TURNS IT ON, because the site promises that a gateway calls
// nobody, and an isolated installation must have nothing to do to keep it true.
// Turned on, it is one GET of a public file, once a day, by one node: no
// version, no identifier, nothing about this installation in the request.
//
// The backend asks and not the browser: a request that fails in a page is an
// error in the browser's console, which no code can silence, and a gateway
// with no way out would print one on every visit.
const (
	defaultUpdateFeed = "https://meerkat.softwarity.dev/releases.json"
	// updateEvery is how often the list is read, updateWatch how often a node
	// asks itself whether that is due. A restart must not read it again, and
	// the date of the last read is what says so.
	updateEvery = 24 * time.Hour
	updateWatch = time.Hour
	updateLock  = "update-check"
	// updateFeedLimit bounds what is read of the list: its notes are a hundred
	// kilobytes today, and a public address is no reason to trust a size.
	updateFeedLimit = 8 << 20
)

// updateClient goes through the environment's proxy like the default client,
// and gives up on a network that swallows packets.
var updateClient = &http.Client{Timeout: 15 * time.Second}

func (a *API) registerUpdates(mux Mux) {
	mux.Handle("GET /api/updates", a.infraAdmin(a.getUpdates))
	// Root: whether this gateway reaches out at all is the owner's decision.
	mux.Handle("PUT /api/updates", a.rootOnly(a.putUpdates))
}

// updatesInfo is what the Release notes screen shows, and what lights the
// console's indicator.
type updatesInfo struct {
	Enabled   bool       `json:"enabled"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
	Error     string     `json:"error,omitempty"`
	// Releases are the ones newer than this gateway, newest first.
	Releases []availableRelease `json:"releases"`
	// LicenseUntil is how far the license's updates go, absent without one.
	LicenseUntil *time.Time `json:"licenseUntil,omitempty"`
}

// availableRelease is a release, and what installing it would do to the license.
type availableRelease struct {
	store.UpdateRelease
	// OutOfLicense says it was built after the license's date: installed, it
	// runs as an evaluation (license.Covered).
	OutOfLicense bool `json:"outOfLicense,omitempty"`
}

type updatesPayload struct {
	Enabled bool `json:"enabled"`
}

func (a *API) getUpdates(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.updatesFor(r.Context()))
}

func (a *API) putUpdates(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body updatesPayload
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	ctx := r.Context()
	before := a.st.GetUpdateCheck(ctx).Enabled
	if body.Enabled != before {
		// Both ways it starts clean: off forgets what was found, so no
		// indicator outlives the switch.
		if err := a.st.SetUpdateCheck(ctx, store.UpdateCheck{Enabled: body.Enabled}); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.auditUpdate(ctx, actor, "updates.update", "settings", "", "", "",
			updatesPayload{Enabled: before}, body)
		a.tellUpdates(ctx)
		if body.Enabled {
			a.lookForUpdates(ctx)
		}
	}
	writeJSON(w, http.StatusOK, a.updatesFor(ctx))
}

// updatesFor filters on the running version at every read: a gateway updated an
// hour ago must not offer itself what it now runs.
func (a *API) updatesFor(ctx context.Context) updatesInfo {
	held := a.st.GetUpdateCheck(ctx)
	info := updatesInfo{Enabled: held.Enabled, Error: held.Error, Releases: []availableRelease{}}
	if !held.CheckedAt.IsZero() {
		info.CheckedAt = &held.CheckedAt
	}
	var lic *license.License
	if edition.Enterprise {
		if current, _, _ := a.currentLicense(ctx); current != nil {
			lic = current.lic
			info.LicenseUntil = &lic.ExpiresAt
		}
	}
	for _, r := range newerThan(held.Releases, a.runningRelease()) {
		info.Releases = append(info.Releases, availableRelease{
			UpdateRelease: r,
			OutOfLicense:  lic != nil && r.Built != nil && !lic.Covered(*r.Built),
		})
	}
	return info
}

// runningRelease is the version to compare with: this binary's, or for a
// development build the last release its notes carry, the same answer the
// Release notes screen gives.
func (a *API) runningRelease() string {
	return notesFor(version.Version, a.ReleaseNotes).Version
}

// newerThan keeps the releases past running, newest first. Nothing without a
// version to compare with: an indicator that cannot be right stays off.
func newerThan(releases []store.UpdateRelease, running string) []store.UpdateRelease {
	major, minor, patch, ok := semver(running)
	if !ok {
		return nil
	}
	here := [3]int{major, minor, patch}
	var newer []store.UpdateRelease
	for _, r := range releases {
		if x, y, z, ok := semver(r.Version); ok && compareVersions([3]int{x, y, z}, here) > 0 {
			newer = append(newer, r)
		}
	}
	slices.SortFunc(newer, func(p, q store.UpdateRelease) int {
		a, b, c, _ := semver(p.Version)
		x, y, z, _ := semver(q.Version)
		return compareVersions([3]int{x, y, z}, [3]int{a, b, c})
	})
	return newer
}

func compareVersions(p, q [3]int) int {
	return cmp.Or(cmp.Compare(p[0], q[0]), cmp.Compare(p[1], q[1]), cmp.Compare(p[2], q[2]))
}

// WatchUpdates reads the list when it is due, on one node. It looks at the
// setting every time and reaches out only if root turned it on.
func (a *API) WatchUpdates(ctx context.Context) {
	t := time.NewTicker(updateWatch)
	defer t.Stop()
	for {
		a.lookForUpdatesIfDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *API) lookForUpdatesIfDue(ctx context.Context) {
	if held := a.st.GetUpdateCheck(ctx); held.Enabled && time.Since(held.CheckedAt) >= updateEvery {
		a.lookForUpdates(ctx)
	}
}

// lookForUpdates reads the list and keeps what it says. A node that finds the
// lock taken has nothing to add: the other one tells every console when it ends.
func (a *API) lookForUpdates(ctx context.Context) {
	_, err := a.st.TryLock(ctx, updateLock, func(ctx context.Context) error {
		held := a.st.GetUpdateCheck(ctx)
		if !held.Enabled {
			return nil
		}
		releases, err := a.readUpdateFeed(ctx)
		// Read again: root may have turned it off during a slow answer.
		now := a.st.GetUpdateCheck(ctx)
		if !now.Enabled {
			return nil
		}
		now.CheckedAt, now.Error = time.Now().UTC(), ""
		if err != nil {
			// A line on the screen, and Info here: a gateway with no way out
			// is a normal thing to be, and not worth a warning a day.
			slog.Info("update check", "problem", err)
			now.Error = err.Error()
		} else {
			now.Releases = releases
		}
		if err := a.st.SetUpdateCheck(ctx, now); err != nil {
			return err
		}
		a.tellUpdates(ctx)
		return nil
	})
	if err != nil {
		slog.Error("update check", "err", err)
	}
}

// readUpdateFeed asks for the list and keeps what is newer than this gateway:
// the list holds every release's notes, and the setting has no use for those.
func (a *API) readUpdateFeed(ctx context.Context) ([]store.UpdateRelease, error) {
	feed := a.UpdateFeed
	if feed == "" {
		feed = defaultUpdateFeed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return nil, fmt.Errorf("the update list %s: %w", feed, err)
	}
	res, err := updateClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the update list could not be read: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the update list %s answered %s", feed, res.Status)
	}
	var list struct {
		Releases []store.UpdateRelease `json:"releases"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, updateFeedLimit)).Decode(&list); err != nil {
		return nil, fmt.Errorf("the update list %s is not a list of releases: %w", feed, err)
	}
	return newerThan(list.Releases, a.runningRelease()), nil
}

// tellUpdates wakes the consoles held by this node and by the others. The rows
// are the truth, so a lost signal costs a screen one reload.
func (a *API) tellUpdates(ctx context.Context) {
	if a.Bus != nil {
		a.Bus.Signal(ctx, store.TopicUpdates, "")
	}
	if a.UpdatesMoved != nil {
		a.UpdatesMoved()
	}
}
