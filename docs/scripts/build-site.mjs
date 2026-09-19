// Turns the SITE SOURCE - Markdown, one file per page and per language, under
// docs/content - into what the browser reads at runtime:
//
//   src/assets/site/<lang>/p/<slug>.json        one page: title, html, headings
//   src/assets/site/<lang>/nav.json             the rail, its sections, its pages
//   src/assets/site/<lang>/search.json          the search index, built here
//
// Markdown rather than Angular templates, because the content IS the product's
// prose: it has to be writable, reviewable in a diff, and translatable without
// anyone opening a component. And JSON rather than shipping a Markdown parser
// to the browser: the conversion happens once, at build, and a reader
// downloads the page they asked for rather than the book.
//
// GitHub Pages serves files and nothing else - no server, no index endpoint -
// which is why the search index is a file too.
//
// THREE AXES, and only one of them is a build:
//   - language: a folder under content/, one JSON tree per language, ONE bundle
//     serving them all. Adding a language is adding a folder and a line in the
//     shell's dictionary; it is not a second compilation to deploy.
//   - version: only the `docs` area is versioned - see content/versions.json.
//     The current version is served with no version segment so its URLs never
//     move; the working tree is `next`; frozen ones live under content/versions/.
//   - area: the rail entries, declared once in content/areas.json.

import { mkdir, readFile, readdir, rm, writeFile, stat } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const CONTENT = join(here, '..', 'content');
const OUT = join(here, '..', 'src', 'assets', 'site');
const PUBLIC = join(here, '..', 'public');
const LANGS = ['en', 'fr'];
const STRICT = process.argv.includes('--strict');

// ---- Markdown --------------------------------------------------------------
//
// A small renderer rather than a dependency: the site uses a dozen constructs
// (headings, lists, tables, fenced code, links, emphasis, callouts, and the
// layout blocks below) and a parser that handles every corner of CommonMark
// would be a megabyte to audit for the same result. What is NOT supported
// fails loudly instead of rendering as something nobody meant.

const escapeHtml = (s) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Inline: code first, so nothing inside a span of code is taken for markup.
function inline(text) {
  const code = [];
  let out = text.replace(/`([^`]+)`/g, (_, c) => {
    code.push(`<code>${escapeHtml(c)}</code>`);
    // The marker is a NUL, written as an escape so it is VISIBLE here: a plain
    // number in the prose must not be mistaken for a placeholder, and a
    // literal NUL in the source reads as a space to everyone who looks.
    return `\u0000${code.length - 1}\u0000`;
  });
  out = escapeHtml(out);
  // Image paths are RELATIVE and stay relative: the document carries a <base
  // href>, so they resolve against the site root whatever route the reader is
  // on. Nothing to rewrite at runtime.
  out = out.replace(
    /!\[([^\]]*)\]\(([^)\s]+)\)/g,
    (_, alt, src) => `<img src="${src}" alt="${alt}" loading="lazy" />`,
  );
  out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, label, href) => {
    if (/^https?:/.test(href)) return `<a href="${href}" target="_blank" rel="noopener">${label}</a>`;
    if (href.startsWith('#')) return `<a href="${href}" data-at="${href.slice(1)}">${label}</a>`;
    // An internal link names a PAGE, not a URL: the reader's language and the
    // documentation version they are on decide the address, and only the
    // runtime knows both. See the page component, which fills the href in.
    const [page, at] = href.replace(/^\//, '').split('#');
    return `<a data-page="${page}"${at ? ` data-at="${at}"` : ''}>${label}</a>`;
  });
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  out = out.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
  return out.replace(/\u0000(\d+)\u0000/g, (_, i) => code[Number(i)]);
}

const slugify = (s) =>
  s
    .toLowerCase()
    .replace(/<[^>]+>/g, '')
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');

// Layout blocks. A landing page is not a document, it is a composition - and
// asking prose to carry a hero, a card grid and a gallery is what usually
// pushes a site back into hand-written HTML. These keep it in Markdown:
//
//   ::: cards
//   ### One door
//   Everything a request meets...
//   :::
//
// `cards` and `grid` cut their body at each `###` and wrap the pieces;
// `gallery` turns images into captioned figures; everything else is a div with
// the block's name, rendered normally inside.
const BLOCKS = new Set(['hero', 'cards', 'grid', 'gallery', 'steps', 'cta', 'lead', 'split', 'quote']);

function renderBlock(name, body, file) {
  if (name === 'cards' || name === 'grid') {
    const parts = body.split(/^### /m).filter((p) => p.trim());
    const cards = parts.map((part) => {
      const [head, ...rest] = part.split('\n');
      const inner = render(rest.join('\n'), file).html;
      return `<article class="mk-card"><h3>${inline(head.trim())}</h3>${inner}</article>`;
    });
    return `<div class="mk-${name}">${cards.join('')}</div>`;
  }
  if (name === 'gallery') {
    const figures = [];
    for (const line of body.split('\n')) {
      const m = line.match(/^!\[([^\]]*)\]\(([^)\s]+)\)\s*(.*)$/);
      if (!m) continue;
      const [, alt, src, caption] = m;
      figures.push(
        `<figure class="mk-shot"><img src="${src}" alt="${alt}" loading="lazy" />` +
          `<figcaption>${inline((caption || alt).trim())}</figcaption></figure>`,
      );
    }
    if (!figures.length) throw new Error(`${file}: a gallery block holds no image`);
    return `<div class="mk-gallery">${figures.join('')}</div>`;
  }
  return `<div class="mk-${name}">${render(body, file).html}</div>`;
}

function render(md, file) {
  const lines = md.split('\n');
  const html = [];
  const headings = [];
  let i = 0;

  const closeList = (stack) => {
    while (stack.length) html.push(`</${stack.pop()}>`);
  };
  const listStack = [];

  while (i < lines.length) {
    const line = lines[i];

    // A layout block. Nesting one inside another is not supported and says so.
    const block = line.match(/^:::\s*([a-z-]+)\s*$/);
    if (block) {
      const name = block[1];
      if (!BLOCKS.has(name)) {
        throw new Error(`${file}: unknown block "${name}" - known blocks are ${[...BLOCKS].join(', ')}`);
      }
      const body = [];
      i++;
      while (i < lines.length && !/^:::\s*$/.test(lines[i])) {
        if (/^:::\s*[a-z-]+\s*$/.test(lines[i])) throw new Error(`${file}: a block opened inside a block`);
        body.push(lines[i++]);
      }
      if (i >= lines.length) throw new Error(`${file}: the "${name}" block is never closed`);
      i++;
      closeList(listStack);
      html.push(renderBlock(name, body.join('\n'), file));
      continue;
    }

    // Fenced code. The language is kept as a class, and a fence that never
    // closes is a mistake in the source rather than a page that swallows the
    // rest of itself.
    if (line.startsWith('```')) {
      const lang = line.slice(3).trim();
      const body = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) body.push(lines[i++]);
      if (i >= lines.length) throw new Error(`${file}: a code fence is never closed`);
      i++;
      closeList(listStack);
      const cls = lang ? ` class="language-${lang}"` : '';
      html.push(`<pre><code${cls}>${escapeHtml(body.join('\n'))}</code></pre>`);
      continue;
    }

    // Callouts: > [!NOTE] / [!WARNING] / [!TIP], then the quoted body.
    const callout = line.match(/^>\s*\[!(NOTE|WARNING|TIP)\]\s*(.*)$/i);
    if (callout) {
      const kind = callout[1].toLowerCase();
      const body = [callout[2]];
      i++;
      while (i < lines.length && lines[i].startsWith('>')) body.push(lines[i++].replace(/^>\s?/, ''));
      closeList(listStack);
      html.push(`<div class="callout ${kind}">${inline(body.join(' ').trim())}</div>`);
      continue;
    }

    // Tables: a header row, a separator, then rows.
    if (line.includes('|') && lines[i + 1] && /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(lines[i + 1])) {
      const row = (l) =>
        l
          .trim()
          .replace(/^\||\|$/g, '')
          .split('|')
          .map((c) => c.trim());
      const head = row(line);
      i += 2;
      const body = [];
      while (i < lines.length && lines[i].includes('|')) body.push(row(lines[i++]));
      closeList(listStack);
      html.push('<div class="table-wrap"><table><thead><tr>');
      for (const c of head) html.push(`<th>${inline(c)}</th>`);
      html.push('</tr></thead><tbody>');
      for (const r of body) {
        html.push('<tr>');
        for (const c of r) html.push(`<td>${inline(c)}</td>`);
        html.push('</tr>');
      }
      html.push('</tbody></table></div>');
      continue;
    }

    const heading = line.match(/^(#{1,4})\s+(.*)$/);
    if (heading) {
      closeList(listStack);
      const level = heading[1].length;
      const text = heading[2].trim();
      const id = slugify(text);
      // Only h2 reaches the on-page table of contents: a list of every h3 is a
      // second copy of the page, and nobody navigates by it.
      if (level === 2) headings.push({ id, text });
      html.push(`<h${level} id="${id}">${inline(text)}</h${level}>`);
      i++;
      continue;
    }

    const bullet = line.match(/^(\s*)[-*]\s+(.*)$/);
    const numbered = line.match(/^(\s*)\d+\.\s+(.*)$/);
    if (bullet || numbered) {
      const tag = bullet ? 'ul' : 'ol';
      if (listStack[listStack.length - 1] !== tag) {
        closeList(listStack);
        listStack.push(tag);
        html.push(`<${tag}>`);
      }
      // An item may run over several lines: everything until a blank line, a
      // new item, or a block of another kind belongs to it. Without this, a
      // wrapped item closed the list and left its tail as a stray paragraph.
      const item = [(bullet || numbered)[2]];
      i++;
      while (
        i < lines.length &&
        lines[i].trim() &&
        !/^(#{1,4}\s|```|>|:::)/.test(lines[i]) &&
        !/^\s*[-*]\s/.test(lines[i]) &&
        !/^\s*\d+\.\s/.test(lines[i]) &&
        !(lines[i].includes('|') && lines[i + 1] && /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(lines[i + 1]))
      ) {
        item.push(lines[i].trim());
        i++;
      }
      html.push(`<li>${inline(item.join(' '))}</li>`);
      continue;
    }

    if (!line.trim()) {
      closeList(listStack);
      i++;
      continue;
    }

    // A paragraph runs until a blank line.
    const para = [line];
    i++;
    while (
      i < lines.length &&
      lines[i].trim() &&
      !/^(#{1,4}\s|```|>|:::|\s*[-*]\s|\s*\d+\.\s)/.test(lines[i])
    ) {
      para.push(lines[i++]);
    }
    closeList(listStack);
    html.push(`<p>${inline(para.join(' '))}</p>`);
  }
  closeList(listStack);
  return { html: html.join('\n'), headings };
}

// ---- front matter ----------------------------------------------------------

function frontMatter(raw, file) {
  if (!raw.startsWith('---')) throw new Error(`${file}: no front matter (title, section, order)`);
  const end = raw.indexOf('\n---', 3);
  if (end < 0) throw new Error(`${file}: the front matter is never closed`);
  const meta = {};
  for (const line of raw.slice(3, end).split('\n')) {
    const [key, ...rest] = line.split(':');
    if (!key.trim() || !rest.length) continue;
    meta[key.trim()] = rest.join(':').trim();
  }
  return { meta, body: raw.slice(end + 4) };
}

// ---- search ----------------------------------------------------------------
//
// An inverted index built here, so the browser downloads words rather than
// pages: term -> the pages that hold it, with where it was seen (a title
// counts for more than a paragraph). It is deliberately crude - lower-cased,
// split on non-letters, no stemming - because a documentation search is judged
// on "does typing strip-prefix find the page", not on linguistics.

const STOP = new Set(
  ('the a an and or of to in on for with is are be it its this that as at by from not no you your ' +
    'le la les un une des et ou de du au aux en dans sur pour avec est sont ce cet cette qui que ne pas vous votre')
    .split(' '),
);

function terms(text) {
  return text
    .toLowerCase()
    .split(/[^\p{L}\p{N}_-]+/u)
    .filter((w) => w.length > 1 && w.length < 40 && !STOP.has(w));
}

function addToIndex(index, pageId, text, weight) {
  for (const term of terms(text)) {
    const postings = (index[term] ||= {});
    postings[pageId] = (postings[pageId] || 0) + weight;
  }
}

// An index, packed: pages are NUMBERED rather than named, because a slug like
// "docs/filters/rewrite-response-header" appears in thousands of postings and
// spelling it out each time was half the file.
function packIndex(index, pages) {
  const number = new Map(pages.map((p, i) => [p.slug, i]));
  const packed = {};
  for (const [term, postings] of Object.entries(index)) {
    const row = {};
    for (const [slug, weight] of Object.entries(postings)) {
      if (number.has(slug)) row[number.get(slug)] = weight;
    }
    if (Object.keys(row).length) packed[term] = row;
  }
  return {
    pages: pages.map((p) => ({
      slug: p.slug,
      title: p.title,
      area: p.area,
      section: p.section,
      summary: p.summary,
    })),
    index: packed,
  };
}

// ---- walk ------------------------------------------------------------------

async function* markdownFiles(dir) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return; // a tree that does not exist yet (a language with no frozen version)
  }
  for (const entry of entries) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* markdownFiles(full);
    else if (entry.name.endsWith('.md')) yield full;
  }
}

const exists = (p) =>
  stat(p).then(
    () => true,
    () => false,
  );

// ---- the build -------------------------------------------------------------

const areas = JSON.parse(await readFile(join(CONTENT, 'areas.json'), 'utf8'));
const versionsFile = JSON.parse(await readFile(join(CONTENT, 'versions.json'), 'utf8'));
const frozen = versionsFile.versions || [];
const current = versionsFile.current || null;
if (current && !frozen.includes(current)) {
  throw new Error(`content/versions.json: current is ${current}, which is not in versions`);
}

// What the docs area serves, and under which path segment. The current version
// answers at /docs/... with NO segment of its own, so the address a reader
// bookmarks or a search engine indexes never moves when a version ships.
const docTrees = [];
if (current) {
  docTrees.push({ id: current, prefix: '', root: (lang) => join(CONTENT, 'versions', current, lang, 'docs') });
  for (const v of frozen) {
    if (v === current) continue;
    docTrees.push({ id: v, prefix: v, root: (lang) => join(CONTENT, 'versions', v, lang, 'docs') });
  }
  docTrees.push({ id: 'next', prefix: 'next', root: (lang) => join(CONTENT, lang, 'docs') });
} else {
  // Nothing frozen yet: the working tree IS the current version. Emitting a
  // `next` copy of it too would double the site to say the same thing twice.
  docTrees.push({ id: 'next', prefix: '', root: (lang) => join(CONTENT, lang, 'docs') });
}

const areaOf = (slug) => {
  for (const a of areas) {
    for (const p of a.prefixes) {
      if (slug === p || slug.startsWith(`${p}/`)) return a.id;
    }
  }
  return null;
};

// Read one markdown tree into pages. `slugBase` is what the URL says; `root` is
// where the files are - the two differ for a frozen version.
async function readTree(root, slugBase, sink) {
  for await (const file of markdownFiles(root)) {
    const shown = relative(CONTENT, file);
    const raw = await readFile(file, 'utf8');
    const { meta, body } = frontMatter(raw, shown);
    if (!meta.title) throw new Error(`${shown}: the front matter needs a title`);
    const tail = relative(root, file).replace(/\.md$/, '').split(/[\\/]/).join('/');
    const slug = slugBase ? `${slugBase}/${tail}` : tail;
    const area = areaOf(slug);
    if (!area) throw new Error(`${shown}: no area owns "${slug}" - declare its prefix in content/areas.json`);
    const { html, headings } = render(body, shown);
    sink.push({
      file: shown,
      raw,
      slug,
      area,
      title: meta.title,
      section: meta.section || '',
      summary: meta.summary || '',
      order: Number(meta.order || 999),
      hideNav: meta.hideNav === 'true',
      layout: meta.layout || '',
      widget: meta.widget || '',
      headings,
      html,
    });
  }
}

const built = {};
for (const lang of LANGS) {
  const outDir = join(OUT, lang);
  await rm(outDir, { recursive: true, force: true });
  await mkdir(join(outDir, 'p'), { recursive: true });

  // Everything that has no version: every area but the documentation.
  const unversioned = [];
  for (const entry of await readdir(join(CONTENT, lang), { withFileTypes: true })) {
    if (entry.name === 'docs') continue;
    if (entry.isDirectory()) await readTree(join(CONTENT, lang, entry.name), entry.name, unversioned);
  }
  for (const entry of await readdir(join(CONTENT, lang), { withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.md')) continue;
    const raw = await readFile(join(CONTENT, lang, entry.name), 'utf8');
    const shown = `${lang}/${entry.name}`;
    const { meta, body } = frontMatter(raw, shown);
    if (!meta.title) throw new Error(`${shown}: the front matter needs a title`);
    const slug = entry.name.replace(/\.md$/, '');
    const area = areaOf(slug);
    if (!area) throw new Error(`${shown}: no area owns "${slug}" - declare its prefix in content/areas.json`);
    const { html, headings } = render(body, shown);
    unversioned.push({
      file: shown,
      raw,
      slug,
      area,
      title: meta.title,
      section: meta.section || '',
      summary: meta.summary || '',
      order: Number(meta.order || 999),
      hideNav: meta.hideNav === 'true',
      layout: meta.layout || '',
      widget: meta.widget || '',
      headings,
      html,
    });
  }

  const perVersion = {};
  for (const tree of docTrees) {
    const pages = [];
    await readTree(tree.root(lang), tree.prefix ? `docs/${tree.prefix}` : 'docs', pages);
    perVersion[tree.id] = pages;
  }

  const all = [...unversioned, ...Object.values(perVersion).flat()];
  for (const page of all) {
    const { file, raw, ...ship } = page;
    await writeFile(join(outDir, 'p', `${ship.slug.split('/').join('__')}.json`), JSON.stringify(ship));
  }

  // The navigation: areas in the order they are declared, sections inside them
  // in the order the front matter asked for.
  const navFor = (pages) => {
    const sections = [];
    for (const page of [...pages].sort((a, b) => a.order - b.order || a.title.localeCompare(b.title))) {
      if (page.hideNav) continue;
      const name = page.section || '';
      let group = sections.find((s) => s.name === name);
      if (!group) sections.push((group = { name, pages: [] }));
      group.pages.push({ slug: page.slug, title: page.title, summary: page.summary });
    }
    return sections;
  };

  const navAreas = areas.map((a) => ({
    id: a.id,
    label: a.label[lang] || a.label.en,
    // The rail is 96px wide, so its entries carry a SHORT name; the drawer
    // that opens off them carries the full one.
    short: (a.short || a.label)[lang] || (a.short || a.label).en,
    icon: a.icon,
    home: a.home,
    versioned: !!a.versioned,
    sections: navFor(a.id === 'docs' ? perVersion[current || 'next'] : unversioned.filter((p) => p.area === a.id)),
  }));

  await writeFile(
    join(outDir, 'nav.json'),
    JSON.stringify({
      areas: navAreas,
      docs: {
        current: current || 'next',
        versions: docTrees.map((t) => ({ id: t.id, prefix: t.prefix })),
      },
    }),
  );

  // A version other than the current one carries its own navigation, fetched
  // only if someone switches to it.
  for (const tree of docTrees) {
    if (!tree.prefix) continue;
    await writeFile(join(outDir, `nav-docs-${tree.id}.json`), JSON.stringify(navFor(perVersion[tree.id])));
  }

  // Search: one corpus for what the site says TODAY - every unversioned page
  // plus the current documentation. An older version searches its own.
  const indexOf = (pages) => {
    const index = {};
    for (const p of pages) {
      addToIndex(index, p.slug, p.title, 8);
      addToIndex(index, p.slug, p.summary, 4);
      for (const h of p.headings) addToIndex(index, p.slug, h.text, 3);
      addToIndex(index, p.slug, p.raw.replace(/```[\s\S]*?```/g, ' '), 1);
    }
    return packIndex(index, pages);
  };
  await writeFile(
    join(outDir, 'search.json'),
    JSON.stringify(indexOf([...unversioned, ...perVersion[current || 'next']])),
  );
  for (const tree of docTrees) {
    if (!tree.prefix) continue;
    await writeFile(
      join(outDir, `search-docs-${tree.id}.json`),
      JSON.stringify(indexOf([...unversioned, ...perVersion[tree.id]])),
    );
  }

  built[lang] = all;
}

// ---- what the build refuses to let rot -------------------------------------

const problems = [];

// A page written in one language and not the other. With the content growing
// on both sides at once, this is the failure that goes unnoticed: the reader
// switches to French and the page silently is not there.
for (const lang of LANGS) {
  const mine = new Set(built[lang].map((p) => p.slug));
  for (const other of LANGS) {
    if (other === lang) continue;
    const theirs = new Set(built[other].map((p) => p.slug));
    for (const slug of mine) {
      if (!theirs.has(slug)) problems.push(`${slug}: written in ${lang}, missing in ${other}`);
    }
  }
}

// An internal link that points at no page. A link names a page, so this is
// checkable, and a link to a page nobody wrote in the end is the kind of rot a
// reader finds before anyone else does.
for (const lang of LANGS) {
  const known = new Set(built[lang].map((p) => p.slug));
  for (const page of built[lang]) {
    // A link written inside a versioned page names a page of ITS OWN tree:
    // "/docs/filters/respond" read from 1.3 is 1.3's own filter page.
    const versionPrefix = /^docs\/(next|\d[\w.]*)\//.test(page.slug)
      ? page.slug.slice(0, page.slug.indexOf('/', 5))
      : '';
    for (const [, href] of page.raw.matchAll(/\]\((\/[^)\s#]+)/g)) {
      const target = href.replace(/^\//, '').replace(/\/$/, '');
      const candidates = [target];
      if (versionPrefix && target.startsWith('docs/')) {
        candidates.push(target.replace(/^docs\//, `${versionPrefix}/`));
      }
      if (!candidates.some((c) => known.has(c))) problems.push(`${page.file} -> ${href}`);
    }
  }
}

// An image reference with no file behind it.
for (const lang of LANGS) {
  for (const page of built[lang]) {
    for (const [, src] of page.raw.matchAll(/!\[[^\]]*\]\(([^)\s]+)\)/g)) {
      if (/^https?:/.test(src)) continue;
      if (!(await exists(join(PUBLIC, src)))) problems.push(`${page.file} -> missing image ${src}`);
    }
  }
}

if (problems.length) {
  const say = STRICT ? console.error : console.warn;
  say(`[build-site] ${problems.length} problem(s):`);
  for (const line of [...new Set(problems)].sort().slice(0, 60)) say(`  ${line}`);
  if (STRICT) process.exit(1);
}

const counts = LANGS.map((l) => `${l}: ${built[l].length} pages`).join(', ');
const versions = docTrees.map((t) => t.id).join(', ');
console.log(`[build-site] ${counts} | docs: ${versions} | current: ${current || 'next'}`);
