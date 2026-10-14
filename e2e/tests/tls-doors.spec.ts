import { expect, request, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';
import { expectNoErrors, openScreen, watchErrors } from '../lib/console';

// Domains & TLS, plane by plane (SSL-14): the application's door moves behind
// a proxy and back, its public names and its proxy's addresses are saved as
// they are typed, and a public certificate left on the plane is shown as not
// used while a proxy serves the names - then serves again on the way back.
test('tls-doors: a plane moves behind a proxy and back, its certificates kept', async ({ browser }) => {
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const cert = await api.post('/api/certificates/self-signed', { data: { names: ['*.e2e-doors.test'], app: true } });
  expect(cert.status(), await cert.text()).toBe(201);
  const certId = ((await cert.json()) as { id: string }).id;

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  const errors = watchErrors(page);
  try {
    await openScreen(page, '/infra/tls');
    await page.getByRole('tab', { name: /Application/ }).click();
    const app = page.getByRole('tabpanel', { name: /Application/ });
    await expect(app.locator('app-door-names')).toBeVisible();

    // A name Meerkat serves itself: covered by the wildcard certificate.
    const names = app.locator('app-door-names mat-chip-grid input').first();
    await names.fill('shop.e2e-doors.test');
    await names.press('Enter');
    await expect(app.locator('app-door-names mat-chip-row', { hasText: 'shop.e2e-doors.test' })).toBeVisible();

    // Behind a proxy: the toggle only proposes, Apply and a confirmation
    // save. The public certificate stays, not used.
    await app.locator('mat-button-toggle', { hasText: 'A proxy in front' }).click();
    await expect(app.locator('.placed', { hasText: '*.e2e-doors.test' })).not.toContainText('Inactive in this mode');
    await app.getByRole('button', { name: 'Apply' }).click();
    await page.locator('mat-dialog-container').getByRole('button', { name: 'Apply' }).click();
    await expect(app.locator('.placed', { hasText: '*.e2e-doors.test' })).toContainText('Inactive in this mode');
    const addresses = app.locator('app-door-names .addresses mat-chip-grid input');
    await addresses.fill('203.0.113.0/24');
    await addresses.press('Enter');
    await expect(app.locator('app-door-names mat-chip-row', { hasText: '203.0.113.0/24' })).toBeVisible();

    const tls = await (await api.get('/api/settings/tls')).json();
    expect(tls.doors.app).toMatchObject({ proxied: true, names: ['shop.e2e-doors.test'], proxyAddresses: ['203.0.113.0/24'] });

    // And back: the certificate serves the names again.
    await app.locator('mat-button-toggle', { hasText: 'Meerkat' }).click();
    await app.getByRole('button', { name: 'Apply' }).click();
    await page.locator('mat-dialog-container').getByRole('button', { name: 'Apply' }).click();
    await expect(app.locator('.placed', { hasText: '*.e2e-doors.test' })).not.toContainText('Inactive in this mode');
    expectNoErrors(errors, 'the Domains & TLS screen');
  } finally {
    await context.close();
    await api.put('/api/settings/tls/doors/app', { data: { proxied: false, names: [], proxyAddresses: [] } });
    await api.delete(`/api/certificates/${certId}`);
    await api.dispose();
  }
});

// A plane's plain door (SSL-15): HTTPS has no switch, it follows the
// certificates. HTTP off is refused while nothing serves HTTPS, and on the
// console the door this session uses does not move - its switch says why and
// stays on, and no request is sent.
test('tls-ports: HTTP switched off and on, the door in use kept', async ({ browser }) => {
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const cert = await api.post('/api/certificates/self-signed', { data: { names: ['ports.e2e.test'], app: true } });
  expect(cert.status(), await cert.text()).toBe(201);
  const certId = ((await cert.json()) as { id: string }).id;
  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  const errors = watchErrors(page);
  const confirm = () => page.locator('mat-dialog-container').getByRole('button', { name: 'Apply' }).click();
  const closed = async () => ((await (await api.get('/api/settings/tls')).json()).closed.app ?? []) as string[];
  try {
    await openScreen(page, '/infra/tls');
    await page.getByRole('tab', { name: /Application/ }).click();
    const app = page.getByRole('tabpanel', { name: /Application/ });
    await expect(app.locator('mat-slide-toggle', { hasText: 'HTTPS:' })).toHaveCount(0);
    const appHttp = app.locator('mat-slide-toggle', { hasText: 'HTTP:' });
    await appHttp.locator('button').click();
    await confirm();
    await expect.poll(closed).toEqual(['http']);
    await appHttp.locator('button').click();
    await confirm();
    await expect.poll(closed).toEqual([]);

    await page.getByRole('tab', { name: /Console/ }).click();
    const consoleTab = page.getByRole('tabpanel', { name: /Console/ });
    const consoleHttp = consoleTab.locator('mat-slide-toggle', { hasText: 'HTTP:' });
    await consoleHttp.locator('button').click();
    await expect(page.locator('mat-snack-bar-container')).toContainText(/Access the console through HTTPS|Add a certificate first/);
    await expect(consoleHttp.locator('button')).toHaveAttribute('aria-checked', 'true');
    await expect(page.locator('mat-dialog-container')).toHaveCount(0);
    expectNoErrors(errors, 'the Domains & TLS screen');
  } finally {
    await context.close();
    await api.put('/api/settings/tls/ports/app', { data: { http: true } });
    await api.delete(`/api/certificates/${certId}`);
    await api.dispose();
  }
});
