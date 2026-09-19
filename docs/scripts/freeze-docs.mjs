// Freeze the documentation as it stands, under a version number.
//
//   npm run docs:freeze -- 1.0
//
// The working tree under content/<lang>/docs keeps moving with the code - it
// is what /docs/next serves. This copies it to content/versions/<v>/, copies
// the screenshots it names beside it so an old page keeps the console it was
// written about, and makes that version the current one: from then on
// /<lang>/docs/... serves the frozen copy, and the addresses never move again.
import { cp, mkdir, readFile, readdir, writeFile, stat } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const CONTENT = join(here, '..', 'content');
const PUBLIC = join(here, '..', 'public');
const LANGS = ['en', 'fr'];

const version = process.argv[2];
if (!version || !/^\d+(\.\d+)*$/.test(version)) {
  console.error('usage: npm run docs:freeze -- 1.0   (a version number, digits and dots)');
  process.exit(1);
}

const file = join(CONTENT, 'versions.json');
const versions = JSON.parse(await readFile(file, 'utf8'));
if (versions.versions.includes(version)) {
  console.error(`${version} is already frozen - delete content/versions/${version} first to redo it`);
  process.exit(1);
}

async function* markdown(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* markdown(full);
    else if (entry.name.endsWith('.md')) yield full;
  }
}

const imageDir = join(PUBLIC, 'img', 'v', version);
let images = 0;
let pages = 0;

for (const lang of LANGS) {
  const from = join(CONTENT, lang, 'docs');
  const to = join(CONTENT, 'versions', version, lang, 'docs');
  await cp(from, to, { recursive: true });

  // An old page keeps the screenshots it was written about. Only the ones it
  // names are copied - the gallery of the site as it is today is not history.
  for await (const md of markdown(to)) {
    let raw = await readFile(md, 'utf8');
    let touched = false;
    for (const [, src] of raw.matchAll(/!\[[^\]]*\]\(([^)\s]+)\)/g)) {
      if (/^https?:/.test(src) || src.startsWith(`img/v/${version}/`)) continue;
      const source = join(PUBLIC, src);
      if (!(await stat(source).catch(() => null))) continue;
      const target = join(imageDir, relative(join(PUBLIC, 'img'), source));
      await mkdir(dirname(target), { recursive: true });
      await cp(source, target);
      raw = raw.split(`(${src})`).join(`(img/v/${version}/${relative(join(PUBLIC, 'img'), source)})`);
      touched = true;
      images++;
    }
    if (touched) await writeFile(md, raw);
    pages++;
  }
}

versions.versions = [version, ...versions.versions];
versions.current = version;
await writeFile(file, JSON.stringify(versions, null, 2) + '\n');

console.log(`[freeze-docs] ${version}: ${pages} pages, ${images} images -> content/versions/${version}`);
console.log(`[freeze-docs] current is now ${version}; the working tree is served at /<lang>/docs/next`);
