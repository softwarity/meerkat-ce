import { expect, request, test, type Page } from '@playwright/test';
import screensFile from '../screens.json';
import { ADMIN_URL } from '../playwright.config';
import { authFile, ENTERPRISE, profiles, seeded } from '../lib/fixtures';

// What each capability opens in the console, and what it does not.
//
// screens.json says, screen by screen, who may open it; every-screen.spec.ts
// proves each one renders for those people. This is the other half, the one an
// administrator asks about first: for each profile - root, infrastructure
// administrator alone, application administrator alone, a tenant's
// administrator alone, an account with no capability at all -
//   - the menu of every section lists exactly the screens it may open, no
//     more and no fewer;
//   - every other screen turns it away, even typed into the address bar;
//   - a tenant's administrator reaches its own tenant and not another's.
// The expectations come from screens.json, so the file the documentation's
// test page publishes and the console cannot drift apart.

interface Screen {
  path: string;
  title: string;
  allowed: string[];
  edition?: 'ee' | 'ce';
}

const screens = (screensFile.screens as Screen[]).filter((s) => !s.edition || (s.edition === 'ee') === ENTERPRISE);
// The Enterprise screens a community image still shows in its menus, locked:
// the console's rule for what the image does not carry.
const lockedHere = (screensFile.screens as Screen[]).filter((s) => !ENTERPRISE && s.edition === 'ee');

// The section a screen's menu entry sits in, and the entry itself: one entry
// stands for a screen with tabs (built-in pages, configuration), and a
// tenant's screens are listed under whichever tenant is open.
const SECTIONS = ['/infra', '/application', '/tenants', '/data-plane', '/system'] as const;
const sectionOf = (path: string) => SECTIONS.find((s) => path === s || path.startsWith(s + '/'));
const entryOf = (path: string) =>
  path
    .replace(/^(\/application\/built-in-pages)\/.*/, '$1')
    .replace(/^(\/system\/configuration)\/.*/, '$1')
    .replace(/^\/tenants\/[^/]+\//, '/tenants/{tenantId}/');
const normalize = (href: string) => href.replace(/^\/tenants\/[^/]+\//, '/tenants/{tenantId}/');
// Entries the menu shows only when the installation has the feature on: the
// application's Plug appears once an infrastructure administrator opens plug.
const CONDITIONAL = ['/application/plug'];
const firm = (entries: string[]) => entries.filter((e) => !CONDITIONAL.includes(e)).sort();

// The entries of the section menu as the page draws them (role CSS may keep an
// entry in the DOM and hide it), apart: those that open, and those locked
// because the image does not carry them.
async function menu(page: Page, locked = false): Promise<string[]> {
  return page.locator('nav.left a.section-link').evaluateAll(
    (links, wantLocked) =>
      links
        .filter((a) => getComputedStyle(a).display !== 'none')
        .filter((a) => (getComputedStyle(a).pointerEvents === 'none') === wantLocked)
        .map((a) => a.getAttribute('href') ?? ''),
    locked,
  );
}

const resolve = (path: string) => path.replace('{tenantId}', seeded().tenantId);

for (const profile of profiles) {
  const mine = screens.filter((s) => s.allowed.includes(profile.id));

  test.describe(`cap-${profile.id}`, () => {
    // ── the menus ────────────────────────────────────────────────────────────
    for (const section of SECTIONS) {
      // The tenant list is a page, not an entry of the tenant's own menu.
      const expected = [
        ...new Set(mine.filter((s) => sectionOf(s.path) === section && s.path !== '/tenants').map((s) => entryOf(s.path))),
      ].sort();
      if (!expected.length) continue;
      test(`the ${section} menu lists what ${profile.id} may open, and nothing else`, async ({ browser }) => {
        const context = await browser.newContext({ storageState: authFile(profile.id) });
        const page = await context.newPage();
        // Open the section through a screen of it the profile may open.
        const first = mine.find((s) => sectionOf(s.path) === section && s.path !== '/tenants')!;
        await page.goto(ADMIN_URL + resolve(first.path));
        await expect.poll(async () => firm((await menu(page)).map(normalize))).toEqual(firm(expected));
        // And what is shown but locked: the Enterprise screens it would have.
        const locked = [...new Set(lockedHere.filter((s) => s.allowed.includes(profile.id) && sectionOf(s.path) === section && s.path !== '/tenants').map((s) => entryOf(s.path)))].sort();
        expect((await menu(page, true)).map(normalize).sort()).toEqual(locked);
        await context.close();
      });
    }

    // ── what it may not open ─────────────────────────────────────────────────
    for (const screen of screens.filter((s) => !s.allowed.includes(profile.id))) {
      test(`${profile.id} is turned away from ${screen.path}`, async ({ browser }) => {
        const context = await browser.newContext({ storageState: authFile(profile.id) });
        const page = await context.newPage();
        const path = resolve(screen.path);
        await page.goto(ADMIN_URL + path);
        await page.waitForLoadState('networkidle');
        // Sent to a screen of its own: never left on the one it may not open.
        await expect.poll(() => new URL(page.url()).pathname).not.toBe(path);
        const landed = new URL(page.url()).pathname;
        expect(
          mine.some((s) => landed === resolve(s.path) || landed.startsWith(resolve(s.path) + '/') || /^\/tenants\/[^/]+\//.test(landed)),
          `${profile.id} landed on ${landed}, which is not one of its screens`,
        ).toBeTruthy();
        await context.close();
      });
    }
  });
}

// ── an account with no capability ──────────────────────────────────────────
test('cap-plain-account: an account that administers nothing lands on License, and the rail shows only Meerkat', async ({ browser }) => {
  const context = await browser.newContext({ storageState: authFile('user') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/');
  await expect(page).toHaveURL(/\/system\/license$/);
  const rail = await page.locator('rail-nav-item[label]').evaluateAll((items) =>
    items.filter((i) => getComputedStyle(i).display !== 'none').map((i) => i.getAttribute('label')),
  );
  expect(rail).toEqual(['Meerkat']);
  await context.close();
});

// ── a tenant's administrator, and another tenant ───────────────────────────
test('cap-tenant-scope: a tenant administrator reaches its own tenant and not another one', async ({ browser }) => {
  test.skip(!ENTERPRISE, 'one tenant only on the community build');
  const s = seeded();
  const own = s.tenants.acme.id;
  const other = s.tenants.globex.id;

  // The API lists its own tenant only, and refuses the other one.
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('tenant-admin') });
  const listed = ((await (await api.get('/api/tenants')).json()) as { id: string }[]).map((t) => t.id);
  expect(listed).toContain(own);
  expect(listed).not.toContain(other);
  expect((await api.get(`/api/tenants/${other}`)).status()).toBe(403);
  expect((await api.get(`/api/tenants/${other}/members`)).status()).toBe(403);
  await api.dispose();

  // The console the same: its tenant opens, the other one sends it home.
  const context = await browser.newContext({ storageState: authFile('tenant-admin') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + `/tenants/${own}/members`);
  await expect(page).toHaveURL(new RegExp(`/tenants/${own}/members$`));
  await page.goto(ADMIN_URL + `/tenants/${other}/members`);
  await expect.poll(() => new URL(page.url()).pathname).not.toBe(`/tenants/${other}/members`);
  await context.close();

  // Root reaches both.
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  for (const id of [own, other]) expect((await root.get(`/api/tenants/${id}`)).status()).toBe(200);
  await root.dispose();
});
