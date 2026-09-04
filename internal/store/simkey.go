package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// SettingSimulationKey holds the HMAC key that signs the ephemeral test
// tokens of the routing simulator (DEV-10).
//
// A SECRET, on the same terms as SettingSigningKeys: never returned by an API,
// never logged. Whoever holds it can mint a token naming any username and any
// role, and the data plane will believe it.
//
// It lives in the database because it used to be drawn at every boot, and that
// is a bug the moment there are two gateways: a token minted by one node is
// gibberish to the other, so the simulator - and the agent's test_routing -
// work or fail depending on which node the load balancer picked. Per boot also
// meant every restart invalidated the token an operator had just copied.
const SettingSimulationKey = "simulation_token_key"

// simulationKeyLock is the name the nodes agree on while one of them is
// creating the key. Without it two gateways starting together both generate,
// the second overwrites the first, and the tokens the first has already handed
// out stop working.
const simulationKeyLock = "simulation-key"

// EnsureSimulationKey returns the key, creating it the first time it is asked
// for. 32 bytes, which is the block size of the SHA-256 the tokens are signed
// with.
func (s *Store) EnsureSimulationKey(ctx context.Context) ([]byte, error) {
	if key, err := s.simulationKey(ctx); err == nil {
		return key, nil
	} else if !errors.Is(err, ErrNoRows) {
		return nil, err
	}
	var key []byte
	err := s.WithLock(ctx, simulationKeyLock, func(ctx context.Context) error {
		// Read again under the lock: whoever held it before us has just
		// written one, and taking theirs is the whole point of waiting.
		if existing, err := s.simulationKey(ctx); err == nil {
			key = existing
			return nil
		} else if !errors.Is(err, ErrNoRows) {
			return err
		}
		fresh := make([]byte, 32)
		if _, err := rand.Read(fresh); err != nil {
			return fmt.Errorf("store: simulation key: %w", err)
		}
		if err := s.SetSetting(ctx, SettingSimulationKey, base64.StdEncoding.EncodeToString(fresh)); err != nil {
			return fmt.Errorf("store: simulation key: %w", err)
		}
		key = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Store) simulationKey(ctx context.Context) ([]byte, error) {
	var encoded string
	if err := s.GetSetting(ctx, SettingSimulationKey, &encoded); err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) == 0 {
		// Unreadable is the same as absent: a key nobody can use signs
		// nothing, and refusing to start over it would be refusing to serve
		// over a feature nobody has switched on.
		return nil, ErrNoRows
	}
	return key, nil
}
