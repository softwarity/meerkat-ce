import { expect, request, test } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import screensFile from '../screens.json';
import { ADMIN_URL } from '../playwright.config';
import { authFile, ENTERPRISE, profiles, seeded } from '../lib/fixtures';
import { expectNoErrors, signIn, watchErrors, watchLive } from '../lib/console';

// Every routed screen opens for every profile allowed on it (QUAL-01): the
// title says it rendered, the screen's component holds something, the router
// did not bounce the visitor, and the page reported no error - no exception,
// no console error, no failed request. One screen out of forty-six was driven
// by a test before this one; the others could ship broken and green.

interface Screen {
  path: string;
  module: string;
  title: string;
  allowed: string[];
  edition?: 'ee' | 'ce';
  refused?: Array<{ profile: string; request: string; why: string }>;
}

const all = screensFile.screens as Screen[];
const screens = all.filter((s) => !s.edition || (s.edition === 'ee') === ENTERPRISE);

test('screens.json lists every screen app.routes.ts routes', () => {
  const routes = readFileSync(join(__dirname, '..', '..', 'console', 'src', 'app', 'app.routes.ts'), 'utf8');
  const routed = [...routes.matchAll(/loadComponent:\s*\(\)\s*=>\s*import\('([^']+)'\)/g)].map((m) => m[1]).sort();
  const listed = [...screensFile.containers, ...all.map((s) => s.module)].sort();
  expect(listed, 'a screen was added to or removed from app.routes.ts: list it in e2e/screens.json').toEqual(routed);
});

for (const screen of screens) {
  test.describe(screen.path, () => {
    for (const profile of screen.allowed) {
      test(`renders for ${profile}`, async ({ browser }) => {
        const context = await browser.newContext({ storageState: authFile(profile) });
        const page = await context.newPage();
        const errors = watchErrors(page);
        const path = screen.path.replace('{tenantId}', seeded().tenantId);
        await page.goto(ADMIN_URL + path);
        await page.waitForLoadState('networkidle');

        // Not bounced. /tenants is the one screen that moves on by design: it
        // opens the first organisation.
        const landed = new URL(page.url()).pathname;
        if (screen.path === '/tenants') {
          expect(landed).toMatch(/^\/tenants\/[^/]+/);
        } else {
          expect(landed, `${profile} was sent away from ${path}`).toBe(path);
        }

        // The title: the active section of the left menu where there is one,
        // the rail entry of the screen otherwise (the vault has no menu).
        const nav = page.locator('nav.left').last();
        if (await nav.count()) {
          await expect(nav.locator('a.section-link.active')).toContainText(screen.title);
        } else {
          await expect(page.locator(`rail-nav-item[label="${screen.title}"]`)).toBeVisible();
        }
        // And the screen itself: the component the router put after the
        // innermost outlet is there and holds something.
        await expect(page.locator('router-outlet + *').last().locator('*').first()).toBeAttached();

        // A refusal written down in screens.json is a decision pending, not a
        // regression; anything else is.
        const known = (screen.refused ?? []).filter((r) => r.profile === profile).map((r) => {
          const [method, url] = r.request.split(' ');
          return `http 403: ${method} ${url}`;
        });
        expectNoErrors(errors.filter((e) => !known.includes(e)), `${path} as ${profile}`);
        await context.close();
      });
    }
  });
}

// The way a person gets in, which the storage states of the setup project
// skip: the sign-in form of the admin plane, for every profile.
test.describe('sign in through the form', () => {
  for (const profile of profiles) {
    test(profile.id, async ({ browser }) => {
      const { context, page } = await signIn(browser, profile.id);
      const errors = watchErrors(page);
      await expect(page.locator('rail-nav-item:visible').first()).toBeVisible();
      expectNoErrors(errors, `the console after signing in as ${profile.id}`);
      await context.close();
    });
  }
});

// The console that ships is the one inside the binary, and it refuses to be
// framed by another site (clickjacking).
test('the console refuses to be shown in a frame', async () => {
  const anon = await request.newContext({ baseURL: ADMIN_URL });
  for (const path of ['/', '/infra/routes']) {
    const res = await anon.get(path, { headers: { Accept: 'text/html' } });
    expect(res.headers()['x-frame-options'], path).toBe('DENY');
    expect(res.headers()['content-security-policy'], path).toBe("frame-ancestors 'none'");
  }
  await anon.dispose();
});

// The live channel opens and answers the screen that subscribes to it: the
// metrics screen's traffic.
test('the live channel answers a screen', async ({ browser }) => {
  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  const live = watchLive(page);
  await page.goto(ADMIN_URL + '/data-plane/metrics');
  const channel = await live;
  await channel.next();
  expect(channel.frames.length).toBeGreaterThan(0);
  await context.close();
});
