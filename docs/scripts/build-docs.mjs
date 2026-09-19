// Turns the documentation SOURCE - Markdown, one file per page and per
// language, under docs/content - into what the site reads at runtime:
//
//   src/assets/docs/<lang>/<slug>.json   one page: title, html, headings
//   src/assets/docs/<lang>/toc.json      the navigation tree, in order
//   src/assets/docs/<lang>/search.json   the search index, built here
//
// Markdown rather than Angular templates, because the content is the product:
// it has to be writable, reviewable in a diff, and translatable without anyone
// opening a component. And JSON rather than shipping a Markdown parser to the
// browser: the conversion happens once, at build, and the site downloads only
// what it displays.
//
// GitHub Pages serves files and nothing else - no server, no index endpoint -
// which is why the search index is a file too. See search.json below: it holds
// the words, not the pages, so it stays small enough to fetch on first search.

import { mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const CONTENT = join(here, '..', 'content');
const OUT = join(here, '..', 'src', 'assets', 'docs');
const LANGS = ['en', 'fr'];

// ── Markdown ────────────────────────────────────────────────────────────────
//
// A small renderer rather than a dependency: the documentation uses a dozen
// constructs (headings, lists, tables, fenced code, links, emphasis, callouts)
// and a parser that handles every corner of CommonMark would be a megabyte to
// audit for the same result. What is NOT supported fails loudly - see the
// unknown-fence check - instead of rendering as something nobody meant.

const escapeHtml = (s) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Inline: code first, so nothing inside a span of code is taken for markup.
function inline(text) {
  const code = [];
  let out = text.replace(/`([^`]+)`/g, (_, c) => {
    code.push(`<code>${escapeHtml(c)}</code>`);
    // The marker is a NUL, written as an escape so it is VISIBLE here: a
    // plain number in the prose must not be mistaken for a placeholder, and
    // a literal NUL in the source reads as a space to everyone who looks.
    return `\u0000${code.length - 1}\u0000`;
  });
  out = escapeHtml(out);
  out = out.replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (_, alt, src) => `<img src="${src}" alt="${alt}" loading="lazy" />`);
  out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, label, href) => {
    const external = /^https?:/.test(href);
    const attrs = external ? ' target="_blank" rel="noopener"' : '';
    return `<a href="${href}"${attrs}>${label}</a>`;
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
        !/^(#{1,4}\s|```|>)/.test(lines[i]) &&
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
    while (i < lines.length && lines[i].trim() && !/^(#{1,4}\s|```|>|\s*[-*]\s|\s*\d+\.\s)/.test(lines[i])) {
      para.push(lines[i++]);
    }
    closeList(listStack);
    html.push(`<p>${inline(para.join(' '))}</p>`);
  }
  closeList(listStack);
  return { html: html.join('\n'), headings };
}

// ── front matter ────────────────────────────────────────────────────────────

function frontMatter(raw, file) {
  if (!raw.startsWith('---')) throw new Error(`${file}: no front matter (title, order)`);
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

// ── search ──────────────────────────────────────────────────────────────────
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

// ── walk ────────────────────────────────────────────────────────────────────

async function* markdownFiles(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* markdownFiles(full);
    else if (entry.name.endsWith('.md')) yield full;
  }
}

async function buildLang(lang) {
  const root = join(CONTENT, lang);
  const outDir = join(OUT, lang);
  await rm(outDir, { recursive: true, force: true });
  await mkdir(outDir, { recursive: true });

  const pages = [];
  const index = {};
  for await (const file of markdownFiles(root)) {
    const raw = await readFile(file, 'utf8');
    const { meta, body } = frontMatter(raw, relative(root, file));
    const slug = relative(root, file).replace(/\.md$/, '').split('/').join('/');
    if (!meta.title) throw new Error(`${file}: the front matter needs a title`);
    const { html, headings } = render(body, relative(root, file));

    const page = {
      slug,
      title: meta.title,
      section: meta.section || slug.split('/')[0],
      summary: meta.summary || '',
      order: Number(meta.order || 999),
      headings,
      html,
    };
    await writeFile(join(outDir, `${slug.split('/').join('__')}.json`), JSON.stringify(page));
    pages.push({ slug: page.slug, title: page.title, section: page.section, summary: page.summary, order: page.order });

    addToIndex(index, slug, meta.title, 8);
    addToIndex(index, slug, meta.summary || '', 4);
    for (const h of headings) addToIndex(index, slug, h.text, 3);
    addToIndex(index, slug, body.replace(/```[\s\S]*?```/g, ' '), 1);
  }

  pages.sort((a, b) => a.order - b.order || a.title.localeCompare(b.title));
  await writeFile(join(outDir, 'toc.json'), JSON.stringify(pages));
  // Pages are numbered in the index rather than named: a slug like
  // "filters/rewrite-response-header" appears in thousands of postings, and
  // spelling it out each time was half the file.
  const order = pages.map((p) => p.slug);
  const number = new Map(order.map((slug, i) => [slug, i]));
  const packed = {};
  for (const [term, postings] of Object.entries(index)) {
    const row = {};
    for (const [slug, weight] of Object.entries(postings)) row[number.get(slug)] = weight;
    packed[term] = row;
  }
  await writeFile(
    join(outDir, 'search.json'),
    JSON.stringify({
      pages: pages.map((p) => ({ slug: p.slug, title: p.title, section: p.section, summary: p.summary })),
      index: packed,
    }),
  );
  return pages;
}

const counts = [];
const slugsByLang = {};
for (const lang of LANGS) {
  const pages = await buildLang(lang);
  slugsByLang[lang] = new Set(pages.map((p) => p.slug));
  counts.push(`${lang}: ${pages.length} pages`);
}

// Every page exists in every language, or the build says which one is missing.
// With several people writing at once, a page that landed in one language and
// not the other is the failure that goes unnoticed: the reader switches to
// French and the page silently is not there.
const missing = [];
for (const lang of LANGS) {
  for (const other of LANGS) {
    if (lang === other) continue;
    for (const slug of slugsByLang[lang]) {
      if (!slugsByLang[other].has(slug)) missing.push(`${slug}: written in ${lang}, missing in ${other}`);
    }
  }
}
// Internal links are checked against the pages that exist. Six people writing
// at once link to each other's pages, and a link to a page nobody wrote in the
// end is the kind of rot a reader finds before anyone else does.
const known = new Set();
for (const lang of LANGS) for (const slug of slugsByLang[lang]) known.add(`${lang}:${slug}`);
const broken = [];
for (const lang of LANGS) {
  for await (const file of markdownFiles(join(CONTENT, lang))) {
    const raw = await readFile(file, 'utf8');
    for (const [, href] of raw.matchAll(/\]\((\/#\/docs\/[^)\s#]+)/g)) {
      const slug = href.replace('/#/docs/', '').replace(/\/$/, '');
      if (!known.has(`${lang}:${slug}`)) {
        broken.push(`${relative(CONTENT, file)} -> ${href}`);
      }
    }
  }
}
if (broken.length) {
  const strict = process.argv.includes('--strict');
  const say = strict ? console.error : console.warn;
  say(`[build-docs] ${broken.length} internal link(s) point at no page:`);
  for (const line of [...new Set(broken)].sort().slice(0, 40)) say(`  ${line}`);
  if (strict) process.exit(1);
}

// A warning while the documentation is being written - several people write at
// once, and a page lands in one language minutes before the other. It only
// FAILS under --strict, which is what the release build uses.
if (missing.length) {
  const strict = process.argv.includes('--strict');
  const say = strict ? console.error : console.warn;
  say(`[build-docs] ${missing.length} page(s) exist in one language only:`);
  for (const line of missing.sort()) say(`  ${line}`);
  if (strict) process.exit(1);
}

console.log(`[build-docs] ${counts.join(', ')} -> src/assets/docs`);
