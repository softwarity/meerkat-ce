package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// Seeding from a file at startup (CFG-03).
//
// The file is a DOOR IN, never a master. It seeds a gateway that holds no
// configuration yet, and from then on the console is the source of truth: a
// gateway that has been configured is not silently rewritten every time it
// restarts, which is what would make the console a lie.
//
// This is the first form of CFG-03. Storing a file that DIFFERS as an available
// configuration to inspect and activate needs several configurations to coexist
// (CFG-01/02), which is not built yet; until then a changed file is announced
// in the log and left alone.

// SeedMark records that a file was seeded, and which one. Not exported in a
// configuration: it describes THIS install's history, not how it is set up.
type SeedMark struct {
	SHA256 string `json:"sha256"`
	At     int64  `json:"at"`
	Path   string `json:"path"`
}

// Seed applies path to an uninitialised gateway and reports whether it did.
//
// It refuses to act on a gateway that already carries routes, even when no mark
// is stored: a file dropped next to a gateway that has been running for months
// is someone's bootstrap left behind, not an instruction.
func Seed(ctx context.Context, st *store.Store, path string, now int64) (bool, error) {
	if path == "" {
		return false, nil
	}
	body, err := os.ReadFile(path) //nolint:gosec // the path is the operator's own flag
	if err != nil {
		// Asked for explicitly and unreadable: failing here beats starting a
		// gateway that silently is not the one the operator described.
		return false, fmt.Errorf("config: seed file %s: %w", path, err)
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])

	var mark SeedMark
	if err := st.GetSetting(ctx, store.SettingConfigSeed, &mark); err == nil {
		if mark.SHA256 == digest {
			slog.Debug("configuration file already seeded, ignoring", "file", path)
			return false, nil
		}
		slog.Info("the configuration file has changed since it seeded this gateway: "+
			"the console is the source of truth from the first start on, so it is not applied",
			"file", path, "seeded", mark.At)
		offerFile(ctx, st, path, body, now)
		return false, nil
	}

	routes, err := st.CountRoutes(ctx)
	if err != nil {
		return false, err
	}
	if routes > 0 {
		slog.Info("this gateway is already configured, the configuration file is not applied: "+
			"a file seeds an EMPTY gateway and never overwrites one",
			"file", path, "routes", routes)
		offerFile(ctx, st, path, body, now)
		return false, nil
	}

	doc, err := Unmarshal(body)
	if err != nil {
		return false, fmt.Errorf("%w (%s)", err, path)
	}
	plan, err := Apply(ctx, st, doc, false)
	if err != nil {
		return false, fmt.Errorf("config: seed from %s: %w", path, err)
	}
	if err := st.SetSetting(ctx, store.SettingConfigSeed, SeedMark{
		SHA256: digest, At: now, Path: path,
	}); err != nil {
		return false, err
	}
	slog.Info("first start: seeded from the configuration file",
		"file", path, "objects", len(plan.Changes), "vault entries reserved", len(plan.Missing))
	for _, m := range plan.Missing {
		// Worth a line each: the gateway is up but these are empty, and the
		// admin has to know which ones before wondering why a route fails.
		slog.Warn("the configuration expects a vault entry that does not exist, reserved empty",
			"name", m.Name, "kind", m.Kind, "used by", m.Used)
	}
	return true, nil
}

// offerFile shelves a configuration file the gateway did not apply (CFG-03,
// LIFE-02): a file that changed since it seeded this gateway, or one handed to
// a gateway already configured. Applying it would overwrite what the console
// made; ignoring it, as before, left it a line in a log that nobody reads, and
// the operator who edited the file believing it would land found nothing.
// Shelved, it is a SAVED CONFIGURATION on the Configuration screen: compared
// with what runs, set as current by someone who decides to, and never applied
// behind anyone's back.
//
// Offered once per content: a file already on the shelf, byte for byte once
// normalised, is not added twice. Best effort - a start never fails because a
// file could not be shelved; the reason is logged instead.
func offerFile(ctx context.Context, st *store.Store, path string, body []byte, now int64) {
	doc, err := Unmarshal(body)
	if err != nil {
		slog.Warn("the configuration file is not a configuration, it was not shelved", "file", path, "err", err)
		return
	}
	normal, err := Marshal(doc)
	if err != nil {
		slog.Warn("the configuration file could not be shelved", "file", path, "err", err)
		return
	}
	digest := store.DigestOf(string(normal))
	shelf, err := st.ListConfigurations(ctx)
	if err != nil {
		slog.Warn("the configuration file could not be shelved", "file", path, "err", err)
		return
	}
	for _, c := range shelf {
		if c.Digest == digest {
			slog.Debug("the configuration file is already on the shelf", "file", path, "as", c.Name)
			return
		}
	}
	if !edition.Enterprise && len(shelf) >= store.FreeConfigurations {
		slog.Warn("the configuration file differs from what runs, and the shelf is full: delete a saved "+
			"configuration to have it offered at the next start",
			"file", path, "saved", len(shelf), "limit", store.FreeConfigurations)
		return
	}
	c := store.Configuration{
		ID:   store.NewEventID(),
		Name: fmt.Sprintf("%s (%s)", filepath.Base(path), digest[:8]),
		Description: "Offered at startup from " + path + ": it differs from what this gateway runs. " +
			"Compare it, or set it as current.",
		Document: string(normal),
	}
	if err := st.SaveConfiguration(ctx, &c); err != nil {
		slog.Warn("the configuration file could not be shelved", "file", path, "err", err)
		return
	}
	_ = st.AddAuditEvent(ctx, store.AuditEvent{
		At: now, Action: "configuration.offer", Target: "configuration", TargetID: c.ID, TargetName: c.Name,
		Detail: "from " + path,
	})
	slog.Info("the configuration file was shelved as a saved configuration", "file", path, "as", c.Name)
}
