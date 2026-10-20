package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Setting resolution - the validated configuration model: the most specific
// level wins, membership -> tenant -> global (TENANT-04/05). A level whose
// value carries Inherited=true (or, for the TTL, an empty string) defers to
// the level above. tenantID "" resolves straight from the global level (users
// without membership: root, console-only accounts).

// ResolveBusinessAccess returns the working-hours window that applies to
// userID within tenantID, in groupID when one is active (TENANT-04). The most
// precise level that sets one wins whole, the others no longer count: the
// membership, else the group, else the account, else the organisation, else
// the gateway. A window there says who may come in when - an override is a
// different rule for this person, not a narrower one. The group counts only
// in an organisation in exclusive mode, where one is chosen per session: in
// cumulative mode a person holds several at once, and no one of them could
// say which hours. Without an organisation (none, or not chosen yet) it is
// the account's, else the gateway's. Root is held to none: the person who
// must always be able to repair the gateway.
func (s *Store) ResolveBusinessAccess(ctx context.Context, userID, tenantID, groupID string) (BusinessAccess, error) {
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return BusinessAccess{}, err
	}
	if u.Root {
		return BusinessAccess{}, nil
	}
	// A member's date bounds hold whichever level its hours come from: they
	// say when this person's access to the organisation runs, not which hours.
	var from, to string
	if tenantID != "" {
		m, err := s.GetMembership(ctx, userID, tenantID)
		if err != nil {
			return BusinessAccess{}, err
		}
		if !m.BusinessAccess.Inherited {
			return m.BusinessAccess, nil
		}
		from, to = m.BusinessAccess.DateFrom, m.BusinessAccess.DateTo
		if groupID != "" && s.EffectiveGroupMode(ctx, tenantID) == GroupModeSingle {
			g, err := s.GetGroup(ctx, groupID)
			if err != nil {
				return BusinessAccess{}, err
			}
			if g.TenantID == tenantID && !g.BusinessAccess.Inherited {
				ba := g.BusinessAccess
				ba.DateFrom, ba.DateTo = from, to
				return ba, nil
			}
		}
	}
	if !u.BusinessAccess.Inherited {
		ba := u.BusinessAccess
		if from != "" || to != "" {
			ba.DateFrom, ba.DateTo = from, to
		}
		return ba, nil
	}
	if tenantID != "" {
		t, err := s.GetTenant(ctx, tenantID)
		if err != nil {
			return BusinessAccess{}, err
		}
		if !t.BusinessAccess.Inherited {
			ba := t.BusinessAccess
			ba.DateFrom, ba.DateTo = from, to
			return ba, nil
		}
	}
	var global BusinessAccess
	if err := s.GetSetting(ctx, SettingBusinessAccess, &global); err != nil {
		return BusinessAccess{}, err
	}
	global.Inherited = false
	global.DateFrom, global.DateTo = from, to
	return global, nil
}

// ResolveSessionTTL returns the ISO-8601 session lifetime that applies to
// userID within tenantID (membership -> tenant -> global - TENANT-05).
func (s *Store) ResolveSessionTTL(ctx context.Context, userID, tenantID string) (string, error) {
	if tenantID != "" {
		m, err := s.GetMembership(ctx, userID, tenantID)
		if err != nil {
			return "", err
		}
		if m.SessionTTL != "" {
			return m.SessionTTL, nil
		}
		t, err := s.GetTenant(ctx, tenantID)
		if err != nil {
			return "", err
		}
		if t.SessionTTL != "" {
			return t.SessionTTL, nil
		}
	}
	var global string
	if err := s.GetSetting(ctx, SettingSessionTTL, &global); err != nil {
		return "", err
	}
	return global, nil
}

// WithinBusinessAccess reports whether now falls inside the window: date
// bounds (membership level) and per-day hour ranges (TENANT-04). now is a UTC
// instant; it is brought into the window's timezone once, so the ranges'
// wall-clock hours stay honest across DST.
func WithinBusinessAccess(ba BusinessAccess, now time.Time) (bool, error) {
	loc := time.UTC
	if ba.Timezone != "" {
		l, err := time.LoadLocation(ba.Timezone)
		if err != nil {
			return false, fmt.Errorf("business access: timezone %q is not a valid IANA name", ba.Timezone)
		}
		loc = l
	}
	local := now.In(loc)
	day := local.Format("2006-01-02")
	if ba.DateFrom != "" && day < ba.DateFrom {
		return false, nil
	}
	if ba.DateTo != "" && day > ba.DateTo {
		return false, nil
	}
	if len(ba.Days) == 0 {
		return true, nil // no weekday/hour restriction
	}
	// time.Weekday counts Sunday=0; the window counts Monday=1 ... Sunday=7.
	iso := int(local.Weekday())
	if iso == 0 {
		iso = 7
	}
	hhmm := local.Format("15:04")
	for _, r := range ba.Days {
		if r.Day != iso {
			continue
		}
		from, to := r.From, r.To
		if from == "" {
			from = "00:00"
		}
		if to == "" {
			to = "23:59"
		}
		if hhmm >= from && hhmm <= to {
			return true, nil
		}
	}
	return false, nil
}

// ParseISODuration parses the ISO-8601 duration subset Meerkat uses for TTLs
// (PT10M, PT1H, P1D, P7D, and combinations like P1DT12H). Weeks, months and
// years are not accepted - a session TTL has no business being that long.
func ParseISODuration(s string) (time.Duration, error) {
	orig := s
	if len(s) < 3 || s[0] != 'P' {
		return 0, fmt.Errorf("duration %q is not ISO-8601: expected forms like PT30M, PT2H, P1D", orig)
	}
	s = s[1:]
	var d time.Duration
	inTime := false
	n := 0
	hasDigits := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 'T' && !inTime:
			inTime = true
		case c >= '0' && c <= '9':
			n = n*10 + int(c-'0')
			hasDigits = true
		case c == 'D' && !inTime && hasDigits:
			d += time.Duration(n) * 24 * time.Hour
			n, hasDigits = 0, false
		case c == 'H' && inTime && hasDigits:
			d += time.Duration(n) * time.Hour
			n, hasDigits = 0, false
		case c == 'M' && inTime && hasDigits:
			d += time.Duration(n) * time.Minute
			n, hasDigits = 0, false
		case c == 'S' && inTime && hasDigits:
			d += time.Duration(n) * time.Second
			n, hasDigits = 0, false
		default:
			return 0, fmt.Errorf("duration %q is not ISO-8601: expected forms like PT30M, PT2H, P1D", orig)
		}
	}
	if hasDigits || d == 0 {
		return 0, fmt.Errorf("duration %q is not ISO-8601: expected forms like PT30M, PT2H, P1D", orig)
	}
	return d, nil
}

func encodeBusinessAccess(b BusinessAccess) string {
	SanitizeBusinessAccess(&b)
	raw, err := json.Marshal(b)
	if err != nil {
		return `{"inherited":true}`
	}
	return string(raw)
}

func decodeBusinessAccess(s string) BusinessAccess {
	b := BusinessAccess{Inherited: true}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &b)
	}
	return b
}

// OpenBusinessAccess says whether the window is open at now and, when it is,
// until when: the instant it closes, the zero time when it never does. A
// session let in under it ends there (TENANT-04).
func OpenBusinessAccess(ba BusinessAccess, now time.Time) (bool, time.Time, error) {
	open, err := WithinBusinessAccess(ba, now)
	if err != nil || !open {
		return false, time.Time{}, err
	}
	closes, err := businessAccessCloses(ba, now)
	if err != nil {
		return false, time.Time{}, err
	}
	return true, closes, nil
}

// businessAccessCloses is the first instant after now, inside an open window,
// at which it is no longer open; zero when it never closes. A range's end is
// inclusive to the minute (18:00 covers 18:00:59), and a closing that runs
// straight into the next open range - Monday 23:59 into Tuesday 00:00 - is
// not one: it walks on, a fortnight at most, to the real end.
func businessAccessCloses(ba BusinessAccess, now time.Time) (time.Time, error) {
	if len(ba.Days) == 0 && ba.DateTo == "" {
		return time.Time{}, nil
	}
	loc := time.UTC
	if ba.Timezone != "" {
		l, err := time.LoadLocation(ba.Timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("business access: timezone %q is not a valid IANA name", ba.Timezone)
		}
		loc = l
	}
	at := now.In(loc)
	for range 15 * 24 * 60 / 30 {
		next := endOfOpenStretch(ba, at, loc)
		if next.IsZero() {
			return time.Time{}, nil
		}
		open, err := WithinBusinessAccess(ba, next)
		if err != nil {
			return time.Time{}, err
		}
		if !open {
			return next, nil
		}
		at = next
	}
	return time.Time{}, nil
}

// endOfOpenStretch is where the range covering at ends (the minute after its
// "to"), or the end of DateTo's day when no weekday range restricts.
func endOfOpenStretch(ba BusinessAccess, at time.Time, loc *time.Location) time.Time {
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, loc)
	var end time.Time
	if ba.DateTo != "" {
		if d, err := time.ParseInLocation("2006-01-02", ba.DateTo, loc); err == nil {
			end = d.AddDate(0, 0, 1)
		}
	}
	if len(ba.Days) == 0 {
		return end
	}
	iso := int(at.Weekday())
	if iso == 0 {
		iso = 7
	}
	hhmm := at.Format("15:04")
	for _, r := range ba.Days {
		from, to := r.From, r.To
		if from == "" {
			from = "00:00"
		}
		if to == "" {
			to = "23:59"
		}
		if r.Day != iso || hhmm < from || hhmm > to {
			continue
		}
		t, err := time.ParseInLocation("15:04", to, loc)
		if err != nil {
			continue
		}
		closes := day.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute()+1)*time.Minute)
		if end.IsZero() || closes.Before(end) {
			end = closes
		}
		return end
	}
	return end
}
