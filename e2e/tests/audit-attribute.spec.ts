import { expect, request, test, type APIRequestContext } from '@playwright/test';
import { ADMIN_URL, HTTPBIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// What an audit event says it audits (AUD-03, AUD-04): a resource attribute,
// `audit` by default, whose value is a template - `{servicename}` by default,
// the service behind the route (the first label of its upstream's host). The
// installation sets the attribute and the template, a route with an OpenAPI
// spec may set its own value. A log line carries nothing, so a log backend
// tells the two apart with one selector.
//
// What is proven here: the two settings are taken and refused with the
// characters a label value can hold, the route keeps its own value, and both
// screens show the value RESOLVED on a real service, so the form of
// {servicename} is seen rather than explained.

const ROUTE = 'e2e-audit-attr';
// httpbin answers on localhost: the service behind the route is `localhost`.
const SERVICE = 'localhost';

const SPEC = {
  openapi: '3.1.0',
  info: { title: 'Audit attribute e2e', version: '1.0.0' },
  paths: {
    '/anything': {
      get: { operationId: 'anything', summary: 'Echo the request', responses: { '200': { description: 'The request' } } },
    },
  },
};

type Telemetry = Record<string, unknown> & { auditStreamKey?: string; auditStream?: string };

async function telemetry(root: APIRequestContext): Promise<Telemetry> {
  const res = await root.get('/api/settings/telemetry');
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

// The PUT takes back what the GET gave, read-only fields included (they are
// ignored): the round trip the console makes.
async function putTelemetry(root: APIRequestContext, cfg: Telemetry) {
  return root.put('/api/settings/telemetry', { data: cfg });
}

async function saveRoute(root: APIRequestContext, extra: Record<string, unknown> = {}) {
  return root.put(`/api/routes/${ROUTE}`, {
    data: {
      id: ROUTE,
      name: 'Audit attribute',
      order: 30, // ahead of the seeded catch-all
      enabled: true,
      upstream: HTTPBIN_URL,
      predicates: [{ type: 'path', args: { patterns: [`/${ROUTE}/**`] } }],
      filters: [{ type: 'strip-prefix', args: { parts: 1 } }],
      ...extra,
    },
  });
}

test('audit attribute: the installation sets the attribute and its template, within what a label takes', async () => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await telemetry(root);
  try {
    const ok = await putTelemetry(root, { ...before, auditStreamKey: ' audit_trail ', auditStream: '{servicename}.prod' });
    expect(ok.ok(), await ok.text()).toBeTruthy();
    const saved = await telemetry(root);
    expect(saved.auditStreamKey).toBe('audit_trail');
    expect(saved.auditStream).toBe('{servicename}.prod');

    // A space, a brace that is not the token, an equals sign: none is a label
    // value, and the refusal says what is allowed.
    for (const auditStream of ['orders api', '{service}', 'a=b']) {
      const res = await putTelemetry(root, { ...before, auditStream });
      expect(res.status(), auditStream).toBe(422);
      expect(await res.text()).toContain('{servicename}');
    }
    const badKey = await putTelemetry(root, { ...before, auditStreamKey: 'audit=x' });
    expect(badKey.status()).toBe(422);
    expect(await badKey.text()).toContain('auditStreamKey');
  } finally {
    await putTelemetry(root, before);
    await root.dispose();
  }
});

test('audit attribute: a route keeps its own value, trimmed, and refuses one a label cannot hold', async () => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  try {
    const ok = await saveRoute(root, { auditStream: ' orders.{servicename} ' });
    expect(ok.ok(), await ok.text()).toBeTruthy();
    expect((await ok.json()).auditStream).toBe('orders.{servicename}');

    const bad = await saveRoute(root, { auditStream: 'orders/api' });
    expect(bad.ok()).toBeFalsy();
    expect(await bad.text()).toContain('auditStream');
  } finally {
    await root.delete(`/api/routes/${ROUTE}`);
    await root.dispose();
  }
});

test('audit attribute: the Audit tab resolves the template on a real service', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await telemetry(root);
  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  try {
    const reset = await putTelemetry(root, { ...before, auditStreamKey: '', auditStream: '' });
    expect(reset.ok(), await reset.text()).toBeTruthy();

    await page.goto(ADMIN_URL + '/infra/opentelemetry');
    await page.getByRole('tab', { name: 'Audit' }).click();
    await expect(page.getByText('Selector in your log backend')).toBeVisible();
    // Blank fields show the defaults as placeholders, and the example uses them.
    const value = page.getByPlaceholder('{servicename}');
    await expect(page.getByPlaceholder('audit', { exact: true })).toBeVisible();
    await expect(page.getByText(/^e\.g\. audit=\S+$/)).toBeVisible();

    // Committed on change, like every switch of the screen.
    const saved = page.waitForResponse((r) => r.url().endsWith('/api/settings/telemetry') && r.request().method() === 'PUT');
    await value.fill('{servicename}.e2e');
    await value.press('Tab');
    expect((await saved).status()).toBe(200);
    await expect(page.getByText(/^e\.g\. audit=\S+\.e2e$/)).toBeVisible();
    expect((await telemetry(root)).auditStream).toBe('{servicename}.e2e');
  } finally {
    await context.close();
    await putTelemetry(root, before);
    await root.dispose();
  }
});

test('audit attribute: the route offers its own value only once it has an OpenAPI spec', async ({ browser }) => {
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await telemetry(root);
  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  try {
    const reset = await putTelemetry(root, { ...before, auditStreamKey: '', auditStream: '' });
    expect(reset.ok(), await reset.text()).toBeTruthy();
    const created = await saveRoute(root);
    expect(created.ok(), await created.text()).toBeTruthy();

    // No spec: the section says where one is set, and neither offers the
    // Endpoint audit screen nor a value nothing would ever carry.
    // The left menu has an Endpoint audit entry of its own: the editor only.
    const editor = page.locator('app-route-editor');
    await page.goto(ADMIN_URL + `/infra/routes/${ROUTE}/tracing`);
    await expect(editor.getByText('Needs an OpenAPI spec')).toBeVisible();
    await expect(editor.getByRole('link', { name: 'Endpoint audit' })).toHaveCount(0);
    await expect(page.getByLabel('Audited as')).toHaveCount(0);

    const deposit = await root.put(`/api/routes/${ROUTE}/spec`, { data: SPEC });
    expect(deposit.ok(), await deposit.text()).toBeTruthy();

    // With one: the link, and the field with the installation's value resolved
    // for THIS route's service as its placeholder.
    await page.reload();
    await expect(editor.getByRole('link', { name: 'Endpoint audit' })).toBeVisible();
    const field = page.getByLabel('Audited as');
    await expect(field).toHaveAttribute('placeholder', SERVICE);
    await expect(page.getByText(`Sent as audit=${SERVICE}`)).toBeVisible();

    await field.fill('orders.{servicename}');
    await expect(page.getByText(`Sent as audit=orders.${SERVICE}`)).toBeVisible();
    const saved = page.waitForResponse((r) => r.url().includes(`/api/routes/${ROUTE}`) && r.request().method() === 'PUT');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect((await saved).status()).toBe(200);
    const route = await (await root.get(`/api/routes/${ROUTE}`)).json();
    expect(route.auditStream).toBe('orders.{servicename}');
  } finally {
    await context.close();
    await root.delete(`/api/routes/${ROUTE}`);
    await putTelemetry(root, before);
    await root.dispose();
  }
});
