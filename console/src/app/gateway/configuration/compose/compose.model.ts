import { ComposeDocument, ComposePick } from '../../../api.service';

// The compose screen's reading of two documents (CFG-08). The server composes
// (internal/config/compose.go): this file only reads the three documents it
// answers into rows, fields and elements the loom draws, and keeps the
// recipe. Nothing here decides what the result is.

// The sections a document carries, in the order they are drawn, with the
// kind a path names them by.
export const KINDS: { kind: string; section: string; label: string; key: 'id' | 'name' | 'map' | 'single' }[] = [
  { kind: 'route', section: 'routes', label: 'Routes', key: 'id' },
  { kind: 'role', section: 'roles', label: 'Roles', key: 'name' },
  { kind: 'tenant', section: 'tenants', label: 'Tenants', key: 'id' },
  { kind: 'group', section: 'groups', label: 'Groups', key: 'id' },
  { kind: 'authProvider', section: 'authProviders', label: 'Authorities', key: 'id' },
  { kind: 'oidcClient', section: 'oidcClients', label: 'OIDC clients', key: 'id' },
  { kind: 'theme', section: 'themes', label: 'Themes', key: 'id' },
  { kind: 'mailRelay', section: 'mailRelay', label: 'Mail relay', key: 'single' },
  { kind: 'setting', section: 'settings', label: 'Settings', key: 'map' },
];

export type Obj = Record<string, unknown>;

// One object seen across the three documents: a row of the loom.
export interface Entity {
  kind: string;
  id: string;
  path: string;
  label: string;
  left?: Obj;
  right?: Obj;
  result?: Obj;
  // 'same' (both sides agree), 'update' (both sides, differing), 'left-only',
  // 'right-only'.
  status: 'same' | 'update' | 'left-only' | 'right-only';
  // Which side the result's object comes from, when it has one.
  origin?: 'left' | 'right';
  // A right-only row whose name a left-only row shares: paired, the right
  // object is read under the left id.
  pairedWith?: string;
}

// One field of an entity on both sides: a leaf, or a list (set).
export interface Field {
  path: string;
  set: boolean;
  left: unknown;
  right: unknown;
  result: unknown;
  same: boolean;
}

// One element of a list field, known by its key.
export interface Element {
  key: string;
  val: unknown;
  inL: boolean;
  inR: boolean;
  inRes: boolean;
}

const isObj = (v: unknown): v is Obj => !!v && typeof v === 'object' && !Array.isArray(v);

// The objects of one section by id, in order, as the server keys them: id,
// else code, else name; a setting by its key; the relay as one.
function objectsOf(doc: ComposeDocument | undefined, kind: (typeof KINDS)[number]): Map<string, Obj> {
  const out = new Map<string, Obj>();
  const raw = doc?.[kind.section];
  if (kind.key === 'single') {
    if (isObj(raw)) out.set(kind.kind, raw);
    return out;
  }
  if (kind.key === 'map') {
    if (isObj(raw)) {
      for (const k of Object.keys(raw).sort()) if (isObj(raw[k])) out.set(k, raw[k] as Obj);
    }
    return out;
  }
  if (Array.isArray(raw)) {
    raw.forEach((o, i) => {
      if (!isObj(o)) return;
      const id = ['id', 'code', 'name'].map((k) => o[k]).find((v) => typeof v === 'string' && v) as string | undefined;
      out.set(id ?? '#' + i, o);
    });
  }
  return out;
}

// The rows, from the three documents: every object either side has, in the
// left's order then the right's additions; what the result holds for it.
export function entities(
  left: ComposeDocument,
  right: ComposeDocument,
  result: ComposeDocument,
  pairs: { left: string; right: string; kind: string }[],
  media?: { left: Record<string, MediaInfo>; right: Record<string, MediaInfo> },
): Entity[] {
  const out: Entity[] = [];
  for (const K of KINDS) {
    const L = objectsOf(left, K);
    const R = objectsOf(right, K);
    const Res = objectsOf(result, K);
    // A paired right object reads under the left id, as the server does.
    for (const p of pairs) {
      if (p.kind !== K.kind) continue;
      const o = R.get(p.right);
      if (o && L.has(p.left)) {
        R.delete(p.right);
        R.set(p.left, { ...o, id: p.left });
      }
    }
    const ids = [...L.keys(), ...[...R.keys()].filter((id) => !L.has(id))];
    for (const id of ids) {
      const l = L.get(id);
      const r = R.get(id);
      const res = Res.get(id);
      const path = K.key === 'single' ? K.kind : `${K.kind}:${id}`;
      const pair = pairs.find((p) => p.kind === K.kind && p.left === id);
      // A route's files and spec ride beside it: one rewritten makes the
      // route differ though its JSON does not.
      const moved = K.kind === 'route' && !!media && mediaMoved(media.left[id], media.right[pair?.right ?? id]).length > 0;
      const status = l && r ? (same(l, r) && !moved ? 'same' : 'update') : l ? 'left-only' : 'right-only';
      const e: Entity = { kind: K.kind, id, path, label: labelOf(l ?? r ?? {}, id), left: l, right: r, result: res, status };
      if (res) e.origin = l && same(res, l) ? 'left' : r && same(res, r) ? 'right' : l ? 'left' : 'right';
      if (pair) e.pairedWith = pair.right;
      out.push(e);
    }
  }
  return out;
}

export function labelOf(o: Obj, id: string): string {
  return typeof o['name'] === 'string' && o['name'] ? (o['name'] as string) : id;
}

// Two values the same: by canonical JSON, the way the server compares.
export function same(a: unknown, b: unknown): boolean {
  return canonical(a) === canonical(b);
}

// Canonical JSON: keys sorted, blanks dropped (nothing, '', [], {}), the
// store's stamps out - what internal/config canonical() writes, so an
// element's key is the same bytes here and there.
export function canonical(v: unknown): string {
  return JSON.stringify(tidy(v));
}

const STAMPS = new Set(['createdAt', 'updatedAt', 'rev']);

function tidy(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(tidy);
  if (isObj(v)) {
    const out: Obj = {};
    for (const k of Object.keys(v).sort()) {
      if (STAMPS.has(k)) continue;
      const t = tidy(v[k]);
      if (blank(t)) continue;
      out[k] = t;
    }
    return out;
  }
  return v;
}

function blank(v: unknown): boolean {
  return v === undefined || v === null || v === '' || (Array.isArray(v) && !v.length) || (isObj(v) && !Object.keys(v).length);
}

// The key an element is named by between brackets: a string is itself,
// anything else its canonical JSON.
export function elementKey(v: unknown): string {
  return typeof v === 'string' ? v : canonical(v);
}

// The fields of an entity on both sides, recursively: an object opens into
// its keys until a leaf; a list is one field whose elements are chips.
export function fieldsOf(e: Entity): Field[] {
  const out: Field[] = [];
  const walk = (path: string, l: unknown, r: unknown, res: unknown) => {
    if (Array.isArray(l) || Array.isArray(r)) {
      out.push({ path, set: true, left: l, right: r, result: res, same: same(l, r) });
      return;
    }
    if (isObj(l) || isObj(r)) {
      const keys = new Set([...(isObj(l) ? Object.keys(l) : []), ...(isObj(r) ? Object.keys(r) : [])]);
      for (const k of [...keys].sort()) {
        if (STAMPS.has(k) || (path === '' && k === 'id') || (path === '' && e.kind === 'role' && k === 'name')) continue;
        if (path === '' && e.kind === 'route' && k === 'order') continue; // follows the route, never a field
        walk(path ? `${path}.${k}` : k, isObj(l) ? l[k] : undefined, isObj(r) ? r[k] : undefined, isObj(res) ? res[k] : undefined);
      }
      return;
    }
    out.push({ path, set: false, left: l, right: r, result: res, same: same(l, r) });
  };
  walk('', e.left, e.right, e.result);
  return out;
}

// The elements of a list field: the result's in its order, then the rest
// of the left's, then the right's additions.
export function elementsOf(f: Field): Element[] {
  const out: Element[] = [];
  const seen = new Map<string, Element>();
  const add = (list: unknown, where: 'res' | 'left' | 'right') => {
    for (const v of Array.isArray(list) ? list : []) {
      const key = elementKey(v);
      let el = seen.get(key);
      if (!el) {
        el = { key, val: v, inL: false, inR: false, inRes: false };
        seen.set(key, el);
        out.push(el);
      }
      if (where === 'res') el.inRes = true;
      if (where === 'left') el.inL = true;
      if (where === 'right') el.inR = true;
    }
  };
  add(f.result, 'res');
  add(f.left, 'left');
  add(f.right, 'right');
  return out;
}

// Where a list's result comes from: only left-side elements, only
// right-side ones, some of each, or nothing.
export function listOrigin(els: Element[]): 'left' | 'right' | 'both' | 'empty' | 'common' {
  const present = els.filter((el) => el.inRes);
  if (!present.length) return 'empty';
  const lo = present.some((el) => el.inL && !el.inR);
  const ro = present.some((el) => el.inR && !el.inL);
  return lo && ro ? 'both' : ro ? 'right' : lo ? 'left' : 'common';
}

// A value as a chip or a line reads it.
export function short(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v !== 'object') return String(v);
  if (Array.isArray(v)) return v.map((x) => (typeof x === 'object' ? compact(x) : String(x))).join(', ');
  return compact(v);
}

// A picture (an icon, a logo, any SVG or data URI) says nothing in a
// summary and hides what does: it is left out of one.
const PICTURES = new Set(['icon', 'logo', 'image', 'favicon']);
function picture(k: string, v: unknown): boolean {
  return PICTURES.has(k) || (typeof v === 'string' && (v.startsWith('<svg') || v.startsWith('data:image/')));
}

export function compact(o: unknown): string {
  if (Array.isArray(o)) return '[' + o.map(compact).join(', ') + ']';
  if (isObj(o)) {
    return (
      '{' +
      Object.keys(o)
        .filter((k) => !picture(k, o[k]))
        .map((k) => k + ': ' + compact(o[k]))
        .join(', ') +
      '}'
    );
  }
  return String(o);
}

// The recipe as the page keeps it: picks by path, pairs, and the order of
// the lists dragged into order.
export class Recipe {
  readonly picks = new Map<string, ComposePick>();
  readonly pairs: { kind: string; left: string; right: string }[] = [];

  // What the operator said about a path, if anything.
  said(path: string): ComposePick | undefined {
    return this.picks.get(path);
  }

  // The object whole, from a side or out of the result: what was said under
  // it is forgotten, a whole is a whole.
  whole(path: string, from: 'left' | 'right' | 'none'): void {
    this.forgetUnder(path);
    this.picks.set(path, { path, from });
  }

  // A field from a side, or cleared.
  field(path: string, from: 'left' | 'right' | 'unset'): void {
    this.forgetUnder(path);
    this.picks.set(path, { path, from });
  }

  element(listPath: string, key: string, op: 'add' | 'drop'): void {
    const path = `${listPath}[${key}]`;
    this.picks.set(path, { path, op });
  }

  order(listPath: string, keys: string[]): void {
    const path = `${listPath}#order`;
    this.picks.set(path, { path: listPath, order: keys });
  }

  pair(kind: string, left: string, right: string): void {
    this.pairs.push({ kind, left, right });
  }

  // Forget what was said about an object and everything under it.
  forget(path: string): void {
    this.picks.delete(path);
    this.forgetUnder(path);
  }

  private forgetUnder(path: string): void {
    for (const k of [...this.picks.keys()]) {
      if (k.startsWith(path + '/') || k.startsWith(path + '[') || k.startsWith(path + '#') || k.startsWith(path + '.')) {
        this.picks.delete(k);
      }
    }
  }

  // Whether the operator said something about an object or anything under it.
  touched(path: string): boolean {
    for (const k of this.picks.keys()) {
      if (k === path || k.startsWith(path + '/')) return true;
    }
    return false;
  }

  list(): ComposePick[] {
    return [...this.picks.values()];
  }

  reset(): void {
    this.picks.clear();
    this.pairs.length = 0;
  }
}

// What rides beside a route, per side: its deposited spec and its uploaded
// files, with their content's hashes.
export interface MediaInfo {
  spec?: boolean;
  files?: number;
  specSum?: string;
  fileSums?: Record<string, string>;
}

// What moved beside a route between two sides: 'files', 'spec'.
export function mediaMoved(l?: MediaInfo, r?: MediaInfo): string[] {
  const out: string[] = [];
  if (canonical(l?.fileSums ?? {}) !== canonical(r?.fileSums ?? {})) out.push('files');
  if ((l?.specSum ?? '') !== (r?.specSum ?? '')) out.push('spec');
  return out;
}

// One file of a route seen on both sides: the same, rewritten, or on one
// side only.
export interface FileLine {
  name: string;
  state: 'same' | 'changed' | 'left' | 'right';
}

export function fileLines(l?: MediaInfo, r?: MediaInfo): FileLine[] {
  const lf = l?.fileSums ?? {};
  const rf = r?.fileSums ?? {};
  const names = [...new Set([...Object.keys(lf), ...Object.keys(rf)])].sort();
  return names.map((name) => ({
    name,
    state: !(name in rf) ? 'left' : !(name in lf) ? 'right' : lf[name] === rf[name] ? 'same' : 'changed',
  }));
}

// One block of code as the two sides have it, for the side-by-side view: a
// side that lacks it is null.
export interface CodePair {
  label: string;
  lang: 'css' | 'js' | 'text';
  left: string | null;
  right: string | null;
}

// The code a field holds that differs between the sides. A route's own
// blocks (ui.injections) are matched in three passes, so that only a change
// of code is shown as one: first the blocks whose code is the same on both
// sides - moved to another place, switched off or on, but not rewritten, and
// so no code to compare (the chips say where each one is); then the blocks
// of the same kind, place and file, in order, rewritten in place; then those
// of the same kind and file, rewritten AND moved. What is left is on one side
// only. A file's content is compared in the files line. Any other text over
// several lines - a locale hook, a template - is one block.
export function codePairs(f: Field): CodePair[] {
  if (f.same) return [];
  if (f.set) {
    if (!/(^|\.)injections$/.test(f.path)) return [];
    const blocks = (list: unknown) => (Array.isArray(list) ? list : []).filter(isObj) as Obj[];
    const L = blocks(f.left);
    const R = blocks(f.right);
    const usedL = new Set<number>();
    const usedR = new Set<number>();
    const code = (o: Obj) => (typeof o['code'] === 'string' ? (o['code'] as string) : '');
    // What the code says, blanks at the ends of lines and of the block left
    // out: a block moved in the editor often gains or loses a last empty line,
    // and that is not a change of code.
    const said = (t: string | null) => (t ?? '').split('\n').map((l) => l.trimEnd()).join('\n').trim();
    const file = (o: Obj) => (typeof o['file'] === 'string' ? (o['file'] as string) : '');
    const what = (o: Obj) => `${String(o['kind']).toUpperCase()} ${file(o) || 'code'}`;
    const lang = (o: Obj): 'css' | 'js' => (o['kind'] === 'js' ? 'js' : 'css');
    // Pairs every unused left block with the first unused right one the
    // test accepts, in order.
    const match = (accept: (l: Obj, r: Obj) => boolean, found: (l: Obj, r: Obj) => void) => {
      L.forEach((l, i) => {
        if (usedL.has(i)) return;
        const j = R.findIndex((r, k) => !usedR.has(k) && accept(l, r));
        if (j < 0) return;
        usedL.add(i);
        usedR.add(j);
        found(l, R[j]);
      });
    };
    const out: CodePair[] = [];
    // 1. The same code: nothing to compare.
    match(
      (l, r) => l['kind'] === r['kind'] && file(l) === file(r) && said(code(l)) === said(code(r)),
      () => undefined,
    );
    // 2. Rewritten where it stands.
    match(
      (l, r) => l['kind'] === r['kind'] && file(l) === file(r) && l['position'] === r['position'],
      (l, r) => out.push({ label: `${what(l)}, ${l['position']}`, lang: lang(l), left: code(l), right: code(r) }),
    );
    // 3. Rewritten and moved.
    match(
      (l, r) => l['kind'] === r['kind'] && file(l) === file(r),
      (l, r) =>
        out.push({ label: `${what(l)}, moved from ${l['position']} to ${r['position']}`, lang: lang(l), left: code(l), right: code(r) }),
    );
    // 4. On one side only.
    L.forEach((l, i) => !usedL.has(i) && out.push({ label: `${what(l)}, ${l['position']}`, lang: lang(l), left: code(l), right: null }));
    R.forEach((r, j) => !usedR.has(j) && out.push({ label: `${what(r)}, ${r['position']}`, lang: lang(r), left: null, right: code(r) }));
    // A block of a file has no code of its own here: one on a side only says
    // nothing the chips do not.
    return out.filter((p) => (p.left === null || p.right === null ? said(p.left ?? p.right) !== '' : said(p.left) !== said(p.right)));
  }
  const multi = (v: unknown) => typeof v === 'string' && v.includes('\n');
  if (!multi(f.left) && !multi(f.right)) return [];
  const text = (v: unknown) => (typeof v === 'string' ? v : v === undefined ? null : String(v));
  return [{ label: f.path, lang: /onChange$/.test(f.path) ? 'js' : 'text', left: text(f.left), right: text(f.right) }];
}
