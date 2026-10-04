// The figures the prose quotes are not typed, they are READ: a page writes
// {{memory.idle}} and the build puts in what the CI measured last, a page
// writes {{features.built}} and the build counts FEATURES.md. A figure typed
// by hand is right on the day it is typed and nobody comes back for it - the
// home page said 37 MB at peak while the benchmark page beside it said 43.
import { readFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const ROOT = join(here, '..', '..');
// The same file the benchmark page renders (widgets/benchmark.component.ts).
const BENCH_URL = 'https://raw.githubusercontent.com/softwarity/meerkat-ce/bench/latest.json';

async function memory() {
  try {
    const res = await fetch(BENCH_URL, { signal: AbortSignal.timeout(8000) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const run = (await res.json()).runs.find((r) => r.machine.arch === 'amd64');
    const mk = run.gateways.find((g) => g.name === 'meerkat');
    const day = new Date(run.date);
    const on = (locale) => day.toLocaleDateString(locale, { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' });
    return { idle: Math.round(mk.memoryIdleMiB), peak: Math.round(mk.memoryPeakMiB), 'date.en': on('en-GB'), 'date.fr': on('fr-FR') };
  } catch (err) {
    // Offline, or the branch is not there yet: the last figures somebody
    // checked, and a line saying so. A site build must not need the network.
    const fallback = JSON.parse(await readFile(join(here, 'facts-fallback.json'), 'utf8'));
    console.warn(`[facts] the latest benchmark could not be read (${err.message}): using facts-fallback.json`);
    return fallback.memory;
  }
}

async function features() {
  const count = { x: 0, '~': 0, ' ': 0 };
  for (const line of (await readFile(join(ROOT, 'FEATURES.md'), 'utf8')).split('\n')) {
    const m = line.match(/^\| \[(.)\] \| [A-Z0-9]+-\d+ \|/);
    if (m && m[1] in count) count[m[1]]++;
  }
  return { built: count.x, partial: count['~'], todo: count[' '] };
}

export async function loadFacts() {
  const facts = { memory: await memory(), features: await features() };
  const flat = {};
  for (const [group, values] of Object.entries(facts)) {
    for (const [k, v] of Object.entries(values)) flat[`${group}.${k}`] = String(v);
  }
  console.log(`[facts] ${Object.entries(flat).map(([k, v]) => `${k}=${v}`).join(' ')}`);
  // An unknown name is a typo, and a typo must not ship as literal braces.
  return (raw, file) =>
    // A letter first: {{.Username}} is a Go template in a page about templates.
    raw.replace(/\{\{([a-z]+(?:\.[a-z]+)+)\}\}/g, (_, name) => {
      if (!(name in flat)) throw new Error(`${file}: unknown figure {{${name}}} - known: ${Object.keys(flat).join(', ')}`);
      return flat[name];
    });
}
