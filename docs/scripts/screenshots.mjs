// Retake the site's screenshots instead of remembering how they were taken.
//
// A documentation screenshot rots silently: the screen moves, the picture does
// not, and the first person to notice is a reader who no longer finds the
// button. screenshots.json says where every one of them comes from - which
// instance, which plane, which address, which viewport, what to click before
// the picture, and which pages use it - and this reads that file and takes
// them again.
//
// THE LOOP, end to end (screenshots.json `instance` has the exact commands):
//
//   1. a binary that EMBEDS the console, or the admin port answers a JSON
//      status page instead of a screen:
//        make ui && CGO_ENABLED=0 go build -tags ee -o /tmp/meerkat-doc ./cmd/meerkat
//   2. the upstream the demo routes point at, left running:
//        node scripts/screenshots-upstream.mjs
//   3. the instances on the spare ports - 8086/9096 (multi) and 8087/9097
//      (single, for the screens only one organisation shows), never 8082 and
//      9092, which is where this repository's own dev instance lives - each on
//      an EMPTY data directory, then seeded:
//        node scripts/screenshots-seed.mjs
//        node scripts/screenshots-seed.mjs --instance single
//   4. node scripts/screenshots.mjs --check      does every address still render
//      node scripts/screenshots.mjs --shoot all  retake, convert, write
//      node scripts/screenshots.mjs --shoot tls  only the files whose name has "tls"
//
// A shot may carry:
//   instance     a key of `instances`, for a shot another instance must take
//   colorScheme  "dark" or "light" (default: dark on the console, light on
//                the applications)
//   signedIn     data plane only: sign in before (default true)
//   steps        what to do once the page is up, in order - each one of
//                {"click": selector}, {"hover": selector}, {"scrollTo": selector},
//                {"waitFor": selector}, {"wait": ms}; selectors are Playwright's
//   clip         [x, y, width, height] of the viewport to keep
//   settle       ms to wait before the picture (default 1500)
//   status       the HTTP status the page answers when it is not a 2xx - a
//                route under maintenance answers 503, and that IS the picture
//   traffic      true: the screen shows live traffic. A traffic run is
//                started in the background when the shooting starts (it
//                reuses the sessions the seed kept, so it signs nobody in),
//                and these shots are taken last, once it has run for a while.
//
// A capture of an empty screen is worse than an old one, so --check fails
// loudly when a page comes up with nothing in it, or when a step finds nothing
// to click, rather than quietly writing a blank picture.
import { readFile, mkdir, rm } from 'node:fs/promises';
import { spawn, spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const manifest = JSON.parse(await readFile(join(root, 'screenshots.json'), 'utf8'));
const args = process.argv.slice(2);
const mode = args.includes('--shoot') ? 'shoot' : 'check';
// Only meaningful with --shoot: without the guard, indexOf returns -1 and
// the first flag becomes the filter, which matches nothing.
const only = mode === 'shoot' ? args[args.indexOf('--shoot') + 1] : '';

// Playwright is the integration suite's dependency, not a second copy here.
let chromium;
try {
  const require = createRequire(join(root, '..', 'e2e', 'package.json'));
  ({ chromium } = require('playwright'));
} catch {
  console.error('playwright comes from e2e/: run `npm install` there first');
  process.exit(1);
}

const instanceOf = (shot) =>
  shot.instance ? { ...manifest.instance, ...manifest.instances[shot.instance] } : manifest.instance;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const selected = manifest.shots.filter((s) => (only && only !== 'all' ? s.file.includes(only) : true));
// The console first, so the audit trail and the sessions it shows do not
// start with this run's own sign-ins on the applications; the live screens
// last, since the traffic run needs time to fill the last minute.
const shots = [
  ...selected.filter((s) => !s.traffic && s.plane === 'admin'),
  ...selected.filter((s) => !s.traffic && s.plane !== 'admin'),
  ...selected.filter((s) => s.traffic),
];
const tmp = join(root, '.screenshots');
await rm(tmp, { recursive: true, force: true });
await mkdir(tmp, { recursive: true });

// Traffic for the Metrics screen, which shows the last minute and nothing
// older: sent while the pictures are taken, by the seed's own traffic mode.
let traffic = null;
let trafficSince = 0;
if (mode === 'shoot' && shots.some((s) => s.traffic)) {
  traffic = spawn(process.execPath, [join(here, 'screenshots-seed.mjs'), '--traffic-only', '900'], {
    stdio: ['ignore', 'ignore', 'inherit'],
  });
  trafficSince = Date.now();
}

const browser = await chromium.launch();

// One signed-in browser per instance, plane and colour scheme, made when a
// shot first needs it.
const contexts = new Map();
async function pageFor(shot) {
  const inst = instanceOf(shot);
  const scheme = shot.colorScheme || (shot.plane === 'admin' ? 'dark' : 'light');
  const signedIn = shot.plane === 'admin' || shot.signedIn !== false;
  const key = [shot.instance || '', shot.plane, scheme, signedIn].join('|');
  if (contexts.has(key)) return contexts.get(key);
  const ctx = await browser.newContext({
    viewport: { width: 1600, height: 1000 },
    deviceScaleFactor: 2,
    colorScheme: scheme,
    // Said out loud: headless Chromium may send no Accept-Language at all,
    // and the pages then speak whichever language the gateway falls back to.
    locale: 'en-US',
  });
  const page = await ctx.newPage();
  if (signedIn) {
    const base = inst[shot.plane];
    await page.goto(base + '/login', { waitUntil: 'networkidle' }).catch(() => {});
    if (await page.locator('input[name=username]').count()) {
      const [user, password] = inst.signIn.split(' / ');
      await page.fill('input[name=username]', user);
      await page.fill('input[name=password]', password);
      await Promise.all([page.waitForLoadState('networkidle'), page.click('button[type=submit]')]);
    }
  }
  contexts.set(key, page);
  return page;
}

async function runSteps(page, steps = []) {
  for (const step of steps) {
    if (step.wait) await sleep(step.wait);
    if (step.waitFor) await page.locator(step.waitFor).first().waitFor({ timeout: 10000 });
    if (step.click) await page.locator(step.click).first().click({ timeout: 10000 });
    if (step.hover) await page.locator(step.hover).first().hover({ timeout: 10000 });
    if (step.scrollTo) await page.locator(step.scrollTo).first().scrollIntoViewIfNeeded({ timeout: 10000 });
  }
}

let problems = 0;
for (const shot of shots) {
  const inst = instanceOf(shot);
  const url = inst[shot.plane] + shot.path.trim();
  if (shot.traffic && mode === 'shoot') {
    const left = 100000 - (Date.now() - trafficSince);
    if (left > 0) {
      console.log(`  waiting ${Math.round(left / 1000)}s for the traffic to fill the last minute`);
      await sleep(left);
    }
  }
  const page = await pageFor(shot);
  await page.setViewportSize({ width: shot.viewport[0], height: shot.viewport[1] });
  const res = await page.goto(url, { waitUntil: 'networkidle' }).catch(() => null);
  await sleep(800);
  let stepError = '';
  try {
    await runSteps(page, shot.steps);
  } catch (e) {
    stepError = e.message.split('\n')[0];
  }
  await sleep(shot.settle ?? 1500);
  const text = (await page.locator('body').innerText().catch(() => '')).trim();
  const status = res ? res.status() : 0;
  const ok = (shot.status ? status === shot.status : status > 0 && status < 400) && text.length > 120 && !stepError;
  if (!ok) {
    problems++;
    console.error(`  EMPTY     ${shot.file.padEnd(44)} ${url}  (${stepError || text.length + ' characters'})`);
    continue;
  }
  if (mode === 'check') {
    console.log(`  ok        ${shot.file.padEnd(44)} ${url}${shot.steps ? `  (+${shot.steps.length} steps)` : ''}`);
    continue;
  }
  // Nothing left FOCUSED or blinking: a stray focus ring in a published image
  // reads as a state the reader is meant to notice, and it is not one.
  await page.evaluate(() => document.activeElement?.blur?.()).catch(() => {});
  await page.mouse.move(shot.viewport[0] - 2, 2);
  await sleep(250);
  const png = join(tmp, shot.file.replace(/[/]/g, '_').replace('.webp', '.png'));
  const clip = shot.clip ? { x: shot.clip[0], y: shot.clip[1], width: shot.clip[2], height: shot.clip[3] } : undefined;
  await page.screenshot({ path: png, clip, caret: 'hide' });
  const out = join(root, 'public', shot.file);
  const cwebp = spawnSync('cwebp', ['-quiet', '-q', '82', '-resize', '1600', '0', png, '-o', out]);
  if (cwebp.status !== 0) {
    problems++;
    console.error(`  CONVERT   ${shot.file}  (is cwebp installed?)`);
    continue;
  }
  console.log(`  written   ${shot.file}`);
}

await browser.close();
if (traffic) traffic.kill();
console.log(
  problems
    ? `\n${problems} screenshot(s) could not be taken - the instance is probably missing the data screenshots.json describes.`
    : `\n${shots.length} screenshot(s) accounted for.`,
);
process.exit(problems ? 1 : 0);
