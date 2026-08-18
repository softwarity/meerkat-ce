import { expect, request, test } from '@playwright/test';
import { ADMIN_URL, CONSOLE_PORT, DATA_URL } from '../playwright.config';
import { authDataFile, authFile } from '../lib/fixtures';

// The belonging axis, over a real gateway (RBAC-06). This is the bug of
// 2026-08-06: signing in answered "your account is awaiting access" and the
// same session then walked through a route gated on authentication, reaching
// the application with no organisation and therefore no role.
//
// The rule for "auth" is unchanged on purpose - that level says who it is and
// nothing more, and it is how one serves the page explaining how to ask for
// access. What is new is that a route can demand an organisation, and that
// being refused for the lack of one LEADS SOMEWHERE.

const UPSTREAM = `http://localhost:${CONSOLE_PORT}`;
const NEWCOMER_PASSWORD = 'e2e-Newcomer-Pass-1';

test('flow-access-levels: a pending account passes "signed in" and is sent to the waiting room by "in an organisation"', async () => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });

  // Two UI routes differing only by their level. A route is created by PUT on
  // its id (there is no POST /api/routes).
  const routeIds = ['e2e-auth', 'e2e-tenant'];
  for (const [id, level] of [
    ['e2e-auth', 'auth'],
    ['e2e-tenant', 'tenant'],
  ] as const) {
    const res = await root.put(`/api/routes/${id}`, {
      data: {
        id,
        name: id,
        order: 10, // ahead of the seeded catch-all, which would answer first
        enabled: true,
        isUI: true,
        upstream: UPSTREAM,
        access: { level },
        predicates: [{ type: 'path', args: { patterns: [`/${id}/**`] } }],
        filters: [{ type: 'strip-prefix', args: { parts: 1 } }],
      },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
  }

  // The seeded catch-all has to stand aside. Since a route's security became
  // part of SELECTING it (2026-08-16), a refusal is no longer terminal: the
  // matcher keeps looking, and /** answers whoever the route above turned
  // away. That is the intended behaviour - two /** routes may differ by
  // organisation - but it means the refusal below can only be observed when
  // nothing else is willing to serve the request.
  const trap = await (await root.get('/api/routes/trap')).json();
  const parked = await root.put('/api/routes/trap', { data: { ...trap, enabled: false } });
  expect(parked.ok(), await parked.text()).toBeTruthy();

  // Someone confirmed but granted nothing: no membership anywhere. Creation
  // answers a one-time password, and the first sign-in must spend it - a
  // session still pending a password change is not a session.
  const created = await root.post('/api/users', {
    data: { username: 'e2e-newcomer', fullname: 'New Comer', enabled: true },
  });
  expect(created.status(), await created.text()).toBe(201);
  const body = (await created.json()) as { user: { id: string }; password: string };

  const data = await request.newContext({ baseURL: DATA_URL });
  const signIn = await data.post('/login', {
    form: { username: 'e2e-newcomer', password: body.password },
    maxRedirects: 0,
  });
  expect(signIn.status(), 'the newcomer must be able to sign in').toBe(303);
  const settled = await data.post('/update-password', {
    form: { password: NEWCOMER_PASSWORD, confirm: NEWCOMER_PASSWORD },
    maxRedirects: 0,
  });
  expect(settled.status(), 'the one-time password must be spent').toBe(303);

  // "Signed in" takes them: deliberate, and the reason the level above exists.
  const onAuth = await data.get('/e2e-auth/en/', { maxRedirects: 0 });
  expect(onAuth.status(), 'a pending account must still pass a plain authenticated route').toBeLessThan(400);

  // "In an organisation" does not - and says where to go rather than 403.
  const onTenant = await data.get('/e2e-tenant/en/', { maxRedirects: 0 });
  expect(onTenant.status()).toBe(303);
  expect(onTenant.headers()['location']).toBe('/account-pending');

  await data.dispose();
  for (const id of routeIds) {
    expect((await root.delete(`/api/routes/${id}`)).ok()).toBeTruthy();
  }
  expect((await root.delete(`/api/users/${body.user.id}`)).ok()).toBeTruthy();
  const back = await root.put('/api/routes/trap', { data: { ...trap, enabled: true } });
  expect(back.ok(), await back.text()).toBeTruthy();
  await root.dispose();
});

// flow-api-token lives HERE, next to its neighbour, and that is not filing.
// Both need the seeded catch-all out of the way to see a refusal at all -
// since a route's security became part of selecting it, /** answers whoever
// the route above turned away. Playwright runs FILES in parallel and the tests
// inside one file in order, so two specs parking the same global route raced:
// one put it back while the other was still counting on its absence, and the
// anonymous call sailed through to the upstream. Same file, same worker, no
// race.
// flow-api-token: a user mints a personal token in the browser, then uses it
// as a Bearer to reach an authenticated data-plane route WITHOUT a session;
// revoking it closes the door.
test('flow-api-token: Bearer token authenticates API calls, revoke closes it', async ({ browser }) => {
  const context = await browser.newContext({ storageState: authDataFile('user') });
  const page = await context.newPage();
  await page.goto(DATA_URL + '/profile/tokens');
  // Creation happens in a modal: open it, fill, submit.
  await page.click('#tk-open');
  await page.fill('#tk-create-dlg input[name=name]', 'e2e-token');
  await page.selectOption('#tk-create-dlg select[name=days]', '90');
  await page.click('#tk-create-dlg button[value="create"]');

  // The token is revealed once in its own modal.
  const shown = await page.locator('#tk-reveal-dlg #tk-value').textContent();
  const token = (shown || '').trim();
  expect(token).toMatch(/^mk_/);
  await page.click('#tk-done');

  // The seeded "/secure" route is an AUTHENTICATED UI route. A fresh client
  // with NO cookies but the Bearer token gets through; without it, refused.
  // We assert the AUTH GATE (401 or not), not the upstream status, so the
  // test never depends on the demo upstream being reachable.
  //
  // The catch-all stands aside first. A refusal stopped being terminal when a
  // route's security became part of selecting it: without this, the anonymous
  // call is turned away by /secure and then served by /**, and the gate this
  // test exists for is never reached.
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const trap = await (await root.get('/api/routes/trap')).json();
  expect((await root.put('/api/routes/trap', { data: { ...trap, enabled: false } })).ok()).toBeTruthy();

  const api = await request.newContext({ baseURL: DATA_URL });
  const withTok = await api.get('/secure/get', {
    headers: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
  });
  expect(withTok.status(), 'token must pass the auth gate').not.toBe(401);
  const without = await api.get('/secure/get', { headers: { Accept: 'application/json' } });
  expect(without.status()).toBe(401);

  // Revoke it in the browser (a confirm modal guards the destructive action).
  const row = page.locator('.tk').filter({ hasText: 'e2e-token' });
  await row.locator('button[data-revoke]').click();
  await page.click('#tk-revoke-dlg button[value="revoke"]');
  await expect(page.locator('.tk').filter({ hasText: 'e2e-token' })).toHaveCount(0);
  const afterRevoke = await api.get('/secure/get', {
    headers: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
  });
  expect(afterRevoke.status()).toBe(401);

  // Only now: every assertion above needs the catch-all absent, the last one
  // included - a revoked token is refused by /secure, and /** would serve it.
  expect((await root.put('/api/routes/trap', { data: { ...trap, enabled: true } })).ok()).toBeTruthy();
  await root.dispose();
  await api.dispose();
  await context.close();
});
