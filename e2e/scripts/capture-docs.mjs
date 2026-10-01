// Retakes the console screenshots the documentation site publishes.
//
// The doc images are the product's face, and a stale one is a lie somebody
// acts on: a section list that still names a screen we renamed sends a reader
// hunting for it. They were taken by hand until now, which is why they drifted.
// This does it the way the e2e suite does everything - a binary built from the
// working tree, a disposable database, a browser driven at a fixed size - so a
// retake is one command and produces the same frame every time.
//
//   node e2e/scripts/capture-docs.mjs
//
// Ports are offset again from the e2e ones, so a running `make dev` AND a
// running suite both stay out of the way.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, rmSync, readdirSync, unlinkSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { chromium, request } from '@playwright/test';

const repo = fileURLToPath(new URL('../..', import.meta.url));
const tmp = fileURLToPath(new URL('../.capture', import.meta.url));
const out = `${repo}/docs/public/img/console`;
const bin = `${tmp}/meerkat`;
const DATA = 'http://localhost:18084';
const ADMIN = 'http://localhost:19094';
const PASSWORD = 'Capture-Root-Password-1';

// The frame every published image already has. Changing it would reflow every
// caption that describes what is visible, so it is a constant, not an option.
const VIEW = { width: 1600, height: 1000 };

const log = (...a) => console.log('[capture]', ...a);

// ─── the gateway ────────────────────────────────────────────────────────────

function startGateway() {
  const tags = existsSync(`${repo}/ee`) ? ['-tags', 'ee'] : [];
  log('building', tags.length ? '(enterprise)' : '(community)');
  const build = spawnSync('go', ['build', ...tags, '-o', bin, './cmd/meerkat'], {
    cwd: repo,
    stdio: 'inherit',
  });
  if (build.status !== 0) process.exit(build.status ?? 1);

  rmSync(`${tmp}/data`, { recursive: true, force: true });
  mkdirSync(`${tmp}/data`, { recursive: true });
  const child = spawn(
    bin,
    [
      '-addr', ':18084',
      '-admin-addr', ':19094',
      '-data', `${tmp}/data`,
      '-tenancy', 'multi',
    ],
    {
      // The gateway's own log would drown the progress of this script; its
      // failures still surface, because nothing below works without it.
      stdio: ['ignore', 'ignore', 'inherit'],
      env: { ...process.env, MEERKAT_ADMIN_PASSWORD: PASSWORD },
    },
  );
  return child;
}

async function waitFor(url, tries = 60) {
  for (let i = 0; i < tries; i++) {
    try {
      const r = await fetch(url);
      if (r.ok || r.status === 401) return;
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`${url} never answered`);
}

// ─── the fixture ────────────────────────────────────────────────────────────
//
// What the published captions describe, so a retake matches the words beside
// it: five routes in reading order, a failing upstream, a maintenance route,
// and a catch-all last.

const ROUTES = [
  {
    id: 'billing', name: 'Billing', order: 1, enabled: true, isUi: true,
    upstream: 'http://billing.svc.acme.internal:8080',
    access: { level: 'auth' },
    predicates: [{ type: 'path', args: { patterns: ['/billing/**'] } }],
    filters: [
      { type: 'strip-prefix', args: { parts: 1, announcePrefix: true } },
      { type: 'set-request-header', args: { name: 'X-Acme-Channel', value: 'gateway' } },
      { type: 'set-response-header', args: { name: 'X-Frame-Options', value: 'SAMEORIGIN' } },
      { type: 'set-response-header', args: { name: 'Cache-Control', value: 'no-store' } },
    ],
    limits: [{ per: 'user', requests: 120, window: 'PT1M' }],
    ui: {
      userButton: { enabled: true },
      scheme: {
        select: true, mechanism: 'class', tag: 'html',
        light: 'acme-light', dark: 'acme-dark',
        storageOverride: true, storage: 'acme:theme',
        storageAuto: 'system', storageDark: 'dark', storageLight: 'light',
      },
    },
  },
  {
    id: 'docs-portal', name: 'Docs portal', order: 2, enabled: true, isUi: true,
    upstream: 'http://docs.svc.acme.internal:80',
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/docs/**'] } }],
    filters: [{ type: 'strip-prefix', args: { parts: 1, announcePrefix: true } }],
    ui: { userButton: { enabled: true } },
  },
  {
    id: 'orders-api', name: 'Orders API', order: 3, enabled: true,
    upstream: 'http://orders.svc.acme.internal:9000',
    access: { level: 'auth' },
    timeouts: { connect: 'PT1S', response: 'PT20S' },
    breaker: { enabled: true, trip: 5, cool: 'PT1M' },
    predicates: [
      { type: 'path', args: { patterns: ['/api/orders/**', '/api/shipments/**'] } },
      { type: 'method', args: { methods: ['GET', 'POST', 'PUT', 'DELETE'] } },
      { type: 'header', args: { name: 'X-Acme-Api-Version', values: ['2024-11', '2025-06'] } },
    ],
    filters: [
      { type: 'strip-prefix', args: { parts: 1, announcePrefix: true } },
      { type: 'set-request-header', args: { name: 'X-Acme-Channel', value: 'gateway' } },
      { type: 'set-request-header', args: { name: 'X-Acme-Tier', value: 'internal' } },
      { type: 'set-response-header', args: { name: 'Cache-Control', value: 'no-store' } },
    ],
    limits: [
      { per: 'user', requests: 600, window: 'PT1M' },
      { per: 'ip', requests: 60, window: 'PT1S' },
    ],
  },
  {
    id: 'inventory', name: 'Inventory (maintenance)', order: 4, enabled: true, isUi: true,
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/inventory/**'] } }],
    filters: [{ type: 'maintenance', args: { reason: 'Stock count in progress' } }],
    ui: { userButton: { enabled: true } },
  },
  {
    id: 'catch-all', name: 'Catch-all', order: 5, enabled: true,
    upstream: 'http://www.svc.acme.internal:80',
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/**'] } }],
    filters: [],
  },
];

// The catalogue the Portal screen shows: a bar with three applications, one of
// them carrying children (PORTAL-03).
const PORTAL = {
  mode: 'portal',
  layout: 'header',
  side: 'left',
  display: 'both',
  showAppName: true,
  entries: [
    {
      routeId: 'billing', label: 'Billing', icon: 'receipt_long',
      children: [
        { routeId: 'docs-portal', label: 'Overview', icon: 'home' },
        { routeId: 'inventory', label: 'Price list', icon: 'menu_book' },
      ],
    },
    { routeId: 'inventory', label: 'Inventory', icon: 'inventory_2' },
    { routeId: 'docs-portal', label: 'Docs', icon: 'help' },
  ],
};

// The suite's own way in: a FORM post to /login answering 303, with the
// cookie jar kept by the request context - which the browser then reuses, so
// the screenshots are taken as the same signed-in root.
async function seed() {
  const api = await request.newContext({ baseURL: ADMIN });
  const res = await api.post('/login', {
    form: { username: 'admin', password: PASSWORD },
    maxRedirects: 0,
  });
  if (res.status() !== 303) throw new Error(`login answered ${res.status()}`);
  log('signed in');

  const ok = async (r, what) => {
    if (r.status() >= 300) throw new Error(`${what} -> ${r.status()} ${await r.text()}`);
    return r;
  };
  // The routes a first start seeds for a demo are not the ones a screenshot
  // should teach: they point at httpbin and they are named after us.
  for (const id of ['demo', 'demo-secure', 'trap']) await api.delete(`/api/routes/${id}`);
  for (const rt of ROUTES) {
    await ok(await api.put(`/api/routes/${rt.id}`, { data: rt }), `route ${rt.id}`);
  }
  log(`${ROUTES.length} routes`);

  const settings = await (await ok(await api.get('/api/settings'), 'settings')).json();
  await ok(await api.put('/api/settings', { data: { ...settings, portal: PORTAL } }), 'portal');
  log('catalogue, in portal mode');

  const br = await api.get('/api/branding');
  if (br.status() < 300) {
    const branding = await br.json();
    await ok(await api.put('/api/branding', { data: { ...branding, appName: 'Acme Corp' } }), 'branding');
    log('branded');
  }

  const state = `${tmp}/state.json`;
  await api.storageState({ path: state });
  await api.dispose();
  return state;
}

// ─── the shots ──────────────────────────────────────────────────────────────

const SHOTS = [
  { file: 'routes-list', path: '/infra/routes' },
  { file: 'route-editor-target', path: '/infra/routes/orders-api/target' },
  { file: 'route-editor-security', path: '/infra/routes/orders-api/security' },
  { file: 'route-editor-predicates', path: '/infra/routes/orders-api/predicates' },
  { file: 'route-editor-filters', path: '/infra/routes/billing/modin' },
  { file: 'route-editor-color-scheme', path: '/infra/routes/billing/scheme' },
  // The portal preview is a real iframe doing a handshake before it draws
  // anything: caught too early it publishes an empty bar.
  { file: 'portal', path: '/application/portal', settle: 3500 },
];

async function shoot(storageState) {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    viewport: VIEW, colorScheme: 'dark', deviceScaleFactor: 1, storageState,
  });
  const page = await ctx.newPage();
  log('console open');

  mkdirSync(`${tmp}/png`, { recursive: true });
  for (const s of SHOTS) {
    await page.goto(ADMIN + '#' + s.path).catch(() => {});
    await page.goto(ADMIN + s.path);
    // The console animates drawers in; a frame taken mid-slide is a blurred
    // panel nobody can read.
    await page.waitForTimeout(s.settle ?? 1200);
    // And nothing is left FOCUSED: a stray focus ring in a published image
    // reads as a state the reader is meant to notice, and it is not one.
    await page.evaluate(() => document.activeElement?.blur?.());
    await page.waitForTimeout(150);
    const png = `${tmp}/png/${s.file}.png`;
    await page.screenshot({ path: png });
    const conv = spawnSync('cwebp', ['-quiet', '-q', '82', png, '-o', `${out}/${s.file}.webp`]);
    if (conv.status !== 0) throw new Error(`cwebp failed for ${s.file}`);
    log('captured', s.file);
  }
  await browser.close();
}

// ─── run ────────────────────────────────────────────────────────────────────

// A run that dies half way used to leave its gateway holding the ports, and
// the next run's wait-for-health was answered by that ZOMBIE - so a rebuilt
// binary never ran and the capture showed a console from an hour ago. Cost an
// evening once; never again.
function killStale(port) {
  const out = spawnSync('lsof', ['-nP', '-tiTCP:' + port, '-sTCP:LISTEN'], { encoding: 'utf8' });
  for (const pid of (out.stdout || '').split('\n').filter(Boolean)) {
    // Only ours: the dev gateway air runs must never be touched.
    const cmd = spawnSync('ps', ['-o', 'command=', '-p', pid], { encoding: 'utf8' }).stdout || '';
    if (!cmd.includes('/e2e/.capture/meerkat')) continue;
    log('a previous run still held :' + port + ', stopping pid ' + pid);
    try {
      process.kill(Number(pid));
    } catch {
      /* already gone */
    }
  }
}

killStale(18084);
killStale(19094);
const gw = startGateway();
try {
  await waitFor(ADMIN + '/api/health');
  const state = await seed();
  await shoot(state);
  log('done ->', out);
} finally {
  gw.kill();
}
