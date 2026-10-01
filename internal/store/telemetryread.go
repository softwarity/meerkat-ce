package store

import (
	"context"

	"github.com/softwarity/meerkat/internal/vault"
)

// Reading the telemetry setting, in the two ways the two readers need it.
//
// The SCREEN reads it raw - references as references, so `$otlp-token` is what
// an operator sees and what the audit records. The EXPORTER reads it resolved,
// at the moment it is about to use it. That split is the whole rule: a
// reference is public, a literal never is (VAULT-05).

// RawTelemetry is the setting as it was written, references and all. What the
// console shows and what an export carries.
func (s *Store) RawTelemetry(ctx context.Context) TelemetryConfig {
	cfg := DefaultTelemetry()
	_ = s.GetSetting(ctx, SettingTelemetry, &cfg)
	return cfg
}

// ResolvedTelemetry is the setting with its vault references filled in, for
// the exporter and nobody else. INFRA scope: a collector's credential is the
// gateway's own, and resolving it against the application's scope would let an
// app admin decide what the gateway authenticates to a monitoring stack with -
// the same reasoning as the SMTP relay's password.
//
// A reference the vault does not hold is left AS IT STANDS rather than
// becoming empty: a header whose value is literally `$otlp-token` fails at the
// collector with something an operator can search for, where an empty header
// fails with a 401 that names nothing.
func (s *Store) ResolvedTelemetry(ctx context.Context) TelemetryConfig {
	cfg := s.RawTelemetry(ctx)
	if len(cfg.Headers) == 0 {
		return cfg
	}
	values, err := s.VaultValues(ctx, vault.ScopeInfra)
	if err != nil {
		return cfg
	}
	lookup := func(n string) (string, bool) { v, ok := values[n]; return v, ok }
	out := make(map[string]string, len(cfg.Headers))
	for name, value := range cfg.Headers {
		out[name], _ = vault.Expand(value, lookup)
	}
	cfg.Headers = out
	return cfg
}
