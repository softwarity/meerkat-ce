package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/signing"
)

// SettingSigningKeys holds the gateway-wide identity signing material (private
// keys included) as a settings blob. It is a SECRET: never returned by an API,
// never logged, and sealed under the master key since v83 (a dump of the
// database used to hand over what it takes to forge an identity every service
// believes). Only the public halves leave, through the JWKS and PEM views.
const SettingSigningKeys = "identity_signing_keys"

// sealedSetting marks a setting stored sealed under the master key, as the
// TOTP secrets are marked: what follows it is the vault cipher's blob.
const sealedSetting = "sealed:"

// setSealed seals plain and stores it under key.
func (s *Store) setSealed(ctx context.Context, key, plain string) error {
	blob, err := s.vaultCipher.Seal(plain)
	if err != nil {
		return fmt.Errorf("store: seal setting %q: %w", key, err)
	}
	return s.SetSetting(ctx, key, sealedSetting+blob)
}

// getSealed reads what setSealed stored. A value written before it was sealed
// (a JSON object, or a string without the marker) comes back as it is: the
// v83 step seals those, and reading one meanwhile weakens nothing.
func (s *Store) getSealed(ctx context.Context, key string) (string, error) {
	var raw json.RawMessage
	if err := s.GetSetting(ctx, key, &raw); err != nil {
		return "", err
	}
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return string(raw), nil
	}
	blob, sealed := strings.CutPrefix(text, sealedSetting)
	if !sealed {
		return text, nil
	}
	plain, err := s.vaultCipher.Open(blob)
	if err != nil {
		return "", fmt.Errorf("store: setting %q: %w", key, err)
	}
	return plain, nil
}

// GetSigningSet loads the identity signing material; ok is false when none was
// ever generated (a fresh install that never used signed-jwt).
func (s *Store) GetSigningSet(ctx context.Context) (set *signing.Set, ok bool, err error) {
	raw, e := s.getSealed(ctx, SettingSigningKeys)
	if e != nil {
		if errors.Is(e, ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, e
	}
	set, err = signing.Load([]byte(raw))
	if err != nil {
		return nil, false, err
	}
	return set, true, nil
}

// EnsureSigningSet returns the signing material, generating and persisting a
// fresh set the first time it is needed.
func (s *Store) EnsureSigningSet(ctx context.Context) (*signing.Set, error) {
	set, ok, err := s.GetSigningSet(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return set, nil
	}
	set, err = signing.Generate()
	if err != nil {
		return nil, err
	}
	if err := s.saveSigningSet(ctx, set); err != nil {
		return nil, err
	}
	return set, nil
}

// RenewSigningKeys rotates every algorithm (the previous public keys linger in
// the JWKS for the grace window) and persists the result.
func (s *Store) RenewSigningKeys(ctx context.Context) (*signing.Set, error) {
	set, err := s.EnsureSigningSet(ctx)
	if err != nil {
		return nil, err
	}
	if err := set.Renew(time.Now(), signing.DefaultGrace); err != nil {
		return nil, err
	}
	if err := s.saveSigningSet(ctx, set); err != nil {
		return nil, err
	}
	return set, nil
}

func (s *Store) saveSigningSet(ctx context.Context, set *signing.Set) error {
	blob, err := set.Marshal()
	if err != nil {
		return err
	}
	return s.setSealed(ctx, SettingSigningKeys, string(blob))
}
