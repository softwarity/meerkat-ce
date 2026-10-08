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
import { hostname } from 'node:os';
import { chromium, request } from '@playwright/test';

const repo = fileURLToPath(new URL('../..', import.meta.url));
const tmp = fileURLToPath(new URL('../.capture', import.meta.url));
const out = `${repo}/docs/public/img/console`;
const bin = `${tmp}/meerkat`;
// The data directory sits at a path that says nothing about the machine: the
// snapshot screen prints it in its restore script, and a home directory in a
// published image is somebody's name.
const DATA_DIR = '/tmp/meerkat-docs/data';
const DATA = 'http://localhost:18084';
const ADMIN = 'http://localhost:19094';
const PASSWORD = 'Capture-Root-Password-1';

// The frame every published image already has. Changing it would reflow every
// caption that describes what is visible, so it is a constant, not an option.
const VIEW = { width: 1600, height: 1000 };

const log = (...a) => console.log('[capture]', ...a);

// ─── the gateway ────────────────────────────────────────────────────────────

// The binary embeds the console STAGED in internal/admin/ui/dist, not the one
// last built in console/dist: a capture from a stale stage shows yesterday's
// screens under today's captions. So the fresh build is staged first, when
// there is one.
function stageConsole() {
  const built = `${repo}/console/dist/console/browser`;
  if (!existsSync(`${built}/index.html`)) {
    log('no console build in console/dist: the staged one is used as it is');
    return;
  }
  const staged = `${repo}/internal/admin/ui/dist`;
  rmSync(staged, { recursive: true, force: true });
  mkdirSync(staged, { recursive: true });
  spawnSync('cp', ['-R', `${built}/.`, staged], { stdio: 'inherit' });
  spawnSync('touch', [`${staged}/.gitkeep`]);
  log('console staged from console/dist');
}

function startGateway() {
  stageConsole();
  const tags = existsSync(`${repo}/ee`) ? ['-tags', 'ee'] : [];
  log('building', tags.length ? '(enterprise)' : '(community)');
  const build = spawnSync('go', ['build', ...tags, '-o', bin, './cmd/meerkat'], {
    cwd: repo,
    stdio: 'inherit',
  });
  if (build.status !== 0) process.exit(build.status ?? 1);

  rmSync(DATA_DIR, { recursive: true, force: true });
  mkdirSync(DATA_DIR, { recursive: true });
  const child = spawn(
    bin,
    [
      '-addr', ':18084',
      '-admin-addr', ':19094',
      '-data', DATA_DIR,
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

// Set by the seed, read by the shots: the organisation and its group rule.
let TENANT = '';
let RULE = '';

// The catalogue the Portal screen shows: a bar with a container of modules and
// two modules of their own (PORTAL-01, PORTAL-03).
const PORTAL = {
  mode: 'portal',
  layout: 'header',
  side: 'left',
  display: 'both',
  showAppName: true,
  entries: [
    {
      label: 'Billing', icon: 'receipt_long', description: 'Invoices and price lists',
      children: [
        { routeId: 'billing', label: 'Invoices', icon: 'description' },
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

  await seedEverythingElse(api);

  const state = `${tmp}/state.json`;
  await api.storageState({ path: state });
  await api.dispose();
  return state;
}

// What the rest of the screens show. Each step is soft: a screen whose data
// could not be laid down is still captured, and the log says which step
// failed - one missing fixture must not cost the other thirty images.
async function seedEverythingElse(api) {
  const soft = async (what, call) => {
    try {
      const r = await call();
      if (r && typeof r.status === 'function' && r.status() >= 300) {
        log(`! ${what}: ${r.status()} ${(await r.text()).slice(0, 160)}`);
        return null;
      }
      return r;
    } catch (e) {
      log(`! ${what}: ${e.message}`);
      return null;
    }
  };
  const json = async (r) => (r ? r.json() : null);

  // People, an organisation and its members.
  const users = {};
  const oneTime = {};
  for (const u of [
    { username: 'alice', fullname: 'Alice Martin', email: 'alice@acme.example', enabled: true, dev: true },
    { username: 'bob', fullname: 'Bob Okafor', email: 'bob@acme.example', enabled: true },
    { username: 'carla', fullname: 'Carla Rossi', email: 'carla@acme.example', enabled: true, appAdmin: true },
    { username: 'dmitri', fullname: 'Dmitri Volkov', email: 'dmitri@acme.example', enabled: true, infraAdmin: true },
    { username: 'erin', fullname: 'Erin Walsh', email: 'erin@acme.example', enabled: true, tenantCreator: true },
  ]) {
    const body = await json(await soft(`user ${u.username}`, () => api.post('/api/users', { data: u })));
    if (body) users[u.username] = body.user.id;
    if (body) oneTime[u.username] = body.password;
  }
  const tenant = await json(await soft('tenant', () => api.post('/api/tenants', { data: { name: 'Acme Logistics' } })));
  TENANT = tenant?.id ?? '';
  if (TENANT) {
    for (const [name, type] of [['alice', 'ADMIN'], ['bob', 'USER'], ['carla', 'USER']]) {
      const userId = users[name];
      if (!userId) continue;
      await soft(`member ${name}`, () =>
        api.put(`/api/tenants/${TENANT}/members/${userId}`, {
          data: { userId, tenantId: TENANT, type, enabled: true, businessAccess: { inherited: true }, sessionTTL: '' },
        }),
      );
    }
  }

  // Two people signed in on the data plane, so the sessions screen has rows:
  // a first sign-in with the one-time password, then the forced change.
  for (const name of ['alice', 'bob']) {
    if (!oneTime[name]) continue;
    const plane = await request.newContext({ baseURL: DATA });
    await soft(`sign-in ${name}`, () => plane.post('/login', { form: { username: name, password: oneTime[name] }, maxRedirects: 0 })
      .then((r) => (r.status() === 303 ? null : r)));
    await soft(`password ${name}`, () => plane.post('/update-password', {
      form: { password: 'Capture-User-Password-1', confirm: 'Capture-User-Password-1' }, maxRedirects: 0 })
      .then((r) => (r.status() === 303 ? null : r)));
    await plane.dispose();
  }

  // The role catalogue and a group of the organisation.
  for (const role of [
    { name: 'BILLING', description: 'Reads and issues invoices', tags: ['billing'] },
    { name: 'BILLING_ADMIN', description: 'Refunds and credit notes', parent: 'BILLING', tags: ['billing'] },
    { name: 'OPS', description: 'Operates the warehouse', tags: ['inventory'] },
  ]) {
    await soft(`role ${role.name}`, () => api.post('/api/roles', { data: role }));
  }
  let group = null;
  if (TENANT) {
    group = await json(await soft('group', () =>
      api.post(`/api/tenants/${TENANT}/groups`, { data: { name: 'Finance', description: 'The accounting team', roles: ['BILLING'] } })));
    for (const g of [
      { name: 'Warehouse', description: 'Stock and shipping', roles: ['OPS'] },
      { name: 'Support', description: 'Answers customers', roles: ['BILLING'] },
    ]) {
      await soft(`group ${g.name}`, () => api.post(`/api/tenants/${TENANT}/groups`, { data: g }));
    }
  }

  // The vault: a secret and a plain value.
  await soft('vault secret', () => api.put('/api/vault/infra/stripe-key', {
    data: { kind: 'secret', value: 'sk_live_capture_only', description: 'Payment provider key', tags: ['billing'] } }));
  await soft('vault value', () => api.put('/api/vault/infra/support-email', {
    data: { kind: 'value', value: 'support@acme.example', description: 'Where the error pages send people' } }));
  await soft('vault ldap', () => api.put('/api/vault/infra/ldap-bind', {
    data: { kind: 'secret', value: 'capture-only', description: 'Bind password of the corporate directory' } }));

  // External authorities, and a rule from a directory group to the group.
  await soft('ldap authority', () => api.put('/api/auth-providers/corp-ldap', {
    data: { id: 'corp-ldap', kind: 'ldap', name: 'Corporate directory', enabled: true,
      config: { url: 'ldap://ldap.acme.internal:389', baseDn: 'dc=acme,dc=internal',
        bindDn: 'cn=gateway,ou=services,dc=acme,dc=internal', bindPassword: '$ldap-bind' } } }));
  await soft('github authority', () => api.put('/api/auth-providers/github', {
    data: { id: 'github', kind: 'github', name: 'GitHub', enabled: false,
      config: { clientId: 'Iv1.capture', clientSecret: '$stripe-key', allowedOrgs: 'acme' } } }));
  if (TENANT && group?.id) {
    const rule = await json(await soft('group rule', () => api.post(`/api/tenants/${TENANT}/group-rules`, {
      data: { providerId: 'corp-ldap', external: 'finance', groupId: group.id } })));
    RULE = rule?.id ?? '';
  }

  // The mail relay, a token for an agent, the agent endpoint and the issue
  // tracker switched on.
  await soft('mail relay', () => api.put('/api/settings/mail-relay', {
    data: { host: 'smtp.acme.example', port: 587, security: 'starttls', auth: 'password',
      username: 'gateway@acme.example', password: '$stripe-key', passwordSet: false, from: 'noreply@acme.example' } }));
  await soft('agent', () => api.put('/api/settings/agent', { data: { enabled: true } }));
  await soft('issues', () => api.put('/api/settings/issues', { data: { enabled: true } }));
  for (const t of [
    { name: 'claude-desktop', days: 90, scope: 'readonly', from: '' },
    { name: 'ci-deploy', days: 30, scope: 'full', from: '10.0.0.0/8' },
    { name: 'orders-scheduler', days: 0, scope: 'schedules', from: '' },
  ]) {
    await soft(`token ${t.name}`, () => api.post('/api/admin-tokens', { data: t }));
  }

  // An OpenAPI spec on the orders API, and per-operation rules on it: one
  // operation closed to everybody but a named user.
  const spec = {
    openapi: '3.0.3', info: { title: 'Orders', version: '2.4.0' },
    paths: {
      '/orders': {
        get: { operationId: 'listOrders', summary: 'List orders', responses: { 200: { description: 'ok' } } },
        post: { operationId: 'createOrder', summary: 'Create an order', responses: { 201: { description: 'created' } } },
      },
      '/orders/{id}': {
        get: { operationId: 'getOrder', summary: 'One order', responses: { 200: { description: 'ok' } } },
        delete: { operationId: 'cancelOrder', summary: 'Cancel an order', responses: { 204: { description: 'cancelled' } } },
      },
      '/shipments': { get: { operationId: 'listShipments', summary: 'List shipments', responses: { 200: { description: 'ok' } } } },
      '/admin/reindex': { post: { operationId: 'reindex', summary: 'Rebuild the search index', responses: { 202: { description: 'accepted' } } } },
    },
  };
  await soft('orders spec', () => api.put('/api/routes/orders-api/spec', {
    data: JSON.stringify(spec), headers: { 'Content-Type': 'application/json' } }));
  await soft('orders rules', () => api.put('/api/routes/orders-api/security', {
    data: { endpoints: [
      { method: 'POST', path: '/admin/reindex', level: 'deny', users: ['dmitri'] },
      { method: 'DELETE', path: '/orders/{id}', level: 'auth', roles: ['BILLING_ADMIN'] },
      { method: 'POST', path: '/orders', inherit: true, limits: [{ per: 'user', requests: 30, window: 'PT1M' }] },
      { method: 'GET', path: '/orders', inherit: true, limits: [{ per: 'user', requests: 300, window: 'PT1M' }] },
      { method: 'GET', path: '/shipments', inherit: true, limits: [{ per: 'ip', requests: 50, window: 'PT1S' }] },
    ] } }));
  // A request header read from the vault, now that the entry exists.
  const billing = ROUTES.find((rt) => rt.id === 'billing');
  await soft('billing vault header', () => api.put('/api/routes/billing', { data: { ...billing, filters: [
    ...billing.filters.slice(0, 2),
    { type: 'set-request-header', args: { name: 'X-Acme-Support', value: '$support-email' } },
    ...billing.filters.slice(2),
  ] } }));

  // Three calls through the orders API: a cadence, a cron line and one date.
  for (const sc of [
    { name: 'Nightly reindex', path: '/api/orders/admin/reindex', method: 'POST', roles: ['OPS'], cron: '0 2 * * *', timezone: 'Europe/Paris' },
    { name: 'Sync shipments', path: '/api/shipments', method: 'POST', roles: ['OPS'], every: 'PT15M' },
    { name: 'Close the quarter', path: '/api/orders', method: 'POST', roles: ['BILLING'], at: '2026-12-31T23:00:00Z' },
  ]) {
    await soft(`schedule ${sc.name}`, () => api.post('/api/schedules', {
      data: { routeId: 'orders-api', overlap: 'skip', catchUp: 3600, ...sc } }));
  }
  // Two saved configurations under the current one.
  for (const c of [
    { name: 'Before the SAML switch', description: 'Kept to come back to' },
    { name: 'Staging', description: 'What staging runs' },
  ]) {
    await soft(`configuration ${c.name}`, () => api.post('/api/configurations', { data: c }));
  }

  // Some traffic, so the metrics have curves: a refusal, a maintenance page,
  // upstreams that do not answer.
  const plane = await request.newContext({ baseURL: DATA });
  for (let i = 0; i < 40; i++) {
    const path = ['/billing/invoices', '/inventory/', '/docs/start', '/api/orders/42', '/'][i % 5];
    await plane.get(path, { maxRedirects: 0, timeout: 4000 }).catch(() => {});
  }
  await plane.dispose();
  log('the rest of the fixture');
}

// ─── the shots ──────────────────────────────────────────────────────────────

// One per screen of the console, and one per drawer worth showing. A path may
// be a function of what the seed created; `act` opens what a click opens.
const click = (name) => async (page) => {
  await page.getByRole('button', { name }).first().click();
  await page.waitForTimeout(900);
};
const SHOTS = [
  // ── Infra ──
  { file: 'routes-list', path: '/infra/routes' },
  { file: 'route-editor-target', path: '/infra/routes/orders-api/target' },
  { file: 'route-editor-security', path: '/infra/routes/orders-api/security' },
  { file: 'route-editor-predicates', path: '/infra/routes/orders-api/predicates' },
  { file: 'route-editor-filters', path: '/infra/routes/billing/modin' },
  { file: 'route-editor-color-scheme', path: '/infra/routes/billing/scheme' },
  { file: 'endpoint-security', path: '/infra/endpoint-security?route=orders-api', settle: 2000 },
  {
    file: 'endpoint-rule', path: '/infra/endpoint-security?route=orders-api', settle: 2000,
    act: async (page) => {
      await page.getByText('/admin/reindex').first().click();
      await page.waitForTimeout(1000);
    },
  },
  { file: 'endpoint-limits', path: '/infra/endpoint-limits?route=orders-api', settle: 2000 },
  { file: 'endpoint-audit', path: '/infra/endpoint-audit?route=orders-api', settle: 2000 },
  { file: 'auth-providers', path: '/infra/auth-providers' },
  { file: 'auth-provider-editor', path: '/infra/auth-providers/corp-ldap', settle: 1600 },
  {
    // The bottom of the same drawer: what the authority says about people.
    file: 'auth-provider-test', path: '/infra/auth-providers/corp-ldap', settle: 1600,
    act: async (page) => {
      await page.getByText('Seen at sign-in').first().scrollIntoViewIfNeeded();
      await page.waitForTimeout(500);
    },
  },
  { file: 'mail-relay', path: '/infra/mail-relay' },
  { file: 'tls', path: '/infra/tls' },
  { file: 'tls-authorities', path: '/infra/tls', act: click('ACME') },
  { file: 'tls-local', path: '/infra/tls', act: click('On a local machine') },
  { file: 'access-tokens', path: '/infra/access-tokens' },
  { file: 'mcp', path: '/infra/mcp' },
  { file: 'model', path: '/infra/model' },
  { file: 'opentelemetry', path: '/infra/opentelemetry' },
  { file: 'opentelemetry-collector', path: '/infra/opentelemetry', act: click('No collector yet?') },
  { file: 'plug', path: '/infra/plug' },
  { file: 'plug-machine', path: '/infra/plug', act: click("On a developer's machine") },
  // ── Application ──
  { file: 'general', path: '/application/general' },
  { file: 'roles', path: '/application/roles' },
  { file: 'users', path: '/application/users' },
  { file: 'built-in-pages-theme', path: '/application/built-in-pages/theme', settle: 2000 },
  { file: 'built-in-pages-layout', path: '/application/built-in-pages/layout', settle: 2000 },
  { file: 'built-in-pages-branding', path: '/application/built-in-pages/branding', settle: 2000 },
  { file: 'built-in-pages-locale', path: '/application/built-in-pages/locale' },
  // The portal preview is a real iframe doing a handshake before it draws
  // anything: caught too early it publishes an empty bar.
  { file: 'portal', path: '/application/portal', settle: 3500 },
  { file: 'security', path: '/application/security' },
  // ── Tenants ──
  { file: 'tenants', path: '/tenants' },
  { file: 'groups', path: () => TENANT && `/tenants/${TENANT}/groups` },
  { file: 'members', path: () => TENANT && `/tenants/${TENANT}/members` },
  { file: 'group-rules', path: () => TENANT && RULE && `/tenants/${TENANT}/rules/${RULE}`, settle: 1600 },
  // ── Vault ──
  { file: 'vault', path: '/vault' },
  // ── Data plane ──
  { file: 'traffic', path: '/data-plane/metrics', settle: 2500 },
  {
    // The same screen, scrolled to the ranking of routes.
    file: 'metrics', path: '/data-plane/metrics', settle: 2500,
    act: async (page) => {
      await page.getByText('Slowest').first().scrollIntoViewIfNeeded();
      await page.waitForTimeout(500);
    },
  },
  { file: 'sessions', path: '/data-plane/sessions' },
  { file: 'scheduler', path: '/data-plane/scheduler' },
  { file: 'issues', path: '/data-plane/issues' },
  // ── Meerkat ──
  { file: 'audit', path: '/system/audit' },
  {
    file: 'logs', path: '/system/logs', settle: 1800,
    act: async (page) => {
      await page.getByText('upstream error').first().click();
      await page.waitForTimeout(600);
    },
  },
  { file: 'configuration', path: '/system/configuration' },
  { file: 'database', path: '/system/configuration/snapshot' },
  { file: 'api-reference', path: '/system/api', settle: 2500 },
  { file: 'release-notes', path: '/system/release-notes' },
  { file: 'license', path: '/system/license' },
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
    const path = typeof s.path === 'function' ? s.path() : s.path;
    if (!path) {
      log('! skipped', s.file, '- the seed did not create what it shows');
      continue;
    }
    await page.goto(ADMIN + '#' + path).catch(() => {});
    await page.goto(ADMIN + path);
    // The console animates drawers in; a frame taken mid-slide is a blurred
    // panel nobody can read.
    await page.waitForTimeout(s.settle ?? 1200);
    if (s.act) {
      try {
        await s.act(page);
      } catch (e) {
        log('! could not open what', s.file, 'shows:', e.message.split('\n')[0]);
      }
    }
    // Nothing of the machine the capture ran on: its name (the logs screen
    // says which node answered) and the path of the disposable data
    // directory (the snapshot screen prints it) are a stranger's details in
    // a public image. Replaced in the page, in the text people would read.
    await page.evaluate(([host, data]) => {
      const clean = (v) => v.split(data).join('/data').split(host).join('gateway-1');
      // Shadow roots too: a code snippet draws its text inside one.
      const roots = [document.body];
      while (roots.length) {
        const root = roots.pop();
        const walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
        for (let n = walk.nextNode(); n; n = walk.nextNode()) {
          if (n.nodeType === Node.TEXT_NODE) {
            if (n.nodeValue.includes(host) || n.nodeValue.includes(data)) n.nodeValue = clean(n.nodeValue);
          } else {
            if (n.shadowRoot) roots.push(n.shadowRoot);
            if ((n.tagName === 'TEXTAREA' || n.tagName === 'INPUT') && n.value) n.value = clean(n.value);
          }
        }
      }
    }, [hostname(), DATA_DIR]).catch(() => {});
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
