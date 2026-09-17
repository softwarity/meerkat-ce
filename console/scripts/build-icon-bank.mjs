#!/usr/bin/env node
// Builds internal/icons/bank.json from the SVGs shipped by @material-symbols/svg-400
// (Material Symbols Outlined, weight 400; Apache License 2.0), read straight from
// node_modules - NOTHING is fetched over the network.
//
// Run: npm run icons:build   (from console/)
//
// The bank is embedded by the Go package internal/icons and used both to answer
// the console's icon-picker search (GET /api/portal/icons) and to resolve a bare
// icon NAME to its SVG when a portal is saved. The served bar renders the SVG as
// a CSS mask - no icon font on the data plane. Only the viewBox and the path
// geometry survive (no size, fill, script or handler), so the markup is safe to
// embed and colours follow currentColor.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(here, '../node_modules/@material-symbols/svg-400/outlined');
const OUT = path.join(here, '../../internal/icons/bank.json');

function normalize(svg) {
  const viewBox = svg.match(/viewBox="([^"]+)"/)?.[1];
  const paths = [...svg.matchAll(/<path\b[^>]*\sd="([^"]+)"/g)].map((m) => m[1]);
  if (!viewBox || paths.length === 0) return null;
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}">` +
    paths.map((d) => `<path d="${d}"/>`).join('') +
    `</svg>`
  );
}

if (!fs.existsSync(SRC)) {
  console.error(`[icons] source missing: ${SRC}\nRun: npm install -D @material-symbols/svg-400`);
  process.exit(1);
}

const files = fs
  .readdirSync(SRC)
  .filter((f) => f.endsWith('.svg') && !f.endsWith('-fill.svg')) // the outlined (non-filled) face
  .sort();

const icons = [];
let skipped = 0;
for (const file of files) {
  const name = file.slice(0, -4); // drop .svg
  const svg = normalize(fs.readFileSync(path.join(SRC, file), 'utf8'));
  if (!svg) {
    skipped++;
    continue;
  }
  icons.push({ name, svg });
}

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, JSON.stringify(icons) + '\n');
console.log(`[icons] ${icons.length} icon(s) written to ${path.relative(path.join(here, '../..'), OUT)} (${skipped} skipped)`);
