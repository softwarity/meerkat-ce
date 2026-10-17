import { expect, request, test, type APIRequestContext } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL } from '../playwright.config';
import { authDataFile, authFile } from '../lib/fixtures';

// The roles an application declares in its own spec (RBAC-12), end to end.
//
// The spec says, per operation, which roles the APPLICATION checks: the scopes
// of its security requirement. The gateway reads them, the route maps each one
// onto a catalogue role (created here with the route's prefix and tagged with
// its name, the import the console's drawer does), and Auth forward then sends
// a caller the declared roles whose catalogue role they hold, under the
// APPLICATION's names - the route writes no expression for it.
//
// What the caller holds is posed with the gateway's simulation headers, which a
// signed-in root may send: one call holding the mapped role and another one,
// one holding the other only. httpbin's /headers echoes what the application
// would have received.

const ROUTE = 'e2e-declared';
const PREFIX = `${ROUTE}-`;
const MAPPED = `${PREFIX}ROLE_ADMIN`;
const ROLES_HEADER = 'X-E2e-Roles';
const USER_HEADER = 'X-E2e-User';
const DEV = 'e2e-declared-dev';
const DEV_PASSWORD = 'e2e-Declared-Dev-1';

// Two operations under one global "signed in": /headers asks for ROLE_ADMIN,
// /anything for nothing more than a credential. OpenAPI 3.1 lets a scheme
// that is not OAuth carry scopes, and that is where the roles go.
const SPEC = {
  openapi: '3.1.0',
  info: { title: 'Declared roles e2e', version: '1.0.0' },
  components: { securitySchemes: { bearer: { type: 'http', scheme: 'bearer' } } },
  security: [{ bearer: [] }],
  paths: {
    '/headers': {
      get: {
        operationId: 'headers',
        summary: 'Echo the request headers',
        security: [{ bearer: ['ROLE_ADMIN'] }],
        responses: { '200': { description: 'The headers' } },
      },
    },
    '/anything': {
      get: {
        operationId: 'anything',
        summary: 'Echo the request',
        security: [{ bearer: [] }],
        responses: { '200': { description: 'The request' } },
      },
    },
  },
};

interface Operation {
  method: string;
  path: string;
  upstream?: { access: string; roles?: string[] };
}

// One header of httpbin's echo, whatever case and shape it comes back in
// (go-httpbin answers a list of values per name).
function echoed(headers: Record<string, string | string[]>, name: string): string | undefined {
  const key = Object.keys(headers).find((k) => k.toLowerCase() === name.toLowerCase());
  if (!key) return undefined;
  const v = headers[key];
  return Array.isArray(v) ? v.join(',') : v;
}

// Both of root's sessions in one context: the data plane's, and the admin
// plane's that makes the simulation headers honoured. Cookies are scoped to
// the host, so both ride to the data plane's port.
function rootOnBothPlanes() {
  const admin = JSON.parse(readFileSync(authFile('root'), 'utf8'));
  const data = JSON.parse(readFileSync(authDataFile('root'), 'utf8'));
  return { cookies: [...admin.cookies, ...data.cookies], origins: [] };
}

async function cleanUp(root: APIRequestContext): Promise<void> {
  await root.delete(`/api/routes/${ROUTE}`);
  await root.delete(`/api/roles/${MAPPED}`);
  const users = (await (await root.get('/api/users')).json()) as Array<{ id: string; username: string }>;
  const dev = users.find((u) => u.username === DEV);
  if (dev) await root.delete(`/api/users/${dev.id}`);
}

test('declared roles: read from the spec, imported into the catalogue, forwarded under the application names', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  await cleanUp(root);

  try {
    // A signed-in route that strips its first segment, and forwards the
    // caller's roles as a header with NO expression: on a route with a role
    // map, that is the declared roles under the application's names.
    const route = await root.put(`/api/routes/${ROUTE}`, {
      data: {
        id: ROUTE,
        name: ROUTE,
        order: 30, // ahead of the seeded catch-all
        enabled: true,
        upstream: HTTPBIN_URL,
        predicates: [{ type: 'path', args: { patterns: [`/${ROUTE}/**`] } }],
        filters: [{ type: 'strip-prefix', args: { parts: 1 } }],
        access: { level: 'auth' },
        identity: {
          mechanism: 'headers',
          attributes: [
            { field: 'username', as: USER_HEADER },
            { field: 'roles', as: ROLES_HEADER },
          ],
        },
      },
    });
    expect(route.ok(), await route.text()).toBeTruthy();

    // The spec as a deposited file: the body is the document itself.
    const deposit = await root.put(`/api/routes/${ROUTE}/spec`, { data: SPEC });
    expect(deposit.ok(), await deposit.text()).toBeTruthy();
    expect(await deposit.json()).toMatchObject({ title: 'Declared roles e2e', operations: 2 });

    // Per operation, what the application requires; and the roles declared.
    const opsRes = await root.get(`/api/routes/${ROUTE}/operations`);
    expect(opsRes.ok(), await opsRes.text()).toBeTruthy();
    const ops = (await opsRes.json()) as { operations: Operation[]; declaredRoles?: string[]; roleMap?: Record<string, string> };
    const op = (path: string) => ops.operations.find((o) => o.method === 'GET' && o.path === path);
    expect(op('/headers')?.upstream).toEqual({ access: 'roles', roles: ['ROLE_ADMIN'] });
    expect(op('/anything')?.upstream).toEqual({ access: 'authenticated' });
    expect(ops.declaredRoles).toEqual(['ROLE_ADMIN']);
    expect(ops.roleMap ?? {}).toEqual({});

    // The import: ROLE_ADMIN becomes a new catalogue role under the route's
    // prefix, tagged with its name, and the map ties the two.
    const mapped = await root.put(`/api/routes/${ROUTE}/role-map`, {
      data: { roleMap: { ROLE_ADMIN: MAPPED }, create: [{ name: MAPPED, tags: [ROUTE] }] },
    });
    expect(mapped.ok(), await mapped.text()).toBeTruthy();
    expect(await mapped.json()).toEqual({ roleMap: { ROLE_ADMIN: MAPPED }, created: [MAPPED] });
    const roles = (await (await root.get('/api/roles')).json()) as Array<{ name: string; tags: string[] }>;
    expect(roles.find((r) => r.name === MAPPED)?.tags).toEqual([ROUTE]);
    const reread = (await (await root.get(`/api/routes/${ROUTE}/operations`)).json()) as { roleMap?: Record<string, string> };
    expect(reread.roleMap).toEqual({ ROLE_ADMIN: MAPPED });

    // A caller holding the mapped role and another one: the application gets
    // ROLE_ADMIN, its own name, and nothing of the other.
    const caller = await request.newContext({ baseURL: DATA_URL, storageState: rootOnBothPlanes() });
    const call = async (roles: string) => {
      const res = await caller.get(`/${ROUTE}/headers`, {
        headers: { 'X-Meerkat-Simulate-User': 'e2e-declared-caller', 'X-Meerkat-Simulate-Roles': roles },
      });
      expect(res.status(), await res.text()).toBe(200);
      return ((await res.json()) as { headers: Record<string, string | string[]> }).headers;
    };
    const holder = await call(`${MAPPED},some-other-role`);
    // Served by this route, with its identity forwarding, as the posed caller.
    expect(echoed(holder, USER_HEADER)).toBe('e2e-declared-caller');
    expect(echoed(holder, ROLES_HEADER)).toBe('ROLE_ADMIN');
    // The other role alone: nothing declared is held, nothing is sent.
    const other = await call('some-other-role');
    expect(echoed(other, USER_HEADER)).toBe('e2e-declared-caller');
    expect(echoed(other, ROLES_HEADER) ?? '').toBe('');
    await caller.dispose();

    // The developer API docs say it on the operation, with the mapped role.
    // They need the dev capability on a data session: a developer made for it.
    const created = await root.post('/api/users', { data: { username: DEV, enabled: true, dev: true } });
    expect(created.status(), await created.text()).toBe(201);
    const oneTime = ((await created.json()) as { password: string }).password;
    const dev = await request.newContext({ baseURL: DATA_URL });
    const signIn = await dev.post('/login', { form: { username: DEV, password: oneTime }, maxRedirects: 0 });
    expect(signIn.status(), 'the developer signs in').toBe(303);
    const settled = await dev.post('/update-password', {
      form: { password: DEV_PASSWORD, confirm: DEV_PASSWORD },
      maxRedirects: 0,
    });
    expect(settled.status(), 'the one-time password is spent').toBe(303);
    const docs = await dev.get(`/meerkat/apidocs/spec/${ROUTE}`);
    expect(docs.ok(), await docs.text()).toBeTruthy();
    const doc = (await docs.json()) as { paths: Record<string, Record<string, { description?: string }>> };
    const described = (path: string) =>
      Object.entries(doc.paths).find(([p]) => p.endsWith(path))?.[1].get.description ?? '';
    expect(described('/headers')).toContain(
      '**Required by the application:** `ROLE_ADMIN` (Meerkat role `' + MAPPED + '`).',
    );
    expect(described('/anything')).toContain('**Required by the application:** a signed-in caller.');
    await dev.dispose();

    // The console: the operation's chip in "Application requires", and the
    // header button counting the mapped roles.
    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(`${ADMIN_URL}/infra/endpoint-security?route=${ROUTE}`);
    await expect(page.getByRole('columnheader', { name: 'Application requires' })).toBeVisible();
    const chip = page.locator('app-upstream-mark .up.role', { hasText: 'ROLE_ADMIN' });
    await expect(chip).toBeVisible();
    await expect(chip).not.toHaveClass(/\bunmapped\b/);
    await expect(page.getByRole('button', { name: /Declared roles: 1 of 1 mapped/ })).toBeVisible();
    await context.close();
  } finally {
    await cleanUp(root);
    await root.dispose();
  }
});
