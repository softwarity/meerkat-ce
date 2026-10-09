// Downloads, ONCE, the fonts the gateway ships for its built-in pages
// (THEME-09), and writes them under internal/fonts for go:embed:
//
//   node tools/fonts/fetch.mjs
//
// Nothing is fetched at run time: a gateway serves these files itself, so a
// visitor never contacts Google and an offline installation has them too. Run
// it again only to change the selection; the files it writes are committed.
//
// What is kept, and why:
//   - the variable version of each family, weight axis only: one file covers
//     every weight the pages use, smaller than five static files;
//   - only the subsets our twenty languages need: latin, latin-ext (Polish,
//     Turkish), vietnamese and the two cyrillic ones for a family the
//     integrator chooses, and the one script of each Noto companion (Arabic,
//     Hebrew, Devanagari, Thai). Chinese, Japanese and Korean are left to the
//     system's own fonts, which are excellent on every platform and would
//     weigh 13 MB here;
//   - each family's licence (all are SIL OFL or Apache 2.0, which allow
//     redistribution with the licence attached).
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const out = fileURLToPath(new URL('../../internal/fonts/', import.meta.url));
// A browser that asks for woff2: Google answers each user agent with the
// format it can read.
const UA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36';
const BASE = ['latin', 'latin-ext', 'vietnamese', 'cyrillic', 'cyrillic-ext'];

// kind: what the console offers it for. A companion is never chosen: it is
// what the browser falls back to, character by character, for a script the
// chosen family does not draw.
const FAMILIES = [
  ['Inter', 'sans'], ['Roboto', 'sans'], ['Open Sans', 'sans'], ['Montserrat', 'sans'],
  ['Nunito', 'sans'], ['Raleway', 'sans'], ['Noto Sans', 'sans'],
  ['Source Serif 4', 'serif'], ['Lora', 'serif'], ['Merriweather', 'serif'],
  ['Playfair Display', 'serif'], ['Noto Serif', 'serif'],
  ['JetBrains Mono', 'mono'], ['Roboto Mono', 'mono'],
  ['Noto Sans Arabic', 'companion', ['arabic']], ['Noto Sans Hebrew', 'companion', ['hebrew']],
  ['Noto Sans Devanagari', 'companion', ['devanagari']], ['Noto Sans Thai', 'companion', ['thai']],
];

const get = async (url, as = 'text') => {
  const res = await fetch(url, { headers: { 'User-Agent': UA } });
  if (!res.ok) throw new Error(`${url}: ${res.status}`);
  return as === 'text' ? res.text() : Buffer.from(await res.arrayBuffer());
};

const meta = JSON.parse(await get('https://fonts.google.com/metadata/fonts'));
const byName = new Map(meta.familyMetadataList.map((f) => [f.family, f]));
const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-');

rmSync(`${out}files`, { recursive: true, force: true });
rmSync(`${out}licenses`, { recursive: true, force: true });
mkdirSync(`${out}files`, { recursive: true });
mkdirSync(`${out}licenses`, { recursive: true });

const catalogue = [];
let total = 0;
for (const [family, kind, only] of FAMILIES) {
  const m = byName.get(family);
  if (!m) throw new Error(`${family}: not in the Google Fonts catalogue`);
  const wght = m.axes.find((a) => a.tag === 'wght');
  if (!wght) throw new Error(`${family}: no weight axis`);
  const range = `${wght.min}..${wght.max}`;
  const css = await get(`https://fonts.googleapis.com/css2?family=${encodeURIComponent(family)}:wght@${range}&display=swap`);
  const keep = only ?? BASE;
  const faces = [];
  for (const block of css.split('/* ').slice(1)) {
    const subset = block.slice(0, block.indexOf(' */'));
    if (!keep.includes(subset)) continue;
    const url = block.match(/url\((https:[^)]+)\)/)[1];
    const unicodeRange = block.match(/unicode-range: ([^;]+);/)[1];
    const file = `${slug(family)}-${subset}.woff2`;
    const data = await get(url, 'buffer');
    writeFileSync(`${out}files/${file}`, data);
    total += data.length;
    faces.push({ subset, file, unicodeRange });
  }
  const missing = keep.filter((s) => !faces.some((f) => f.subset === s) && s !== 'cyrillic-ext');
  if (missing.length) throw new Error(`${family}: Google served no ${missing.join(', ')}`);
  // The licence, from the repository Google publishes the families from.
  const dir = family.toLowerCase().replace(/[^a-z0-9]/g, '');
  let licence = '';
  for (const path of [`ofl/${dir}/OFL.txt`, `apache/${dir}/LICENSE.txt`, `ufl/${dir}/UFL.txt`]) {
    try {
      licence = await get(`https://raw.githubusercontent.com/google/fonts/main/${path}`);
      break;
    } catch {
      /* the next licence directory */
    }
  }
  if (!licence) throw new Error(`${family}: no licence found in google/fonts`);
  writeFileSync(`${out}licenses/${slug(family)}.txt`, licence);
  catalogue.push({
    family, slug: slug(family), kind, category: m.category,
    weight: [wght.min, wght.max], faces,
  });
  console.log(`${family.padEnd(22)} ${kind.padEnd(9)} ${faces.length} files`);
}
writeFileSync(`${out}catalogue.json`, JSON.stringify(catalogue, null, 1) + '\n');
console.log(`total ${(total / 1024).toFixed(0)} KB in ${catalogue.reduce((n, f) => n + f.faces.length, 0)} files`);
