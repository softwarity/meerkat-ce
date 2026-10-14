import { expect, request, test as setup, type APIRequestContext } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL, ROOT_PASSWORD } from '../playwright.config';
import { AUTH_DIR, authDataFile, authFile, ENTERPRISE, type Seeded } from '../lib/fixtures';

// Seeds the five profiles THROUGH THE REAL FLOWS (no store shortcut): root
// creates the users over the admin API, each user signs in with its one-time
// password and completes the forced update-password step, and the resulting
// session is saved as that profile's storage state. Cookies are host-scoped
// (localhost), so one state serves both planes.

const USER_PASSWORD = 'e2e-User-Password-1';

// Redirects are never followed here: a successful step answers 303 and the
// landing target may be an app route (the data plane's catch-all) that has no
// reachable upstream in the test environment.
async function login(ctx: APIRequestContext, username: string, password: string): Promise<void> {
  const res = await ctx.post('/login', { form: { username, password }, maxRedirects: 0 });
  expect(res.status(), `login ${username}`).toBe(303);
}

// First login with a one-time password lands on the forced password change.
async function loginFirstTime(ctx: APIRequestContext, username: string, oneTime: string): Promise<void> {
  await login(ctx, username, oneTime);
  const res = await ctx.post('/update-password', {
    form: { password: USER_PASSWORD, confirm: USER_PASSWORD },
    maxRedirects: 0,
  });
  expect(res.status(), `update-password ${username}`).toBe(303);
  const me = await ctx.get('/api/me');
  expect(me.ok(), `session incomplete for ${username}`).toBeTruthy();
}

async function organisation(root: APIRequestContext, name = 'acme'): Promise<string> {
  if (ENTERPRISE) {
    const res = await root.post('/api/tenants', { data: { name } });
    expect(res.status(), await res.text()).toBe(201);
    return ((await res.json()) as { id: string }).id;
  }
  const res = await root.get('/api/edition');
  expect(res.ok(), await res.text()).toBeTruthy();
  const primary = ((await res.json()) as { primaryTenant: string }).primaryTenant;
  expect(primary, 'a single installation still serves one organisation').toBeTruthy();
  return primary;
}

const HTTPBIN = HTTPBIN_URL;

// What an organisation holds besides its people, so that every tenant-scoped
// read has something to show and every cross-organisation probe something to
// refuse (TENANT-06): a group its members are in, and a secret of its own in
// the vault.
async function furnish(
  root: APIRequestContext,
  tenantId: string,
  name: string,
  members: Array<{ id: string; type: 'ADMIN' | 'USER' }>,
): Promise<Seeded['tenants'][string]> {
  for (const m of members) {
    const res = await root.put(`/api/tenants/${tenantId}/members/${m.id}`, {
      data: { userId: m.id, tenantId, type: m.type, enabled: true, businessAccess: { inherited: true }, sessionTTL: '' },
    });
    expect(res.ok(), `member of ${name}: ${await res.text()}`).toBeTruthy();
  }
  const group = await root.post(`/api/tenants/${tenantId}/groups`, {
    data: { name: `${name}-staff`, description: `Everyone at ${name}`, roles: [] },
  });
  expect(group.ok(), `group of ${name}: ${await group.text()}`).toBeTruthy();
  const groupId = ((await group.json()) as { id: string }).id;
  for (const m of members) {
    const res = await root.put(`/api/tenants/${tenantId}/members/${m.id}/groups`, { data: [groupId] });
    expect(res.ok(), `groups of a member of ${name}: ${await res.text()}`).toBeTruthy();
  }
  return {
    id: tenantId,
    admin: members.find((m) => m.type === 'ADMIN')!.id,
    members: members.map((m) => m.id),
    groups: [{ id: groupId, name: `${name}-staff` }],
    vault: [],
  };
}

// A secret of the organisation's own, written by its administrator: a tenant
// scope belongs to whoever administers that organisation, not to root (who
// writes the global ones).
async function stashTenantSecret(admin: APIRequestContext, tenant: Seeded['tenants'][string], name: string): Promise<void> {
  const secret = `${name}-api-key`;
  const res = await admin.put(`/api/vault/${encodeURIComponent(`tenant:${tenant.id}`)}/${secret}`, {
    data: { kind: 'secret', value: `${name}-secret-value` },
  });
  expect(res.ok(), `vault of ${name}: ${await res.text()}`).toBeTruthy();
  tenant.vault.push(secret);
}

async function seedRoutes(root: APIRequestContext): Promise<void> {
  const path = (p: string) => [{ type: 'path', args: { patterns: [p] } }];
  const strip = [{ type: 'strip-prefix', args: { parts: 1 } }];
  const routes = [
    { id: 'demo', name: 'demo', order: 100, enabled: true, isUi: true, upstream: HTTPBIN,
      predicates: path('/demo/**'), filters: strip, access: {} },
    { id: 'demo-secure', name: 'demo-secure', order: 101, enabled: true, isUi: true, upstream: HTTPBIN,
      predicates: path('/secure/**'), filters: strip, access: { level: 'auth' } },
    { id: 'trap', name: 'trap', order: 900, enabled: true, upstream: HTTPBIN,
      predicates: path('/**'), filters: [], access: {} },
  ];
  for (const r of routes) {
    const res = await root.put(`/api/routes/${r.id}`, { data: r });
    expect(res.ok(), `route ${r.id}: ${await res.text()}`).toBeTruthy();
  }
  // And the portal that offers the two applications (PORTAL-03): the apps
  // menu of the built-in pages lists them.
  const settings = await (await root.get('/api/settings')).json();
  settings.portal = {
    ...(settings.portal ?? {}),
    mode: 'links',
    entries: [{ routeId: 'demo', label: 'Demo' }, { routeId: 'demo-secure', label: 'Demo (secure)' }],
  };
  const saved = await root.put('/api/settings', { data: settings });
  expect(saved.ok(), `portal: ${await saved.text()}`).toBeTruthy();
}

setup('seed profiles and tenant', async () => {
  mkdirSync(AUTH_DIR, { recursive: true });

  const root = await request.newContext({ baseURL: ADMIN_URL });
  await login(root, 'admin', ROOT_PASSWORD);
  expect((await root.get('/api/me')).ok()).toBeTruthy();

  // The issue tracker (ISSUE-04) ships OFF; the seed flips it the way an infra
  // admin would on the Issues screen, so the matrix exercises /api/issues.
  expect((await root.put('/api/settings/issues', { data: { enabled: true } })).ok()).toBeTruthy();

  // The agent endpoint (MCP-01) ships OFF too, and the two scenarios that
  // probe it were written against a gateway where somebody had turned it on.
  // Left off, /mcp and /oauth/authorize answer 404 to EVERYONE - which is the
  // right product behaviour and makes the access matrix meaningless: "who may
  // reach this" has no answer while the door does not exist. Root flips it
  // here the way an administrator would on the MCP screen.
  expect((await root.put('/api/settings/agent', { data: { enabled: true } })).ok()).toBeTruthy();

  // The routes the tests exercise. A gateway starts EMPTY - demonstration
  // routes were ours, not an operator's - so the suite lays down its own,
  // through the API an administrator would use: a public UI at /demo, an
  // authenticated one at /secure, and the /** catch-all ("trap") ordered last.
  // All of them answer from the local httpbin the suite runs.
  await seedRoutes(root);

  const create = async (user: Record<string, unknown>) => {
    const res = await root.post('/api/users', { data: user });
    expect(res.status(), `create ${user.username}: ${await res.text()}`).toBe(201);
    const body = (await res.json()) as { user: { id: string }; password: string };
    return { id: body.user.id, oneTime: body.password };
  };

  const gwadmin = await create({ username: 'gwadmin', enabled: true, infraAdmin: true });
  const appadmin = await create({ username: 'appadmin', enabled: true, appAdmin: true });
  const tadmin = await create({ username: 'tadmin', enabled: true });
  const alice = await create({ username: 'alice', enabled: true, fullname: 'Alice Martin' });

  // The organisation tadmin administers and alice belongs to.
  //
  // On the Enterprise build it is created for the occasion. On the community
  // one a second organisation is refused BY DESIGN, so the profiles are seeded
  // into the one every installation already serves - the same memberships, the
  // same matrix, on the shape that build can serve.
  const tenantId = await organisation(root);
  const tenants: Seeded['tenants'] = {
    acme: await furnish(root, tenantId, 'acme', [
      { id: tadmin.id, type: 'ADMIN' },
      { id: alice.id, type: 'USER' },
    ]),
  };

  // A SECOND organisation, with its own administrator, member, group and
  // secret: what proves that one organisation's administrator reaches nothing
  // of another's (TENANT-06). The community build serves one organisation BY
  // DESIGN, so it has none. Its people are not profiles of scenarios.json -
  // the access matrix stays the five it was written for - but they are signed
  // in and saved like them, for the specs that need them.
  const extra: Array<[string, { id: string; oneTime: string; username: string }]> = [];
  if (ENTERPRISE) {
    const gadmin = { ...(await create({ username: 'gadmin', enabled: true })), username: 'gadmin' };
    const bob = { ...(await create({ username: 'bob', enabled: true, fullname: 'Bob Durand' })), username: 'bob' };
    tenants.globex = await furnish(root, await organisation(root, 'globex'), 'globex', [
      { id: gadmin.id, type: 'ADMIN' },
      { id: bob.id, type: 'USER' },
    ]);
    extra.push(['globex-admin', gadmin], ['globex-user', bob]);
  }

  await root.storageState({ path: authFile('root') });

  // Each plane keeps its own session cookie: sign the profile in on the DATA
  // plane too and save that state separately (flow-page suites use it).
  const dataLogin = async (profile: string, username: string, password: string) => {
    const ctx = await request.newContext({ baseURL: DATA_URL });
    await login(ctx, username, password);
    await ctx.storageState({ path: authDataFile(profile) });
    await ctx.dispose();
  };
  await dataLogin('root', 'admin', ROOT_PASSWORD);

  const seededUsers: Seeded['users'] = {
    root: { id: 'root', username: 'admin', password: ROOT_PASSWORD },
  };
  for (const [profile, u] of [
    ['infra-admin', { ...gwadmin, username: 'gwadmin' }],
    ['app-admin', { ...appadmin, username: 'appadmin' }],
    ['tenant-admin', { ...tadmin, username: 'tadmin' }],
    ['user', { ...alice, username: 'alice' }],
    ...extra,
  ] as const) {
    const ctx = await request.newContext({ baseURL: ADMIN_URL });
    await loginFirstTime(ctx, u.username, u.oneTime);
    await ctx.storageState({ path: authFile(profile) });
    const administered = Object.entries(tenants).find(([, t]) => t.admin === u.id);
    if (administered) await stashTenantSecret(ctx, administered[1], administered[0]);
    seededUsers[profile] = { id: u.id, username: u.username, password: USER_PASSWORD };
    await ctx.dispose();
    await dataLogin(profile, u.username, USER_PASSWORD);
  }

  const seeded: Seeded = { tenantId, tenants, users: seededUsers };
  writeFileSync(`${AUTH_DIR}/seeded.json`, JSON.stringify(seeded, null, 2));
  await root.dispose();
});
