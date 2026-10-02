// Put the demo data the documentation screenshots show into a fresh instance.
//
// screenshots.json says where every picture comes from; this is the other half
// of that sentence - WHAT is on screen. Acme Corp, five routes, a role tree,
// six accounts, three groups, a vault, a portal, two certificates, an order
// service with a deposited OpenAPI spec and rules on some of its operations,
// a scheduled call, an OpenTelemetry collector, and a few minutes of traffic
// so the charts are curves. Everything goes through the admin API, the way an
// administrator would do it; nothing touches the database directly.
//
//   node docs/scripts/screenshots-seed.mjs                    seed the instance in screenshots.json
//   node docs/scripts/screenshots-seed.mjs --instance single  seed a per-shot instance (`instances`)
//   node docs/scripts/screenshots-seed.mjs --traffic 120      seed, then send traffic for 120 s
//   node docs/scripts/screenshots-seed.mjs --traffic-only 600 only traffic, on a seeded instance
//
// Run the seed ONCE on an empty data directory: it creates, it does not
// reconcile. The upstream the routes point at must be up first
// (screenshots-upstream.mjs). The Metrics screen shows the last minute only,
// which is why screenshots.mjs starts a --traffic-only run of its own while it
// shoots the shots that need one.
import { readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const manifest = JSON.parse(await readFile(join(root, 'screenshots.json'), 'utf8'));
const args = process.argv.slice(2);
const flag = (name, fallback) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : fallback;
};
const which = flag('--instance', '');
const instance = which ? { ...manifest.instance, ...(manifest.instances || {})[which] } : manifest.instance;
const trafficOnly = args.includes('--traffic-only');
const trafficSeconds = Number(trafficOnly ? flag('--traffic-only', '600') : flag('--traffic', '0'));
const ADMIN = instance.admin;
const DATA = instance.data;
const [ROOT_USER, ROOT_PASSWORD] = instance.signIn.split(' / ');
// The accounts' own password, once they have replaced the one-time one. A
// throwaway instance on loopback, like the root one in screenshots.json.
const USER_PASSWORD = 'test1234';

let request;
try {
  const require = createRequire(join(root, '..', 'e2e', 'package.json'));
  ({ request } = require('playwright'));
} catch {
  console.error('playwright comes from e2e/: run `npm install` there first');
  process.exit(1);
}

const log = (...a) => console.log('[seed]', ...a);
async function ok(res, what) {
  if (res.status() >= 300 && res.status() !== 303) {
    throw new Error(`${what} -> ${res.status()} ${await res.text()}`);
  }
  const text = await res.text();
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

// ---- the data -------------------------------------------------------------

const UPSTREAM = 'http://127.0.0.1:8099';

// A mark for Acme: a rounded square, a peak. Plain SVG, so it scales anywhere
// the pages put it.
const LOGO =
  'data:image/svg+xml;base64,' +
  Buffer.from(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="#1a7f9c"/>' +
      '<path d="M32 15 L49 45 H15 Z" fill="none" stroke="#fff" stroke-width="5" stroke-linejoin="round"/>' +
      '<circle cx="32" cy="45" r="4.5" fill="#fff"/></svg>',
  ).toString('base64');

// A parent IMPLIES its children (docs/access/roles): holding staff satisfies a
// rule that asks for any role below it. Parents before children: a child
// names its parent by id.
const ROLES = [
  { key: 'staff', name: 'staff', description: 'Every Acme employee: everything below', tags: ['core'] },
  { key: 'orders-manager', parent: 'staff', name: 'orders-manager', description: 'Creates, changes and cancels orders', tags: ['orders'] },
  { key: 'orders-viewer', parent: 'orders-manager', name: 'orders-viewer', description: 'Reads orders and shipments', tags: ['orders'] },
  { key: 'finance', parent: 'staff', name: 'finance', description: 'Refunds, invoices and exports', tags: ['orders', 'billing'] },
  { key: 'billing-admin', parent: 'finance', name: 'billing-admin', description: 'Issues and corrects invoices', tags: ['billing'] },
  { key: 'billing-viewer', parent: 'billing-admin', name: 'billing-viewer', description: 'Reads invoices', tags: ['billing'] },
  { key: 'warehouse', parent: 'staff', name: 'warehouse', description: 'Counts stock and ships parcels', tags: ['inventory'] },
  { key: 'partner', name: 'partner', description: 'External partners, read-only', tags: ['external'] },
  { key: 'auditor', name: 'auditor', description: 'Reads the trail and the exports, changes nothing', tags: ['compliance'] },
];

const USERS = [
  { username: 'd.okonkwo', fullname: 'Daniel Okonkwo', email: 'daniel.okonkwo@acme.example', groups: ['Order desk'] },
  { username: 'l.moreau', fullname: 'Lucie Moreau', email: 'lucie.moreau@acme.example', appAdmin: true, type: 'ADMIN', groups: ['Finance'] },
  { username: 'm.haddad', fullname: 'Mariam Haddad', email: 'mariam.haddad@acme.example', groups: ['Warehouse'] },
  { username: 's.oliveira', fullname: 'Sofia Oliveira', email: 'sofia.oliveira@acme.example', groups: ['Order desk', 'Finance'] },
  { username: 't.nakamura', fullname: 'Takeshi Nakamura', email: 'takeshi.nakamura@acme.example', infraAdmin: true, groups: ['Warehouse'] },
];

const GROUPS = [
  { name: 'Order desk', description: 'Takes and follows up orders', roles: ['orders-manager', 'billing-viewer'] },
  { name: 'Warehouse', description: 'Picks, packs and ships', roles: ['warehouse', 'orders-viewer'] },
  { name: 'Finance', description: 'Invoices, refunds, exports', roles: ['finance', 'orders-viewer'] },
];

const ROUTES = [
  {
    id: 'billing', name: 'Billing', order: 1, enabled: true, isUi: true,
    upstream: 'http://billing.svc.acme.internal:8080',
    access: { level: 'auth' },
    predicates: [{ type: 'path', args: { patterns: ['/billing/**'] } }],
    filters: [
      { type: 'strip-prefix', args: { parts: 1, announcePrefix: true } },
      { type: 'set-request-header', args: { name: 'X-Acme-Channel', value: 'gateway' } },
      { type: 'set-request-header', args: { name: 'X-Billing-Key', value: '${billing-api-key}' } },
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
    locales: { mechanism: 'query', param: 'lang', speaks: ['en', 'fr'] },
  },
  {
    id: 'docs-portal', name: 'Docs portal', order: 2, enabled: true, isUi: true,
    upstream: UPSTREAM,
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/docs/**'] } }],
    filters: [{ type: 'strip-prefix', args: { parts: 1, announcePrefix: true } }],
    ui: { userButton: { enabled: true } },
    locales: { mechanism: 'accept', speaks: ['en', 'fr', 'de'] },
  },
  {
    id: 'orders-api', name: 'Orders API', order: 3, enabled: true,
    upstream: UPSTREAM,
    access: { level: 'auth', roles: ['orders-viewer', 'partner'], users: ['t.nakamura'] },
    timeouts: { connect: 'PT1S', response: 'PT20S' },
    breaker: { enabled: true, trip: 5, cool: 'PT1M' },
    predicates: [
      { type: 'path', args: { patterns: ['/api/**'] } },
      { type: 'method', args: { methods: ['GET', 'POST', 'PUT', 'DELETE'] } },
      { type: 'header', args: { name: 'X-Acme-Api-Version', values: ['2024-11', '2025-06'] } },
    ],
    filters: [
      { type: 'strip-prefix', args: { parts: 1, announcePrefix: true } },
      { type: 'set-request-header', args: { name: 'X-Acme-Channel', value: 'gateway' } },
      { type: 'set-request-header', args: { name: 'X-Acme-Tier', value: 'internal' } },
      { type: 'set-request-header', args: { name: 'X-Orders-Key', value: '${orders-api-key}' } },
      { type: 'set-response-header', args: { name: 'Cache-Control', value: 'no-store' } },
    ],
    limits: [
      { per: 'user', requests: 600, window: 'PT1M' },
      { per: 'ip', requests: 60, window: 'PT1S' },
    ],
    identity: {
      mechanism: 'headers',
      attributes: [
        { field: 'username', as: 'X-Acme-User' },
        { field: 'roles', as: 'X-Acme-Roles' },
      ],
    },
  },
  {
    id: 'inventory', name: 'Inventory (maintenance)', order: 4, enabled: true, isUi: true,
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/inventory/**'] } }],
    filters: [{ type: 'maintenance', args: { reason: 'upgrade' } }],
    ui: { userButton: { enabled: true } },
  },
  {
    id: 'catch-all', name: 'Catch-all', order: 5, enabled: true,
    upstream: '${www-origin}',
    access: {},
    predicates: [{ type: 'path', args: { patterns: ['/**'] } }],
    filters: [],
  },
];

// The order service's contract: fourteen operations. Deposited on the route
// rather than fetched, so the inventory does not depend on the upstream.
const op = (summary, tag, extra = {}) => ({ summary, tags: [tag], responses: { 200: { description: 'OK' } }, ...extra });
const idParam = [{ name: 'id', in: 'path', required: true, schema: { type: 'string' } }];
const SPEC = {
  openapi: '3.0.3',
  info: { title: 'Acme Orders', version: '2025-06' },
  paths: {
    '/orders': { get: op('List orders', 'orders'), post: op('Create an order', 'orders') },
    '/orders/export': { get: op('Export orders as CSV', 'orders') },
    '/orders/reindex': { post: op('Rebuild the search index', 'admin') },
    '/orders/{id}': {
      parameters: idParam,
      get: op('Read an order', 'orders'),
      put: op('Change an order', 'orders'),
      delete: op('Cancel an order', 'orders'),
    },
    '/orders/{id}/lines': { parameters: idParam, get: op('List the lines of an order', 'orders'), post: op('Add a line', 'orders') },
    '/orders/{id}/refund': { parameters: idParam, post: op('Refund an order', 'billing') },
    '/shipments': { get: op('List shipments', 'shipments'), post: op('Book a shipment', 'shipments') },
    '/shipments/{id}': { parameters: idParam, get: op('Read a shipment', 'shipments') },
    '/shipments/{id}/tracking': { parameters: idParam, get: op('Track a shipment', 'shipments') },
  },
};

// Who may call what, operation by operation, and how often (RBAC-07, QUOTA-05).
const SECURITY = {
  access: {},
  denyUnlisted: false,
  endpoints: [
    { method: 'POST', path: '/orders/reindex', level: 'deny', users: ['t.nakamura'] },
    { method: 'DELETE', path: '/orders/{id}', level: 'auth', roles: ['orders-manager'] },
    { method: 'POST', path: '/orders/{id}/refund', level: 'auth', roles: ['finance'],
      limits: [{ per: 'user', requests: 20, window: 'PT1H' }] },
    { method: 'GET', path: '/orders/export', inherit: true,
      limits: [{ per: 'user', requests: 10, window: 'PT1H' }, { per: 'route', requests: 100, window: 'PT1H' }] },
    { method: 'POST', path: '/orders', inherit: true,
      limits: [{ per: 'user', requests: 60, window: 'PT1M' }] },
  ],
};

// Which calls leave a trace in the trail (AUD-04).
const AUDIT = {
  endpoints: [
    {
      method: 'POST', path: '/orders/{id}/refund', description: 'Refund an order',
      fields: [{ name: 'order', from: 'path', key: 'id' }, { name: 'reason', from: 'body', key: '/reason' }],
      body: true,
    },
    {
      method: 'DELETE', path: '/orders/{id}', description: 'Cancel an order',
      fields: [{ name: 'order', from: 'path', key: 'id' }],
    },
  ],
};

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

// ---- the seed -------------------------------------------------------------

// A browser's name rather than Playwright's: the Sessions screen shows it.
// Not for a traffic run, which has nothing to administer: a console sign-in
// is a line in the audit trail.
const api = trafficOnly ? null : await request.newContext({ baseURL: ADMIN, extraHTTPHeaders: { 'user-agent': agentFor('admin') } });
if (api) {
  const login = await api.post('/login', { form: { username: ROOT_USER, password: ROOT_PASSWORD }, maxRedirects: 0 });
  if (login.status() !== 303) throw new Error(`admin login answered ${login.status()}: is the instance up at ${ADMIN}?`);
  log('signed in on', ADMIN);
}

async function seed() {

  const edition = await ok(await api.get('/api/edition'), 'edition');
  if (!edition.enterprise) throw new Error('this is not the Enterprise image: build with -tags ee');
  const tenant = edition.primaryTenant;

  if ((await ok(await api.get('/api/routes'), 'routes')).length) {
    throw new Error('this instance already has routes: the seed runs once, on an empty data directory');
  }

  // The organisation every installation starts with, named.
  const t = await ok(await api.get(`/api/tenants/${tenant}`), 'tenant');
  await ok(await api.put(`/api/tenants/${tenant}`, { data: { ...t, name: 'Acme Corp', description: 'Head office and the Lyon warehouse' } }), 'tenant name');

  // Branding.
  const branding = await ok(await api.get('/api/branding'), 'branding');
  await ok(await api.put('/api/branding', {
    data: { ...branding, appName: 'Acme Corp', tagline: 'Acme applications, one sign-in', logo: LOGO },
  }), 'branding');
  log('Acme Corp, branded');

  // The vault, before the routes that reference it.
  const VAULT = [
    { scope: 'infra', name: 'orders-api-key', kind: 'secret', value: 'demo-orders-key', description: 'Key the order service expects from the gateway' },
    { scope: 'infra', name: 'billing-api-key', kind: 'secret', value: 'demo-billing-key', description: 'Key the billing service expects' },
    { scope: 'infra', name: 'www-origin', kind: 'value', value: 'http://www.svc.acme.internal:80', description: 'Where the public site is served from' },
  ];
  for (const v of VAULT) {
    await ok(await api.put(`/api/vault/${v.scope}/${v.name}`, { data: { kind: v.kind, value: v.value, description: v.description } }), `vault ${v.name}`);
  }
  log(`${VAULT.length} vault entries`);

  // Roles.
  const roleIds = {};
  for (const r of ROLES) {
    const saved = await ok(await api.post('/api/roles', {
      data: { name: r.name, description: r.description, parentId: r.parent ? roleIds[r.parent] : '', tags: r.tags },
    }), `role ${r.name}`);
    roleIds[r.key] = saved.id;
  }
  log(`${ROLES.length} roles`);

  // Routes.
  for (const rt of ROUTES) await ok(await api.put(`/api/routes/${rt.id}`, { data: rt }), `route ${rt.id}`);
  log(`${ROUTES.length} routes`);

  // The order service's spec, and its per-operation rules.
  await ok(await api.put('/api/routes/orders-api/spec?filename=openapi.json', {
    headers: { 'content-type': 'application/json' },
    data: JSON.stringify(SPEC),
  }), 'spec');
  await ok(await api.put('/api/routes/orders-api/security', { data: SECURITY }), 'endpoint security');
  await ok(await api.put('/api/routes/orders-api/audit', { data: AUDIT }), 'endpoint audit');
  log('orders-api: spec, 5 endpoint rules, 2 audited operations');

  // Groups.
  const groupIds = {};
  for (const g of GROUPS) {
    const saved = await ok(await api.post(`/api/tenants/${tenant}/groups`, {
      data: { name: g.name, description: g.description, roleIds: g.roles.map((k) => roleIds[k]) },
    }), `group ${g.name}`);
    groupIds[g.name] = saved.id;
  }
  log(`${GROUPS.length} groups`);

  // Accounts: created by root, each one signs in on the data plane with its
  // one-time password and sets its own - which is what makes the sessions,
  // the last connections and the members screen real.
  const accounts = [];
  for (const u of USERS) {
    const created = await ok(await api.post('/api/users', {
      data: {
        username: u.username, fullname: u.fullname, email: u.email, enabled: true,
        infraAdmin: !!u.infraAdmin, appAdmin: !!u.appAdmin, dev: !!u.dev,
      },
    }), `user ${u.username}`);
    const id = created.user.id;
    await ok(await api.put(`/api/tenants/${tenant}/members/${id}`, {
      data: { userId: id, tenantId: tenant, type: u.type || 'USER', enabled: true, businessAccess: { inherited: true }, sessionTTL: '' },
    }), `member ${u.username}`);
    await ok(await api.put(`/api/tenants/${tenant}/members/${id}/groups`, { data: u.groups.map((g) => groupIds[g]) }), `groups of ${u.username}`);

    const data = await request.newContext({ baseURL: DATA, extraHTTPHeaders: { 'user-agent': agentFor(u.username) } });
    const first = await data.post('/login', { form: { username: u.username, password: created.password }, maxRedirects: 0 });
    if (first.status() !== 303) throw new Error(`first sign-in of ${u.username} answered ${first.status()}`);
    const changed = await data.post('/update-password', { form: { password: USER_PASSWORD, confirm: USER_PASSWORD }, maxRedirects: 0 });
    if (changed.status() !== 303) throw new Error(`password change of ${u.username} answered ${changed.status()}: ${await changed.text()}`);
    // And once more with the new password: the forced change does not stamp
    // the last connection, which the Members screen shows.
    await data.post('/logout', { maxRedirects: 0 });
    const again = await data.post('/login', { form: { username: u.username, password: USER_PASSWORD }, maxRedirects: 0 });
    if (again.status() !== 303) throw new Error(`sign-in of ${u.username} answered ${again.status()}`);
    accounts.push({ ...u, id, data });
  }
  log(`${USERS.length} accounts, signed in on the data plane`);

  // The portal, the TLS names and two certificates.
  const settings = await ok(await api.get('/api/settings'), 'settings');
  await ok(await api.put('/api/settings', { data: { ...settings, portal: PORTAL } }), 'portal');
  for (const c of [
    { plane: 'console', host: 'console.acme.example' },
    { plane: 'app', host: 'apps.acme.example' },
  ]) {
    await ok(await api.post('/api/certificates/self-signed', { data: { ...c, days: 365 } }), `certificate ${c.host}`);
  }
  const tls = await ok(await api.get('/api/settings/tls'), 'tls');
  await ok(await api.put('/api/settings/tls', {
    data: { consoleName: 'console.acme.example', appNames: ['apps.acme.example', 'docs.acme.example'], redirect: false, hstsMaxAge: tls.hstsMaxAge || 0, acme: tls.acme },
  }), 'tls names');
  log('portal, TLS names, 2 certificates');

  // OpenTelemetry: a collector address, as a compose file would name it.
  const otel = await ok(await api.get('/api/settings/telemetry'), 'telemetry');
  await ok(await api.put('/api/settings/telemetry', {
    data: {
      enabled: true, traces: true, metrics: true, endpoint: 'http://opentelemetry:4318',
      headers: otel.headers, sample: 0.25, maxPerSecond: 200,
      gatewayDetail: true, caller: true, logs: false, logsPush: false, audit: true, auditConsole: false,
    },
  }), 'telemetry');
  log('OpenTelemetry collector');

  // The agent endpoint and the audit trail's retention.
  await ok(await api.put('/api/settings/agent', { data: { enabled: true } }), 'agent');
  await ok(await api.put('/api/settings/audit', { data: { retentionDays: 365, choices: [] } }), 'audit retention');

  // Scheduled calls: a cadence, a cron line and one date. They call as
  // "meerkat" with a role, which is what the route's sign-in rule reads.
  const v = { 'X-Acme-Api-Version': '2025-06' };
  // From the next full hour: a first turn NOW would run before the picture is
  // taken, and a scheduled call is refused on a route carrying endpoint rules
  // (requireSession does not read the scheduled caller - reported, not fixed
  // here), which would put a red "failed" on the Scheduler screen.
  const nextHour = Math.ceil(Date.now() / 3600000) * 3600 + 3600;
  for (const sc of [
    { name: 'Hourly order intake', routeId: 'orders-api', method: 'POST', path: '/api/orders', every: 'PT1H', startAt: nextHour, role: 'orders-manager', headers: v, body: { source: 'edi' }, metadata: { channel: 'edi' } },
    { name: 'Nightly carrier sync', routeId: 'orders-api', method: 'POST', path: '/api/shipments', cron: '0 3 * * *', timezone: 'Europe/Paris', role: 'orders-manager', headers: v, body: { carrier: 'dhl' }, metadata: { carrier: 'dhl' } },
    { name: 'Close the year-end order', routeId: 'orders-api', method: 'PUT', path: '/api/orders/ORD-8819', at: '2026-12-31T18:00:00+01:00', role: 'orders-manager', headers: v, body: { status: 'closed' }, metadata: { order: 'ORD-8819' } },
  ]) {
    await ok(await api.post('/api/schedules', { data: { overlap: 'skip', ...sc } }), `schedule ${sc.name}`);
  }
  log('3 schedules');

  // Control-plane tokens, each with its own perimeter.
  for (const tok of [
    { name: 'ci-deploy', days: 90, scope: 'full', from: '10.20.0.0/16' },
    { name: 'grafana-readonly', days: 365, scope: 'readonly', from: '' },
    { name: 'batch-scheduler', days: 0, scope: 'schedules', from: '' },
  ]) {
    await ok(await api.post('/api/admin-tokens', { data: tok }), `token ${tok.name}`);
  }
  log('3 control-plane tokens');

  // The developer tunnel's public address, left switched off: opening it
  // would open a port on the machine taking the pictures.
  await ok(await api.put('/api/settings/plug', { data: { enabled: false, host: 'plug.acme.example', port: 30022 } }), 'plug');

  // Personal tokens, made the way their owners make them: on their profile.
  for (const [who, name, days] of [['d.okonkwo', 'orders-cli', '90'], ['s.oliveira', 'finance-export', '30']]) {
    const a = accounts.find((x) => x.username === who);
    const res = await a.data.post('/profile/tokens', { form: { action: 'create', name, days }, maxRedirects: 0 });
    if (res.status() >= 400) throw new Error(`token ${name} of ${who} answered ${res.status()}`);
  }
  log('2 application tokens');

  // A few changes made after the fact, so the audit trail shows what an
  // update looks like - a field before and after - and not only creations.
  // They are the ones docs/console/audit-and-issues describes under its
  // picture: two routes whose rate limits were written, a group renamed, a
  // tagline rewritten and a session TTL shortened (then the two captures).
  const st = await ok(await api.get('/api/settings'), 'settings');
  await ok(await api.put('/api/settings', { data: { ...st, sessionTTL: 'PT20M' } }), 'session TTL');
  const br = await ok(await api.get('/api/branding'), 'branding');
  await ok(await api.put('/api/branding', { data: { ...br, tagline: 'Every Acme application behind one sign-in' } }), 'tagline');
  const groups = await ok(await api.get(`/api/tenants/${tenant}/groups`), 'groups');
  const desk = groups.find((g) => g.name === 'Order desk');
  await ok(await api.put(`/api/tenants/${tenant}/groups/${desk.id}`, { data: { ...desk, name: 'Front desk' } }), 'group rename');
  const orders = await ok(await api.get('/api/routes/orders-api'), 'orders-api');
  await ok(await api.put('/api/routes/orders-api', {
    data: { ...orders, limits: [{ per: 'user', requests: 600, window: 'PT1M' }, { per: 'ip', requests: 120, window: 'PT1S' }] },
  }), 'orders-api limits');
  const billing = await ok(await api.get('/api/routes/billing'), 'billing');
  await ok(await api.put('/api/routes/billing', { data: { ...billing, limits: [{ per: 'user', requests: 240, window: 'PT1M' }] } }), 'billing limits');
  log('5 updates for the trail');

  // Two saved configurations beside the running one - last, so the running
  // one is not marked as changed since.
  for (const c of [
    { name: 'Before the portal', description: 'Routes and roles, no portal bar yet' },
    { name: 'Known-good baseline', description: 'What went live on 14 April' },
  ]) {
    await ok(await api.post('/api/configurations', { data: c }), `configuration ${c.name}`);
  }
  log('2 saved configurations');

  await saveSessions(accounts);
  return accounts;
}

// The accounts the seed created, signed in again - for a traffic run on its own.
// The accounts' sessions are kept in a file beside the system's temporary
// files, so a traffic run reuses them instead of signing in again: every
// sign-in is a line in the audit trail and a row on the Sessions screen, and
// the pictures of those two screens should not be full of our own traffic.
const sessionsFile = join(tmpdir(), `meerkat-doc-sessions-${new URL(DATA).port}.json`);

async function saveSessions(accounts) {
  const states = {};
  for (const a of accounts) states[a.username] = await a.data.storageState();
  await writeFile(sessionsFile, JSON.stringify(states));
}

async function signIn() {
  let states = {};
  try {
    states = JSON.parse(await readFile(sessionsFile, 'utf8'));
  } catch {
    /* none kept: sign in again */
  }
  const out = [];
  for (const u of USERS) {
    if (states[u.username]) {
      const data = await request.newContext({
        baseURL: DATA, storageState: states[u.username], extraHTTPHeaders: { 'user-agent': agentFor(u.username) },
      });
      const probe = await data.get('/api/orders', { headers: { 'X-Acme-Api-Version': '2025-06' }, maxRedirects: 0 });
      if (probe.status() !== 401) {
        out.push({ ...u, data });
        continue;
      }
      await data.dispose();
    }
    const data = await request.newContext({ baseURL: DATA, extraHTTPHeaders: { 'user-agent': agentFor(u.username) } });
    const res = await data.post('/login', { form: { username: u.username, password: USER_PASSWORD }, maxRedirects: 0 });
    if (res.status() !== 303) throw new Error(`sign-in of ${u.username} answered ${res.status()}: was the instance seeded?`);
    out.push({ ...u, data });
  }
  return out;
}

// ---- traffic --------------------------------------------------------------

function agentFor(name) {
  const agents = [
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0 Safari/537.36',
    'Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0',
  ];
  return agents[[...name].reduce((n, c) => n + c.charCodeAt(0), 0) % agents.length];
}

async function traffic(accounts, seconds) {
  log(`traffic for ${seconds}s`);
  const v = { 'X-Acme-Api-Version': '2025-06' };
  // Mostly answers, some refusals and failures - the proportions of a working
  // day rather than of a test: a missing order (404), a bound reached (429), a
  // refund without the finance role (403), a route under maintenance (503),
  // and two services that do not exist on this machine (502).
  const weighted = [
    [8, 'get', '/api/orders', v], [5, 'get', '/api/orders/ORD-8814', v], [3, 'get', '/api/orders/ORD-8816/lines', v],
    [4, 'get', '/api/shipments', v], [3, 'get', '/api/shipments/SHP-310/tracking', v], [2, 'post', '/api/orders', v],
    [1, 'get', '/api/orders/ORD-0001', v], [1, 'get', '/api/orders/export', v], [1, 'post', '/api/orders/ORD-8817/refund', v],
    [5, 'get', '/docs/', {}], [1, 'get', '/billing/', {}], [1, 'get', '/inventory/', {}], [1, 'get', '/about', {}],
  ];
  const calls = weighted.flatMap(([n, ...call]) => Array.from({ length: n }, () => call));
  const until = Date.now() + seconds * 1000;
  let sent = 0;
  while (Date.now() < until) {
    // Bursts and lulls, so the curve has a shape.
    const phase = Math.sin((Date.now() / 1000 / 40) * Math.PI);
    const burst = 4 + Math.round(8 * (phase + 1));
    await Promise.all(
      Array.from({ length: burst }, () => {
        const a = accounts[Math.floor(Math.random() * accounts.length)];
        const [method, path, headers] = calls[Math.floor(Math.random() * calls.length)];
        return a.data[method](path, { headers, maxRedirects: 0, data: method === 'post' ? { reason: 'damaged parcel' } : undefined }).catch(() => null);
      }),
    );
    sent += burst;
    await new Promise((r) => setTimeout(r, 1000));
  }
  log(`${sent} requests sent`);
}

const accounts = trafficOnly ? await signIn() : await seed();
if (trafficSeconds > 0) await traffic(accounts, trafficSeconds);
for (const a of accounts) await a.data.dispose();
await api?.dispose();
log('done');
