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
// Each head also says, in structured data (JSON-LD), what the page is: the
// home page describes the product and its publisher, every other page where it
// sits in the site (a breadcrumb a search result can show), and a
// documentation page is a technical article with the date it last changed.
//
// Last, the two files a crawler asks for first: sitemap.xml, one address per
// page with the date it last changed, and robots.txt, which names it.
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdir, readFile, readdir, writeFile, copyFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const OUT = join(here, '..', 'dist', 'doc-site', 'browser');
const SITE = join(OUT, 'assets', 'site');
const CONTENT = join(here, '..', 'content');
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
  // Not a page's own links: run twice over one build, the index read here is
  // the one this script wrote, and they would be copied into every page.
  ...(index.match(/<link[^>]*>/g) ?? []).filter((l) => !/rel="(canonical|alternate)"/.test(l)),
  ...(index.match(/<script[^>]*><\/script>/g) ?? []),
];

const AREAS = JSON.parse(await readFile(join(CONTENT, 'areas.json'), 'utf8'));
const VERSION = JSON.parse(await readFile(join(OUT, 'assets', 'version.json'), 'utf8')).version;

// When a page last changed: the last commit that touched its Markdown, in any
// language. The deploy checks out the whole history, so this is the page's own
// date; a page generated from data (no Markdown of its own) has none, and a
// sitemap says nothing rather than something false.
function changed(slug) {
  const files = [];
  for (const lang of LANGS) {
    for (const f of [`${slug}.md`, `${slug}/index.md`]) {
      const path = join(CONTENT, lang, f);
      if (existsSync(path)) files.push(path);
    }
  }
  if (files.length === 0) return '';
  try {
    return execFileSync('git', ['log', '-1', '--format=%cI', '--', ...files], { cwd: CONTENT, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

const escape = (s) =>
  String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// One file for a page: its head, and the shell holding the page's text.
//
//   canonical  the address search engines keep for this file
//   alternates the same page in each language, for a crawler: an address names
//              a page and ?lg= names the language it is read in
function document(lang, page, canonical, alternates, modified) {
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
<script type="application/ld+json">${ld(lang, page, canonical, title, modified)}</script>
<style>.prerender{max-width:860px;margin:32px auto;padding:0 16px;font-family:Roboto,Helvetica,Arial,sans-serif;line-height:1.5}.prerender img,.prerender svg{max-width:100%;height:auto}</style>
${head.join('\n')}
</head>
<body><app-root><main class="prerender">${page.html ?? ''}</main></app-root></body>
</html>
`;
}

// What the page is, for a search engine. The publisher and the product are
// named once, on the home page, and the other pages point at them by @id.
function ld(lang, page, canonical, title, modified) {
  const home = `${ORIGIN}${base}`;
  const org = { '@type': 'Organization', '@id': `${home}#softwarity`, name: 'Softwarity', url: `${home}softwarity/`, logo: `${home}img/softwarity.svg` };
  const graph = [];
  if (page.slug === 'index') {
    graph.push(
      org,
      { '@type': 'WebSite', '@id': `${home}#site`, url: home, name: 'Meerkat', inLanguage: ['en', 'fr'], publisher: { '@id': org['@id'] } },
      {
        '@type': 'SoftwareApplication',
        '@id': `${home}#meerkat`,
        name: 'Meerkat',
        description: page.summary,
        url: home,
        image: `${home}${CARD}`,
        applicationCategory: 'SecurityApplication',
        operatingSystem: 'Linux, Docker, Kubernetes',
        ...(VERSION && VERSION !== '0.0.0' ? { softwareVersion: VERSION } : {}),
        downloadUrl: 'https://hub.docker.com/r/softwarity/meerkat',
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'EUR', description: 'Community edition' },
        publisher: { '@id': org['@id'] },
      },
    );
  } else {
    // Home, the part of the site, the page: the part only when it is not the
    // page itself.
    // A page read in a named language (?lg=fr) keeps it along the trail.
    const area = AREAS.find((a) => a.id === page.area);
    const q = canonical.includes('?lg=') ? `?lg=${lang}` : '';
    const crumbs = [{ name: lang === 'fr' ? 'Accueil' : 'Home', item: `${home}${q}` }];
    if (area && !area.hidden && area.home !== page.slug) {
      crumbs.push({ name: area.label?.[lang] ?? area.label?.en, item: `${home}${area.home}/${q}` });
    }
    crumbs.push({ name: page.title, item: canonical });
    graph.push({
      '@type': 'BreadcrumbList',
      itemListElement: crumbs.map((c, i) => ({ '@type': 'ListItem', position: i + 1, ...c })),
    });
    if (page.slug.startsWith('docs/')) {
      graph.push({
        '@type': 'TechArticle',
        headline: page.title,
        description: page.summary,
        url: canonical,
        inLanguage: lang,
        ...(modified ? { dateModified: modified } : {}),
        about: { '@id': `${home}#meerkat` },
        publisher: { '@id': org['@id'] },
      });
    }
  }
  // A closing tag inside a string would end the script element early.
  return JSON.stringify({ '@context': 'https://schema.org', '@graph': graph }).replace(/</g, '\\u003c');
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
  // Each language's address: the first one is the address that names none,
  // the others name theirs (?lg=fr), which the host answers with that
  // language's file (functions/_middleware.js).
  const at = (l) => (l === langs[0] ? bare : `${bare}?lg=${l}`);
  const alternates = [
    ...langs.map((l) => `<link rel="alternate" hreflang="${l}" href="${at(l)}">`),
    `<link rel="alternate" hreflang="x-default" href="${bare}">`,
  ];
  // The page itself, at the address that names no language. Its head is in
  // the first language it exists in - a crawler reads one - and the reader's
  // own is chosen when the application starts.
  // The home page too: index.html is rewritten with its own head and text,
  // after 404.html took the shell as the builder wrote it.
  const modified = changed(slug);
  await write(path, document(langs[0], pages.get(`${langs[0]}/${slug}`), bare, alternates, modified));
  for (const l of langs) sitemap.push({ loc: at(l), modified, langs: langs.map((o) => [o, at(o)]), bare });
  written++;
  // One file per language under /en/ and /fr/: what ?lg= is answered with,
  // and the addresses of before, which were sent and indexed - the host
  // redirects those to the address that replaced them, and a host that cannot
  // (a plain file server) answers the file, whose canonical names it.
  for (const lang of langs) {
    const old = slug === 'index' ? lang : `${lang}/${slug}`;
    await write(old, document(lang, pages.get(`${lang}/${slug}`), at(lang), alternates, modified));
    written++;
  }
}

// One address per page and language, each naming its versions in the other
// languages - the sitemap form of hreflang. The /en/ and /fr/ files are left
// out: they are what an address is answered with, not addresses.
const xhtml = (u) =>
  [...u.langs.map(([l, href]) => [l, href]), ['x-default', u.bare]]
    .map(([l, href]) => `<xhtml:link rel="alternate" hreflang="${l}" href="${escape(href)}"/>`)
    .join('');
await writeFile(
  join(OUT, 'sitemap.xml'),
  `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${sitemap
  .map(
    (u) =>
      `  <url><loc>${escape(u.loc)}</loc>${u.modified ? `<lastmod>${u.modified}</lastmod>` : ''}${u.langs.length > 1 ? xhtml(u) : ''}</url>`,
  )
  .join('\n')}
</urlset>
`,
);

// For Cloudflare Pages, which reads both files and which GitHub Pages serves
// as two more files nobody asks for.
//
// _headers: the bundle's files carry a hash in their name, so they never
// change - a year in the browser's cache, where a file host gives everything
// ten minutes. Everything else keeps the host's default, which revalidates.
// And the headers a product about security should send on its own site.
//
// _routes.json: only pages run the function that serves ?lg=; the bundle, the
// content and the pictures are files, served for nothing.
const hashed = (await readdir(OUT)).filter((f) => /-[A-Za-z0-9_-]{8}\.(js|css)$/.test(f));
await writeFile(
  join(OUT, '_headers'),
  `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  X-Frame-Options: SAMEORIGIN
  Strict-Transport-Security: max-age=31536000
${hashed.map((f) => `\n/${f}\n  Cache-Control: public, max-age=31536000, immutable`).join('\n')}
`,
);
// The directories that hold no page: assets, pictures, downloads.
const pageDirs = new Set([...langsOf.keys()].map((slug) => slug.split('/')[0]));
const dirs = (await readdir(OUT, { withFileTypes: true }))
  .filter((d) => d.isDirectory() && !pageDirs.has(d.name) && !LANGS.includes(d.name))
  .map((d) => `/${d.name}/*`);
await writeFile(
  join(OUT, '_routes.json'),
  JSON.stringify({ version: 1, include: ['/*'], exclude: [...dirs, ...hashed.map((f) => `/${f}`), '/*.json', '/*.xml', '/*.txt', '/*.svg', '/*.png'] }, null, 2),
);
await writeFile(join(OUT, 'robots.txt'), `User-agent: *\nAllow: /\n\nSitemap: ${ORIGIN}${base}sitemap.xml\n`);

console.log(`[static-pages] ${written} pages + 404.html, sitemap.xml (${sitemap.length} addresses), robots.txt, _headers, _routes.json`);
