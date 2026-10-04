// GitHub Pages serves files, not routes - so every page of the site is written
// as a file. Two things follow, and both matter for a site that is a product's
// front door:
//
//   - a deep link answers 200 instead of the 404 an SPA fallback returns, with
//     the page's own <title>, description and Open Graph tags. A link pasted in
//     a chat or crawled by a search engine says what the page is.
//   - 404.html is still written, for an address no page claims: the shell boots
//     there too and shows its own not-found.
//
// The body is not pre-rendered - the content is fetched by the application, as
// it is everywhere else. What is written here is the head, which is what a
// crawler and a link preview read before any script runs.
import { mkdir, readFile, readdir, writeFile, copyFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const OUT = join(here, '..', 'dist', 'doc-site', 'browser');
const SITE = join(OUT, 'assets', 'site');
const LANGS = ['en', 'fr'];
const ORIGIN = 'https://www.softwarity.io';

const index = await readFile(join(OUT, 'index.html'), 'utf8');
await writeFile(join(OUT, '404.html'), index);

// What the bundle needs, taken from the index the builder just wrote: the
// hashes change on every build, so they are read rather than written down.
const base = index.match(/<base href="([^"]*)"/)?.[1] ?? '/';
const head = [
  ...(index.match(/<link[^>]*>/g) ?? []),
  ...(index.match(/<script[^>]*><\/script>/g) ?? []),
];

const escape = (s) =>
  String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// One file for a page: its head, and the shell.
//
//   canonical  the address search engines keep for this file
//   alternates the same page in each language, for a crawler: an address names
//              a page and ?lg= names the language it is read in
function document(lang, page, canonical, alternates) {
  // Same rule as the runtime: the company is the brand, Meerkat is the
  // product, and the tab says which of the two the page is about.
  const house = page.area === 'softwarity' || page.area === 'project' ? 'Softwarity' : 'Meerkat';
  const title = page.slug === 'index' ? `${page.title} | Softwarity` : `${page.title} | ${house}`;
  return `<!doctype html>
<html lang="${lang}">
<head>
<meta charset="utf-8">
<title>${escape(title)}</title>
<base href="${base}">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="description" content="${escape(page.summary)}">
<link rel="canonical" href="${canonical}">
${alternates.join('\n')}
<meta property="og:type" content="article">
<meta property="og:url" content="${canonical}">
<meta property="og:title" content="${escape(page.title)}">
<meta property="og:description" content="${escape(page.summary)}">
<meta name="author" content="Softwarity">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="${escape(page.title)}">
<meta name="twitter:description" content="${escape(page.summary)}">
${head.join('\n')}
</head>
<body><app-root></app-root></body>
</html>
`;
}

async function write(path, html) {
  const target = join(OUT, path, 'index.html');
  await mkdir(dirname(target), { recursive: true });
  await writeFile(target, html);
}

// Which languages a page exists in, to say so in its head.
const langsOf = new Map();
const pages = new Map();
for (const lang of LANGS) {
  const dir = join(SITE, lang, 'p');
  for (const file of await readdir(dir)) {
    if (!file.endsWith('.json')) continue;
    const page = JSON.parse(await readFile(join(dir, file), 'utf8'));
    pages.set(`${lang}/${page.slug}`, page);
    langsOf.set(page.slug, [...(langsOf.get(page.slug) ?? []), lang]);
  }
}

let written = 0;
for (const [slug, langs] of langsOf) {
  // With the trailing slash Pages itself serves: every page is a directory,
  // so /docs answers a 301 to /docs/, and a canonical that redirects is one
  // nobody should have written.
  const path = slug === 'index' ? '' : `${slug}/`;
  const bare = `${ORIGIN}${base}${path}`;
  const alternates = [
    ...langs.map((l) => `<link rel="alternate" hreflang="${l}" href="${bare}?lg=${l}">`),
    `<link rel="alternate" hreflang="x-default" href="${bare}">`,
  ];
  // The page itself, at the address that names no language. Its head is in
  // the first language it exists in - a crawler reads one - and the reader's
  // own is chosen when the application starts.
  if (slug !== 'index') {
    // The home page is the index.html the builder wrote; it is left alone.
    await write(path, document(langs[0], pages.get(`${langs[0]}/${slug}`), bare, alternates));
    written++;
  }
  // And the addresses of before, /en/... and /fr/..., which were sent and
  // indexed: they answer 200 with their own head, name the new address as the
  // canonical one, and the application takes the reader there.
  for (const lang of langs) {
    const old = slug === 'index' ? lang : `${lang}/${slug}`;
    await write(old, document(lang, pages.get(`${lang}/${slug}`), `${bare}?lg=${lang}`, alternates));
    written++;
  }
}

console.log(`[static-pages] ${written} pages + 404.html`);
