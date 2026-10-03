package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SettingInstallationID names THIS installation: generated once, on the first
// start, and shared by every node through the database.
//
// What it is for is the browser's side of things: cookies are scoped to a
// host and never to a port, so two gateways a browser reaches under one name
// - an Enterprise and a community one side by side on localhost - each
// overwrote the other's session. Suffixed with this, their cookies stop
// meeting, with nothing to configure.
//
// It is NOT a setting of the configuration and is never exported: a document
// moved to another gateway must not make that gateway's cookies the same as
// this one's. A snapshot restored elsewhere does carry it, which only brings
// back the situation before this existed.
const SettingInstallationID = "installation_id"

// InstallationID returns this installation's identifier, creating it on the
// first call. Two nodes starting together on an empty database both try; the
// first insert wins and both read the same value back.
func (s *Store) InstallationID(ctx context.Context) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: installation id: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO NOTHING`,
		SettingInstallationID, `"`+hex.EncodeToString(b[:])+`"`); err != nil {
		return "", fmt.Errorf("store: installation id: %w", err)
	}
	var id string
	if err := s.GetSetting(ctx, SettingInstallationID, &id); err != nil {
		return "", err
	}
	return id, nil
}
