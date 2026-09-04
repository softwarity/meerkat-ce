import { Spec } from '../../api.service';

// Small readers over a predicate/filter Spec's args. Dedicated components read
// their own typed fields through these instead of poking raw `args` records.
export function argStr(spec: Spec | undefined, key: string): string {
  const v = spec?.args?.[key];
  return v === undefined || v === null ? '' : String(v);
}

export function argList(spec: Spec | undefined, key: string): string[] {
  const v = spec?.args?.[key];
  return Array.isArray(v) ? v.map(String) : [];
}

export function argBool(spec: Spec | undefined, key: string): boolean {
  return Boolean(spec?.args?.[key]);
}

// Build a Spec, dropping empty args so the server applies its own defaults and
// flags genuinely-missing required ones (same contract as the generic path).
export function spec(type: string, args: Record<string, unknown>): Spec {
  const kept: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(args)) {
    if (v === undefined || v === null || v === '') continue;
    if (Array.isArray(v) && v.length === 0) continue;
    kept[k] = v;
  }
  return Object.keys(kept).length ? { type, args: kept } : { type };
}

// Turn a kebab-case brick type into a readable label: "add-request-header" ->
// "Add request header". Keeps the technical type available elsewhere (mono).
export function humanize(type: string): string {
  const s = type.replace(/-/g, ' ');
  return s.charAt(0).toUpperCase() + s.slice(1);
}

// Immutably set (or clear, when blank) one arg on a spec, dropping the args
// object entirely when nothing is left. Dedicated filter fields edit through it.
export function patchSpec(s: Spec, key: string, value: unknown): Spec {
  const args = { ...(s.args ?? {}) };
  if (value === '' || value === null || value === undefined) delete args[key];
  else args[key] = value;
  return Object.keys(args).length ? { type: s.type, args } : { type: s.type };
}

// Working predicate specs keep half-typed, empty rows so the UI stays stable
// while editing. This drops them at save: blank list entries and blank scalars
// go, and a predicate with no meaningful (non-flag) content is removed entirely.
export function cleanPredicates(specs: Spec[]): Spec[] {
  const out: Spec[] = [];
  for (const s of specs) {
    const args: Record<string, unknown> = {};
    let hasContent = false;
    for (const [k, v] of Object.entries(s.args ?? {})) {
      if (Array.isArray(v)) {
        const arr = v.map(String).map((x) => x.trim()).filter((x) => x !== '');
        if (arr.length) {
          args[k] = arr;
          hasContent = true;
        }
      } else if (typeof v === 'boolean') {
        if (v) args[k] = v; // a lone flag is not, by itself, a predicate
      } else if (v !== '' && v !== null && v !== undefined) {
        args[k] = v;
        hasContent = true;
      }
    }
    if (hasContent) out.push(Object.keys(args).length ? { type: s.type, args } : { type: s.type });
  }
  return out;
}

// Filters are NOT predicates, and sharing one cleaner cost exactly this: a
// brick whose arguments were all left at their defaults has no args at all,
// was read as a half-typed row, and vanished on save. Someone had clicked
// "add" - that IS the intent - and several filters are complete with nothing
// filled in: preserve-host takes no argument by design, strip-prefix strips
// one segment, security-headers and cookie-attributes carry sensible values of
// their own. Seven bricks in the catalogue have no required parameter, and
// preserve-host could never be saved from the console at all.
//
// So a filter is kept whatever it holds; only the empties INSIDE it go. The
// order is significant and preserved.
export function cleanSpecs(specs: Spec[]): Spec[] {
  return specs.map((s) => {
    const args: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(s.args ?? {})) {
      if (Array.isArray(v)) {
        const arr = v.map(String).map((x) => x.trim()).filter((x) => x !== '');
        if (arr.length) args[k] = arr;
      } else if (typeof v === 'boolean') {
        args[k] = v;
      } else if (v !== '' && v !== null && v !== undefined) {
        args[k] = v;
      }
    }
    return Object.keys(args).length ? { type: s.type, args } : { type: s.type };
  });
}
