package store

import (
	"context"
	"fmt"
	"time"
)

// The daily notice about accounts whose access window is closing (MODEL-02).
//
// An account's window is the one setting on it that acts WITHOUT anybody
// touching anything: a date passes and somebody cannot sign in. That is the
// point of it - a contractor's access ends on the day the contract does,
// whether or not a human remembers. But it also means the only warning an
// administrator would otherwise get is the person telephoning them, which
// makes a deliberate mechanism look like a fault.
//
// So the gateway says it first, once a day, to the people who could act: the
// accounts that are about to lose access, and the ones that just did.
const (
	// SettingExpiryDigest is the configuration: whether, at what hour, how far
	// ahead. Exported with the rest - it describes the installation.
	SettingExpiryDigest = "expiry_digest"
	// SettingExpiryDigestSent is the GUARD, and not configuration: the day
	// this installation last sent, so a restart at 07:59 does not send a
	// second time and every node of a cluster sends the same one once. Like
	// the other guards it stays out of an exported document - it means
	// something only for the database that wrote it.
	SettingExpiryDigestSent = "expiry_digest_sent"
)

// ExpiryDigest is the daily notice's configuration.
type ExpiryDigest struct {
	Enabled bool `json:"enabled"`
	// Hour is when it goes out, 0-23, in the gateway's own local time. A
	// timezone of its own would be a fourth place where one is chosen, and
	// this one is a habit ("in my morning"), not a contract.
	Hour int `json:"hour"`
	// Days is how far ahead it looks. A week by default: long enough to
	// arrange something, short enough that the same account is not announced
	// every morning for a month.
	Days int `json:"days"`
}

// DefaultExpiryDigest ships ON, unlike the switches that open a new surface:
// this one opens nothing. It sends only when there is something to say, only
// where a relay is configured, and only to administrators who already
// administer these accounts - and its absence is what makes an expiry look
// like a bug on the morning it fires.
func DefaultExpiryDigest() ExpiryDigest {
	return ExpiryDigest{Enabled: true, Hour: 7, Days: 7}
}

// SanitizeExpiryDigest fills the blanks and refuses the impossible, naming
// what is allowed.
func SanitizeExpiryDigest(d *ExpiryDigest) error {
	if d.Hour < 0 || d.Hour > 23 {
		return fmt.Errorf("the digest hour is an hour of the day, 0 to 23, not %d", d.Hour)
	}
	if d.Days == 0 {
		d.Days = DefaultExpiryDigest().Days
	}
	if d.Days < 1 || d.Days > 90 {
		return fmt.Errorf("the digest looks between 1 and 90 days ahead, not %d", d.Days)
	}
	return nil
}

// GetExpiryDigest answers the configuration, defaults included: a setting
// nobody ever saved and one that cannot be read look the same to a loop, and
// neither is a reason to stop telling administrators about expiries.
func (s *Store) GetExpiryDigest(ctx context.Context) ExpiryDigest {
	cfg := DefaultExpiryDigest()
	_ = s.GetSetting(ctx, SettingExpiryDigest, &cfg)
	if err := SanitizeExpiryDigest(&cfg); err != nil {
		return DefaultExpiryDigest()
	}
	return cfg
}

// LastExpiryDigest is the day (YYYY-MM-DD) the last notice went out, or "".
func (s *Store) LastExpiryDigest(ctx context.Context) string {
	var day string
	_ = s.GetSetting(ctx, SettingExpiryDigestSent, &day)
	return day
}

// MarkExpiryDigestSent records the day, so the next tick - and the next node -
// finds the work done.
func (s *Store) MarkExpiryDigestSent(ctx context.Context, day string) error {
	return s.SetSetting(ctx, SettingExpiryDigestSent, day)
}

// UsersEndingBetween lists the enabled accounts whose window ends in
// [from, to), oldest deadline first.
//
// The bound is the LAST DAY the account works, stored as that day's midnight
// UTC - the same value ValidAt reads, so the list and the refusal at sign-in
// can never disagree about who is out. Disabled accounts are left out: they
// are already refused, and announcing that one of them will also be out of
// its window is news about nothing.
func (s *Store) UsersEndingBetween(ctx context.Context, from, to int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users
		 WHERE valid_until > 0 AND valid_until >= ? AND valid_until < ? AND enabled = ?
		 ORDER BY valid_until ASC, username ASC`, from, to, true)
	if err != nil {
		return nil, fmt.Errorf("store: users ending between %d and %d: %w", from, to, err)
	}
	defer func() { _ = rows.Close() }()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: users ending between %d and %d: %w", from, to, err)
	}
	return out, nil
}

// DayStart is the midnight UTC of t's day - the unit a window is written in.
func DayStart(t time.Time) time.Time { return t.UTC().Truncate(24 * time.Hour) }
