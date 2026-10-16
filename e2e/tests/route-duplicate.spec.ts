import { expect, request, test } from '@playwright/test';
import { ADMIN_URL, HTTPBIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// Duplicating a route sent the original's revision on the copy's new id, which
// the store reads as a route deleted since it was read: every copy was refused
// with a 409, and two snack bars chased each other off the screen (2026-10-15).
// A second copy of the same route then needs a name of its own, names being
// unique. In the browser, from the button: two copies, both created, disabled.
test('route-duplicate: a route is copied twice from the routes screen', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const id = 'e2e-duplicate';
  const route = {
    id,
    name: id,
    order: 10, // ahead of the seeded catch-all
    enabled: true,
    upstream: HTTPBIN_URL,
    predicates: [{ type: 'path', args: { patterns: [`/${id}/**`] } }],
    filters: [],
  };
  const first = await root.put(`/api/routes/${id}`, { data: route });
  expect(first.ok(), await first.text()).toBeTruthy();
  // A second write, so the route the screen reads is past its first revision.
  const second = await root.put(`/api/routes/${id}`, { data: { ...(await first.json()), upstream: HTTPBIN_URL + '/' } });
  expect(second.ok(), await second.text()).toBeTruthy();

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  const copies: string[] = [];
  for (const expected of [`${id}-copy`, `${id}-copy-2`]) {
    await page.goto(ADMIN_URL + '/infra/routes');
    const row = page.locator('mat-row', { hasText: id }).filter({ hasNotText: `${id}-copy` });
    const saved = page.waitForResponse((r) => r.url().includes('/api/routes/') && r.request().method() === 'PUT');
    // The row's actions are a toolbar floating over the hovered row, outside it.
    // Other specs write routes meanwhile, the live list redraws, and the
    // toolbar closes under the cursor: hover and click are retried together.
    // A click landing on another row would name a different copy, and fail below.
    await expect(async () => {
      await row.hover();
      await page.getByRole('button', { name: 'Duplicate route' }).click({ timeout: 2000 });
    }).toPass({ timeout: 20_000 });
    const answer = await saved;
    expect(answer.status(), await answer.text()).toBe(200);
    const copy = await answer.json();
    copies.push(copy.id);
    expect(copy.name).toBe(expected);
    expect(copy.enabled).toBe(false);
    expect(copy.id).not.toBe(id);
    expect(copy.upstream).toBe(HTTPBIN_URL + '/');
  }

  await context.close();
  for (const c of [...copies, id]) await root.delete(`/api/routes/${c}`);
  await root.dispose();
});
