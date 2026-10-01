package cron

import (
	"strings"
	"testing"
	"time"
)

func at(t *testing.T, s string, loc *time.Location) time.Time {
	t.Helper()
	when, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatalf("bad time in test: %v", err)
	}
	return when
}

// The expressions an operator actually writes, and the moment each one is next
// owed. One table, because this is a parser: the interesting part is the cases,
// not the plumbing.
func TestNextIsWhenItSays(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		expr string
		from string
		want string
	}{
		// Every minute, and the minute in progress is already gone.
		{"* * * * *", "2026-03-10 08:15", "2026-03-10 08:16"},
		// Nightly, the case a duration cannot hold without drifting.
		{"0 3 * * *", "2026-03-10 08:15", "2026-03-11 03:00"},
		{"0 3 * * *", "2026-03-10 02:59", "2026-03-10 03:00"},
		// Every Monday at three.
		{"0 3 * * MON", "2026-03-10 08:15", "2026-03-16 03:00"},
		{"0 3 * * 1", "2026-03-10 08:15", "2026-03-16 03:00"},
		// Weekdays at eight.
		{"0 8 * * MON-FRI", "2026-03-13 09:00", "2026-03-16 08:00"},
		// The first of the month.
		{"30 2 1 * *", "2026-03-10 08:15", "2026-04-01 02:30"},
		// Every fifteen minutes, and every six hours.
		{"*/15 * * * *", "2026-03-10 08:16", "2026-03-10 08:30"},
		{"0 */6 * * *", "2026-03-10 08:16", "2026-03-10 12:00"},
		// A list, and a named month.
		{"0 0,12 * * *", "2026-03-10 08:16", "2026-03-10 12:00"},
		{"0 0 1 JAN *", "2026-03-10 08:16", "2027-01-01 00:00"},
		// Shortcuts.
		{"@daily", "2026-03-10 08:16", "2026-03-11 00:00"},
		{"@hourly", "2026-03-10 08:16", "2026-03-10 09:00"},
		{"@monthly", "2026-03-10 08:16", "2026-04-01 00:00"},
		// Vixie's or-rule: the first of the month OR any Monday, never their
		// intersection.
		{"0 0 1 * MON", "2026-03-02 01:00", "2026-03-09 00:00"},
		{"0 0 1 * MON", "2026-03-30 01:00", "2026-04-01 00:00"},
		// A range that wraps rather than matching nothing.
		{"0 22-2 * * *", "2026-03-10 23:30", "2026-03-11 00:00"},
	}
	for _, c := range cases {
		s, err := Parse(c.expr)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		got, ok := s.Next(at(t, c.from, utc))
		if !ok {
			t.Errorf("%q from %s: no occurrence", c.expr, c.from)
			continue
		}
		if want := at(t, c.want, utc); !got.Equal(want) {
			t.Errorf("%q from %s: next is %s, want %s", c.expr, c.from, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	}
}

// A calendar is a question about WHERE. The same expression in two zones is two
// different moments, and that is the whole reason a schedule carries a zone.
func TestTheZoneDecidesTheMoment(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no tzdata")
	}
	s, err := Parse("0 3 * * *")
	if err != nil {
		t.Fatal(err)
	}
	inParis, ok := s.Next(at(t, "2026-03-10 08:00", paris))
	if !ok {
		t.Fatal("no occurrence")
	}
	if h := inParis.UTC().Hour(); h != 2 {
		t.Errorf("three in the morning in Paris is %dh UTC in March, want 2h", h)
	}
}

// Spring forward: 2:30 does not exist that morning in Paris. The job must
// still run - normalised into the hour that does exist - rather than being
// skipped for the year.
func TestAnHourThatDoesNotExistStillRuns(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no tzdata")
	}
	s, err := Parse("30 2 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// The night of 28-29 March 2026: 02:00 becomes 03:00 in Paris.
	got, ok := s.Next(at(t, "2026-03-28 12:00", paris))
	if !ok {
		t.Fatal("no occurrence")
	}
	if got.Day() != 29 {
		t.Fatalf("the run skipped the day the hour went missing: %s", got)
	}
	if got.Hour() == 2 {
		t.Fatalf("2:30 cannot exist that morning: %s", got)
	}
}

// A schedule fires ONCE on the day that has twenty-five hours.
func TestTheLongDayFiresOnce(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no tzdata")
	}
	s, err := Parse("30 2 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// The night of 24-25 October 2026: 03:00 becomes 02:00 in Paris.
	first, ok := s.Next(at(t, "2026-10-24 12:00", paris))
	if !ok {
		t.Fatal("no occurrence")
	}
	second, ok := s.Next(first)
	if !ok {
		t.Fatal("no second occurrence")
	}
	if second.Sub(first) < 20*time.Hour {
		t.Errorf("fired twice on the long day: %s then %s", first, second)
	}
}

// An expression that is legal and never happens must be an answer, not a loop.
func TestAnImpossibleDateEndsRatherThanSpins(t *testing.T) {
	s, err := Parse("0 0 30 2 *")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Next(at(t, "2026-03-10 08:00", time.UTC)); ok {
		t.Errorf("30 February happened on %s", got)
	}
}

// A refusal has to say what is allowed - the rule this whole product is held
// to, and the reason the parser is ours.
func TestARefusalNamesWhatIsAllowed(t *testing.T) {
	cases := []struct{ expr, says string }{
		{"", "five fields"},
		{"0 3 * *", "five fields"},
		{"0 3 * * * *", "five fields"},
		{"@reboot", "@hourly"},
		{"0 99 * * *", "hour"},
		{"0 3 * * FUNDAY", "day of the week"},
		{"0 3 * FOO *", "month"},
		{"*/0 * * * *", "above zero"},
		{"0,,3 * * * *", "empty value"},
	}
	for _, c := range cases {
		_, err := Parse(c.expr)
		if err == nil {
			t.Errorf("%q was accepted", c.expr)
			continue
		}
		if !strings.Contains(err.Error(), c.says) {
			t.Errorf("%q: the refusal does not say %q: %v", c.expr, c.says, err)
		}
	}
}

// Sunday is both 0 and 7, the way every crontab in the world has it.
func TestSundayIsBothZeroAndSeven(t *testing.T) {
	zero, err := Parse("0 0 * * 0")
	if err != nil {
		t.Fatal(err)
	}
	seven, err := Parse("0 0 * * 7")
	if err != nil {
		t.Fatal(err)
	}
	from := at(t, "2026-03-10 08:00", time.UTC)
	a, _ := zero.Next(from)
	b, _ := seven.Next(from)
	if !a.Equal(b) {
		t.Errorf("0 and 7 are not the same Sunday: %s vs %s", a, b)
	}
	if a.Weekday() != time.Sunday {
		t.Errorf("not a Sunday: %s", a)
	}
}
