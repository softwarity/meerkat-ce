package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The strings an installation corrected or added, per language (I18N).
//
// A row holds ONLY what differs from what the binary embeds. Storing a whole
// copy would freeze a language at the day it was copied: every later fix we
// ship would sit invisible behind it, and the installation would drift further
// from the product with each release. A thin layer picks up everything it does
// not override, and resetting a language is deleting its row.
//
// The code is a BCP 47 tag and the primary key. No foreign key to anything:
// a language can be added here that the binary has never heard of, which is
// how an integrator brings their own.

// LocaleOverride is one language's corrections.
type LocaleOverride struct {
	Code      string            `json:"code"`
	Entries   map[string]string `json:"entries"`
	UpdatedAt int64             `json:"updatedAt"`
}

// LocaleOverrides reads the whole layer, which is what the auth package wants:
// it holds the merged catalogues in memory and is handed the layer entire, so
// a language that was just reset cannot stay overridden in a partial push.
func (s *Store) LocaleOverrides(ctx context.Context) (map[string]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, entries FROM locale_overrides`)
	if err != nil {
		return nil, fmt.Errorf("store: list locale overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]map[string]string{}
	for rows.Next() {
		var code, raw string
		if err := rows.Scan(&code, &raw); err != nil {
			return nil, fmt.Errorf("store: locale override: %w", err)
		}
		var entries map[string]string
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			// A row we cannot read is one language's corrections lost, not the
			// gateway's text: the others still load, and the broken one falls
			// back to what we embed.
			continue
		}
		out[code] = entries
	}
	return out, rows.Err()
}

// SetLocaleOverride replaces one language's corrections. An empty map KEEPS the
// row: existence and content are two different facts since a language can be
// added here, and a language created in the console has to survive the moment
// between its code and its first wording. Removing one is DeleteLocaleOverride.
//
// An empty row says nothing beyond "this language exists here": it is not
// reported as edited, and for a language the binary already carries it says
// nothing at all.
func (s *Store) SetLocaleOverride(ctx context.Context, code string, entries map[string]string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("store: a locale override needs a language code")
	}
	clean := make(map[string]string, len(entries))
	for k, v := range entries {
		k = strings.TrimSpace(k)
		// An empty value is a key RESET, not a key set to nothing: a blank
		// string on a page is invisible, so it can never be what somebody
		// meant to store.
		if k == "" || strings.TrimSpace(v) == "" {
			continue
		}
		clean[k] = v
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return fmt.Errorf("store: locale override %q: %w", code, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO locale_overrides (code, entries, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(code) DO UPDATE SET entries = excluded.entries, updated_at = excluded.updated_at`,
		code, string(raw), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: save locale override %q: %w", code, err)
	}
	return nil
}

// DeleteLocaleOverride drops one language's corrections: the reset that puts a
// language back to what the product ships.
func (s *Store) DeleteLocaleOverride(ctx context.Context, code string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM locale_overrides WHERE code = ?`, code); err != nil {
		return fmt.Errorf("store: delete locale override %q: %w", code, err)
	}
	return nil
}
