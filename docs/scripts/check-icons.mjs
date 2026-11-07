// The icon font is cut down to the icons the site uses (icon_names in
// src/index.html): 7 KB instead of 720 KB - and instead of the 4 MB variable
// font the projects menu would otherwise load (it uses this one when the page
// has it). The price is that an icon missing from that list shows as its name,
// in letters. This check refuses the build first: every icon a template names,
// every area's icon, and the projects menu's own, must be in the list.
//
// An icon chosen in an expression ({{ x ? 'a' : 'b' }}) is found by its quoted
// names; a new expression of another shape belongs in EXTRA below.
import { readFile, readdir } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
// The projects menu (unpkg.com/@softwarity/projects), drawn in its own shadow
// root with this page's font.
const EXTRA = ['chevron_left', 'chevron_right', 'expand_more', 'favorite', 'open_in_new'];

const index = await readFile(join(root, 'src', 'index.html'), 'utf8');
const names = index.match(/icon_names=([a-z0-9_,]+)/)?.[1].split(',') ?? [];
const listed = new Set(names);
if (names.join(',') !== [...names].sort().join(',')) {
  console.error('[check-icons] src/index.html: icon_names must be in alphabetical order, or Google Fonts refuses the list');
  process.exit(1);
}

const used = new Set(EXTRA);
async function walk(dir) {
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) await walk(p);
    else if (/\.(html|ts)$/.test(e.name)) {
      const src = await readFile(p, 'utf8');
      for (const m of src.matchAll(/<mat-icon(?![^>]*svgIcon)[^>]*>([^<]*)<\/mat-icon>/g)) {
        const body = m[1].trim();
        if (/^[a-z0-9_]+$/.test(body)) used.add(body);
        else for (const q of body.matchAll(/'([a-z0-9_]+)'/g)) if (!['light', 'dark', 'system'].includes(q[1])) used.add(q[1]);
      }
    }
  }
}
await walk(join(root, 'src'));
for (const a of JSON.parse(await readFile(join(root, 'content', 'areas.json'), 'utf8'))) {
  if (!a.icon.startsWith('svg:')) used.add(a.icon);
}

const missing = [...used].filter((n) => !listed.has(n)).sort();
if (missing.length) {
  console.error(`[check-icons] used but not in icon_names (src/index.html): ${missing.join(', ')}`);
  process.exit(1);
}
const unused = [...listed].filter((n) => !used.has(n));
console.log(`[check-icons] ${listed.size} icons${unused.length ? `, unused: ${unused.join(', ')}` : ''}`);
