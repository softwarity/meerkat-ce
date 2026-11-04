import { expect, test } from '@playwright/test';
import type { Server } from 'node:http';
import { mkdirSync, rmSync } from 'node:fs';
import { eventually, Gateway, license, seedRoute, upstream, work, writeFile } from './lib';

// EVAL-06. A license MOUNTED from a Secret (MEERKAT_LICENSE_FILE): the deployment owns
// it, the console shows it and takes no other, and a Secret updated in place is
// read again with no restart - which is how a renewal reaches a cluster.
test.describe.configure({ mode: 'serial' });

const gw = new Gateway(1);
const file = `${work}/mounted-secret/license.lic`;
let app: Server;

test.beforeAll(async () => {
  app = await upstream(18391);
  rmSync(`${work}/mounted`, { recursive: true, force: true });
  mkdirSync(`${work}/mounted-secret`, { recursive: true });
  writeFile(file, license('team'));
  await gw.start({ data: `${work}/mounted`, licenseFile: file });
  await seedRoute(gw, 18391);
});

test.afterAll(async () => {
  await gw.stop();
  app?.close();
  rmSync(`${work}/mounted-secret`, { recursive: true, force: true });
});

test.afterEach(async () => {
  expect(await gw.routeAnswers(), 'the route, after this step').toBe(200);
});

test('a mounted license is in force from the start, and said to come from the deployment', async () => {
  const e = await gw.edition();
  expect(e.licenseSource).toBe('file');
  expect(e.license?.plan).toBe('team');
  expect(await gw.noticeIs(false)).toBe(true);
});

test('the console neither takes another license nor removes the mounted one', async () => {
  const api = await gw.api();
  const pasted = await api.put('/api/license', { data: { license: license('enterprise') } });
  expect(pasted.status()).toBe(409);
  expect(await pasted.text()).toContain('MEERKAT_LICENSE_FILE');
  const removed = await api.delete('/api/license');
  expect(removed.status()).toBe(409);
  expect((await gw.edition()).license?.plan).toBe('team');
});

test('the Secret updated in place is read again, with no restart', async () => {
  writeFile(file, license('enterprise'));
  await eventually(async () => (await gw.edition()).license?.plan === 'enterprise', 45_000, 'the renewed file to be read');
  expect(await gw.noticeIs(false)).toBe(true);
});

test('a broken file in the Secret brings the notice back and says why, without stopping anything', async () => {
  writeFile(file, 'not a license any more');
  await eventually(async () => (await gw.edition()).licenseProblem?.includes('MEERKAT_LICENSE_FILE') ?? false,
    45_000, 'the broken file to be reported');
  await eventually(() => gw.noticeIs(true), 45_000, 'the notice to come back');

  writeFile(file, license('team'));
  await eventually(() => gw.noticeIs(false), 45_000, 'the repaired file to take the notice off again');
});
