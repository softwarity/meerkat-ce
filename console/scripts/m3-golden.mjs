// Writes internal/m3color/testdata/golden.json: schemes and palettes made by
// the console's own generator (src/app/theme/m3.ts, on
// @material/material-color-utilities), which the gateway's Go port must
// reproduce exactly. Run it again after touching either side:
//
//   node scripts/m3-golden.mjs
//
// The cases are drawn from a fixed seed, so a rerun with nothing changed
// rewrites the same file.
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { generatePalettes, generateScheme, SCHEME_NAMES } from '../src/app/theme/m3.ts';

let state = 20261009;
const rand = () => {
  state = (state * 1103515245 + 12345) % 2147483648;
  return state / 2147483648;
};
const color = () =>
  '#' + Math.floor(rand() * 0x1000000).toString(16).padStart(6, '0').toUpperCase();

// The corners a random draw rarely reaches: greys, black, white, the disliked
// yellow-greens, and the presets' blues.
const fixed = ['#000000', '#FFFFFF', '#808080', '#6B8E23', '#32D8F4', '#6750A4', '#B33B15', '#FFFF00'];
const keys = ['secondary', 'tertiary', 'error', 'neutral', 'neutralVariant'];
const cases = [];
for (let i = 0; i < 32; i++) {
  const core = { primary: i < fixed.length ? fixed[i] : color() };
  for (const k of keys) if (rand() < 0.35) core[k] = color();
  const colorMatch = rand() < 0.5;
  const schemes = {};
  for (const [name, dark, level] of SCHEME_NAMES) schemes[name] = generateScheme(core, dark, level, colorMatch);
  cases.push({ core, colorMatch, schemes, palettes: generatePalettes(core) });
}
const out = fileURLToPath(new URL('../../internal/m3color/testdata/golden.json', import.meta.url));
writeFileSync(out, JSON.stringify(cases) + '\n');
console.log(`wrote ${cases.length} cases to ${out}`);
