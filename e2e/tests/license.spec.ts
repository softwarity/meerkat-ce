import { expect, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// The License screen lists the Enterprise features from FEATURES.md
// (CONSOLE-14). How far each is built is an icon, explained once in a legend,
// and the contract's IDs (AUTH-03...) are not shown: nobody reading the screen
// knows a feature by its key (2026-10-16).
test('license: each feature says how far it is built by its icon, and no ID shows', async ({ browser }) => {
  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/system/license');
  const features = page.locator('.features .feature');
  await expect(features.first()).toBeVisible();
  const legend = page.locator('.legend');
  await expect(legend).toContainText('built');
  await expect(legend).toContainText('partly built');
  await expect(legend).toContainText('planned');

  await expect(page.locator('.features')).not.toContainText(/\b[A-Z]+-\d+\b/);
  const icons = await features.locator('> mat-icon').allTextContents();
  expect(icons.length).toBe(await features.count());
  for (const icon of icons) expect(['check_circle', 'incomplete_circle', 'schedule']).toContain(icon.trim());
  // Kerberos is not built: FEATURES.md says so, and so does its icon.
  const kerberos = features.filter({ hasText: 'Kerberos' });
  await expect(kerberos.locator('> mat-icon')).toHaveText('schedule');
  await context.close();
});
