// ISO-8601 durations as the console offers them.
//
// The lists are what an operator picks from, but a value can arrive from
// somewhere else entirely - the API, a seeded configuration file, an imported
// one - and a select with no matching option renders EMPTY. Which is the worst
// of both: the setting is in force, the screen says nothing is, and saving the
// form wipes it. So an unlisted value joins the list rather than disappearing.

export interface DurationChoice {
  key: string;
  label: string;
}

// withCurrent returns the offered choices, plus the one in force when it is
// not among them.
export function withCurrent(choices: DurationChoice[], current: string): DurationChoice[] {
  if (!current || choices.some((c) => c.key === current)) return choices;
  return [...choices, { key: current, label: humanIso(current) }];
}

// humanIso turns PT30S into "30 s" and PT5M into "5 min". Anything it cannot
// read comes back as it was: showing the raw ISO is honest, and better than
// showing nothing.
export function humanIso(iso: string): string {
  const m = /^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/.exec(iso);
  if (!m) return iso;
  const [, days, hours, mins, secs] = m;
  if (days) return $localize`:@@N_days:${days}:N: days`;
  if (hours) return $localize`:@@N_hours:${hours}:N: h`;
  if (mins) return $localize`:@@N_minutes:${mins}:N: min`;
  if (secs) return $localize`:@@N_seconds:${secs}:N: s`;
  return iso;
}
