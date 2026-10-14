import { expect, request, test } from '@playwright/test';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// Behind an application whose policy restricts styles by nonce - GeoServer's,
// 2026-10-14 - the user button and the portal's bar ran, then drew themselves
// with a <style> the browser refused, and spread unstyled over the page. The
// gateway tells the policy its nonce for styles too, and its scripts set that
// nonce on the styles they create: in a real browser, every style sheet of the
// injected elements applies, and the page reports no violation.
test('csp-styles: the injected elements keep their styles under a nonce-based style-src', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const id = 'e2e-csp-styles';
  const put = await root.put(`/api/routes/${id}`, {
    data: {
      id,
      name: id,
      order: 10, // ahead of the seeded catch-all
      enabled: true,
      isUi: true,
      ui: { userButton: { enabled: true } },
      upstream: HTTPBIN_URL,
      predicates: [{ type: 'path', args: { patterns: [`/${id}/**`] } }],
      filters: [
        { type: 'strip-prefix', args: { parts: 1 } },
        {
          type: 'set-response-header',
          args: {
            name: 'Content-Security-Policy',
            value: "script-src 'strict-dynamic' 'nonce-app'; style-src 'nonce-app'; frame-src 'self';, form-action 'self'",
          },
        },
      ],
    },
  });
  expect(put.ok(), await put.text()).toBeTruthy();

  const context = await browser.newContext();
  const page = await context.newPage();
  const violations: string[] = [];
  page.on('console', (m) => {
    if (/Content Security Policy|Content-Security-Policy/i.test(m.text())) violations.push(m.text());
  });
  await page.goto(`${DATA_URL}/${id}/html`);
  const button = page.locator('meerkat-user-button');
  await expect(button).toBeAttached();
  // Every <style> in the button's shadow root was accepted: a refused one has
  // no sheet.
  await expect
    .poll(() =>
      button.evaluate((el) => {
        const styles = [...(el.shadowRoot?.querySelectorAll('style') ?? [])];
        return styles.length > 0 && styles.every((s) => s.sheet !== null);
      }),
    )
    .toBe(true);
  expect(violations, 'the page reported policy violations').toEqual([]);

  await context.close();
  await root.delete(`/api/routes/${id}`);
  await root.dispose();
});
