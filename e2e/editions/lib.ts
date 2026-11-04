// What the edition tests drive: real binaries, built here with the dates and
// the grace they need, real licenses, real gateways started and stopped by the
// tests themselves - the point is what a customer's install does with the file
// Softwarity sends, so nothing here is a mock.
//
// The licenses are the CI's, read from this machine or from the CI's secrets,
// NEVER from the repository: it is mirrored in public, and a license file that
// leaks cannot be revoked. Missing ones FAIL the run rather than skipping it:
// a skip is green, and a green run about nothing is what this suite exists to
// prevent.
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { request, type APIRequestContext } from '@playwright/test';

export const repo = join(__dirname, '..', '..');
export const work = join(__dirname, '..', '.editions');
export const PASSWORD = 'Editions-Root-Password-1';

// ─── licenses ───────────────────────────────────────────────────────────────

export type Kind = 'team' | 'enterprise';

// E2E_LICENSE_TEAM / E2E_LICENSE_ENTERPRISE name the files (the CI writes its
// secrets there); otherwise the CI licenses kept on this machine.
export function licenseFile(kind: Kind): string {
  const named = process.env[`E2E_LICENSE_${kind.toUpperCase()}`];
  if (named) {
    if (!existsSync(named)) throw new Error(`E2E_LICENSE_${kind.toUpperCase()} names ${named}, which does not exist`);
    return named;
  }
  const dir = `${homedir()}/.config/softwarity/licenses`;
  const found = existsSync(dir) ? readdirSync(dir).filter((f) => f.startsWith(`ci-test-license-${kind}-`) && f.endsWith('.lic')) : [];
  if (!found.length) {
    throw new Error(`no ${kind} license: set E2E_LICENSE_${kind.toUpperCase()} to a license file, ` +
      `or keep the CI's in ${dir}/ci-test-license-${kind}-*.lic`);
  }
  return `${dir}/${found[0]}`;
}

export const license = (kind: Kind) => readFileSync(licenseFile(kind), 'utf8');

// The same file with one character of its signed content changed: what a hand
// that edits a license to widen it produces. The envelope still parses, so what
// refuses it is the signature, not the format.
export function tampered(kind: Kind): string {
  const text = license(kind);
  const lines = text.trim().split('\n');
  const body = lines.slice(1, -1).join('');
  const envelope = JSON.parse(Buffer.from(body, 'base64').toString('utf8'));
  const at = Math.floor(envelope.enc.length / 2);
  const c = envelope.enc[at];
  envelope.enc = envelope.enc.slice(0, at) + (c === 'A' ? 'B' : 'A') + envelope.enc.slice(at + 1);
  const wrapped = Buffer.from(JSON.stringify(envelope)).toString('base64').match(/.{1,64}/g)!.join('\n');
  return `${lines[0]}\n${wrapped}\n${lines[lines.length - 1]}\n`;
}

// ─── binaries ───────────────────────────────────────────────────────────────

export type Build = 'now' | 'past-term' | 'short-grace';

// The binaries, by what they are for. `now` is dated today, as a release is;
// `past-term` is dated after the CI licenses' updates end; `short-grace` counts
// an overrun after a minute rather than fifteen - set at link time, the only
// way the grace can be changed at all.
export function binary(which: Build): string {
  return `${work}/bin/meerkat-${which}`;
}

export function buildAll(): void {
  mkdirSync(`${work}/bin`, { recursive: true });
  const now = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
  const builds: Record<Build, string[]> = {
    'now': [`-X github.com/softwarity/meerkat/internal/version.Date=${now}`],
    'past-term': ['-X github.com/softwarity/meerkat/internal/version.Date=2028-06-01T00:00:00Z'],
    'short-grace': [`-X github.com/softwarity/meerkat/internal/version.Date=${now}`,
      '-X github.com/softwarity/meerkat/internal/license.instanceGrace=1m'],
  };
  for (const [which, flags] of Object.entries(builds) as [Build, string[]][]) {
    const out = spawnSync('go', ['build', '-tags', 'ee', '-ldflags', flags.join(' '), '-o', binary(which), './cmd/meerkat'],
      { cwd: repo, stdio: 'inherit' });
    if (out.status !== 0) throw new Error(`building ${which} failed`);
  }
}

// ─── gateways ───────────────────────────────────────────────────────────────

export type Start = {
  build?: Build;
  // A directory of its own, kept across a restart when the test says so.
  data: string;
  databaseUrl?: string;
  // MEERKAT_LICENSE_FILE.
  licenseFile?: string;
  tenancy?: 'single' | 'multi';
  env?: Record<string, string>;
};

// One gateway, on its own block of ports so several run side by side and none
// meets a developer's `make dev` (9090) or the integration suite (19092).
export class Gateway {
  readonly data: string;
  readonly admin: string;
  readonly internal: string;
  private child?: ChildProcess;
  private out: string[] = [];
  private exit?: Promise<number>;
  private ctx?: APIRequestContext;

  constructor(readonly slot: number) {
    this.data = `http://localhost:${18300 + slot}`;
    this.admin = `http://localhost:${19300 + slot}`;
    this.internal = `http://localhost:${17300 + slot}`;
  }

  // Starts the process and returns without waiting: ready() or exited() say how
  // it went.
  launch(o: Start): void {
    mkdirSync(o.data, { recursive: true });
    const s = this.slot;
    const args = [
      '-addr', `:${18300 + s}`, '-admin-addr', `:${19300 + s}`, '-internal-addr', `:${17300 + s}`,
      '-tls-addr', `:${18400 + s}`, '-admin-tls-addr', `:${19400 + s}`, '-internal-tls-addr', `:${17400 + s}`,
      '-plug-addr', `:${22300 + s}`, '-data', o.data, '-tenancy', o.tenancy ?? 'single',
      ...(o.databaseUrl ? ['-database-url', o.databaseUrl] : []),
      ...(o.licenseFile ? ['-license', o.licenseFile] : []),
    ];
    this.out = [];
    this.child = spawn(binary(o.build ?? 'now'), args, {
      env: { ...process.env, MEERKAT_ADMIN_PASSWORD: PASSWORD, MEERKAT_LOG_LEVEL: 'info', ...o.env },
    });
    for (const stream of [this.child.stdout!, this.child.stderr!]) {
      stream.on('data', (b: Buffer) => this.out.push(...b.toString().split('\n').filter(Boolean)));
    }
    this.exit = new Promise((resolve) => this.child!.on('exit', (code) => resolve(code ?? 1)));
  }

  async start(o: Start): Promise<this> {
    this.launch(o);
    await this.ready();
    return this;
  }

  // Healthy, or the reason it is not: its own last lines.
  async ready(timeout = 60_000): Promise<void> {
    const until = Date.now() + timeout;
    let gone = false;
    void this.exit!.then(() => { gone = true; });
    while (Date.now() < until) {
      if (gone) throw new Error(`gateway ${this.slot} exited:\n${this.tail()}`);
      try {
        if ((await fetch(`${this.admin}/healthz`)).ok) return;
      } catch { /* not listening yet */ }
      await sleep(250);
    }
    throw new Error(`gateway ${this.slot} not healthy after ${timeout / 1000}s:\n${this.tail()}`);
  }

  // For a start that is meant to refuse: its exit code and what it said.
  async exited(timeout = 60_000): Promise<{ code: number; output: string }> {
    const code = await Promise.race([this.exit!, sleep(timeout).then(() => -1)]);
    if (code === -1) {
      await this.stop();
      throw new Error(`gateway ${this.slot} did not exit within ${timeout / 1000}s:\n${this.tail()}`);
    }
    return { code, output: this.out.join('\n') };
  }

  async stop(): Promise<void> {
    await this.ctx?.dispose();
    this.ctx = undefined;
    if (!this.child || this.child.exitCode !== null) return;
    this.child.kill('SIGTERM');
    await Promise.race([this.exit, sleep(15_000).then(() => this.child?.kill('SIGKILL'))]);
  }

  tail(n = 40): string {
    return this.out.slice(-n).join('\n');
  }

  // Root, signed in on the admin plane.
  async api(): Promise<APIRequestContext> {
    if (this.ctx) return this.ctx;
    const ctx = await request.newContext({ baseURL: this.admin });
    const res = await ctx.post('/login', { form: { username: 'admin', password: PASSWORD }, maxRedirects: 0 });
    if (res.status() !== 303) throw new Error(`gateway ${this.slot}: sign-in answered ${res.status()} ${await res.text()}`);
    this.ctx = ctx;
    return ctx;
  }

  async edition(): Promise<Edition> {
    const res = await (await this.api()).get('/api/edition');
    if (!res.ok()) throw new Error(`gateway ${this.slot}: /api/edition ${res.status()} ${await res.text()}`);
    return res.json();
  }

  // The Infrastructure screen's answer (OBS-07).
  async infrastructure(): Promise<Infrastructure> {
    const res = await (await this.api()).get('/api/infrastructure');
    if (!res.ok()) throw new Error(`gateway ${this.slot}: /api/infrastructure ${res.status()} ${await res.text()}`);
    return res.json();
  }

  // The notice on each surface a person sees: what the API says, the sign-in
  // page of the data plane, and the console's own page when this tree has a
  // console build in it (the watermark is CSS injected into its index.html).
  async notice(): Promise<{ api: boolean; page: boolean; console?: boolean }> {
    const api = !!(await this.edition()).notice;
    const page = (await (await fetch(`${this.data}/login`)).text()).includes('class="mk-eval"');
    const index = await (await (await this.api()).get('/')).text();
    const console = index.includes('<app-root') ? index.includes('content: "EVALUATION"') : undefined;
    return { api, page, console };
  }

  // The notice everywhere, or nowhere: anything in between is the bug.
  async noticeIs(shown: boolean): Promise<boolean> {
    const n = await this.notice();
    return n.api === shown && n.page === shown && (n.console === undefined || n.console === shown);
  }

  // Through the data plane, the route every test keeps answering: a license
  // never touches traffic.
  async routeAnswers(): Promise<number> {
    return (await fetch(`${this.data}/up/ping`)).status;
  }
}

export type Infrastructure = {
  instances: { id: string; host: string; version: string; ready: boolean; self?: boolean }[];
  versions: string[];
  database: { kind: string; location: string; schema?: number };
  synced: boolean;
  plan?: string;
  covers?: number;
  over?: { since: string; noticeAt: string };
};

export type Edition = {
  enterprise: boolean;
  tenancy: string;
  notice?: string;
  license?: { id?: string; licensee: string; plan?: string; tenants?: number; instances?: number;
    covered?: boolean; ends?: boolean; lapsed?: boolean };
  licenseSource?: string;
  licenseProblem?: string;
  licenseOver?: string[];
  hiddenTenants?: number;
};

// ─── what the gateways serve ────────────────────────────────────────────────

// The application behind the route: it answers, and that is all it has to do.
export async function upstream(port: number): Promise<Server> {
  const server = createServer((_, res) => {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end('upstream ok');
  });
  await new Promise<void>((resolve) => server.listen(port, resolve));
  return server;
}

export async function seedRoute(g: Gateway, upstreamPort: number): Promise<void> {
  const res = await (await g.api()).put('/api/routes/up', {
    data: {
      id: 'up', name: 'up', order: 100, enabled: true, upstream: `http://localhost:${upstreamPort}`,
      predicates: [{ type: 'path', args: { patterns: ['/up/**'] } }],
      filters: [{ type: 'strip-prefix', args: { parts: 1 } }], access: {},
    },
  });
  if (!res.ok()) throw new Error(`route: ${res.status()} ${await res.text()}`);
}

// ─── infrastructure ─────────────────────────────────────────────────────────

export function docker(args: string[], what: string): string {
  const out = spawnSync('docker', args, { encoding: 'utf8' });
  if (out.status !== 0) throw new Error(`${what}: docker ${args.join(' ')}\n${out.stderr || out.stdout}`);
  return out.stdout.trim();
}

// A PostgreSQL of its own, empty, through Google's cache of Docker Hub - the
// CI's Docker Hub quota is shared by every job.
export async function postgres(name: string, port: number): Promise<string> {
  spawnSync('docker', ['rm', '-f', name]);
  docker(['run', '-d', '--rm', '--name', name, '-e', 'POSTGRES_USER=meerkat', '-e', 'POSTGRES_PASSWORD=meerkat',
    '-e', 'POSTGRES_DB=meerkat', '-p', `${port}:5432`, 'mirror.gcr.io/library/postgres:18-alpine'], 'PostgreSQL');
  await eventually(async () => spawnSync('docker', ['exec', name, 'pg_isready', '-U', 'meerkat', '-h', '127.0.0.1']).status === 0,
    60_000, 'PostgreSQL to accept connections');
  return `postgres://meerkat:meerkat@localhost:${port}/meerkat?sslmode=disable`;
}

// A data directory as an older release left it: the community image of that
// version starts on it once, creates its schema, and is stopped. Run as this
// user, so the files it writes are this user's to open afterwards.
export async function databaseOf(image: string, dir: string): Promise<void> {
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  const name = 'meerkat-editions-old';
  spawnSync('docker', ['rm', '-f', name]);
  const uid = `${process.getuid!()}:${process.getgid!()}`;
  docker(['run', '-d', '--rm', '--name', name, '--user', uid, '-e', `MEERKAT_ADMIN_PASSWORD=${PASSWORD}`,
    '-v', `${dir}:/data`, '-p', '18399:8080', image], `starting ${image}`);
  try {
    await eventually(async () => {
      try {
        return (await fetch('http://localhost:18399/healthz')).ok;
      } catch {
        return false;
      }
    }, 120_000, `${image} to come up`);
  } finally {
    spawnSync('docker', ['stop', name]);
  }
}

export function copyDir(from: string, to: string): void {
  rmSync(to, { recursive: true, force: true });
  cpSync(from, to, { recursive: true });
}

export function writeFile(path: string, text: string): void {
  writeFileSync(path, text);
}

// ─── time ───────────────────────────────────────────────────────────────────

export const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

// Until it holds, or the reason it never did.
export async function eventually(check: () => Promise<boolean>, timeout: number, what: string, every = 2_000): Promise<void> {
  const until = Date.now() + timeout;
  let last: unknown;
  while (Date.now() < until) {
    try {
      if (await check()) return;
    } catch (e) {
      last = e;
    }
    await sleep(every);
  }
  throw new Error(`waited ${timeout / 1000}s for ${what}${last ? `: ${last}` : ''}`);
}
