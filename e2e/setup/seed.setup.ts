import { expect, request, test as setup, type APIRequestContext } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { ADMIN_URL, DATA_URL, ROOT_PASSWORD } from '../playwright.config';
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

async function organisation(root: APIRequestContext): Promise<string> {
  if (ENTERPRISE) {
    const res = await root.post('/api/tenants', { data: { name: 'acme' } });
    expect(res.status(), await res.text()).toBe(201);
    return ((await res.json()) as { id: string }).id;
  }
  const res = await root.get('/api/edition');
  expect(res.ok(), await res.text()).toBeTruthy();
  const primary = ((await res.json()) as { primaryTenant: string }).primaryTenant;
  expect(primary, 'a single installation still serves one organisation').toBeTruthy();
  return primary;
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
  for (const [userId, type] of [
    [tadmin.id, 'ADMIN'],
    [alice.id, 'USER'],
  ] as const) {
    const res = await root.put(`/api/tenants/${tenantId}/members/${userId}`, {
      data: { userId, tenantId, type, enabled: true, businessAccess: { inherited: true }, sessionTTL: '' },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
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
  ] as const) {
    const ctx = await request.newContext({ baseURL: ADMIN_URL });
    await loginFirstTime(ctx, u.username, u.oneTime);
    await ctx.storageState({ path: authFile(profile) });
    seededUsers[profile] = { id: u.id, username: u.username, password: USER_PASSWORD };
    await ctx.dispose();
    await dataLogin(profile, u.username, USER_PASSWORD);
  }

  const seeded: Seeded = { tenantId, users: seededUsers };
  writeFileSync(`${AUTH_DIR}/seeded.json`, JSON.stringify(seeded, null, 2));
  await root.dispose();
});
