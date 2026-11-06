import { expect, request, type APIRequestContext, type Page } from '@playwright/test';
import { createHmac } from 'node:crypto';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { ADMIN_URL, DATA_URL } from '../playwright.config';
import { authFile } from './fixtures';

// What the user-flow suite shares: the mails the SMTP sink caught, the codes an
// authenticator would show, and throwaway accounts made the way an
// administrator makes them. Each test makes its own people, so nothing it does
// (a second factor, a passkey, a changed password) leaks into another one.

// ── the SMTP sink (scripts/smtp-sink.mjs) ─────────────────────────────────────

const MAIL_DIR = join(__dirname, '..', '.tmp', 'mail');

/** The bodies of the mails the sink stored for an address, oldest first. */
export function mailsTo(rcpt: string): string[] {
  if (!existsSync(MAIL_DIR)) return [];
  return readdirSync(MAIL_DIR)
    .sort((a, b) => parseInt(a, 10) - parseInt(b, 10) || a.localeCompare(b))
    .map((f) => JSON.parse(readFileSync(join(MAIL_DIR, f), 'utf8')) as { to: string[]; data: string })
    .filter((m) => m.to.some((t) => t.toLowerCase().includes(rcpt.toLowerCase())))
    .map((m) => m.data);
}

/** Waits until the address has received `count` mails, and returns the last. */
export async function mailNumber(rcpt: string, count: number): Promise<string> {
  await expect.poll(() => mailsTo(rcpt).length, { timeout: 15_000, message: `mail ${count} to ${rcpt}` }).toBe(count);
  return mailsTo(rcpt)[count - 1];
}

/** The six-digit code of a mail: the platform is plain HTTP, so every mailed
 *  step carries a code, never a link. */
export function codeIn(mail: string): string {
  const code = mail.match(/^\s*(\d{6})\s*$/m)?.[1] ?? mail.match(/\b(\d{6})\b/)?.[1];
  expect(code, 'a six-digit code in the mail').toBeTruthy();
  return code!;
}

/** Points the gateway's relay at the sink. */
export async function relayToSink(root: APIRequestContext): Promise<void> {
  const res = await root.put('/api/settings/mail-relay', {
    data: { host: '127.0.0.1', port: 12525, security: 'none', username: '', password: '', from: 'no-reply@e2e.test' },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

// ── an authenticator app (RFC 6238) ───────────────────────────────────────────

function base32(secret: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const c of secret.replace(/[\s=]/g, '').toUpperCase()) bits += alphabet.indexOf(c).toString(2).padStart(5, '0');
  const bytes: number[] = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  return Buffer.from(bytes);
}

/** The code an authenticator shows for this secret, at this time step. */
export function totp(secret: string, step = Math.floor(Date.now() / 30_000)): string {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(step));
  const mac = createHmac('sha1', base32(secret)).update(counter).digest();
  const at = mac[mac.length - 1] & 0x0f;
  const n = ((mac[at] & 0x7f) << 24) | (mac[at + 1] << 16) | (mac[at + 2] << 8) | mac[at + 3];
  return String(n % 1_000_000).padStart(6, '0');
}

/** Waits for the next time step and returns its code: a code is good once,
 *  so the step that confirmed an enrolment cannot also sign in. */
export async function nextTotp(secret: string): Promise<string> {
  const step = Math.floor(Date.now() / 30_000);
  while (Math.floor(Date.now() / 30_000) === step) await new Promise((r) => setTimeout(r, 250));
  return totp(secret);
}

// ── people ────────────────────────────────────────────────────────────────────

export const PASSWORD = 'Flow-User-Pass-1';

export async function rootApi(): Promise<APIRequestContext> {
  return request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
}

/** An account made as an administrator makes one, still holding its one-time
 *  password: its first sign-in is the forced change. */
export async function newAccount(
  root: APIRequestContext,
  name: string,
  extra: Record<string, unknown> = {},
): Promise<{ id: string; username: string; oneTime: string; email: string }> {
  const username = `${name}-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  const email = `${username}@e2e.test`;
  const res = await root.post('/api/users', { data: { username, email, enabled: true, ...extra } });
  expect(res.status(), `create ${username}: ${await res.text()}`).toBe(201);
  const body = (await res.json()) as { user: { id: string }; password: string };
  return { id: body.user.id, username, oneTime: body.password, email };
}

/** An account past its first sign-in: it holds PASSWORD. */
export async function readyAccount(
  root: APIRequestContext,
  name: string,
  extra: Record<string, unknown> = {},
): Promise<{ id: string; username: string; email: string }> {
  const a = await newAccount(root, name, extra);
  const ctx = await request.newContext({ baseURL: DATA_URL });
  const first = await ctx.post('/login', { form: { username: a.username, password: a.oneTime }, maxRedirects: 0 });
  expect(first.status(), 'first sign-in').toBe(303);
  const changed = await ctx.post('/update-password', { form: { password: PASSWORD, confirm: PASSWORD }, maxRedirects: 0 });
  expect(changed.status(), 'forced change').toBe(303);
  await ctx.dispose();
  return { id: a.id, username: a.username, email: a.email };
}

/** Fills the sign-in form and submits it. */
export async function signIn(page: Page, username: string, password: string, next?: string): Promise<void> {
  if (!page.url().includes('/login')) await page.goto(DATA_URL + '/login' + (next ? '?next=' + encodeURIComponent(next) : ''));
  await page.fill('input[name=username]', username);
  await page.fill('input[name=password]', password);
  await page.locator('form[action="login"] button[type=submit]').click();
}

/** The session cookie of the data plane, if any. */
export async function sessionCookie(page: Page): Promise<string | undefined> {
  return (await page.context().cookies()).find((c) => c.name.startsWith('MEERKAT_SESSION'))?.value;
}

/** Runs `body` with a route's access changed, and puts it back whatever happens. */
export async function withRouteAccess(
  root: APIRequestContext,
  routeId: string,
  access: Record<string, unknown>,
  body: () => Promise<void>,
): Promise<void> {
  const route = await (await root.get(`/api/routes/${routeId}`)).json();
  const put = await root.put(`/api/routes/${routeId}`, { data: { ...route, access } });
  expect(put.ok(), await put.text()).toBeTruthy();
  try {
    await body();
  } finally {
    const back = await (await root.get(`/api/routes/${routeId}`)).json();
    await root.put(`/api/routes/${routeId}`, { data: { ...back, access: route.access } });
  }
}

/** Runs `body` with some global settings changed, and puts them back. */
export async function withSettings(
  root: APIRequestContext,
  change: Record<string, unknown>,
  body: () => Promise<void>,
): Promise<void> {
  const settings = await (await root.get('/api/settings')).json();
  const put = await root.put('/api/settings', { data: { ...settings, ...change } });
  expect(put.ok(), await put.text()).toBeTruthy();
  try {
    await body();
  } finally {
    const now = await (await root.get('/api/settings')).json();
    const restore: Record<string, unknown> = { ...now };
    for (const k of Object.keys(change)) restore[k] = settings[k];
    await root.put('/api/settings', { data: restore });
  }
}

/** Opens self-registration, the captcha off (a robot test is a robot). */
export async function withSelfRegistration(root: APIRequestContext, body: () => Promise<void>): Promise<void> {
  const all = await (await root.get('/api/auth-providers')).json();
  const local = all.find((p: { id: string }) => p.id === 'local');
  const put = await root.put('/api/auth-providers/local', { data: { ...local, autoCreate: 'yes', captcha: false } });
  expect(put.ok(), await put.text()).toBeTruthy();
  try {
    await withSettings(root, { selfRegistration: true }, body);
  } finally {
    await root.put('/api/auth-providers/local', { data: local });
  }
}
