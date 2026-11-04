import { expect, test } from '@playwright/test';
import { rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { Gateway, PASSWORD, postgres, work } from './lib';

// OBS-07. The Topology screen follows the cluster with nobody reloading it: an
// instance that stops leaves the drawing once its silence has lasted, and one
// that comes appears. Read in a real browser, on the live channel - the REST
// answer alone was right while the screen kept the stopped ones.
test.describe.configure({ mode: 'serial' });

const PG = 'meerkat-editions-pg-live';
const a = new Gateway(7);
const b = new Gateway(8);
let url = '';

const start = (g: Gateway) => g.start({ data: `${work}/live-${g.slot}`, databaseUrl: url });

test.beforeAll(async () => {
  url = await postgres(PG, 15498);
  for (const g of [a, b]) rmSync(`${work}/live-${g.slot}`, { recursive: true, force: true });
  await start(a);
  await start(b);
});

test.afterAll(async () => {
  await Promise.all([a.stop(), b.stop()]);
  spawnSync('docker', ['rm', '-f', PG]);
});

test('an instance that stops leaves the screen, and one that starts joins it, with no reload', async ({ browser }) => {
  const context = await browser.newContext();
  const signIn = await context.request.post(`${a.admin}/login`, {
    form: { username: 'admin', password: PASSWORD },
    maxRedirects: 0,
  });
  expect(signIn.status()).toBe(303);
  const page = await context.newPage();
  await page.goto(`${a.admin}/infra/topology`);
  const nodes = page.locator('.node:not(.db)');
  await expect(nodes, 'both instances, once each has heard the other').toHaveCount(2, { timeout: 20_000 });

  await b.stop();
  await expect(nodes, 'the stopped one gone, the page untouched').toHaveCount(1, { timeout: 45_000 });

  await start(b);
  await expect(nodes, 'the one that came back').toHaveCount(2, { timeout: 20_000 });
  await context.close();
});
