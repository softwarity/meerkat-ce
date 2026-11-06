import { expect, request, test, type Page } from '@playwright/test';
import { DATA_URL } from '../playwright.config';
import { ENTERPRISE, seeded } from '../lib/fixtures';
import {
  PASSWORD,
  codeIn,
  mailNumber,
  mailsTo,
  newAccount,
  nextTotp,
  readyAccount,
  relayToSink,
  rootApi,
  sessionCookie,
  signIn,
  totp,
  withRouteAccess,
  withSelfRegistration,
  withSettings,
} from '../lib/flows';

// What an END USER goes through on the gateway's own pages, end to end in a
// real browser: arriving without an account, with and without public pages;
// signing up; signing in and out; the forced password change and the chosen
// one; a second factor, a passkey, a tenant, a group; a forgotten password; an
// application they may not open; a maintenance that ends while they wait; an
// outside authority.
//
// Its own project (playwright.config.ts), run last and alone: it changes global
// settings - public pages, self-registration, maintenance - that every other
// suite reads. Each test makes its own accounts, so a second factor or a
// changed password never leaks into another test.
test.describe.configure({ mode: 'serial' });

const publicLinks = (page: Page) => page.locator('.public-links a');

// ── arriving without an account ──────────────────────────────────────────────

test('uf-anon-public: a visitor is offered the public pages, and only those open', async ({ page }) => {
  await page.goto(DATA_URL + '/login');
  await expect(page.locator('.public-links a[href="/demo"]')).toHaveCount(1);
  // The public application answers without an account...
  const open = await page.request.get(DATA_URL + '/demo/get', { maxRedirects: 0 });
  expect(open.status()).toBe(200);
  // ...the protected one sends to the sign-in, saying where to come back.
  const shut = await page.request.get(DATA_URL + '/secure/get', { maxRedirects: 0, headers: { Accept: 'text/html' } });
  expect([302, 303]).toContain(shut.status());
  expect(shut.headers()['location']).toMatch(/\/login\?next=%2Fsecure%2Fget/);
});

test('uf-anon-no-public: with no public page, the sign-in offers none', async ({ page }) => {
  const root = await rootApi();
  try {
    await withRouteAccess(root, 'demo', { level: 'auth' }, async () => {
      await expect(async () => {
        await page.goto(DATA_URL + '/login');
        await expect(page.locator('.public')).toHaveCount(0, { timeout: 1000 });
      }).toPass({ timeout: 15_000 });
      const shut = await page.request.get(DATA_URL + '/demo/get', { maxRedirects: 0, headers: { Accept: 'text/html' } });
      expect([302, 303]).toContain(shut.status());
    });
  } finally {
    await root.dispose();
  }
});

test('uf-protected-page: a protected page signs in first, then opens itself', async ({ page }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-protected');
  await root.dispose();
  await page.goto(DATA_URL + '/secure/anything/here');
  await expect(page).toHaveURL(/\/login\?next=/);
  await signIn(page, u.username, PASSWORD);
  await expect(page).toHaveURL(DATA_URL + '/secure/anything/here');
  // httpbin's /anything echoes the request: the application was reached.
  await expect(page.locator('body')).toContainText('/anything/here');
});

// ── signing in and out, passwords ────────────────────────────────────────────

test('uf-login-logout: sign in, see the profile, sign out, and the session is gone', async ({ page }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-inout');
  await root.dispose();

  // A wrong password first: refused on the page, no session.
  await signIn(page, u.username, 'not-the-password');
  await expect(page.locator('p.error').first()).toBeVisible();
  expect(await sessionCookie(page)).toBeUndefined();

  await signIn(page, u.username, PASSWORD);
  expect(await sessionCookie(page)).toBeTruthy();
  await page.goto(DATA_URL + '/profile');
  await expect(page.locator('dl.facts')).toContainText(u.username);

  // A link that signs out asks first, and says which way is out.
  await page.goto(DATA_URL + '/logout');
  await expect(page.locator('.lead')).toBeVisible();
  await page.locator('form[action$="logout"] button[type=submit]').first().click();
  await page.goto(DATA_URL + '/profile');
  await expect(page).toHaveURL(/\/login/);
});

test('uf-first-signin: a one-time password must be changed before anything opens', async ({ page }) => {
  const root = await rootApi();
  const a = await newAccount(root, 'uf-first');
  await root.dispose();

  await signIn(page, a.username, a.oneTime, '/profile');
  await expect(page).toHaveURL(/\/update-password/);
  // Nothing else opens meanwhile: the profile sends back to the change.
  await page.goto(DATA_URL + '/profile');
  await expect(page).toHaveURL(/\/update-password/);

  // Two passwords that differ are refused, on the page.
  await page.fill('input[name=password]', PASSWORD);
  await page.fill('input[name=confirm]', PASSWORD + 'x');
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('p.error').first()).toBeVisible();

  await page.fill('input[name=password]', PASSWORD);
  await page.fill('input[name=confirm]', PASSWORD);
  await page.locator('button[type=submit]').first().click();
  await expect(page).not.toHaveURL(/\/update-password/);

  // The one-time password is spent; the new one signs in.
  const anon = await request.newContext({ baseURL: DATA_URL });
  expect((await anon.post('/login', { form: { username: a.username, password: a.oneTime }, maxRedirects: 0 })).status()).toBe(401);
  expect((await anon.post('/login', { form: { username: a.username, password: PASSWORD }, maxRedirects: 0 })).status()).toBe(303);
  await anon.dispose();
});

test('uf-change-password: the current password is asked, then the old one no longer works', async ({ page }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-change');
  await root.dispose();
  await signIn(page, u.username, PASSWORD);

  await page.goto(DATA_URL + '/profile/security');
  await page.click('a[href="/profile/password"]');
  await page.fill('input[name=current]', 'not-the-current-one');
  await page.fill('input[name=password]', 'Flow-Changed-Pass-2');
  await page.fill('input[name=confirm]', 'Flow-Changed-Pass-2');
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('p.error').first()).toBeVisible();

  await page.fill('input[name=current]', PASSWORD);
  await page.fill('input[name=password]', 'Flow-Changed-Pass-2');
  await page.fill('input[name=confirm]', 'Flow-Changed-Pass-2');
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('p.error')).toHaveCount(0);

  const anon = await request.newContext({ baseURL: DATA_URL });
  expect((await anon.post('/login', { form: { username: u.username, password: PASSWORD }, maxRedirects: 0 })).status()).toBe(401);
  expect((await anon.post('/login', { form: { username: u.username, password: 'Flow-Changed-Pass-2' }, maxRedirects: 0 })).status()).toBe(303);
  await anon.dispose();
});

// ── a second factor ──────────────────────────────────────────────────────────

test('uf-totp: set up an authenticator, then sign in with a code and with a backup code', async ({ page, browser }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-totp');
  await root.dispose();
  await signIn(page, u.username, PASSWORD);

  // Set up from the profile: the key, the first code, then the backup codes.
  await page.goto(DATA_URL + '/profile/mfa');
  const secret = (await page.locator('p.secret').textContent())!.trim();
  expect(secret).toMatch(/^[A-Z2-7]{16,}$/);
  await page.fill('input[name=code]', totp(secret));
  await page.locator('form:has(input[name=code]) button[type=submit]').click();
  const codes = (await page.locator('ul.codes li').allTextContents()).map((c) => c.trim());
  expect(codes.length, 'backup codes are shown once').toBeGreaterThanOrEqual(8);
  await page.locator('form:has(input[name=step][value=ack]) button[type=submit]').click();

  // A fresh browser: the password now leads to the code.
  const ctx = await browser.newContext();
  const fresh = await ctx.newPage();
  await signIn(fresh, u.username, PASSWORD, '/profile');
  await expect(fresh).toHaveURL(/\/totp/);
  await fresh.fill('input[name=code]', '000000');
  await fresh.locator('form[action="/totp"] button[type=submit]').first().click();
  await expect(fresh.locator('p.error').first()).toBeVisible();
  await fresh.fill('input[name=code]', await nextTotp(secret));
  await fresh.locator('form[action="/totp"] button[type=submit]').first().click();
  await expect(fresh).toHaveURL(DATA_URL + '/profile');
  await ctx.close();

  // A backup code, typed as a phone would type it (capitals, a space): good
  // once, refused the second time.
  const backup = codes[0];
  const typed = backup.replace('-', ' ').toUpperCase();
  for (const [attempt, works] of [[1, true], [2, false]] as const) {
    const c = await browser.newContext();
    const p = await c.newPage();
    await signIn(p, u.username, PASSWORD, '/profile');
    await expect(p).toHaveURL(/\/totp/);
    await p.fill('input[name=code]', typed);
    await p.locator('form[action="/totp"] button[type=submit]').first().click();
    if (works) await expect(p, `backup code, use ${attempt}`).toHaveURL(DATA_URL + '/profile');
    else await expect(p.locator('p.error').first(), `backup code, use ${attempt}`).toBeVisible();
    await c.close();
  }
});

test('uf-totp-imposed: an account that must have a second factor sets it up at sign-in', async ({ page }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-imposed', { mfaRequired: 'true' });
  await root.dispose();
  await signIn(page, u.username, PASSWORD, '/profile');
  await expect(page).toHaveURL(/\/totp-enroll/);
  const secret = (await page.locator('p.secret').textContent())!.trim();
  await page.fill('input[name=code]', totp(secret));
  await page.locator('form:has(input[name=code]) button[type=submit]').click();
  await expect(page.locator('ul.codes li').first()).toBeVisible();
  await page.locator('form:has(input[name=step][value=ack]) button[type=submit]').click();
  await expect(page).toHaveURL(DATA_URL + '/profile');
  // Imposed: the profile offers no way to turn it off.
  await page.goto(DATA_URL + '/profile/mfa');
  await expect(page.locator('button[value=disable]')).toHaveCount(0);
});

// ── a passkey ────────────────────────────────────────────────────────────────

test('uf-passkey: add a passkey, sign in with it, and removing it asks first', async ({ browser }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-passkey');
  await root.dispose();

  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  // A virtual authenticator stands for the phone or the security key.
  const cdp = await ctx.newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  await cdp.send('WebAuthn.addVirtualAuthenticator', {
    options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });

  await signIn(page, u.username, PASSWORD);
  await page.goto(DATA_URL + '/profile/passkeys');
  await page.click('#pk-add');
  await expect(page.locator('.rows .row')).toHaveCount(1, { timeout: 15_000 });

  // Signed out, the passkey alone signs back in: no username, no password.
  // The page offers it in the username field's autofill as soon as it loads
  // (conditional mediation), and the virtual authenticator, which needs no
  // hand, answers that offer at once; a browser without it takes the button.
  await page.context().clearCookies();
  await page.goto(DATA_URL + '/login?next=%2Fprofile');
  await expect(async () => {
    if (page.url().includes('/login') && (await page.locator('#pk-login').isVisible())) await page.click('#pk-login', { timeout: 1000 });
    await expect(page).toHaveURL(DATA_URL + '/profile', { timeout: 2000 });
  }).toPass({ timeout: 15_000 });
  await expect(page.locator('dl.facts')).toContainText(u.username);

  // The cross asks in the row; only the button there removes.
  await page.goto(DATA_URL + '/profile/passkeys');
  await page.locator('a.pk-x').click();
  await expect(page.locator('form.pk-ask')).toBeVisible();
  await expect(page.locator('.rows .row')).toHaveCount(1);
  await page.locator('form.pk-ask button.pk-yes').click();
  await expect(page.locator('.rows .row')).toHaveCount(0);
  await ctx.close();
});

// ── tenants and groups ───────────────────────────────────────────────────────

test('uf-choose-tenant: a member of two tenants chooses where to work', async ({ page }) => {
  test.skip(!ENTERPRISE, 'one tenant only on the community build');
  const s = seeded();
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-tenants');
  for (const t of [s.tenants.acme, s.tenants.globex]) {
    const res = await root.put(`/api/tenants/${t.id}/members/${u.id}`, { data: { type: 'USER', enabled: true } });
    expect(res.ok(), await res.text()).toBeTruthy();
  }
  await root.dispose();

  await signIn(page, u.username, PASSWORD, '/profile');
  await expect(page).toHaveURL(/\/select-tenant/);
  await expect(page.locator('button.choice')).toHaveCount(2);
  await page.locator('button.choice', { hasText: /globex/i }).click();
  await expect(page).toHaveURL(DATA_URL + '/profile');
  await expect(page.locator('dl.facts')).toContainText(/globex/i);
});

test('uf-choose-group: the group in use is marked current when choosing again', async ({ page }) => {
  const s = seeded();
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-groups');
  const tenant = await (await root.get(`/api/tenants/${s.tenantId}`)).json();
  try {
    const forced = await root.put(`/api/tenants/${s.tenantId}`, { data: { ...tenant, groupMode: 'SINGLE' } });
    expect(forced.ok(), await forced.text()).toBeTruthy();
    const member = await root.put(`/api/tenants/${s.tenantId}/members/${u.id}`, { data: { type: 'USER', enabled: true } });
    expect(member.ok(), await member.text()).toBeTruthy();
    for (const name of ['uf-north', 'uf-south']) {
      const res = await root.post(`/api/tenants/${s.tenantId}/groups`, { data: { name, roles: [] } });
      expect([200, 201, 422]).toContain(res.status());
    }
    const groups = (await (await root.get(`/api/tenants/${s.tenantId}/groups`)).json()) as { id: string; name: string }[];
    const ids = groups.filter((g) => g.name.startsWith('uf-')).map((g) => g.id);
    const assign = await root.put(`/api/tenants/${s.tenantId}/members/${u.id}/groups`, { data: ids });
    expect(assign.ok(), await assign.text()).toBeTruthy();

    await signIn(page, u.username, PASSWORD, '/profile');
    await expect(page).toHaveURL(/\/select-group/);
    await page.locator('button.choice', { hasText: 'uf-north' }).click();
    await expect(page).toHaveURL(DATA_URL + '/profile');

    await page.goto(DATA_URL + '/select-group');
    const current = page.locator('button.choice.current');
    await expect(current).toHaveCount(1);
    await expect(current).toContainText('uf-north');
    await expect(current).toBeDisabled();
    await page.locator('button.choice', { hasText: 'uf-south' }).click();
    await page.goto(DATA_URL + '/select-group');
    await expect(page.locator('button.choice.current')).toContainText('uf-south');
  } finally {
    await root.put(`/api/tenants/${s.tenantId}`, { data: { ...tenant, groupMode: tenant.groupMode || '' } });
    await root.dispose();
  }
});

// ── signing up ───────────────────────────────────────────────────────────────

async function register(page: Page, username: string, email: string): Promise<void> {
  await page.goto(DATA_URL + '/login');
  await page.click('a[href="/register"]');
  await page.fill('input[name=username]', username);
  await page.fill('input[name=email]', email);
  await page.fill('input[name=password]', PASSWORD);
  await page.fill('input[name=confirm]', PASSWORD);
  await page.locator('button[type=submit]').first().click();
  // The page says where the code went, in the visitor's own words.
  await expect(page.locator('.sent-to')).toContainText(email);
}

test('uf-register-public: sign up, confirm with the mailed code, and wait with the public pages', async ({ page }) => {
  const root = await rootApi();
  await relayToSink(root);
  try {
    await withSelfRegistration(root, async () => {
      const name = `uf-reg-${Date.now().toString(36)}`;
      const email = `${name}@e2e.test`;
      await register(page, name, email);
      const mail = await mailNumber(email, 1);

      // A wrong code is refused, on the page; the right one confirms.
      await page.fill('input[name=code]', '000000');
      await page.locator('button[type=submit]').first().click();
      await expect(page.locator('p.error').first()).toBeVisible();
      await page.fill('input[name=code]', codeIn(mail));
      await page.locator('button[type=submit]').first().click();
      await expect(page.locator('a.primary[href="/login"]')).toBeVisible();

      // Confirmed but not let in yet: the waiting room, with what is open.
      await page.goto(DATA_URL + '/login');
      await signIn(page, name, PASSWORD);
      await expect(page).toHaveURL(DATA_URL + '/account-pending');
      await expect(page.locator('.public-links a[href="/demo"]')).toHaveCount(1);
    });
  } finally {
    await root.dispose();
  }
});

test('uf-register-no-public: with nothing open to an account that waits, the waiting room offers none', async ({ page }) => {
  // "Open to you meanwhile" is what a signed-in person without a tenant may
  // open: public pages, and pages for any signed-in account. Both demo
  // applications are made to need a tenant, so nothing is.
  const root = await rootApi();
  await relayToSink(root);
  const tenantOnly = { level: 'tenants', tenants: [seeded().tenantId] };
  try {
    await withRouteAccess(root, 'demo', tenantOnly, () => withRouteAccess(root, 'demo-secure', tenantOnly, async () => {
      await withSelfRegistration(root, async () => {
        const name = `uf-regx-${Date.now().toString(36)}`;
        const email = `${name}@e2e.test`;
        await register(page, name, email);
        await page.fill('input[name=code]', codeIn(await mailNumber(email, 1)));
        await page.locator('button[type=submit]').first().click();
        await page.goto(DATA_URL + '/login');
        await signIn(page, name, PASSWORD);
        await expect(page).toHaveURL(DATA_URL + '/account-pending');
        await expect(page.locator('.public')).toHaveCount(0);
      });
    }));
  } finally {
    await root.dispose();
  }
});

test('uf-register-closed: closed, the sign-in offers no account and the page is not there', async ({ page }) => {
  // Whether strangers may sign up is the LOCAL authority's answer; the global
  // switch only speaks for it when it inherits.
  const root = await rootApi();
  const all = await (await root.get('/api/auth-providers')).json();
  const local = all.find((p: { id: string }) => p.id === 'local');
  try {
    const shut = await root.put('/api/auth-providers/local', { data: { ...local, autoCreate: 'no' } });
    expect(shut.ok(), await shut.text()).toBeTruthy();
    await expect(async () => {
      await page.goto(DATA_URL + '/login');
      await expect(page.locator('a[href="/register"]')).toHaveCount(0, { timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    const res = await page.request.get(DATA_URL + '/register', { maxRedirects: 0 });
    expect(res.status()).toBe(404);
  } finally {
    await root.put('/api/auth-providers/local', { data: local });
    await root.dispose();
  }
});

// ── a forgotten password ─────────────────────────────────────────────────────

test('uf-reset: a mailed code resets the password; sent again on request; an unknown address is told the same', async ({ page, browser }) => {
  const root = await rootApi();
  await relayToSink(root);
  const u = await readyAccount(root, 'uf-reset');
  await root.dispose();

  // An address nobody uses: the same page, and no mail.
  const ghost = `uf-ghost-${Date.now().toString(36)}@e2e.test`;
  const other = await browser.newContext();
  const g = await other.newPage();
  await g.goto(DATA_URL + '/forgot-password');
  await g.fill('input[name=email]', ghost);
  await g.locator('button[type=submit]').first().click();
  await expect(g.locator('.sent-to')).toContainText(ghost);
  await other.close();

  await page.goto(DATA_URL + '/login');
  await page.click('a[href="/forgot-password"]');
  await page.fill('input[name=email]', u.email);
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('.sent-to')).toContainText(u.email);
  const first = await mailNumber(u.email, 1);

  // Sent again on request: a second mail, and only the latest code counts.
  await page.locator('form.resend button').click();
  const second = await mailNumber(u.email, 2);
  expect(codeIn(second)).not.toBe(codeIn(first));
  await page.fill('input[name=code]', codeIn(first));
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('p.error').first(), 'a superseded code is refused').toBeVisible();
  await page.fill('input[name=code]', codeIn(second));
  await page.locator('button[type=submit]').first().click();

  await page.fill('input[name=password]', 'Flow-Reset-Pass-3');
  await page.fill('input[name=confirm]', 'Flow-Reset-Pass-3');
  await page.locator('button[type=submit]').first().click();
  await expect(page.locator('a.primary[href="/login"]')).toBeVisible();

  const anon = await request.newContext({ baseURL: DATA_URL });
  expect((await anon.post('/login', { form: { username: u.username, password: PASSWORD }, maxRedirects: 0 })).status()).toBe(401);
  expect((await anon.post('/login', { form: { username: u.username, password: 'Flow-Reset-Pass-3' }, maxRedirects: 0 })).status()).toBe(303);
  await anon.dispose();
  expect(mailsTo(ghost), 'nothing is mailed to an address nobody uses').toHaveLength(0);
});

// ── what a signed-in person may not open ─────────────────────────────────────

test('uf-refused: a page a person may not open says so, and offers the contact', async ({ page }) => {
  const root = await rootApi();
  const u = await readyAccount(root, 'uf-refused');
  // A member: without a tenant the answer is the waiting room (uf-edge-no-tenant).
  const member = await root.put(`/api/tenants/${seeded().tenantId}/members/${u.id}`, { data: { type: 'USER', enabled: true } });
  expect(member.ok(), await member.text()).toBeTruthy();
  const branding = await (await root.get('/api/branding')).json();
  const put = await root.put('/api/branding', { data: { ...branding, contact: 'it@acme.example' } });
  expect(put.ok(), await put.text()).toBeTruthy();
  try {
    await withRouteAccess(root, 'demo-secure', { level: 'auth', roles: ['uf-nobody-holds-this'] }, async () => {
      await signIn(page, u.username, PASSWORD);
      await expect(async () => {
        await page.goto(DATA_URL + '/secure/get');
        await expect(page).toHaveURL(/\/refused/, { timeout: 1000 });
      }).toPass({ timeout: 15_000 });
      await expect(page.locator('.lead')).toBeVisible();
      // The pages' look and brand are cached a few seconds: the contact shows
      // once the branding just written is read.
      await expect(async () => {
        await page.reload();
        await expect(page.locator('a[href="mailto:it@acme.example"]')).toBeVisible({ timeout: 1000 });
      }).toPass({ timeout: 15_000 });
    });
  } finally {
    await root.put('/api/branding', { data: branding });
    await root.dispose();
  }
});

// ── a maintenance that ends ──────────────────────────────────────────────────

test('uf-maintenance: the page that waits comes back by itself when the maintenance ends', async ({ page }) => {
  const root = await rootApi();
  const off = { enabled: false };
  try {
    expect((await root.put('/api/settings/maintenance', { data: { enabled: true, reason: 'maintenance' } })).ok()).toBeTruthy();
    await expect(async () => {
      const res = await page.goto(DATA_URL + '/demo/get');
      expect(res?.status()).toBe(503);
    }).toPass({ timeout: 15_000 });
    const reloaded = page.waitForEvent('load', { timeout: 20_000 });
    expect((await root.put('/api/settings/maintenance', { data: off })).ok()).toBeTruthy();
    await reloaded;
    await expect(page.locator('body')).toContainText('"url"'); // httpbin's /get answer
  } finally {
    await root.put('/api/settings/maintenance', { data: off });
    await root.dispose();
  }
});

// ── an outside authority ─────────────────────────────────────────────────────

test('uf-oidc: an outside authority signs in and creates the account', async ({ page }) => {
  const discovery = await page.request.get('http://localhost:5556/dex/.well-known/openid-configuration').catch(() => null);
  test.skip(!discovery?.ok(), 'Dex is not running (playwright.config.ts starts it with docker)');
  const root = await rootApi();
  const id = 'uf-dex';
  const made = await root.put(`/api/auth-providers/${id}`, {
    data: {
      id, kind: 'oidc', name: 'Example SSO', enabled: true, autoCreate: 'yes',
      config: { issuer: 'http://localhost:5556/dex', clientId: 'meerkat', clientSecret: 'meerkat-secret' },
    },
  });
  expect(made.ok(), await made.text()).toBeTruthy();
  try {
    await expect(async () => {
      await page.goto(DATA_URL + '/login?next=%2Fprofile');
      await expect(page.locator('a.choice', { hasText: 'Example SSO' })).toBeVisible({ timeout: 1000 });
    }).toPass({ timeout: 15_000 });
    await page.locator('a.choice', { hasText: 'Example SSO' }).click();
    // Dex's mock connector answers without a form: back on the gateway, signed in.
    await expect(page).toHaveURL(DATA_URL + '/profile', { timeout: 20_000 });
    await expect(page.locator('dl.facts')).toBeVisible();
    await page.goto(DATA_URL + '/profile/authorities');
    await expect(page.locator('body')).toContainText('Example SSO');
  } finally {
    await root.delete(`/api/auth-providers/${id}`);
    await root.dispose();
  }
});

// ── the edges of an account: no tenant, no role, another tenant, closed ──────
//
// What a person meets when the account is short of something. Each case is
// what the gateway answers TODAY, read off a running gateway before it was
// written down here.

const HTTPBIN = 'http://localhost:18099';
const edgeRoutes = (tenantId: string) => [
  { id: 'uf-edge-auth', path: '/uf-edge-auth/**', access: { level: 'auth' } },
  { id: 'uf-edge-tenant', path: '/uf-edge-tenant/**', access: { level: 'tenants', tenants: [tenantId] } },
  { id: 'uf-edge-role', path: '/uf-edge-role/**', access: { level: 'auth', roles: ['uf-edge-nobody-holds-this'] } },
];

test.describe('uf-edges', () => {
  test.beforeAll(async () => {
    const root = await rootApi();
    for (const r of edgeRoutes(seeded().tenantId)) {
      const res = await root.put(`/api/routes/${r.id}`, {
        data: {
          id: r.id, name: r.id, order: 60, enabled: true, isUi: true, upstream: HTTPBIN,
          predicates: [{ type: 'path', args: { patterns: [r.path] } }],
          filters: [{ type: 'strip-prefix', args: { parts: 1 } }], access: r.access,
        },
      });
      expect(res.ok(), await res.text()).toBeTruthy();
    }
    await root.dispose();
  });
  test.afterAll(async () => {
    const root = await rootApi();
    for (const r of edgeRoutes('')) await root.delete(`/api/routes/${r.id}`);
    await root.dispose();
  });

  // Signs in through the API and answers where each address lands.
  async function landings(username: string, paths: string[]) {
    const c = await request.newContext({ baseURL: DATA_URL });
    const login = await c.post('/login', { form: { username, password: PASSWORD }, maxRedirects: 0 });
    const out: Record<string, string> = { login: `${login.status()} ${login.headers()['location'] ?? ''}`.trim() };
    for (const p of paths) {
      const r = await c.get(p, { maxRedirects: 0, headers: { Accept: 'text/html' } });
      out[p] = `${r.status()} ${(r.headers()['location'] ?? '').split('?')[0]}`.trim();
    }
    await c.dispose();
    return { out, login };
  }
  const member = async (tenantId: string, userId: string, extra: Record<string, unknown> = {}) => {
    const root = await rootApi();
    const res = await root.put(`/api/tenants/${tenantId}/members/${userId}`, { data: { type: 'USER', enabled: true, ...extra } });
    expect(res.ok(), await res.text()).toBeTruthy();
    await root.dispose();
  };
  // Routes reload a beat after they are written.
  const settled = async (username: string) =>
    expect(async () => {
      const { out } = await landings(username, ['/uf-edge-auth/get']);
      expect(out['/uf-edge-auth/get']).toBe('200');
    }).toPass({ timeout: 15_000 });

  test('uf-edge-no-tenant: an account in no tenant waits, and what needs a tenant sends it to the waiting room', async () => {
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-notenant');
    await root.dispose();
    await settled(u.username);
    const { out } = await landings(u.username, ['/profile', '/uf-edge-tenant/get', '/uf-edge-role/get']);
    expect(out.login).toBe('303 /account-pending');
    expect(out['/profile']).toBe('200');
    expect(out['/uf-edge-tenant/get']).toBe('303 /account-pending');
    expect(out['/uf-edge-role/get']).toBe('303 /account-pending');
  });

  test('uf-edge-no-role: a member without the role a page needs is refused, saying why', async ({ page }) => {
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-norole');
    await root.dispose();
    await member(seeded().tenantId, u.id);
    await settled(u.username);
    const { out } = await landings(u.username, ['/uf-edge-tenant/get', '/uf-edge-role/get']);
    expect(out.login).toBe('303 /');
    expect(out['/uf-edge-tenant/get'], 'its own tenant opens').toBe('200');
    expect(out['/uf-edge-role/get']).toBe('303 /refused');
    await signIn(page, u.username, PASSWORD);
    await page.goto(DATA_URL + '/uf-edge-role/get');
    await expect(page).toHaveURL(/\/refused\?.*why=roles/);
    await expect(page.locator('.lead')).toBeVisible();
    await expect(page.locator('a[href="/profile"]')).toBeVisible();
  });

  test('uf-edge-other-tenant: a page of another tenant is refused for that reason', async () => {
    test.skip(!ENTERPRISE, 'one tenant only on the community build');
    const s = seeded();
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-othertenant');
    await root.dispose();
    await member(s.tenants.globex.id, u.id);
    await settled(u.username);
    const c = await request.newContext({ baseURL: DATA_URL });
    await c.post('/login', { form: { username: u.username, password: PASSWORD }, maxRedirects: 0 });
    const r = await c.get('/uf-edge-tenant/get', { maxRedirects: 0, headers: { Accept: 'text/html' } });
    expect(r.status()).toBe(303);
    expect(r.headers()['location']).toMatch(/^\/refused\?.*why=tenant/);
    await c.dispose();
  });

  test('uf-edge-disabled: a disabled account is told what a wrong password is told', async () => {
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-disabled');
    const users = (await (await root.get('/api/users')).json()) as { id: string }[];
    const off = await root.put(`/api/users/${u.id}`, { data: { ...users.find((x) => x.id === u.id), enabled: false } });
    expect(off.ok(), await off.text()).toBeTruthy();
    await root.dispose();
    const c = await request.newContext({ baseURL: DATA_URL });
    const disabled = await c.post('/login', { form: { username: u.username, password: PASSWORD }, maxRedirects: 0 });
    const wrong = await c.post('/login', { form: { username: u.username, password: 'not-it' }, maxRedirects: 0 });
    expect(disabled.status()).toBe(401);
    const said = (r: string) => r.match(/class="error"[^>]*>([^<]*)</)?.[1];
    expect(said(await disabled.text()), 'nothing tells a disabled account from a wrong password').toBe(said(await wrong.text()));
    await c.dispose();
  });

  test('uf-edge-validity: an account outside its validity is told when it ended or when it opens', async ({ page }) => {
    const root = await rootApi();
    const ended = await readyAccount(root, 'uf-ended');
    const early = await readyAccount(root, 'uf-early');
    const users = (await (await root.get('/api/users')).json()) as { id: string }[];
    const day = 86_400;
    const now = Math.floor(Date.now() / 1000);
    for (const [u, patch] of [[ended, { validUntil: now - day }], [early, { validFrom: now + day }]] as const) {
      const res = await root.put(`/api/users/${u.id}`, { data: { ...users.find((x) => x.id === u.id), ...patch } });
      expect(res.ok(), await res.text()).toBeTruthy();
    }
    await root.dispose();
    for (const [u, words] of [[ended, /ended/i], [early, /opens/i]] as const) {
      await page.context().clearCookies();
      await signIn(page, u.username, PASSWORD);
      await expect(page.locator('p.error').first()).toContainText(words);
      expect(await sessionCookie(page)).toBeUndefined();
    }
  });

  test('uf-edge-hours: outside the working hours sign-in is refused; with another tenant open, the closed one is shown, not offered', async ({ page }) => {
    test.skip(!ENTERPRISE, 'access hours and dated memberships are the Enterprise edition');
    const s = seeded();
    // Open only on a day three days from today, in UTC: closed now.
    const today = ((new Date().getUTCDay() + 6) % 7) + 1;
    const closed = { inherited: false, timezone: 'UTC', days: [{ day: ((today + 2) % 7) + 1, from: '09:00', to: '10:00' }] };
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-hours');
    await root.dispose();
    await member(s.tenantId, u.id, { businessAccess: closed });
    await signIn(page, u.username, PASSWORD);
    await expect(page.locator('p.error').first()).toContainText(/working hours/i);
    expect(await sessionCookie(page)).toBeUndefined();

    await member(s.tenants.globex.id, u.id);
    await page.goto(DATA_URL + '/login');
    await signIn(page, u.username, PASSWORD);
    await expect(page).toHaveURL(/\/select-tenant/);
    const shut = page.locator('button.choice:disabled');
    await expect(shut).toHaveCount(1);
    await expect(shut).toContainText(/outside working hours/i);
    await expect(page.locator('button.choice:not(:disabled)')).toHaveCount(1);
  });

  test('uf-edge-membership-ended: a membership past its last day refuses the sign-in', async () => {
    test.skip(!ENTERPRISE, 'access hours and dated memberships are the Enterprise edition');
    // KNOWN: the message speaks of working hours, not of the access that
    // ended - reported, not asserted here.
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-memberend');
    await root.dispose();
    await member(seeded().tenantId, u.id, { businessAccess: { inherited: false, dateTo: '2020-01-01' } });
    const { out } = await landings(u.username, []);
    expect(out.login).toBe('403');
  });

  test('uf-edge-no-group: exclusive groups, and the member in none signs in all the same', async () => {
    const s = seeded();
    const root = await rootApi();
    const u = await readyAccount(root, 'uf-nogroup');
    const tenant = await (await root.get(`/api/tenants/${s.tenantId}`)).json();
    try {
      const forced = await root.put(`/api/tenants/${s.tenantId}`, { data: { ...tenant, groupMode: 'SINGLE' } });
      expect(forced.ok(), await forced.text()).toBeTruthy();
      await member(s.tenantId, u.id);
      const { out } = await landings(u.username, ['/uf-edge-tenant/get']);
      expect(out.login, 'no group to choose: no chooser').toBe('303 /');
      expect(out['/uf-edge-tenant/get']).toBe('200');
    } finally {
      const now = await (await root.get(`/api/tenants/${s.tenantId}`)).json();
      await root.put(`/api/tenants/${s.tenantId}`, { data: { ...now, groupMode: tenant.groupMode || '' } });
      await root.dispose();
    }
  });
});
