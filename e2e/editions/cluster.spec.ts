import { expect, test } from '@playwright/test';
import type { Server } from 'node:http';
import { rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { eventually, Gateway, license, postgres, seedRoute, sleep, upstream, work } from './lib';

// EVAL-06, STORE-03. Three gateways on one PostgreSQL: a cluster, which is the Enterprise's. A
// license pasted on one node is in force on all of them; a Team license, which
// covers one instance, lets the notice come back once the overrun has outlasted
// the grace - a minute in this build, fifteen in the one that ships - and
// never touches traffic; an Enterprise license covers the three.
test.describe.configure({ mode: 'serial' });

const PG = 'meerkat-editions-pg';
const nodes = [new Gateway(4), new Gateway(5), new Gateway(6)];
const [a, b, c] = nodes;
let url = '';
let app: Server;

const start = (g: Gateway) =>
  g.start({ build: 'short-grace', data: `${work}/cluster-${g.slot}`, databaseUrl: url });

test.beforeAll(async () => {
  app = await upstream(18394);
  url = await postgres(PG, 15499);
  for (const g of nodes) rmSync(`${work}/cluster-${g.slot}`, { recursive: true, force: true });
  // The first one creates the schema and root; the others join it.
  await start(a);
  await Promise.all([start(b), start(c)]);
  await seedRoute(a, 18394);
});

test.afterAll(async () => {
  await Promise.all(nodes.map((g) => g.stop()));
  app?.close();
  spawnSync('docker', ['rm', '-f', PG]);
});

const all = async (check: (g: Gateway) => Promise<boolean>) => (await Promise.all(nodes.map(check))).every(Boolean);
const routesAnswer = async () => {
  for (const g of nodes) expect(await g.routeAnswers(), `the route through gateway ${g.slot}`).toBe(200);
};

test('three nodes, one route, the notice on each: an evaluation', async () => {
  await eventually(async () => (await Promise.all(nodes.map((g) => g.routeAnswers()))).every((s) => s === 200),
    30_000, 'the route saved on one node to be served by all three');
  expect(await all((g) => g.noticeIs(true))).toBe(true);
  // Each node sees the three, itself marked, on the database they share - the
  // URL's password nowhere in the answer.
  // A node hears the others within one word, every five seconds.
  await eventually(() => all(async (g) => (await g.infrastructure()).instances.length === 3), 15_000,
    'every node to hear the two others');
  for (const g of nodes) {
    const infra = await g.infrastructure();
    expect(infra.instances.filter((n) => n.self), `gateway ${g.slot}`).toHaveLength(1);
    expect(infra).toMatchObject({ synced: true, database: { kind: 'postgres' } });
    expect(JSON.stringify(infra)).not.toContain('meerkat:meerkat@');
  }
});

test('a Team license pasted on one node is in force on the others, then the overrun brings the notice back', async () => {
  const res = await (await a.api()).put('/api/license', { data: { license: license('team') } });
  expect(res.status(), await res.text()).toBe(200);
  // Within the grace: licensed, everywhere.
  await eventually(() => all((g) => g.noticeIs(false)), 30_000, 'the license to reach every node');
  for (const g of nodes) expect((await g.edition()).license?.plan, `gateway ${g.slot}`).toBe('team');

  // Then nothing but waiting - no traffic through the route, on purpose: a
  // node that carries none is still a running node, and it used to fall out of
  // the count after twenty seconds of silence.
  await eventually(() => all((g) => g.noticeIs(true)), 200_000, 'the overrun to bring the notice back on every node', 5_000);
  for (const g of nodes) {
    expect((await g.edition()).licenseOver?.join(' '), `gateway ${g.slot}`).toContain('3 instances are running');
    expect((await g.infrastructure()).over, `gateway ${g.slot}: the screen says it too`).toBeTruthy();
  }
  await routesAnswer();
});

test('back to one instance, the Team license is whole again', async () => {
  await Promise.all([b.stop(), c.stop()]);
  await eventually(async () => (await a.infrastructure()).instances.length === 1, 40_000,
    'the stopped nodes to leave the list');
  await eventually(() => a.noticeIs(false), 120_000, 'the notice to go once one node is left', 5_000);
  expect((await a.infrastructure()).over).toBeUndefined();
  expect((await a.edition()).licenseOver ?? []).toEqual([]);
  expect(await a.routeAnswers()).toBe(200);
});

test('an Enterprise license covers the three, past the grace', async () => {
  const res = await (await a.api()).put('/api/license', { data: { license: license('enterprise') } });
  expect(res.status(), await res.text()).toBe(200);
  await Promise.all([start(b), start(c)]);
  await eventually(() => all((g) => g.noticeIs(false)), 30_000, 'the license to be in force on the nodes that joined');
  // Longer than the grace and a look of the watch: a Team would have its
  // notice back by now.
  await sleep(100_000);
  expect(await all((g) => g.noticeIs(false))).toBe(true);
  for (const g of nodes) expect((await g.edition()).licenseOver ?? [], `gateway ${g.slot}`).toEqual([]);
  await routesAnswer();
});

test('removed on one node, the license is gone from all of them', async () => {
  const res = await (await b.api()).delete('/api/license');
  expect(res.status(), await res.text()).toBe(200);
  await eventually(() => all((g) => g.noticeIs(true)), 45_000, 'the notice to come back on every node');
  await routesAnswer();
});
