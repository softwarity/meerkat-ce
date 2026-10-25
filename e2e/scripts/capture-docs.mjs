// Retakes the console screenshots the documentation site publishes.
//
// The documentation's, into img/console/*: each shows the state its page
// explains. The showcase has its own pictures (img/showcase/*), taken from a
// demo instance by docs/scripts/screenshots.mjs, and this never writes them.
//
// The doc images are the product's face, and a stale one is a lie somebody
// acts on: a section list that still names a screen we renamed sends a reader
// hunting for it. They were taken by hand until now, which is why they drifted.
// This does it the way the e2e suite does everything - a binary built from the
// working tree, a disposable database, a browser driven at a fixed size - so a
// retake is one command and produces the same frame every time.
//
//   node e2e/scripts/capture-docs.mjs [file...]
//
// Ports are offset again from the e2e ones, so a running `make dev` AND a
// running suite both stay out of the way.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, rmSync, readdirSync, unlinkSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { hostname } from 'node:os';
import { chromium, request } from '@playwright/test';

const repo = fileURLToPath(new URL('../..', import.meta.url));
const tmp = fileURLToPath(new URL('../.capture', import.meta.url));
const out = `${repo}/docs/public/img/console`;
const bin = `${tmp}/meerkat`;
// The data directory sits at a path that says nothing about the machine: the
// database screen prints it in its restore script, and a home directory in a
// published image is somebody's name.
const DATA_DIR = '/tmp/meerkat-docs/data';
const DATA = 'http://localhost:18084';
const ADMIN = 'http://localhost:19094';
const INTERNAL = 'http://localhost:17074';
const PASSWORD = 'Capture-Root-Password-1';

// The frame every published image already has. Changing it would reflow every
// caption that describes what is visible, so it is a constant, not an option.
const VIEW = { width: 1600, height: 1000 };

const log = (...a) => console.log('[capture]', ...a);

// Retaking a few screens is `node e2e/scripts/capture-docs.mjs routes-list
// tls`: the others are left as they are - a retake of everything also
// redraws the screens whose data moves (curves, times) for nothing.
const ONLY = process.argv.slice(2);

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
      '-internal-addr', ':17074',
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
      // The Custom section's list: a block written here, and a file the seed
      // uploads below.
      injections: [
        { kind: 'css', code: ':root { --acme-brand: #0b5cad; }\n.app-header { background: var(--acme-brand); }', position: 'head-end' },
      ],
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
    // Kept: the notifications shots act as these two readers, and show the
    // page one of them reads.
    await plane.storageState({ path: `${tmp}/data-${name}.json` });
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
    // An internal token: the scheduler's, listed under Internal beside the
    // one the orders service mints below.
    { name: 'billing', days: 0, scope: 'schedules', from: '' },
  ]) {
    await soft(`token ${t.name}`, () => api.post('/api/admin-tokens', { data: t }));
  }

  // An OpenAPI spec on the orders API, and per-operation rules on it: one
  // operation closed to everybody but a named user.
  const spec = {
    openapi: '3.0.3', info: { title: 'Orders', version: '2.4.0' },
    // What the service itself requires, the way its framework writes it: a
    // token on everything, and the roles of two operations as scopes
    // (RBAC-12). The shipments list is open.
    components: { securitySchemes: { bearer: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT' } } },
    security: [{ bearer: [] }],
    paths: {
      '/orders': {
        get: { operationId: 'listOrders', summary: 'List orders', responses: { 200: { description: 'ok' } } },
        post: { operationId: 'createOrder', summary: 'Create an order', security: [{ bearer: ['ORDERS_WRITE'] }], responses: { 201: { description: 'created' } } },
      },
      '/orders/{id}': {
        get: { operationId: 'getOrder', summary: 'One order', responses: { 200: { description: 'ok' } } },
        delete: { operationId: 'cancelOrder', summary: 'Cancel an order', responses: { 204: { description: 'cancelled' } } },
      },
      '/shipments': { get: { operationId: 'listShipments', summary: 'List shipments', security: [], responses: { 200: { description: 'ok' } } } },
      '/admin/reindex': { post: { operationId: 'reindex', summary: 'Rebuild the search index', security: [{ bearer: ['ORDERS_ADMIN'] }], responses: { 202: { description: 'accepted' } } } },
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
  // The roles the service declares, tied to the catalogue: one imported under
  // the route's prefix and tag, one onto a role that already exists.
  await soft('orders declared roles', () => api.put('/api/routes/orders-api/role-map', {
    data: {
      roleMap: { ORDERS_WRITE: 'orders-api-ORDERS_WRITE', ORDERS_ADMIN: 'OPS' },
      create: [{ name: 'orders-api-ORDERS_WRITE', tags: ['orders-api'] }],
    } }));
  // A request header read from the vault, now that the entry exists.
  const billing = ROUTES.find((rt) => rt.id === 'billing');
  await soft('billing vault header', () => api.put('/api/routes/billing', { data: { ...billing, filters: [
    ...billing.filters.slice(0, 2),
    { type: 'set-request-header', args: { name: 'X-Acme-Support', value: '$support-email' } },
    ...billing.filters.slice(2),
  ] } }));

  // Three calls through the orders API: a cadence, a cron line and one date.
  // Written the way a service writes them: its own token, on the internal
  // plane - the console creates none.
  const minted = await api.post('/api/admin-tokens', { data: { name: 'orders-api', scope: 'schedules' } });
  const svc = await request.newContext({ baseURL: INTERNAL, extraHTTPHeaders: { Authorization: `Bearer ${(await minted.json()).token}` } });
  for (const sc of [
    { name: 'Nightly reindex', description: "Rebuilds the order search index from the day's writes",
      path: '/api/orders/admin/reindex', method: 'POST', roles: ['OPS'], cron: '0 2 * * *', timezone: 'Europe/Paris' },
    { name: 'Sync shipments', description: "Pulls the carriers' shipment updates into the orders",
      path: '/api/shipments', method: 'POST', roles: ['OPS'], every: 'PT15M' },
    { name: 'Close the quarter', description: "Locks the quarter's invoices and starts the accounting export",
      path: '/api/orders', method: 'POST', roles: ['BILLING'], at: '2026-12-31T23:00:00Z' },
  ]) {
    await soft(`schedule ${sc.name}`, () => svc.post('/api/schedules', {
      data: { routeId: 'orders-api', overlap: 'skip', catchUp: 3600, ...sc } }));
  }
  await svc.dispose();
  // Two saved configurations under the current one.
  for (const c of [
    { name: 'Before the SAML switch', description: 'Kept to come back to' },
    { name: 'Staging', description: 'What staging runs' },
  ]) {
    await soft(`configuration ${c.name}`, () => api.post('/api/configurations', { data: c }));
  }

  // Billing's custom code takes a script from a file, deferred at the end of
  // the body: uploaded on the route, then named by a block.
  const billingRoute = ROUTES.find((r) => r.id === 'billing');
  if (await soft('billing script', () => api.put('/api/routes/billing/files/acme-analytics.js', {
    headers: { 'Content-Type': 'text/javascript' },
    data: 'window.acmeAnalytics = { page: location.pathname };\n',
  }))) {
    await soft('billing custom code', () => api.put('/api/routes/billing', { data: {
      ...billingRoute,
      ui: { ...billingRoute.ui, injections: [...billingRoute.ui.injections,
        { kind: 'js', file: 'acme-analytics.js', position: 'body-end', load: 'defer' }] },
    } }));
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
// A route in the Files mode (ROUTE-22), laid down just before its own shot:
// in the fixture it would be a sixth route on the routes screen, whose
// captions count five.
const FONTS = {
  id: 'fonts', name: 'Fonts', order: 6, enabled: true,
  predicates: [{ type: 'path', args: { patterns: ['/fonts/**'] } }],
  filters: [{ type: 'files', args: { cors: true, maxAge: 86400 } }],
};
async function seedFonts(page) {
  const api = page.request;
  await api.put(`${ADMIN}/api/routes/fonts`, { data: FONTS });
  const css = '@font-face {\n  font-family: "Inter";\n  src: url(files/inter-latin.woff2) format("woff2");\n}\n';
  await api.put(`${ADMIN}/api/routes/fonts/files/inter.css`, { headers: { 'Content-Type': 'text/css' }, data: css });
  for (const f of ['inter-latin.woff2', 'inter-latin-ext.woff2']) {
    const body = readFileSync(`${repo}/internal/fonts/files/${f}`);
    await api.put(`${ADMIN}/api/routes/fonts/files/files/${f}`, { headers: { 'Content-Type': 'font/woff2' }, data: body });
  }
}

// The OpenID provider (SAUTH-05), laid down just before its shots: a public
// name to be the issuer, three clients - one per kind of secret - and a route
// signing its people into one of them. Not in the fixture, for the same reason
// as the fonts route: the routes screen's captions count five.
const WIKI = {
  id: 'wiki', name: 'Team wiki', order: 7, enabled: true, isUi: true,
  upstream: 'http://wiki.svc.acme.internal:3000',
  access: { level: 'auth' },
  predicates: [{ type: 'path', args: { patterns: ['/wiki/**'] } }],
  identity: {
    mechanism: 'oidc', client: 'wiki',
    attributes: [{ field: 'username' }, { field: 'fullname' }, { field: 'email' }, { field: 'roles' }],
    claims: [
      { name: 'preferred_username', field: 'username' },
      { name: 'name', field: 'fullname' },
      { name: 'email', field: 'email' },
      { name: 'groups', field: 'roles' },
    ],
  },
};
async function seedOidc(page) {
  const api = page.request;
  await api.put(`${ADMIN}/api/settings/tls/doors/app`, {
    data: { proxied: true, names: ['shop.example.com', 'apps.example.com'], proxyAddresses: [] },
  });
  for (const c of [
    { id: 'wiki' },
    { id: 'dashboards', secret: 'chart-generated-4f9c2b7e1d' },
    { id: 'mobile-app', confidential: false },
  ]) {
    await api.post(`${ADMIN}/api/oidc/clients`, { data: c });
  }
  await api.put(`${ADMIN}/api/routes/wiki`, { data: WIKI });
}
const openOidcDrawer = async (page) => {
  await page.getByRole('button', { name: 'OIDC / JWT' }).first().click();
  await page.waitForTimeout(1200);
  await page.locator('app-signing-keys-panel section.clients').scrollIntoViewIfNeeded();
  await page.waitForTimeout(500);
};

// The upstream picker (SVC-02) shows only where the runtime lists several
// namespaces, which a capture on a workstation has none of: the answer of a
// Kubernetes gateway allowed two namespaces is laid in front of /api/services,
// and the orders route points at its service by the full name a pick stores.
const ACME_SERVICES = {
  source: 'kubernetes',
  namespace: 'acme',
  namespaces: ['acme', 'billing'],
  services: [
    { name: 'orders', full: 'orders.acme.svc', ports: [{ target: 9000 }, { target: 9090 }], ready: 2, wanted: 2, counted: true },
    { name: 'docs', full: 'docs.acme.svc', ports: [{ target: 80 }], ready: 1, wanted: 1, counted: true },
    { name: 'inventory', full: 'inventory.acme.svc', ports: [{ target: 8080 }], ready: 1, wanted: 1, counted: true },
    { name: 'www', full: 'www.acme.svc', ports: [{ target: 80 }], ready: 1, wanted: 1, counted: true },
  ],
};
async function seedPicker(page) {
  await page.route('**/api/services**', (r) =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ACME_SERVICES) }));
  // Read back rather than taken from ROUTES: the seed has since given it a
  // spec, which a route written from the fixture alone would drop.
  const orders = await (await page.request.get(`${ADMIN}/api/routes/orders-api`)).json();
  await page.request.put(`${ADMIN}/api/routes/orders-api`, { data: { ...orders, upstream: 'http://orders.acme.svc:9000' } });
}

// The notifications (NOTIF-05, Enterprise), laid down once before their
// shots, which come last: a second tenant, published messages and reader
// marks would otherwise change the tenants, audit and access-tokens screens
// the other captions describe. Turned on by root; a second tenant, Acme
// Retail, holding alice too, so her page has a tenant filter and a message
// sent to both tenants on one row; two services publishing with their own
// token, as a service does; one sent from the console; one withdrawn; bob
// closing an alarm asked of one reader and acknowledging his part of an
// instruction asked of all, so alice still owes hers.
let RETAIL = '';
let notified = null;
const NOTE = {
  alarm: 'Cold room 3 is too warm',
  floor: 'Forklifts now use aisle 4',
  invoice: 'Invoice INV-2026-0142 is overdue',
};
function seedNotifications(page) {
  notified ??= layNotifications(page.request).catch((e) => log('! notifications:', e.message.split('\n')[0]));
  return notified;
}
async function layNotifications(api) {
  const soft = async (what, call) => {
    try {
      const r = await call();
      if (r.status() >= 300) {
        log(`! ${what}: ${r.status()} ${(await r.text()).slice(0, 200)}`);
        return null;
      }
      return r;
    } catch (e) {
      log(`! ${what}: ${e.message}`);
      return null;
    }
  };
  const json = async (r) => (r ? r.json() : null);
  await soft('notifications on', () => api.put(`${ADMIN}/api/notifications/settings`, { data: { enabled: true } }));

  const people = Object.fromEntries(((await json(await soft('users', () => api.get(`${ADMIN}/api/users`)))) ?? [])
    .map((u) => [u.username, u.id]));
  RETAIL = (await json(await soft('tenant Acme Retail', () =>
    api.post(`${ADMIN}/api/tenants`, { data: { name: 'Acme Retail' } }))))?.id ?? '';
  for (const [name, type] of [['alice', 'USER'], ['erin', 'ADMIN']]) {
    if (!RETAIL || !people[name]) continue;
    await soft(`retail member ${name}`, () => api.put(`${ADMIN}/api/tenants/${RETAIL}/members/${people[name]}`, {
      data: { userId: people[name], tenantId: RETAIL, type, enabled: true, businessAccess: { inherited: true }, sessionTTL: '' },
    }));
  }

  // The portal bar gets its Notifications entry, wearing the bell the
  // console's Add notifications gives it.
  const bell = ((await json(await soft('bell', () => api.get(`${ADMIN}/api/portal/icons?q=notifications&limit=20`)))) ?? [])
    .find((i) => i.name === 'notifications')?.svg ?? '';
  const settings = await json(await soft('settings', () => api.get(`${ADMIN}/api/settings`)));
  if (settings) {
    await soft('portal notifications entry', () => api.put(`${ADMIN}/api/settings`, {
      data: { ...settings, portal: { ...PORTAL, entries: [...PORTAL.entries, { special: 'notifications', icon: bell }] } },
    }));
  }

  // Two services, each with its token on the internal plane.
  const services = {};
  for (const name of ['invoicing', 'warehouse']) {
    const minted = await json(await soft(`token ${name}`, () =>
      api.post(`${ADMIN}/api/admin-tokens`, { data: { name, scope: 'notifications' } })));
    if (minted) {
      services[name] = await request.newContext({ baseURL: INTERNAL,
        extraHTTPHeaders: { Authorization: `Bearer ${minted.token}` } });
    }
  }
  const sent = {};
  const publish = async (from, key, message) => {
    if (!services[from]) return;
    const r = await soft(`notification ${key}`, () => services[from].post('/api/notifications', { data: message }));
    if (r) sent[key] = (await r.json()).id;
    // One second apart: the lists sort by the second they were sent in.
    await new Promise((res) => setTimeout(res, 1100));
  };
  const logistics = 'Acme Logistics';
  await publish('invoicing', 'draft', {
    tenant: logistics, title: 'October price list draft', body: 'A first draft, to be replaced by the final list.',
    meta: { pricelist: '2026-10' },
  });
  await publish('warehouse', 'returns', {
    tenant: 'Acme Retail', title: 'Returns are now accepted for 60 days',
    body: 'From November, customers may bring an item back within 60 days with its receipt.',
  });
  await publish('warehouse', 'floor', {
    tenant: logistics, level: 'info', title: NOTE.floor,
    body: 'From Monday the forklift lane moves to aisle 4. Read the new floor plan before your next shift.',
    detail: { service: 'docs', path: '/floor-plan' }, ack: 'all', meta: { site: 'rotterdam' },
  });
  await publish('warehouse', 'alarm', {
    tenant: logistics, level: 'error', title: NOTE.alarm,
    body: 'Sensor CR3 reads 9.4 C since 06:12. Check the door seal and the compressor.',
    ack: 'single', onAck: { service: 'orders', path: '/orders/alerts/acked' }, key: 'cr3-temperature',
  });
  await publish('invoicing', 'saturday', {
    tenant: '*', title: 'Billing is read-only on Saturday night',
    body: 'Invoices can be read but not changed on 2026-11-07 from 22:00 to 24:00 UTC.', meta: { change: 'CHG-2291' },
  });
  await publish('invoicing', 'invoice', {
    tenant: logistics, level: 'warning', title: NOTE.invoice,
    body: 'Northwind Traders has not paid the 4820.00 EUR due on 2026-10-20.',
    detail: { service: 'billing', path: '/invoices/INV-2026-0142' },
    actions: [{ label: 'Send a reminder', service: 'billing', path: '/invoices/INV-2026-0142/remind',
      method: 'POST', body: { channel: 'email' } }],
    key: 'overdue-INV-2026-0142', meta: { invoice: 'INV-2026-0142' },
  });
  // From the console, as the tenant page's drawer sends it.
  if (TENANT) {
    await soft('notification from the console', () => api.post(`${ADMIN}/api/tenants/${TENANT}/notifications`, {
      data: { level: 'success', title: 'Stock count closed',
        body: 'The stock count of 2026-10-24 is closed. 3 differences are left to review.' },
    }));
  }
  if (sent.draft && services.invoicing) {
    await soft('withdraw the draft', () => services.invoicing.delete(`/api/notifications/${sent.draft}`));
  }
  for (const s of Object.values(services)) await s.dispose();

  // The readers: bob closes the alarm for everybody and gives his part of the
  // floor plan; alice has read the alarm and the stock count.
  const reader = async (name) => existsSync(`${tmp}/data-${name}.json`)
    ? request.newContext({ baseURL: DATA, storageState: `${tmp}/data-${name}.json` }) : null;
  const bob = await reader('bob');
  if (bob) {
    for (const key of ['alarm', 'floor']) {
      if (sent[key]) await soft(`bob acknowledges ${key}`, () => bob.post(`/meerkat/notifications/${sent[key]}/ack`));
    }
    await bob.dispose();
  }
  const alice = await reader('alice');
  if (alice) {
    const list = await json(await soft('alice reads her list', () => alice.get('/meerkat/notifications.json')));
    for (const it of list?.items ?? []) {
      if (it.title === NOTE.alarm || it.title === 'Stock count closed') {
        await soft(`alice reads ${it.title}`, () => alice.post(`/meerkat/notifications/${it.id}/read`));
      }
    }
    await alice.dispose();
  }
  log('notifications laid down');
}

// A data-plane page, as one of the seeded readers: what a user of the
// applications sees, in the light scheme most of them use. In the zone of
// the warehouse rather than the machine's, so the times in the image say
// nothing of where it was taken. And as a reader who has not answered the
// desktop question yet: a headless browser answers it "denied" for them, and
// the page would show the line for a blocked browser instead of its button.
const readers = {};
async function readerPage(browser, who) {
  if (!readers[who]) {
    const ctx = await browser.newContext({
      viewport: VIEW, colorScheme: 'light', deviceScaleFactor: 1, timezoneId: 'Europe/Amsterdam',
      storageState: `${tmp}/data-${who}.json`,
    });
    await ctx.addInitScript(() => {
      if (window.Notification) Object.defineProperty(Notification, 'permission', { get: () => 'default' });
    });
    readers[who] = await ctx.newPage();
  }
  return readers[who];
}

// The New notification drawer filled in the way the captions describe: the
// overdue invoice, its detail on Billing, and a reminder sent with POST and a
// JSON body.
async function composeInvoice(page) {
  await page.getByRole('button', { name: /New notification/ }).first().click();
  await page.waitForTimeout(1200);
  const drawer = page.locator('app-tenant-notification-compose');
  await drawer.locator('mat-button-toggle', { hasText: 'Warning' }).locator('button').click();
  await drawer.getByLabel('Title').fill(NOTE.invoice);
  await drawer.getByLabel('Text').fill('Northwind Traders has not paid the 4820.00 EUR due on 2026-10-20.');
  const detail = drawer.locator('section', { has: page.getByRole('heading', { name: 'Detail' }) });
  await detail.locator('mat-button-toggle', { hasText: 'Route' }).locator('button').click();
  await detail.locator('mat-select').first().click();
  await page.getByRole('option', { name: /Billing/ }).first().click();
  await detail.getByLabel('Path').fill('invoices/INV-2026-0142');
  const actions = drawer.locator('section', { has: page.getByRole('heading', { name: 'Actions' }) });
  await actions.getByRole('button', { name: /Add action/ }).click();
  await actions.getByLabel('Label').first().fill('Send a reminder');
  // The method first, beside the route: POST, which opens the body editor.
  await actions.locator('mat-form-field.method mat-select').first().click();
  await page.getByRole('option', { name: 'POST', exact: true }).click();
  await actions.locator('mat-form-field', { has: page.locator('mat-label', { hasText: /^Route$/ }) })
    .locator('mat-select').first().click();
  await page.getByRole('option', { name: /Billing/ }).first().click();
  await actions.getByLabel('Path').first().fill('invoices/INV-2026-0142/remind');
  await page.waitForTimeout(400);
  // The body is a code editor: the text goes in whole, so its own bracket
  // closing adds nothing.
  await actions.locator('app-json-editor .cm-content').first().click();
  await page.keyboard.insertText('{\n  "channel": "email"\n}');
  // Back up to the Detail heading: the link it opens, then the action and
  // its body, in one frame.
  await detail.evaluate((el) => el.scrollIntoView({ block: 'start' }));
  await page.waitForTimeout(600);
}

const SHOTS = [
  // ── Infra ──
  { file: 'routes-list', path: '/infra/routes' },
  { file: 'route-editor-target', path: '/infra/routes/orders-api/target', prepare: seedPicker, settle: 1600 },
  { file: 'route-editor-security', path: '/infra/routes/orders-api/security' },
  { file: 'route-editor-predicates', path: '/infra/routes/orders-api/predicates' },
  { file: 'route-editor-filters', path: '/infra/routes/billing/modin' },
  { file: 'route-editor-color-scheme', path: '/infra/routes/billing/scheme' },
  { file: 'route-editor-custom', path: '/infra/routes/billing/custom' },
  { file: 'route-editor-files', path: '/infra/routes/fonts/target', prepare: seedFonts, settle: 1600 },
  { file: 'endpoint-security', path: '/infra/endpoint-security?route=orders-api', settle: 2000 },
  {
    file: 'endpoint-rule', path: '/infra/endpoint-security?route=orders-api', settle: 2000,
    act: async (page) => {
      await page.getByText('/admin/reindex').first().click();
      await page.waitForTimeout(1000);
    },
  },
  {
    // The mapping drawer, opened from the header: what the service declares,
    // and the catalogue role each one is tied to.
    file: 'endpoint-declared-roles', path: '/infra/endpoint-security?route=orders-api', settle: 2000,
    act: async (page) => {
      await page.getByRole('button', { name: /Declared roles/ }).first().click();
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
  // The Application tab, with two public names served by a proxy in front
  // (SSL-14).
  {
    file: 'tls-proxied',
    path: '/infra/tls',
    act: async (page) => {
      await page.getByRole('tab', { name: /Application/ }).click();
      const app = page.getByRole('tabpanel', { name: /Application/ });
      await app.locator('app-door-names').waitFor();
      await app.locator('mat-button-toggle', { hasText: 'A proxy in front' }).locator('button').click();
      await app.getByRole('button', { name: 'Apply' }).click();
      await page.locator('mat-dialog-container').getByRole('button', { name: 'Apply' }).click();
      await page.waitForTimeout(400);
      const input = app.locator('app-door-names mat-chip-grid input').first();
      for (const name of ['shop.example.com', 'apps.example.com']) {
        await input.fill(name);
        await input.press('Enter');
        await page.waitForTimeout(700);
      }
      await input.blur();
      await page.waitForTimeout(600);
    },
  },
  // After tls-proxied, which declares the same public names through the
  // screen: the provider needs one for its issuer.
  // The OIDC / JWT drawer down to its clients, one of each kind of secret.
  { file: 'oidc-clients', path: '/infra/routes', prepare: seedOidc, settle: 1600, act: openOidcDrawer },
  {
    // A new client: its secret drawn in the form, to copy before saving.
    file: 'oidc-client', path: '/infra/routes', settle: 1600,
    act: async (page) => {
      await openOidcDrawer(page);
      await page.getByRole('button', { name: 'New client' }).first().click();
      await page.waitForTimeout(800);
      await page.locator('mat-dialog-container input').first().fill('reports');
      await page.waitForTimeout(400);
    },
  },
  {
    // A sign-in tried for one person through the wiki's client.
    file: 'oidc-try', path: '/infra/routes', settle: 1600,
    act: async (page) => {
      await openOidcDrawer(page);
      // The row's actions show on hover.
      const row = page.locator('app-signing-keys-panel mat-row', { hasText: 'Team wiki' });
      await row.hover();
      // rowActions lifts them out of the row: found on the page.
      await page.getByRole('button', { name: 'Try a sign-in' }).first().click();
      await page.waitForTimeout(900);
      const dialog = page.locator('mat-dialog-container');
      await dialog.locator('mat-select').first().click();
      await page.getByRole('option', { name: /^alice/ }).click();
      await dialog.getByRole('button', { name: 'Try' }).click();
      await page.waitForTimeout(1200);
    },
  },
  // A route signing its people into a client, with the claims it sends.
  { file: 'route-editor-oidc', path: '/infra/routes/wiki/authforward', settle: 1600 },
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
  // The same catalogue in Links mode: its two levels in the list, and beside
  // it the account button's Applications menu, another framed handshake.
  {
    file: 'portal-links', path: '/application/portal', settle: 2000,
    act: async (page) => {
      await page.locator('mat-button-toggle[value="links"] button').click();
      await page.waitForTimeout(2500);
    },
  },
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
  {
    // The Access drawer: the Meerkat token mode, its Mint a token, and the
    // one service let in. A fresh installation starts in None; this is the
    // state an upgraded one keeps. Cluster token is offered on Kubernetes
    // only, which a workstation is not: the answer says Kubernetes, so the
    // drawer shows the three modes the page describes.
    file: 'scheduler-access', path: '/data-plane/scheduler',
    prepare: async (page) => {
      await page.request.put(`${ADMIN}/api/settings/internal-access`, { data: { mode: 'token', callers: ['orders-api'] } });
      await page.route('**/api/settings/internal-access', async (r) => {
        if (r.request().method() !== 'GET') return r.continue();
        const res = await r.fetch();
        const view = await res.json();
        r.fulfill({ response: res, json: { ...view, modes: ['none', 'token', 'cluster'], platform: 'kubernetes' } });
      });
    },
    act: async (page) => {
      await page.getByRole('button', { name: 'Access', exact: true }).first().click();
      await page.waitForTimeout(1200);
    },
  },
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
  // Making a saved configuration current, one line kept out: a route added
  // since it was saved, which the switch would remove, unticked.
  {
    file: 'configuration-keep',
    path: '/system/configuration',
    act: async (page) => {
      await page.request.put(`${ADMIN}/api/routes/legacy`, {
        data: {
          id: 'legacy', name: 'Legacy portal', order: 20, enabled: true,
          upstream: 'http://legacy.svc.acme.internal:80',
          predicates: [{ type: 'path', args: { patterns: ['/legacy/**'] } }],
        },
      });
      await page.reload();
      await page.waitForTimeout(1200);
      const row = page.getByRole('row', { name: /Before the SAML switch/ }).first();
      await row.hover();
      await page.locator('button[aria-label="Set as current"]:visible').first().click();
      const go = page.getByRole('button', { name: 'Continue' });
      // "What is running is not saved" comes first when it has drifted.
      await go.waitFor({ timeout: 4000 }).then(() => go.click(), () => {});
      const dialog = page.locator('app-apply-plan-dialog');
      await dialog.waitFor();
      await dialog.locator('tr', { hasText: 'Legacy portal' }).locator('mat-checkbox input').uncheck();
      await page.waitForTimeout(500);
    },
  },
  { file: 'database', path: '/system/configuration/database' },
  {
    // The move to PostgreSQL, as the procedure reads before the copy: the
    // target filled in, the SQL that creates it, the steps still to do.
    file: 'database-migrate', path: '/system/configuration/database',
    act: async (page) => {
      await page.locator('mat-button-toggle', { hasText: 'PostgreSQL' }).locator('button').click();
      await page.locator('mat-button-toggle', { hasText: 'A server' }).locator('button').click();
      await page.getByLabel('Host').fill('canopydb-primary.db.svc');
      await page.getByText('How to migrate').scrollIntoViewIfNeeded();
      await page.mouse.wheel(0, 320);
      await page.waitForTimeout(600);
    },
  },
  { file: 'api-reference', path: '/system/api', settle: 2500 },
  { file: 'release-notes', path: '/system/release-notes' },
  { file: 'license', path: '/system/license' },
  // ── Notifications (Enterprise) - last, see seedNotifications ──
  { file: 'notifications', path: '/data-plane/notifications', prepare: seedNotifications, settle: 1600 },
  {
    // Root's drawer: the gateway's switch, a checkbox per tenant, the cap.
    file: 'notifications-settings', path: '/data-plane/notifications', prepare: seedNotifications, settle: 1600,
    act: click('Settings'),
  },
  {
    // The API panel beside Settings and Access, for a service's developer.
    file: 'notifications-api', path: '/data-plane/notifications', prepare: seedNotifications, settle: 1600,
    act: async (page) => {
      await page.getByRole('button', { name: 'API', exact: true }).first().click();
      await page.waitForTimeout(1200);
    },
  },
  {
    file: 'tenant-notifications', path: () => TENANT && `/tenants/${TENANT}/notifications`,
    prepare: seedNotifications, settle: 1600,
  },
  {
    // The instruction asked of every reader: bob gave his, alice and carla
    // still owe theirs.
    file: 'tenant-notification-detail', path: () => TENANT && `/tenants/${TENANT}/notifications`,
    prepare: seedNotifications, settle: 1600,
    act: async (page) => {
      await page.getByText(NOTE.floor).first().click();
      await page.waitForTimeout(1200);
    },
  },
  {
    file: 'notification-compose', path: () => TENANT && `/tenants/${TENANT}/notifications`,
    prepare: seedNotifications, settle: 1600, act: composeInvoice,
  },
  {
    // The same message, as a service would send it.
    file: 'notification-compose-json', path: () => TENANT && `/tenants/${TENANT}/notifications`,
    prepare: seedNotifications, settle: 1600,
    act: async (page) => {
      await composeInvoice(page);
      await page.getByRole('tab', { name: 'JSON' }).or(page.getByRole('link', { name: 'JSON' })).first().click();
      await page.waitForTimeout(1000);
    },
  },
  // The Notifications page card, and the bar's Notifications entry with its
  // sample count.
  { file: 'portal-notifications', path: '/application/portal', prepare: seedNotifications, settle: 3500 },
  {
    // alice's own page, on the data plane: two tenants, the message sent to
    // both on one row, the floor plan she still owes, the invoice unfolded.
    file: 'notifications-page', as: 'alice', path: '/meerkat/notifications', prepare: seedNotifications, settle: 1600,
    act: async (page) => {
      await page.getByText(NOTE.invoice).first().click();
      await page.waitForTimeout(900);
    },
  },
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
    if (ONLY.length && !ONLY.includes(s.file)) continue;
    if (s.prepare) {
      try {
        await s.prepare(page);
      } catch (e) {
        log('! could not lay down what', s.file, 'shows:', e.message.split('\n')[0]);
      }
    }
    const path = typeof s.path === 'function' ? s.path() : s.path;
    if (!path) {
      log('! skipped', s.file, '- the seed did not create what it shows');
      continue;
    }
    // A shot `as` a reader is a page of the applications' side, on the data
    // plane, in that reader's session; the others are the console's.
    const shotPage = s.as ? await readerPage(browser, s.as) : page;
    if (s.as) {
      await shotPage.goto(DATA + path);
    } else {
      await page.goto(ADMIN + '#' + path).catch(() => {});
      await page.goto(ADMIN + path);
    }
    // The console animates drawers in; a frame taken mid-slide is a blurred
    // panel nobody can read.
    await shotPage.waitForTimeout(s.settle ?? 1200);
    if (s.act) {
      try {
        await s.act(shotPage);
      } catch (e) {
        log('! could not open what', s.file, 'shows:', e.message.split('\n')[0]);
      }
    }
    // Nothing of the machine the capture ran on: its name (the logs screen
    // says which node answered) and the path of the disposable data
    // directory (the database screen prints it) are a stranger's details in
    // a public image. Replaced in the page, in the text people would read.
    // So is the internal plane's port, moved here out of the way of a dev
    // gateway: the pages say 7070, the address a service really calls.
    await shotPage.evaluate(([host, data]) => {
      const clean = (v) => v.split(data).join('/data').split(host).join('gateway-1').split(':17074').join(':7070');
      // Shadow roots too: a code snippet draws its text inside one.
      const roots = [document.body];
      while (roots.length) {
        const root = roots.pop();
        const walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
        for (let n = walk.nextNode(); n; n = walk.nextNode()) {
          if (n.nodeType === Node.TEXT_NODE) {
            if (n.nodeValue !== clean(n.nodeValue)) n.nodeValue = clean(n.nodeValue);
          } else {
            if (n.shadowRoot) roots.push(n.shadowRoot);
            if ((n.tagName === 'TEXTAREA' || n.tagName === 'INPUT') && n.value) n.value = clean(n.value);
          }
        }
      }
    }, [hostname(), DATA_DIR]).catch(() => {});
    // And nothing is left FOCUSED: a stray focus ring in a published image
    // reads as a state the reader is meant to notice, and it is not one.
    await shotPage.evaluate(() => document.activeElement?.blur?.());
    await shotPage.waitForTimeout(150);
    const png = `${tmp}/png/${s.file}.png`;
    await shotPage.screenshot({ path: png });
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
  await waitFor(ADMIN + '/healthz');
  const state = await seed();
  await shoot(state);
  log('done ->', out);
} finally {
  gw.kill();
}
