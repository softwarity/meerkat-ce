package store

import (
	"context"
	"database/sql"
	"errors"
)

// SettingLicense is the license a person pasted in the console, kept as the
// file Softwarity issued, byte for byte: its signature covers exactly that, and
// it is checked again on every read rather than trusted once. Shared by every
// node through the database. A license read from MEERKAT_LICENSE_FILE is never
// stored here: the deployment owns that one.
const SettingLicense = "license"

// License returns the stored license file, "" when none was pasted.
func (s *Store) License(ctx context.Context) (string, error) {
	var text string
	if err := s.GetSetting(ctx, SettingLicense, &text); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return text, nil
}

// SetLicense stores a license file. The caller has verified it.
func (s *Store) SetLicense(ctx context.Context, text string) error {
	return s.SetSetting(ctx, SettingLicense, text)
}
