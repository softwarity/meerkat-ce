import { expect, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile, ENTERPRISE, profiles, scenarios, seeded } from '../lib/fixtures';

// Console navigation: the rail entries only show for the profiles that may
// use them (kind=ui scenarios), and "/" lands each profile on its own first
// screen. The role classes come from the gateway's body stamp - this suite
// exercises the real proxy + stamp chain, not a mocked identity.

for (const sc of scenarios.filter((s) => s.kind === 'ui')) {
  test.describe(sc.id, () => {
    for (const profile of profiles) {
      const visible = sc.allowed.includes(profile.id);
      test(`${sc.check!.railItem} is ${visible ? 'visible' : 'hidden'} for ${profile.id}`, async ({ browser }) => {
        const context = await browser.newContext({ storageState: authFile(profile.id) });
        const page = await context.newPage();
        await page.goto(ADMIN_URL + '/');
        const item = page.locator('rail-nav-item').filter({ hasText: sc.check!.railItem });
        if (visible) {
          await expect(item).toBeVisible();
        } else {
          // Role-CSS hides it (display:none); it may also be absent entirely.
          await expect(item).toBeHidden();
        }
        await context.close();
      });
    }
  });
}

// ui-landing (kind=flow, spec=console-nav): each profile's landing screen.
//
// The last two depend on the SHAPE of the installation, and that is the
// product's own rule rather than a quirk of the test: someone who administers
// no gateway and no application lands on the organisations they administer -
// and an installation serving ONE has no such screen, so they land on License,
// the single page no guard can bounce anyone off. See landing() in the
// console's access guards.
const LANDING: Record<string, RegExp> = {
  root: /\/routes$/,
  'infra-admin': /\/routes$/,
  'app-admin': /\/general$/,
  'tenant-admin': ENTERPRISE ? /\/tenants\/[^/]+/ : /\/license$/,
  user: ENTERPRISE ? /\/tenants(\/[^/]+)?/ : /\/license$/,
};

test.describe('ui-landing', () => {
  for (const profile of profiles) {
    test(`${profile.id} lands on its first screen`, async ({ browser }) => {
      seeded(); // fails fast if the setup did not run
      const context = await browser.newContext({ storageState: authFile(profile.id) });
      const page = await context.newPage();
      await page.goto(ADMIN_URL + '/');
      await expect(page).toHaveURL(LANDING[profile.id]);
      await context.close();
    });
  }
});

// Meerkat's own sections: everybody holding the console reads the release
// notes and the licence, the API reference shows for the profiles its route
// guard lets in (apiDocsAccess), and for nobody else - a link that bounces
// whoever follows it is worse than no link.
const API_REFERENCE = ['root', 'infra-admin', 'app-admin'];

test.describe('ui-system-sections', () => {
  for (const profile of profiles) {
    const api = API_REFERENCE.includes(profile.id);
    test(`Meerkat ${api ? 'lists' : 'hides'} the API reference, and lists the licence, for ${profile.id}`, async ({ browser }) => {
      const context = await browser.newContext({ storageState: authFile(profile.id) });
      const page = await context.newPage();
      await page.goto(ADMIN_URL + '/system/license');
      const nav = page.locator('nav.left');
      await expect(nav.getByRole('link', { name: 'License' })).toBeVisible();
      await expect(nav.getByRole('link', { name: 'Release notes' })).toBeVisible();
      const item = nav.getByRole('link', { name: 'API reference' });
      if (api) {
        await expect(item).toBeVisible();
      } else {
        await expect(item).toBeHidden();
      }
      await context.close();
    });
  }
});
