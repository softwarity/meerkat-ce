import { expect, Locator, request, test } from '@playwright/test';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// The portal's rail, measured in a browser (2026-10-15). Collapsed, a module's
// glyph sat 3px left of its pill and of the burger, and its label was cut to
// the pill's 48px ("Refere..."). Open, the rail took a fixed 280px whatever its
// labels said. Now the glyphs, the pills and the burger share one axis, a
// label takes the rail's width, and the open rail is as wide as its widest
// label with the same padding on both sides.
test('portal-rail: the rail lines its icons up and fits its labels', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await (await root.get('/api/settings')).json();
  const icon = '<svg viewBox="0 0 24 24"><path d="M3 3h18v18H3z"/></svg>';
  const settings = structuredClone(before);
  settings.portal = {
    ...(before.portal ?? {}),
    mode: 'portal',
    layout: 'rail',
    side: 'left',
    display: 'both',
    entries: [
      { routeId: 'demo', label: 'References', icon },
      { routeId: 'demo-secure', label: 'Docs', icon },
    ],
  };
  const saved = await root.put('/api/settings', { data: settings });
  expect(saved.ok(), await saved.text()).toBeTruthy();

  try {
    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(`${DATA_URL}/demo/html`);
    const bar = page.locator('meerkat-portal-nav');
    await expect(bar.locator('.rail .ritem').first()).toBeVisible();
    const measure = () =>
      bar.evaluate((el) => {
        const r = el.shadowRoot!;
        const centre = (e: Element) => {
          const b = e.getBoundingClientRect();
          return b.left + b.width / 2;
        };
        const rail = r.querySelector('.rail')!.getBoundingClientRect();
        return {
          rail: { left: rail.left, right: rail.right, width: rail.width },
          burger: centre(r.querySelector('.burger')!),
          items: [...r.querySelectorAll('.ritem')].map((it) => {
            const pill = it.querySelector('.pill')!;
            const lo = it.querySelector('.lblo')!;
            const b = pill.getBoundingClientRect();
            return {
              pill: centre(pill),
              pillRight: b.right,
              glyph: centre(pill.firstElementChild!),
              cut: lo.scrollWidth > lo.clientWidth,
            };
          }),
        };
      });

    const shut = await measure();
    expect(shut.rail.width).toBe(72);
    for (const it of shut.items) {
      expect(Math.abs(it.glyph - it.pill), 'the glyph is centred in its pill').toBeLessThan(1);
      expect(Math.abs(it.glyph - shut.burger), 'the glyph is on the burger axis').toBeLessThan(1);
      expect(it.cut, 'the label under the icon is whole').toBe(false);
    }

    await bar.locator('.rhead').click();
    await expect.poll(async () => (await measure()).rail.width).not.toBe(72);
    await page.waitForTimeout(300); // the width transition
    const open = await measure();
    expect(open.rail.width, 'fitted, not the old fixed 280px').toBeLessThan(280);
    // The widest pill ends 12px from the rail's edge, as the pills start 12px in.
    const widest = Math.max(...open.items.map((i) => i.pillRight));
    expect(Math.abs(open.rail.right - 1 - widest - 12), 'the same padding on both sides').toBeLessThan(2);
    for (const it of open.items) expect(Math.abs(it.glyph - open.burger)).toBeLessThan(1);
    await context.close();
  } finally {
    const back = await root.put('/api/settings', { data: before });
    expect(back.ok(), await back.text()).toBeTruthy();
    await root.dispose();
  }
});

// A portal entry bound to a route turned off (2026-10-16). The portal was
// validated as a whole and required every route ENABLED, so one such entry made
// every save fail, even one only adding a valid entry. A route turned off is
// away, not gone: the save passes, and the bar shows the entry greyed out, in
// its place, leading nowhere.
test('portal: an entry whose route is turned off is kept and greyed out', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const id = 'e2e-portal-off';
  const route = await root.put(`/api/routes/${id}`, {
    data: {
      id,
      name: id,
      order: 10,
      enabled: false,
      isUi: true,
      upstream: HTTPBIN_URL,
      predicates: [{ type: 'path', args: { patterns: [`/${id}/**`] } }],
      filters: [],
    },
  });
  expect(route.ok(), await route.text()).toBeTruthy();
  const before = await (await root.get('/api/settings')).json();
  const settings = structuredClone(before);
  settings.portal = {
    ...(before.portal ?? {}),
    mode: 'portal',
    layout: 'header',
    display: 'label',
    entries: [
      { routeId: id, label: 'Away' },
      { routeId: 'demo', label: 'References' },
    ],
  };

  try {
    const saved = await root.put('/api/settings', { data: settings });
    expect(saved.ok(), await saved.text()).toBeTruthy();

    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(`${DATA_URL}/demo/html`);
    const bar = page.locator('meerkat-portal-nav');
    const away = bar.locator('.top .tab', { hasText: 'Away' });
    await expect(away).toBeVisible();
    await expect(away).toHaveClass(/\boff\b/);
    await expect(away).toHaveAttribute('aria-disabled', 'true');
    expect(await away.getAttribute('href'), 'it leads nowhere').toBeNull();
    const refs = bar.locator('.top .tab', { hasText: 'References' });
    await expect(refs).not.toHaveClass(/\boff\b/);
    await expect(refs).toHaveAttribute('href', /\/demo/);
    await context.close();
  } finally {
    const back = await root.put('/api/settings', { data: before });
    expect(back.ok(), await back.text()).toBeTruthy();
    await root.delete(`/api/routes/${id}`);
    await root.dispose();
  }
});

// A sub-module added from its container's drawer came up with the container's
// name in its Label (2026-10-16): the drawer kept the same editor, which reads
// its entry only when it is made. A new sub-module starts empty, and an empty
// label is the route's name.
test('portal: a sub-module added from its container starts with no label', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await (await root.get('/api/settings')).json();
  const settings = structuredClone(before);
  settings.portal = {
    ...(before.portal ?? {}),
    mode: 'links',
    entries: [{ label: 'Tools', description: 'The toolbox', children: [{ routeId: 'demo' }] }],
  };
  try {
    const saved = await root.put('/api/settings', { data: settings });
    expect(saved.ok(), await saved.text()).toBeTruthy();

    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(ADMIN_URL + '/application/portal');
    // The icon buttons are named by their tooltip only: found by their glyph.
    const iconButton = (scope: Locator, glyph: string) =>
      scope.locator('button', { has: page.locator('mat-icon', { hasText: new RegExp(`^${glyph}$`) }) });
    await iconButton(page.locator('.link-row', { hasText: 'Tools' }), 'edit').click();
    const editor = page.locator('app-module-editor');
    await expect(editor.getByLabel('Label')).toHaveValue('Tools');
    await iconButton(editor.locator('.quick'), 'add').click();
    await expect(editor.getByRole('heading', { name: 'Add a sub-module' })).toBeVisible();
    await expect(editor.getByLabel('Label')).toHaveValue('');
    await expect(editor.getByLabel('Description (tooltip)')).toHaveValue('');
    await context.close();
  } finally {
    const back = await root.put('/api/settings', { data: before });
    expect(back.ok(), await back.text()).toBeTruthy();
    await root.dispose();
  }
});
