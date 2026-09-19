// Retake the site's screenshots instead of remembering how they were taken.
//
// A documentation screenshot rots silently: the screen moves, the picture does
// not, and the first person to notice is a reader who no longer finds the
// button. screenshots.json says where every one of them comes from - which
// plane, which address, which viewport, and which pages use it - and this
// reads that file and takes them again.
//
// THE LOOP, end to end:
//
//   1. a binary that EMBEDS the console, or the admin port answers a JSON
//      status page instead of a screen:
//        make ui && CGO_ENABLED=0 go build -tags ee -o /tmp/meerkat-doc ./cmd/meerkat
//   2. run it on the spare ports - 8086 and 9096, never 8082 and 9092, which
//      is where this repository's own dev instance lives:
//        MEERKAT_TENANCY=multi MEERKAT_ADMIN_PASSWORD=test1234 \
//        /tmp/meerkat-doc -addr 127.0.0.1:8086 -admin-addr 127.0.0.1:9096 -data /tmp/docdata
//   3. put the demo data in it - see `dataset` in screenshots.json for what the
//      captures assume exists
//   4. node scripts/screenshots.mjs --check      does every address still render
//      node scripts/screenshots.mjs --shoot all  retake, convert, write
//
// What this does NOT do is create the data. A capture of an empty screen is
// worse than an old one, so --check fails loudly when a page comes up with
// nothing in it rather than quietly writing a blank picture.
import { readFile, mkdir, rm } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
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

const base = { admin: manifest.instance.admin, data: manifest.instance.data };

// An address with a note in brackets needs a hand - a drawer to open, a tab to
// pick. Those are listed, not taken: a script that pretends to reproduce a
// click it cannot see is how a wrong picture gets published.
const mechanical = (shot) => !shot.path.includes('(');
const address = (shot) => base[shot.plane] + shot.path.trim();

const shots = manifest.shots.filter((s) => (only && only !== 'all' ? s.file.includes(only) : true));
const tmp = join(root, '.screenshots');
await rm(tmp, { recursive: true, force: true });
await mkdir(tmp, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 }, deviceScaleFactor: 2 });
const page = await ctx.newPage();

// The console is behind the admin login, and so is every screen worth a picture.
await page.goto(base.admin + '/login', { waitUntil: 'networkidle' }).catch(() => {});
if (await page.locator('input[name=username]').count()) {
  const [user, password] = manifest.instance.signIn.split(' / ');
  await page.fill('input[name=username]', user);
  await page.fill('input[name=password]', password);
  await Promise.all([page.waitForLoadState('networkidle'), page.click('button[type=submit]')]);
}

let problems = 0;
for (const shot of shots) {
  if (!mechanical(shot)) {
    console.log(`  by hand   ${shot.file.padEnd(44)} ${shot.path}`);
    continue;
  }
  const url = address(shot);
  const res = await page.goto(url, { waitUntil: 'networkidle' }).catch(() => null);
  await page.waitForTimeout(1500);
  const text = (await page.locator('body').innerText().catch(() => '')).trim();
  const ok = res && res.status() < 400 && text.length > 120;
  if (!ok) {
    problems++;
    console.error(`  EMPTY     ${shot.file.padEnd(44)} ${url}  (${text.length} characters)`);
    continue;
  }
  if (mode === 'check') {
    console.log(`  ok        ${shot.file.padEnd(44)} ${url}`);
    continue;
  }
  await page.setViewportSize({ width: shot.viewport[0], height: shot.viewport[1] });
  await page.waitForTimeout(400);
  const png = join(tmp, shot.file.replace(/[/]/g, '_').replace('.webp', '.png'));
  await page.screenshot({ path: png });
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
console.log(
  problems
    ? `\n${problems} screenshot(s) could not be taken - the instance is probably missing the data screenshots.json describes.`
    : `\n${shots.length} screenshot(s) accounted for.`,
);
process.exit(problems ? 1 : 0);
