// Package cron reads the five-field expressions everybody already knows, and
// answers the one question a scheduler asks: what is the next moment after
// this one.
//
// WHY THIS EXISTS AT ALL, having once been refused. A duration says "every six
// hours" and cannot say "every Monday at three", "the first of the month", or
// "weekdays at eight" - and those are most of what an operator actually
// schedules. A duration also DRIFTS: the next turn is armed from the end of the
// last one, so a nightly job creeps later every night. A calendar does neither.
// So a schedule now carries one or the other: `every` for a cadence, `cron` for
// a calendar.
//
// Written here rather than taken from a library, and it is twenty lines of
// parsing plus a search: what a library would add is @reboot, seconds, and a
// job runner we already have.
//
// A CALENDAR NEEDS A ZONE. "Three in the morning" is a question about where,
// which is why a schedule carries a timezone and why Next takes a time that
// already carries its location. Daylight saving follows from that, and the two
// awkward days are documented on Next.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is a parsed expression: one bitmask per field.
type Schedule struct {
	minute uint64 // 0-59
	hour   uint64 // 0-23
	dom    uint64 // 1-31
	month  uint64 // 1-12
	dow    uint64 // 0-6, Sunday is 0
	// Vixie's rule: when BOTH the day of the month and the day of the week are
	// restricted, a day matching either one is a match. "0 0 1 * MON" is the
	// first of the month AND every Monday - not their intersection, which
	// would be a handful of days a year and never what anybody meant.
	domRestricted bool
	dowRestricted bool
	// The expression as it was written, for messages.
	expr string
}

// String gives back the expression as written.
func (s Schedule) String() string { return s.expr }

// The named shortcuts. @reboot is deliberately absent: a gateway restarts for
// reasons that have nothing to do with what a service wants done, and a
// schedule that fires on deployments is a surprise, not a calendar.
var shortcuts = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

var months = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var days = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// Parse reads an expression, or says what is wrong with it in the terms the
// person who wrote it used.
func Parse(expr string) (Schedule, error) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return Schedule{}, fmt.Errorf("an empty cron expression: five fields (minute hour day-of-month month day-of-week), or a shortcut like @daily")
	}
	text := raw
	if strings.HasPrefix(text, "@") {
		sub, ok := shortcuts[strings.ToLower(text)]
		if !ok {
			return Schedule{}, fmt.Errorf("unknown shortcut %q (allowed: @hourly, @daily, @midnight, @weekly, @monthly, @yearly, @annually)", raw)
		}
		text = sub
	}
	f := strings.Fields(text)
	if len(f) != 5 {
		return Schedule{}, fmt.Errorf("a cron expression has five fields (minute hour day-of-month month day-of-week), got %d in %q", len(f), raw)
	}
	s := Schedule{expr: raw}
	var err error
	if s.minute, _, err = field(f[0], 0, 59, nil, "minute"); err != nil {
		return Schedule{}, err
	}
	if s.hour, _, err = field(f[1], 0, 23, nil, "hour"); err != nil {
		return Schedule{}, err
	}
	if s.dom, s.domRestricted, err = field(f[2], 1, 31, nil, "day of the month"); err != nil {
		return Schedule{}, err
	}
	if s.month, _, err = field(f[3], 1, 12, months, "month"); err != nil {
		return Schedule{}, err
	}
	if s.dow, s.dowRestricted, err = field(f[4], 0, 7, days, "day of the week"); err != nil {
		return Schedule{}, err
	}
	// Both 0 and 7 mean Sunday, and the mask only keeps 0.
	if s.dow&(1<<7) != 0 {
		s.dow = s.dow&^(1<<7) | 1
	}
	return s, nil
}

// field reads one field into a bitmask, and says whether it restricts anything
// (anything but a bare star).
func field(spec string, low, high int, names map[string]int, what string) (uint64, bool, error) {
	var mask uint64
	restricted := false
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return 0, false, fmt.Errorf("an empty value in the %s field (%q): use a number, a-b, */n, or a list separated by commas", what, spec)
		}
		step := 1
		if at := strings.Index(part, "/"); at >= 0 {
			n, err := strconv.Atoi(part[at+1:])
			if err != nil || n <= 0 {
				return 0, false, fmt.Errorf("the step in %q (%s field) must be a whole number above zero", part, what)
			}
			step = n
			part = part[:at]
			restricted = true
		}
		// A bare star is the whole range, stepped or not; anything else
		// narrows it.
		lo, hi := low, high
		if part != "*" {
			restricted = true
			bounds := strings.SplitN(part, "-", 2)
			var err error
			if lo, err = value(bounds[0], low, high, names, what); err != nil {
				return 0, false, err
			}
			hi = lo
			if len(bounds) == 2 {
				if hi, err = value(bounds[1], low, high, names, what); err != nil {
					return 0, false, err
				}
				// A range that wraps - FRI-MON, 22-2 - is a reasonable thing to
				// write and a silent no-op if taken literally, so it wraps.
				if hi < lo {
					for v := lo; v <= high; v += step {
						mask |= 1 << uint(v)
					}
					lo = low
				}
			} else if step > 1 {
				// Vixie's "a/n": from a to the end of the range.
				hi = high
			}
		}
		for v := lo; v <= hi; v += step {
			mask |= 1 << uint(v)
		}
	}
	if mask == 0 {
		return 0, false, fmt.Errorf("the %s field (%q) matches nothing", what, spec)
	}
	return mask, restricted, nil
}

func value(text string, low, high int, names map[string]int, what string) (int, error) {
	text = strings.TrimSpace(text)
	if names != nil {
		if v, ok := names[strings.ToLower(text)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		if names != nil {
			allowed := make([]string, 0, len(names))
			for n := range names {
				allowed = append(allowed, strings.ToUpper(n))
			}
			return 0, fmt.Errorf("%q is not a %s: %d-%d, or one of %s", text, what, low, high, strings.Join(sorted(allowed), " "))
		}
		return 0, fmt.Errorf("%q is not a %s: %d-%d", text, what, low, high)
	}
	if v < low || v > high {
		return 0, fmt.Errorf("%d is outside the %s field (%d-%d)", v, what, low, high)
	}
	return v, nil
}

func sorted(in []string) []string {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j] < in[j-1]; j-- {
			in[j], in[j-1] = in[j-1], in[j]
		}
	}
	return in
}

// horizonYears bounds the search: five years without an occurrence is an
// expression that has none.
const horizonYears = 5

// Next is the first matching minute strictly after t, in t's own location.
//
// The search runs on CIVIL components - a calendar, not an instant - and the
// moment is built once at the end. That is what makes the two awkward days a
// year come out right, because a calendar has no missing hours.
//
// DAYLIGHT SAVING. In spring an hour does not exist: 2:30 that morning is
// normalised forward by the standard library, so the job runs at 3:30 rather
// than being skipped for the year. In autumn an hour happens twice: the moment
// built is the first of the two, the run happens once, and the next search
// starts after it and lands on the following day. A daily schedule fires once
// a day even on a day of twenty-five hours.
func (s Schedule) Next(t time.Time) (time.Time, bool) {
	loc := t.Location()
	// The date is walked in UTC deliberately: it stands for a calendar day
	// here, and a day that shrinks or grows by an hour must not turn "add one
	// day" into an off-by-one.
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	h, m := t.Hour(), t.Minute()+1
	if m > 59 {
		m, h = 0, h+1
	}
	if h > 23 {
		h, m = 0, 0
		day = day.AddDate(0, 0, 1)
	}
	// An expression can be legal and never happen - 30 February is the classic
	// - so the search is bounded and says so by coming back empty.
	limit := day.AddDate(horizonYears, 0, 0)
	for day.Before(limit) {
		if s.month&(1<<uint(day.Month())) == 0 {
			day = time.Date(day.Year(), day.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			h, m = 0, 0
			continue
		}
		if !s.matchesDay(day) {
			day = day.AddDate(0, 0, 1)
			h, m = 0, 0
			continue
		}
		for ; h <= 23; h++ {
			if s.hour&(1<<uint(h)) == 0 {
				m = 0
				continue
			}
			for ; m <= 59; m++ {
				if s.minute&(1<<uint(m)) != 0 {
					return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc), true
				}
			}
			m = 0
		}
		day = day.AddDate(0, 0, 1)
		h, m = 0, 0
	}
	return time.Time{}, false
}

// matchesDay applies Vixie's or-rule between the two day fields.
func (s Schedule) matchesDay(t time.Time) bool {
	dom := s.dom&(1<<uint(t.Day())) != 0
	dow := s.dow&(1<<uint(t.Weekday())) != 0
	if s.domRestricted && s.dowRestricted {
		return dom || dow
	}
	return dom && dow
}
