import { APIRequestContext, expect, Locator, Page, request, test } from '@playwright/test';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL } from '../playwright.config';
import { ENTERPRISE, authDataFile, authFile, seeded } from '../lib/fixtures';

// The portal's rail, measured in a browser (2026-10-15). Collapsed, a module's
// glyph sat 3px left of its pill and of the burger, and its label was cut to
// the pill's 48px ("Refere..."). Open, the rail took a fixed 280px whatever its
// labels said. Now the glyphs, the pills and the burger share one axis, a
// label takes the rail's width, and the open rail is as wide as its widest
// label with the same padding on both sides.
test('portal-rail: the rail lines its icons up and fits its labels', async ({ browser }) => {
  test.skip(!ENTERPRISE, 'the navigation bar is Enterprise: the community refusal is the last test');
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
  test.skip(!ENTERPRISE, 'the navigation bar is Enterprise: the community refusal is the last test');
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
  test.skip(!ENTERPRISE, 'the navigation bar is Enterprise: the community refusal is the last test');
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

// An entry turned off in the portal looked like any other in the console's
// preview (2026-10-16): nothing said the bar would not offer it. It is crossed
// out there, and never reaches the people's bar.
test('portal: an entry turned off in the portal is crossed out in the preview', async ({ browser }) => {
  test.skip(!ENTERPRISE, 'the navigation bar is Enterprise: the community refusal is the last test');
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await (await root.get('/api/settings')).json();
  const settings = structuredClone(before);
  settings.portal = {
    ...(before.portal ?? {}),
    mode: 'portal',
    layout: 'header',
    entries: [
      { routeId: 'demo', label: 'References' },
      { routeId: 'demo-secure', label: 'Hidden', disabled: true },
    ],
  };
  try {
    const saved = await root.put('/api/settings', { data: settings });
    expect(saved.ok(), await saved.text()).toBeTruthy();

    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(ADMIN_URL + '/application/portal');
    const preview = page.frameLocator('iframe.preview-frame').locator('meerkat-portal-nav');
    await expect(preview.locator('.top .tab', { hasText: 'Hidden' })).toHaveClass(/\bhid\b/);
    await expect(preview.locator('.top .tab', { hasText: 'References' })).not.toHaveClass(/\bhid\b/);

    // The people's bar does not carry it at all.
    await page.goto(`${DATA_URL}/demo/html`);
    const bar = page.locator('meerkat-portal-nav');
    await expect(bar.locator('.top .tab', { hasText: 'References' })).toBeVisible();
    await expect(bar.locator('.top .tab', { hasText: 'Hidden' })).toHaveCount(0);
    await context.close();
  } finally {
    const back = await root.put('/api/settings', { data: before });
    expect(back.ok(), await back.text()).toBeTruthy();
    await root.dispose();
  }
});

// The notifications (NOTIF-05), from where a reader meets them: the bell of
// the bar and its drawer, the account button's entry and the same drawer
// when there is no bar, and the notifications page that the drawer leads to,
// with the two ways to put one away - Close, which the page lists again
// under Closed until it is reopened, and Delete, for good.
test.describe.serial('notifications: the bell, the drawer and the page', () => {
  test.skip(!ENTERPRISE, 'the notifications are Enterprise');
  let root: APIRequestContext;
  let before: any;
  let notify: any;
  let demo: any;
  const sent: string[] = [];
  const publish = async (title: string, extra: Record<string, unknown> = {}) => {
    const id = `e2e-${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Date.now()}`;
    const res = await root.post(`/api/tenants/${seeded().tenantId}/notifications`, {
      data: { id, level: 'info', title, body: 'Sent by the end-to-end suite.', ...extra },
    });
    expect(res.status(), await res.text()).toBe(201);
    sent.push(id);
    return id;
  };
  // How far the drawer's right edge stands from the window's, or Infinity
  // while it is not laid out.
  const rightGap = async (page: Page, drawer: Locator) => {
    const box = await drawer.boundingBox();
    return box ? Math.abs(box.x + box.width - page.viewportSize()!.width) : Infinity;
  };
  const portal = async (mode: string) => {
    const settings = structuredClone(before);
    settings.portal = { ...(before.portal ?? {}), mode, layout: 'header',
      entries: [{ routeId: 'demo', label: 'References' }] };
    const saved = await root.put('/api/settings', { data: settings });
    expect(saved.ok(), await saved.text()).toBeTruthy();
  };

  test.beforeAll(async () => {
    root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
    before = await (await root.get('/api/settings')).json();
    notify = await (await root.get('/api/notifications/settings')).json();
    const on = await root.put('/api/notifications/settings', { data: { enabled: true } });
    expect(on.ok(), await on.text()).toBeTruthy();
    // The demo application wears the account button, at the bottom left.
    demo = await (await root.get('/api/routes/demo')).json();
    const worn = await root.put('/api/routes/demo', {
      data: { ...demo, ui: { ...(demo.ui ?? {}), userButton: { enabled: true, position: 'bottom-left', height: 32 } } },
    });
    expect(worn.ok(), await worn.text()).toBeTruthy();
  });
  test.afterAll(async () => {
    for (const id of sent) await root.delete(`/api/tenants/${seeded().tenantId}/notifications/${id}`);
    await root.put('/api/routes/demo', { data: demo });
    await root.put('/api/notifications/settings', { data: { enabled: !!notify.enabled } });
    await root.put('/api/settings', { data: before });
    await root.dispose();
  });

  test('the bar carries the bell, and its drawer closes a notification', async ({ browser }) => {
    await portal('portal');
    const title = 'Bell e2e';
    await publish(title);
    const context = await browser.newContext({ storageState: authDataFile('user') });
    const page = await context.newPage();
    await page.goto(`${DATA_URL}/demo/html`);
    const bell = page.locator('meerkat-portal-nav .bell');
    await expect(bell.locator('.badge')).toBeVisible();
    await bell.click();
    const drawer = page.locator('meerkat-notify-drawer .ndr');
    await expect(drawer).toBeVisible();
    const row = drawer.locator('.nr', { hasText: title });
    await expect(row).toBeVisible();
    // Under the header, against the right edge. The drawer paints its list
    // when it arrives: measure once it is there, and again if it repaints.
    await expect.poll(() => rightGap(page, drawer)).toBeLessThan(2);
    // A press on its text reads it, the cross closes it.
    await row.locator('.ntx').click();
    await expect(row).not.toHaveClass(/\bu\b/);
    await row.getByRole('button', { name: new RegExp('^Close: ' + title) }).click();
    await expect(drawer.locator('.nr', { hasText: title })).toHaveCount(0);
    // Escape shuts the drawer.
    await page.keyboard.press('Escape');
    await expect(drawer).toHaveCount(0);
    // Closed, it is kept, read, for the page's Closed list.
    const data = await request.newContext({ baseURL: DATA_URL, storageState: authDataFile('user') });
    const closed = await (await data.get('/meerkat/notifications.json?closed=1')).json();
    expect(closed.items.map((i: any) => i.title)).toContain(title);
    await data.dispose();
    await context.close();
  });

  test('without the bar, the account button opens the drawer opposite itself, which leads to the page', async ({ browser }) => {
    await portal('links');
    const title = 'Button e2e';
    await publish(title);
    const context = await browser.newContext({ storageState: authDataFile('user') });
    const page = await context.newPage();
    await page.goto(`${DATA_URL}/demo/html`);
    const button = page.locator('meerkat-user-button');
    await expect(button.locator('.nb')).toBeVisible();
    await button.locator('#toggle').click();
    await button.locator('[data-notify-drawer]').click();
    const drawer = page.locator('meerkat-notify-drawer .ndr');
    await expect(drawer).toBeVisible();
    // The button sits bottom left: the drawer runs along the right edge,
    // leaving it in sight.
    await expect(drawer.locator('.nr', { hasText: title })).toBeVisible();
    await expect.poll(() => rightGap(page, drawer)).toBeLessThan(2);
    await expect(button.locator('#toggle')).toBeInViewport();

    // All notifications: the page, told where the reader came from.
    await drawer.locator('.ndh a').click();
    await expect(page).toHaveURL(/\/meerkat\/notifications\?back=%2Fdemo%2Fhtml&from=demo/);
    // It wears the application's button where the application wears it.
    await expect(page.locator('meerkat-user-button')).toHaveAttribute('position', 'bottom-left');
    await expect(page.locator('meerkat-user-button')).toHaveAttribute('route', 'demo');
    const back = page.locator('#nx-back');
    await expect(back).toBeVisible();
    await back.click();
    await expect(page).toHaveURL(/\/demo\/html$/);
    await context.close();
  });

  test('the page closes, reopens and deletes for good', async ({ browser }) => {
    await portal('links');
    const title = 'Board e2e';
    await publish(title);
    const context = await browser.newContext({ storageState: authDataFile('user') });
    const page = await context.newPage();
    await page.goto(`${DATA_URL}/meerkat/notifications`);
    const row = () => page.locator('.ntf', { hasText: title });
    await expect(row()).toBeVisible();
    // Close: out of the list, into Closed.
    await row().locator('[data-k="close"]').click();
    await expect(row()).toHaveCount(0);
    await page.locator('#ntf-closed').click();
    await expect(page).toHaveURL(/closed=1/);
    await expect(row()).toBeVisible();
    // Reopen: back in the list.
    await row().locator('[data-k="reopen"]').click();
    await expect(row()).toHaveCount(0);
    await page.locator('#ntf-closed').click();
    await expect(row()).toBeVisible();
    // Delete: gone from the list and from Closed, for good.
    await row().locator('[data-k="del"]').click();
    await expect(row()).toHaveCount(0);
    await page.locator('#ntf-closed').click();
    await expect(row()).toHaveCount(0);
    const data = await request.newContext({ baseURL: DATA_URL, storageState: authDataFile('user') });
    const id = sent[sent.length - 1];
    expect((await data.post(`/meerkat/notifications/${encodeURIComponent(id)}/reopen`)).status(), 'not to be reopened').toBe(404);
    await data.dispose();
    await context.close();
  });

  test('a notification carries one action at most', async () => {
    const res = await root.post(`/api/tenants/${seeded().tenantId}/notifications`, {
      data: { id: `e2e-two-actions-${Date.now()}`, level: 'info', title: 'Two actions',
        actions: [{ label: 'A', url: 'https://example.com/a' }, { label: 'B', url: 'https://example.com/b' }] },
    });
    expect(res.status()).toBe(422);
    expect(await res.text()).toContain('one at most');
  });
});

// The community image does not carry the bar: a write asking for it is refused
// by name, the bar's script is not served, and the Portal toggle is offered
// locked beside the two modes the image has.
test('portal: the community image refuses the bar by name and does not serve it', async ({ browser }) => {
  test.skip(ENTERPRISE, 'the Enterprise image draws the bar: the tests above cover it');
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await (await root.get('/api/settings')).json();
  const res = await root.put('/api/settings', { data: { ...before, portal: { ...(before.portal ?? {}), mode: 'portal' } } });
  expect(res.status()).toBe(422);
  expect(await res.text()).toContain('part of the Enterprise edition');
  await root.dispose();

  const data = await request.newContext({ baseURL: DATA_URL });
  expect(await (await data.get('/meerkat/portal.js')).text()).not.toContain('meerkat-portal-nav');
  await data.dispose();

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/application/portal');
  await expect(page.locator('mat-button-toggle[value="portal"] app-ee-lock')).toBeVisible();
  await context.close();
});
