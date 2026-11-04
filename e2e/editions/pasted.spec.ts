import { expect, test } from '@playwright/test';
import type { Server } from 'node:http';
import { rmSync } from 'node:fs';
import { eventually, Gateway, license, seedRoute, tampered, upstream, work } from './lib';

// EVAL-06. One installation through its whole life with a license PASTED on its License
// screen: an evaluation, then a Team, then none, then an Enterprise - and the
// route answering through every step, since a license never touches traffic.
test.describe.configure({ mode: 'serial' });

const gw = new Gateway(0);
let app: Server;

test.beforeAll(async () => {
  app = await upstream(18390);
  rmSync(`${work}/pasted`, { recursive: true, force: true });
  await gw.start({ data: `${work}/pasted`, tenancy: 'multi' });
  await seedRoute(gw, 18390);
});

test.afterAll(async () => {
  await gw.stop();
  app?.close();
});

test.afterEach(async () => {
  expect(await gw.routeAnswers(), 'the route, after this step').toBe(200);
});

const put = async (text: string) => (await gw.api()).put('/api/license', { data: { license: text } });
const tenantCount = async () => ((await (await (await gw.api()).get('/api/tenants')).json()) as unknown[]).length;
const tenancy = async (to: 'single' | 'multi') => (await gw.api()).put('/api/settings/tenancy', { data: { tenancy: to } });
const createTenant = async (name: string) => (await gw.api()).post('/api/tenants', { data: { name } });
const importTenant = async (id: string) =>
  (await gw.api()).post('/api/config/import', { data: { version: 1, tenants: [{ id, name: id }] } });

test('without a license, the build is an evaluation on every surface', async () => {
  const e = await gw.edition();
  expect(e.enterprise).toBe(true);
  expect(e.license).toBeUndefined();
  // All three surfaces, the console's included: global-setup put it in the
  // binary, so a check that quietly skipped it would be a hole.
  expect(await gw.notice()).toEqual({ api: true, page: true, console: true });
  expect(await gw.noticeIs(true)).toBe(true);
  const infra = await gw.infrastructure();
  expect(infra.instances).toHaveLength(1);
  expect(infra).toMatchObject({ synced: false, database: { kind: 'embedded' } });
});

test('a file that is not a license, or one edited by hand, is refused and the notice stays', async () => {
  const junk = await put('this is not a license');
  expect(junk.status()).toBe(422);
  expect(await junk.text()).toContain('not a license file');

  const edited = await put(tampered('team'));
  expect(edited.status()).toBe(422);
  expect(await edited.text()).toContain('signature');

  expect((await gw.edition()).license).toBeUndefined();
  expect(await gw.noticeIs(true)).toBe(true);
});

test('a Team license on a gateway that already serves several tenants leaves them served and keeps the notice', async () => {
  expect((await createTenant('Second')).ok(), 'an evaluation has no ceiling').toBe(true);
  const before = await tenantCount();
  expect(before).toBeGreaterThan(1);

  const res = await put(license('team'));
  expect(res.status(), await res.text()).toBe(200);
  const e = await gw.edition();
  expect(e.license?.plan).toBe('team');
  expect(e.license).toMatchObject({ tenants: 1, instances: 1 });
  expect(e.licenseOver?.join(' ')).toContain('tenants are served');
  expect(await gw.noticeIs(true)).toBe(true);
  expect(await tenantCount(), 'nobody is cut').toBe(before);
});

test('down to one tenant, the Team license takes the notice off, and refuses another tenant every way', async () => {
  const down = await tenancy('single');
  expect(down.status(), await down.text()).toBe(200);
  await eventually(() => gw.noticeIs(false), 10_000, 'the notice to go once one tenant is served');
  const e = await gw.edition();
  expect(e.licenseOver ?? []).toEqual([]);
  expect(e.hiddenTenants, 'held back, not deleted').toBeGreaterThan(0);

  const created = await createTenant('Third');
  expect(created.status()).toBe(403);
  expect(await created.text()).toContain('Team license covers 1');

  const imported = await importTenant('imported');
  expect(imported.status()).toBe(422);
  expect(await imported.text()).toContain('Team license covers 1');

  const up = await tenancy('multi');
  expect(up.status()).toBe(403);
  expect((await gw.edition()).tenancy).toBe('single');
});

test('removing the license brings the notice back and changes nothing else', async () => {
  const tenants = await tenantCount();
  const res = await (await gw.api()).delete('/api/license');
  expect(res.status(), await res.text()).toBe(200);
  await eventually(() => gw.noticeIs(true), 10_000, 'the notice to come back');
  const e = await gw.edition();
  expect(e.license).toBeUndefined();
  expect(e.tenancy, 'the shape is the installation\'s, not the license\'s').toBe('single');
  expect(await tenantCount()).toBe(tenants);
  // An evaluation again: no ceiling.
  expect((await tenancy('multi')).status()).toBe(200);
});

test('an Enterprise license takes the notice off, ceilings and all', async () => {
  const res = await put(license('enterprise'));
  expect(res.status(), await res.text()).toBe(200);
  await eventually(() => gw.noticeIs(false), 10_000, 'the notice to go');
  const e = await gw.edition();
  expect(e.license?.plan).toBe('enterprise');
  expect(e.licenseOver ?? []).toEqual([]);
  expect(e.tenancy).toBe('multi');

  expect((await createTenant('Fourth')).ok(), 'no tenant ceiling').toBe(true);
  const imported = await importTenant('imported');
  expect(imported.status(), await imported.text()).toBe(200);
  expect(await gw.noticeIs(false)).toBe(true);
});

test('a refused file does not replace the license in force', async () => {
  expect((await put(tampered('enterprise'))).status()).toBe(422);
  expect((await gw.edition()).license?.plan).toBe('enterprise');
  expect(await gw.noticeIs(false)).toBe(true);
});
