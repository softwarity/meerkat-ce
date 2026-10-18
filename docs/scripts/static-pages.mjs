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
// The body carries the page too: its HTML, the same the application fetches,
// inside <app-root>. A crawler and a reader without scripts get the text from
// the file itself; the application replaces it when it starts, as Angular
// empties the host element of the root component it boots.
//
// Last, the two files a crawler asks for first: sitemap.xml, one address per
// page, and robots.txt, which names it.
import { mkdir, readFile, readdir, writeFile, copyFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const OUT = join(here, '..', 'dist', 'doc-site', 'browser');
const SITE = join(OUT, 'assets', 'site');
const LANGS = ['en', 'fr'];
const ORIGIN = 'https://meerkat.softwarity.dev';
// The card a link preview shows: 1200 x 630, the size summary_large_image asks.
const CARD = 'img/og/meerkat.png';

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

// One file for a page: its head, and the shell holding the page's text.
//
//   canonical  the address search engines keep for this file
//   alternates the same page in each language, for a crawler: an address names
//              a page and ?lg= names the language it is read in
function document(lang, page, canonical, alternates) {
  // Same rule as the runtime: the company is the brand, Meerkat is the
  // product, and the tab says which of the two the page is about.
  const house = page.area === 'softwarity' || page.area === 'project' ? 'Softwarity' : 'Meerkat';
  const name = titleOf(lang, page);
  const title = page.slug === 'index' ? `Meerkat: ${name}` : `${name} | ${house}`;
  const card = `${ORIGIN}${base}${CARD}`;
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
<meta property="og:type" content="${page.slug === 'index' ? 'website' : 'article'}">
<meta property="og:site_name" content="Meerkat">
<meta property="og:url" content="${canonical}">
<meta property="og:title" content="${escape(title)}">
<meta property="og:description" content="${escape(page.summary)}">
<meta property="og:image" content="${card}">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="Meerkat, authentication proxy and API gateway, and its administration console">
<meta name="author" content="Softwarity">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="${escape(title)}">
<meta name="twitter:description" content="${escape(page.summary)}">
<meta name="twitter:image" content="${card}">
<style>.prerender{max-width:860px;margin:32px auto;padding:0 16px;font-family:Roboto,Helvetica,Arial,sans-serif;line-height:1.5}.prerender img,.prerender svg{max-width:100%;height:auto}</style>
${head.join('\n')}
</head>
<body><app-root><main class="prerender">${page.html ?? ''}</main></app-root></body>
</html>
`;
}

// Two pages of one language with the same title ("Routes", the console's
// screen and the concept) would share a tab and a search result: the second
// word says which one.
const titleCount = new Map();
function titleOf(lang, page) {
  const n = titleCount.get(`${lang}/${page.title}`) ?? 1;
  return n > 1 && page.section ? `${page.title} - ${page.section}` : page.title;
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

for (const [key, page] of pages) {
  const k = `${key.split('/')[0]}/${page.title}`;
  titleCount.set(k, (titleCount.get(k) ?? 0) + 1);
}

let written = 0;
const sitemap = [];
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
  // The home page too: index.html is rewritten with its own head and text,
  // after 404.html took the shell as the builder wrote it.
  await write(path, document(langs[0], pages.get(`${langs[0]}/${slug}`), bare, alternates));
  sitemap.push(bare);
  written++;
  // And the addresses of before, /en/... and /fr/..., which were sent and
  // indexed: they answer 200 with their own head, name the new address as the
  // canonical one, and the application takes the reader there.
  for (const lang of langs) {
    const old = slug === 'index' ? lang : `${lang}/${slug}`;
    await write(old, document(lang, pages.get(`${lang}/${slug}`), `${bare}?lg=${lang}`, alternates));
    written++;
  }
}

// One address per page, the canonical one: the /en/ and /fr/ addresses of
// before name it themselves and are left out.
await writeFile(
  join(OUT, 'sitemap.xml'),
  `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${sitemap.map((u) => `  <url><loc>${escape(u)}</loc></url>`).join('\n')}
</urlset>
`,
);
await writeFile(join(OUT, 'robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${ORIGIN}${base}sitemap.xml\n`);

console.log(`[static-pages] ${written} pages + 404.html, sitemap.xml (${sitemap.length} addresses), robots.txt`);
