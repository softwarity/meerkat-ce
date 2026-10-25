package auth

import (
	"context"
	"html/template"
	"strings"
	"time"
)

// Every moment a page shows a person, written ONE way - the console's:
// `2026-10-25T17:41`, ISO 8601 on one line, `2026-10-25` for a day alone. In
// the zone the person chose on their profile; with none chosen, in their
// browser's, which only the browser knows.
//
// So the server writes what it can. A moment is a <time> whose datetime is the
// instant in UTC and whose text is the server's writing: in the chosen zone
// when there is one, else in UTC with its Z - an honest fallback for a page
// read without scripts. In the second case the element carries data-local and
// the script every flow page carries (mkWhen, flowTop) writes it again in the
// browser's zone, the same shape. The page never shows an hour somebody reads
// as their own when it is not.

// moment is one time on a page, ready for the template: {{.Since.HTML}}.
type moment struct {
	// At is the instant, RFC 3339 in UTC: what the browser writes again from.
	// Empty on a preview's sample, which is text alone.
	At string
	// Text is the server's writing of it.
	Text string
	// Day writes the date alone.
	Day bool
	// Follow: no zone chosen, the browser's is used.
	Follow bool
}

// HTML is the <time> element, or the text alone for a sample.
func (m moment) HTML() template.HTML {
	if m.At == "" {
		return template.HTML(template.HTMLEscapeString(m.Text)) //nolint:gosec // escaped just above
	}
	var b strings.Builder
	b.WriteString(`<time datetime="`)
	b.WriteString(m.At)
	b.WriteString(`"`)
	if m.Follow {
		if m.Day {
			b.WriteString(` data-local="day"`)
		} else {
			b.WriteString(` data-local="short"`)
		}
	}
	b.WriteString(`>`)
	b.WriteString(template.HTMLEscapeString(m.Text))
	b.WriteString(`</time>`)
	return template.HTML(b.String()) //nolint:gosec // digits and the escaped text, built above
}

// In writes the moment into a translated sentence at its %s ("until %s",
// "%sまで"): the sentence escaped, the moment as its element.
func (m moment) In(sentence string) template.HTML {
	esc := template.HTMLEscapeString(sentence)
	if !strings.Contains(esc, "%s") {
		return template.HTML(esc + " " + string(m.HTML())) //nolint:gosec // escaped above
	}
	return template.HTML(strings.Replace(esc, "%s", string(m.HTML()), 1)) //nolint:gosec // escaped above
}

// sample is a preview's moment: the text, as a served page would write it.
func sample(text string) moment { return moment{Text: text} }

// readerZone is the zone a person chose on their profile; nil when they chose
// none (their browser's is then used) or when it is a name this platform does
// not know.
func (h *Handler) readerZone(ctx context.Context, userID string) *time.Location {
	u, err := h.st.GetUserByID(ctx, userID)
	if err != nil || u.Timezone == "" {
		return nil
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		return nil
	}
	return loc
}

// momentOf writes a unix second for a reader in loc (nil: no zone chosen).
func momentOf(unix int64, loc *time.Location) moment {
	return written(unix, loc, false)
}

// dayOf writes the day alone.
func dayOf(unix int64, loc *time.Location) moment {
	return written(unix, loc, true)
}

func written(unix int64, loc *time.Location, day bool) moment {
	t := time.Unix(unix, 0)
	m := moment{At: t.UTC().Format(time.RFC3339), Day: day, Follow: loc == nil}
	if loc == nil {
		loc = time.UTC
	}
	if day {
		m.Text = t.In(loc).Format(time.DateOnly)
		return m
	}
	m.Text = t.In(loc).Format("2006-01-02T15:04")
	// UTC says so with the one letter that means it everywhere: an hour
	// written alone is an hour somebody reads as their own.
	if name := loc.String(); name == "UTC" || name == "Etc/UTC" {
		m.Text += "Z"
	}
	return m
}

// whenScript is mkWhen, in the head of every flow page so the scripts of a
// page's body can use it: mkWhen(seconds, style) writes a moment the way the
// server does, in the zone stamped on <html data-tz> (the one the person
// chose), else in the browser's. Styles: "short" (the minute), "long" (the
// second), "day", "time" (the clock alone). mkWhen.day(seconds) is the day a
// moment falls on in that zone, for a list grouped by day.
const whenScript = `  <script>
  window.mkWhen = (() => {
    let browser = 'UTC';
    try { browser = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'; } catch {}
    const zone = document.documentElement.dataset.tz || browser;
    const utc = (z) => z.toUpperCase() === 'UTC' || z === 'Etc/UTC';
    const parts = (ms, z) => {
      const p = new Intl.DateTimeFormat('en-GB', { timeZone: z, hourCycle: 'h23', year: 'numeric',
        month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).formatToParts(new Date(ms));
      const at = (t) => (p.find((x) => x.type === t) || {}).value || '';
      return { day: at('year') + '-' + at('month') + '-' + at('day'), hm: at('hour') + ':' + at('minute'), s: at('second') };
    };
    const write = (seconds, style) => {
      if (!seconds) return '';
      let z = zone, p;
      try { p = parts(seconds * 1000, z); } catch { z = 'UTC'; p = parts(seconds * 1000, z); }
      if (style === 'day') return p.day;
      const clock = p.hm + (style === 'long' ? ':' + p.s : '') + (utc(z) ? 'Z' : '');
      return style === 'time' ? clock : p.day + 'T' + clock;
    };
    write.zone = zone;
    write.day = (seconds) => write(seconds, 'day');
    return write;
  })();
  </script>
`

// localScript writes again, in the browser's zone, the moments the server
// wrote in UTC for want of a chosen one (data-local). At the foot of every
// flow page, after the body.
const localScript = `  <script>
  for (const t of document.querySelectorAll('time[data-local]')) {
    const ms = Date.parse(t.dateTime);
    if (!isNaN(ms)) t.textContent = mkWhen(ms / 1000, t.dataset.local);
  }
  </script>
`
