package store

import (
	"context"
	"time"
)

// SettingUpdateCheck is whether this gateway looks for newer releases, and what
// it last found (CONSOLE-16). Off until root turns it on.
const SettingUpdateCheck = "update_check"

// UpdateRelease is one release as the public list describes it.
type UpdateRelease struct {
	Version string `json:"version"`
	// Built is the date inside the binary, the one a license is compared with.
	// Absent for a release older than the list knowing it.
	Built *time.Time `json:"built,omitempty"`
	// Migrates says it changes the database schema: an older build no longer
	// opens the database afterwards.
	Migrates bool   `json:"migrates,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// UpdateCheck is the setting: one row, so every node reads what one of them
// found.
type UpdateCheck struct {
	Enabled bool `json:"enabled"`
	// CheckedAt is the last attempt, and Error why it failed. What an earlier
	// attempt found stays in Releases: a release does not stop existing because
	// the network went away.
	CheckedAt time.Time       `json:"checkedAt"`
	Error     string          `json:"error,omitempty"`
	Releases  []UpdateRelease `json:"releases,omitempty"`
}

// GetUpdateCheck reads the setting. Unreadable counts as off: nothing leaves
// this gateway on a doubt.
func (s *Store) GetUpdateCheck(ctx context.Context) UpdateCheck {
	var c UpdateCheck
	if err := s.GetSetting(ctx, SettingUpdateCheck, &c); err != nil {
		return UpdateCheck{}
	}
	return c
}

// SetUpdateCheck writes the setting.
func (s *Store) SetUpdateCheck(ctx context.Context, c UpdateCheck) error {
	return s.SetSetting(ctx, SettingUpdateCheck, c)
}
