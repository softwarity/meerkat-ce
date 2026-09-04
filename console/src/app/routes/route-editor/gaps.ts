import { CatalogEntry, Spec } from '../../api.service';

// What the gateway will refuse, worked out before it is asked.
//
// The console clears blank arguments on the way out (cleanSpecs), so an
// argument marked required in the catalogue and left empty does not arrive
// empty: it does not arrive at all, and the gateway answers 422 "missing
// required arg". Reading the same catalogue the gateway publishes its own
// parameters from means the console refuses exactly what the gateway would,
// and goes on doing so the day a brick gains an argument - which a list of
// rules written out by hand here would not.
export function missingArgs(s: Spec, entry: CatalogEntry | undefined): string[] {
  // An unknown type is the server's to refuse: it knows bricks this console's
  // catalogue may predate, and inventing a verdict here would block a route
  // the gateway would have taken.
  if (!entry) return [];
  return entry.params.filter((p) => p.required && blank(s.args?.[p.name])).map((p) => p.name);
}

function blank(v: unknown): boolean {
  if (v === undefined || v === null) return true;
  if (typeof v === 'string') return v.trim() === '';
  if (Array.isArray(v)) return v.length === 0 || v.every((x) => String(x).trim() === '');
  return false;
}

// The console's copy of what gateway.Validate refuses of an upstream, so the
// answer arrives while the field is under the cursor rather than as a 422 on
// a screen that has already moved on. Empty when there is nothing to say.
export function upstreamProblem(raw: string): string {
  const v = (raw ?? '').trim();
  if (!v) {
    return $localize`:@@Gap_upstream_missing:an upstream to fetch from`;
  }
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return $localize`:@@Gap_upstream_unreadable:the upstream is not a url: scheme and host are both needed`;
  }
  if (!u.hostname) {
    return $localize`:@@Gap_upstream_no_host:the upstream has no host`;
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') {
    return $localize`:@@Gap_upstream_scheme:only http and https can be proxied`;
  }
  return '';
}
