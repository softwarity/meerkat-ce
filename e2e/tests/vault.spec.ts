import { expect, request, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// A stored secret is never sent to the console, so its box is disabled and
// opened by a pencil. Hiding every writing tool of a disabled field took the
// pencil with it (2026-10-03), and a stored secret could no longer be changed
// from the console at all: this is the browser proving it can again.
test('vault-secret-edit: a stored secret can be typed again and saved', async ({ browser }) => {
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const name = `e2e-secret-${Date.now()}`;
  const put = await api.put(`/api/vault/infra/${name}`, { data: { kind: 'secret', value: 'first-value' } });
  expect(put.ok(), await put.text()).toBeTruthy();

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/vault');
  await page.locator('mat-row', { hasText: name }).click();
  const field = page.locator('app-vault-entry-form mat-form-field', { has: page.locator('mat-label', { hasText: /^Value$/ }) });
  const box = field.locator('textarea');
  await expect(box).toBeDisabled();
  await field.locator('button[aria-label="Edit"]').click();
  await expect(box).toBeEnabled();
  await box.fill('second-value');
  const saved = page.waitForResponse((r) => r.url().includes(`/api/vault/infra/${name}`) && r.request().method() === 'PUT');
  await page.locator('app-vault-entry-form button', { hasText: 'Save' }).click();
  const answer = await saved;
  expect(answer.ok(), await answer.text()).toBeTruthy();
  expect(answer.request().postDataJSON().value).toBe('second-value');

  await context.close();
  await api.delete(`/api/vault/infra/${name}`);
  await api.dispose();
});

// An import brings the names its routes expect, never their values: the vault
// says which entries are empty, and which values are past their reminder date,
// on the entries, in a line above them and as a badge on the rail.
test('vault-attention: an imported name without its value, and a lapsed value, are said', async ({ browser }) => {
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const stamp = Date.now();
  const hole = `e2e-imported-${stamp}`;
  const lapsed = `e2e-lapsed-${stamp}`;
  const route = `e2e-vault-hole-${stamp}`;
  const imported = await api.post('/api/config/import', {
    data: {
      version: 1,
      routes: [{
        id: route, name: route, order: 990, enabled: false, upstream: `http://\${${hole}}:8080`,
        predicates: [{ type: 'path', args: { patterns: [`/${route}/**`] } }],
      }],
    },
  });
  expect(imported.ok(), await imported.text()).toBeTruthy();
  const yesterday = Math.floor(Date.now() / 86_400_000 - 1) * 86_400;
  const put = await api.put(`/api/vault/infra/${lapsed}`, { data: { kind: 'secret', value: 'old', expiresAt: yesterday } });
  expect(put.ok(), await put.text()).toBeTruthy();

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/vault');
  await expect(page.locator('mat-row', { hasText: hole })).toContainText('no value');
  await expect(page.locator('mat-row', { hasText: lapsed })).toContainText('expired on');
  await expect(page.locator('.problem', { hasText: 'no value' })).toBeVisible();
  await expect(page.locator('.problem', { hasText: 'reminder date' })).toBeVisible();
  await expect(page.locator('rail-nav-item[label="Vault"] .rail-badge'), 'the rail says so too').toBeVisible();

  await context.close();
  await api.delete(`/api/routes/${route}`);
  for (const name of [hole, lapsed]) await api.delete(`/api/vault/infra/${name}`);
  await api.dispose();
});
