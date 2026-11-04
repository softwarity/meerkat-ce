import { expect, test } from '@playwright/test';
import type { Server } from 'node:http';
import { rmSync } from 'node:fs';
import { copyDir, databaseOf, Gateway, license, licenseFile, seedRoute, upstream, work } from './lib';

// EVAL-06. The term of a license: the updates it pays for. A version built after it is
// an evaluation for as long as it runs, the version from the term restores the
// license, and a version past the term that would migrate the database stops
// before touching it - the one step that cannot be undone by going back.
test.describe.configure({ mode: 'serial' });

const gw = new Gateway(2);
let app: Server;

test.beforeAll(async () => {
  app = await upstream(18392);
});

test.afterAll(async () => {
  await gw.stop();
  app?.close();
});

test('a version built after the term is an evaluation, and the version from the term restores the license', async () => {
  const data = `${work}/term`;
  rmSync(data, { recursive: true, force: true });
  await gw.start({ build: 'past-term', data });
  await seedRoute(gw, 18392);
  const res = await (await gw.api()).put('/api/license', { data: { license: license('team') } });
  expect(res.status(), await res.text()).toBe(200);
  const past = await gw.edition();
  expect(past.license).toMatchObject({ plan: 'team', covered: false, lapsed: true });
  expect(past.licenseOver?.join(' ')).toContain('built after the updates');
  expect(await gw.noticeIs(true)).toBe(true);
  expect(await gw.routeAnswers()).toBe(200);
  await gw.stop();

  // Back to a version from the term, on the same data: nothing was written
  // down, so there is nothing to undo.
  await gw.start({ build: 'now', data });
  const back = await gw.edition();
  expect(back.license).toMatchObject({ plan: 'team', covered: true });
  expect(back.license?.lapsed).toBeFalsy();
  expect(await gw.noticeIs(false)).toBe(true);
  expect(await gw.routeAnswers()).toBe(200);
  await gw.stop();
});

// A database as 1.19.0 left it (schema 97), and the version past the term
// holding a license: it would migrate, so it refuses, says why and how to go
// on, and writes nothing - asked again, it says the same thing.
test('a version past the term stops before a migration, writing nothing, until it is told to go on', async () => {
  const old = `${work}/old-1.19.0`;
  await databaseOf('docker.io/softwarity/meerkat-ce:1.19.0', old);
  const data = `${work}/migrate`;
  copyDir(old, data);
  const mounted = { data, licenseFile: licenseFile('team'), build: 'past-term' as const };

  for (const attempt of ['first', 'second']) {
    gw.launch(mounted);
    const { code, output } = await gw.exited();
    expect(code, `${attempt} attempt: ${output}`).not.toBe(0);
    expect(output, attempt).toContain('MEERKAT_MIGRATE_PAST_LICENSE');
    expect(output, attempt).toContain('nothing was changed');
    expect(output, `${attempt}: still the schema 1.19.0 left`).toContain('97');
  }

  // Told to: it migrates, runs as an evaluation, and leaves the trail.
  await gw.start({ ...mounted, env: { MEERKAT_MIGRATE_PAST_LICENSE: 'true' } });
  expect((await gw.edition()).license).toMatchObject({ plan: 'team', lapsed: true });
  expect(await gw.noticeIs(true)).toBe(true);
  const audit = await (await (await gw.api()).get('/api/audit?limit=200')).text();
  expect(audit).toContain('license.migrate');
  await gw.stop();

  // The version from the term migrates the same database without asking.
  copyDir(old, data);
  await gw.start({ data, licenseFile: licenseFile('team'), build: 'now' });
  expect(await gw.noticeIs(false)).toBe(true);
  await gw.stop();
});
