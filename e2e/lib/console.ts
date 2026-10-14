import { expect, type Browser, type BrowserContext, type Locator, type Page, type WebSocket } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { seeded } from './fixtures';

// The gestures a console test is made of, written once: sign in through the
// form, watch the page for errors, open a dialog, read a snackbar, wait for the
// live channel. A spec that drives a screen says WHAT it does with these; how
// a Material dialog or the sign-in form is reached stays here, so a markup
// change is one fix and not forty.

// Sign in as a seeded profile THROUGH THE FORM on the admin plane - the path a
// person takes, which the storage states of the setup project skip - and land
// in the console. `profile` is a key of seeded().users (root, infra-admin,
// app-admin, tenant-admin, user, and the second organisation's people).
export async function signIn(browser: Browser, profile: string): Promise<{ context: BrowserContext; page: Page }> {
  const who = seeded().users[profile];
  if (!who) throw new Error(`no seeded profile "${profile}": known are ${Object.keys(seeded().users).join(', ')}`);
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/login');
  await page.locator('input[name="username"]').fill(who.username);
  await page.locator('input[name="password"]').fill(who.password);
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL((url) => !url.pathname.startsWith('/login'));
  await expect(page.locator('app-root')).not.toBeEmpty();
  return { context, page };
}

// Everything that went wrong in the page: uncaught exceptions, console errors
// (an Angular error, a socket refused) and failed requests, named by their
// method and path - the browser's own "Failed to load resource" line says
// neither, so it is replaced by the request. Start it BEFORE the first goto
// and assert with expectNoErrors at the end.
export function watchErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(`pageerror: ${e.message}${e.stack ? ' ' + e.stack.split('\n')[1]?.trim() : ''}`));
  page.on('console', (m) => {
    if (m.type() === 'error' && !m.text().startsWith('Failed to load resource')) errors.push(`console: ${m.text()}`);
  });
  page.on('response', (r) => {
    if (r.status() >= 400) errors.push(`http ${r.status()}: ${r.request().method()} ${new URL(r.url()).pathname}`);
  });
  return errors;
}

export function expectNoErrors(errors: string[], what: string): void {
  expect(errors, `${what}: the page reported errors`).toEqual([]);
}

// Click what opens a Material dialog and return the dialog, once it is shown.
// `trigger` is a locator, or the accessible name of a button.
export async function openDialog(page: Page, trigger: Locator | string | RegExp): Promise<Locator> {
  const button = typeof trigger === 'string' || trigger instanceof RegExp
    ? page.getByRole('button', { name: trigger })
    : trigger;
  const before = await page.locator('mat-dialog-container').count();
  await button.click();
  const dialog = page.locator('mat-dialog-container').nth(before);
  await expect(dialog).toBeVisible();
  return dialog;
}

// The text of the snackbar on screen, once there is one.
export async function snackbar(page: Page): Promise<string> {
  const bar = page.locator('mat-snack-bar-container').last();
  await expect(bar).toBeVisible();
  return (await bar.innerText()).trim();
}

export async function expectSnackbar(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.locator('mat-snack-bar-container').last()).toContainText(text);
}

// The console's live channel (/api/live): one websocket, every screen's
// subscriptions on it. Start watching BEFORE the navigation that opens it.
export interface LiveChannel {
  socket: WebSocket;
  // Every frame the server sent so far.
  frames: string[];
  // Resolves with the next frame the server sends that matches.
  next(match?: string | RegExp | ((payload: string) => boolean), timeout?: number): Promise<string>;
}

export async function watchLive(page: Page, timeout = 15_000): Promise<LiveChannel> {
  const socket = await page.waitForEvent('websocket', { predicate: (ws) => ws.url().includes('/api/live'), timeout });
  const frames: string[] = [];
  const waiting: Array<{ test: (p: string) => boolean; resolve: (p: string) => void }> = [];
  socket.on('framereceived', (f) => {
    const payload = typeof f.payload === 'string' ? f.payload : f.payload.toString('utf8');
    frames.push(payload);
    for (let i = waiting.length - 1; i >= 0; i--) {
      if (waiting[i].test(payload)) waiting.splice(i, 1)[0].resolve(payload);
    }
  });
  const next: LiveChannel['next'] = (match, wait = timeout) => {
    const test = !match
      ? () => true
      : typeof match === 'string'
        ? (p: string) => p.includes(match)
        : match instanceof RegExp
          ? (p: string) => match.test(p)
          : match;
    return new Promise((resolve, reject) => {
      const entry = { test, resolve };
      waiting.push(entry);
      setTimeout(() => {
        const i = waiting.indexOf(entry);
        if (i >= 0) {
          waiting.splice(i, 1);
          reject(new Error(`no live frame matching ${match ?? 'anything'} within ${wait} ms (got ${frames.length})`));
        }
      }, wait);
    });
  };
  return { socket, frames, next };
}

// Navigate the signed-in page to a console screen and wait for the router to
// have rendered something in it.
export async function openScreen(page: Page, path: string): Promise<void> {
  await page.goto(ADMIN_URL + path);
  await expect(page.locator('app-root')).not.toBeEmpty();
}
