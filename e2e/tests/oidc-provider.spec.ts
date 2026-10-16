import { expect, request, test, type APIRequestContext, type Browser } from '@playwright/test';
import { createHash, createPublicKey, randomBytes, verify } from 'node:crypto';
import { ADMIN_URL, DATA_URL, HTTPBIN_URL, INTERNAL_URL } from '../playwright.config';
import { authFile, ENTERPRISE, seeded } from '../lib/fixtures';

// The gateway as the OpenID provider of the applications behind its routes.
//
// The test plays a GENERIC relying party, nothing a particular application
// does: a browser sent to the authorize endpoint with PKCE, the code read off
// the redirect, exchanged from a server's side, and the id_token decoded and
// verified against the provider's keys. Who may sign in, and what the tokens
// say, is the ROUTES' business: two routes on one client, the first letting in
// one named person only, the second everyone signed in - the same fall-through
// as the routing itself, and so two claim sets for two people.
//
// A client declares no redirect URI: the application is served by those
// routes, and the provider accepts an address under their path on the
// gateway's public address, nowhere else. A lookalike of that path is refused.
//
// The issuer is one of the application plane's public names (Domains & TLS):
// the test declares localhost, which gives http://localhost on the data
// plane's port - DATA_URL. Its own project, after everything else
// (playwright.config.ts): the public name moves the issuer the whole gateway
// signs with, and while a client exists the data plane answers the discovery
// path in front of the routes. A spec reading the issuer beside it would read
// this one's.

const CLIENT = 'e2e-oidc-app';
const SECOND = 'e2e-oidc-other';
const REDIRECT = `${DATA_URL}/e2e-oidc/anything/oidc-callback`;
const ROUTES = ['e2e-oidc-admins', 'e2e-oidc-everyone'];

const b64url = (b: Buffer) => b.toString('base64url');

function decode(jwt: string): { header: Record<string, unknown>; claims: Record<string, unknown> } {
  const [h, p] = jwt.split('.');
  return {
    header: JSON.parse(Buffer.from(h, 'base64url').toString('utf8')),
    claims: JSON.parse(Buffer.from(p, 'base64url').toString('utf8')),
  };
}

// The signature, checked the way any library does: the key named by the
// header's kid, taken from the provider's JWKS.
async function verifySignature(jwt: string): Promise<void> {
  const keys = await request.newContext();
  const jwks = (await (await keys.get(`${DATA_URL}/meerkat/oidc/jwks`)).json()) as { keys: Array<Record<string, string>> };
  await keys.dispose();
  const { header } = decode(jwt);
  expect(header.alg).toBe('RS256');
  const jwk = jwks.keys.find((k) => k.kid === header.kid);
  expect(jwk, `kid ${String(header.kid)} in the JWKS`).toBeTruthy();
  const [h, p, s] = jwt.split('.');
  const ok = verify('RSA-SHA256', Buffer.from(`${h}.${p}`), createPublicKey({ key: jwk!, format: 'jwk' }), Buffer.from(s, 'base64url'));
  expect(ok, 'the id_token verifies against the provider keys').toBe(true);
}

// The authorize request an application sends its people to.
function authorizeUrl(redirect: string, challenge: string, state: string, nonce: string): string {
  const authorize = new URL(`${DATA_URL}/meerkat/oidc/authorize`);
  for (const [k, v] of Object.entries({
    response_type: 'code',
    client_id: CLIENT,
    redirect_uri: redirect,
    scope: 'openid profile',
    state,
    nonce,
    code_challenge: challenge,
    code_challenge_method: 'S256',
  })) {
    authorize.searchParams.set(k, v);
  }
  return authorize.toString();
}

// One person through the code flow: signed in at the gateway, sent to
// authorize, back at the application with a code.
async function signIn(browser: Browser, username: string, password: string) {
  const context = await browser.newContext();
  const page = await context.newPage();
  const login = await page.request.post(`${DATA_URL}/login`, { form: { username, password }, maxRedirects: 0 });
  expect(login.status(), `login ${username}`).toBe(303);

  const verifier = b64url(randomBytes(32));
  const challenge = b64url(createHash('sha256').update(verifier).digest());
  const state = b64url(randomBytes(12));
  const nonce = b64url(randomBytes(12));
  await page.goto(authorizeUrl(REDIRECT, challenge, state, nonce));
  // The application's address under the routes, with the code and the state
  // it sent.
  await expect(page).toHaveURL((u) => u.toString().startsWith(REDIRECT));
  const back = new URL(page.url());
  expect(back.searchParams.get('state')).toBe(state);
  const code = back.searchParams.get('code');
  expect(code, 'a code on the redirect').toBeTruthy();

  // An address under no route of the client, a lookalike of its path, gets
  // the gateway's page and no code.
  const refused = await page.goto(authorizeUrl(`${DATA_URL}/e2e-oidcx/anything/cb`, challenge, state, nonce));
  expect(refused?.status(), 'a lookalike path is refused').toBe(400);
  expect(page.url()).toContain('/meerkat/oidc/authorize');
  await context.close();
  return { code: code!, verifier, nonce };
}

// The application's server, exchanging the code with its secret.
async function exchange(base: string, secret: string, code: string, verifier: string) {
  const server = await request.newContext();
  const res = await server.post(`${base}/meerkat/oidc/token`, {
    headers: { Authorization: 'Basic ' + Buffer.from(`${CLIENT}:${secret}`).toString('base64') },
    form: { grant_type: 'authorization_code', code, redirect_uri: REDIRECT, code_verifier: verifier },
  });
  const body = await res.json();
  await server.dispose();
  return { status: res.status(), body };
}

// The application plane's door as found, put back afterwards.
interface Door {
  proxied: boolean;
  names: string[];
  proxyAddresses: string[];
}

async function appDoor(root: APIRequestContext): Promise<Door> {
  const tls = (await (await root.get('/api/settings/tls')).json()) as { doors: { app: Door } };
  const { proxied, names, proxyAddresses } = tls.doors.app;
  return { proxied, names, proxyAddresses };
}

async function cleanUp(root: APIRequestContext, door: Door, certId?: string): Promise<void> {
  for (const id of ROUTES) await root.delete(`/api/routes/${id}`);
  for (const id of [CLIENT, SECOND]) await root.delete(`/api/oidc/clients/${id}`);
  await root.put('/api/settings/tls/doors/app', { data: door });
  if (certId) await root.delete(`/api/certificates/${certId}`);
}

test('oidc provider: an application signs two people in through two routes, and the drawer lists it', async ({ browser }) => {
  test.skip(!ENTERPRISE, 'the provider is Enterprise: the community refusal is the next test');
  const s = seeded();
  const alice = s.users['user'];
  const tadmin = s.users['tenant-admin'];
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const before = await appDoor(root);
  await cleanUp(root, before);
  let certId: string | undefined;

  try {
    // The issuer is the application plane's public name: localhost, served by
    // the gateway itself with a certificate that covers it, which gives
    // http://localhost on the data plane's port. The provider lives there.
    const cert = await root.post('/api/certificates/self-signed', { data: { names: ['localhost'], app: true } });
    expect(cert.status(), await cert.text()).toBe(201);
    certId = ((await cert.json()) as { id: string }).id;
    const door = await root.put('/api/settings/tls/doors/app', { data: { proxied: false, names: ['localhost'], proxyAddresses: [] } });
    expect(door.ok(), await door.text()).toBeTruthy();
    const keys = (await (await root.get('/api/identity/signing-keys')).json()) as { issuer: string; issuers: string[]; publicDiscovery?: string };
    expect(keys.issuers).toEqual([DATA_URL]);
    expect(keys.issuer).toBe(DATA_URL);
    // A free-text issuer is no longer a thing: refused, naming the address.
    const free = await root.put('/api/identity/issuer', { data: { issuer: 'http://meerkat:7070' } });
    expect(free.status()).toBe(422);
    expect(await free.text()).toContain(DATA_URL);
    // A client declares no return address, and one passed is refused.
    const declared = await root.post('/api/oidc/clients', {
      data: { id: CLIENT, confidential: true, redirectUris: [`${DATA_URL}/cb`] },
    });
    expect(declared.status()).toBe(422);
    expect(await declared.text()).toContain('a client declares no return address');
    const created = await root.post('/api/oidc/clients', {
      data: { id: CLIENT, confidential: true },
    });
    expect(created.status(), await created.text()).toBe(201);
    const { secret } = (await created.json()) as { secret: string };
    expect(secret).toBeTruthy();

    // Two routes on the same paths and the same client. The first lets in
    // tadmin alone, and its tokens say so; the second, everyone signed in.
    // Each claim set has a fixed value and a fact of the person.
    const path = [{ type: 'path', args: { patterns: ['/e2e-oidc/**'] } }];
    const routes = [
      {
        id: ROUTES[0], name: ROUTES[0], order: 20, enabled: true, upstream: HTTPBIN_URL, predicates: path, filters: [],
        access: { level: 'deny', users: [tadmin.username] },
        identity: {
          mechanism: 'oidc', client: CLIENT, attributes: [{ field: 'username' }],
          claims: [{ name: 'level', value: 'admin' }, { name: 'preferred_username', field: 'username' }],
        },
      },
      {
        id: ROUTES[1], name: ROUTES[1], order: 21, enabled: true, upstream: HTTPBIN_URL, predicates: path, filters: [],
        access: { level: 'auth' },
        identity: {
          mechanism: 'oidc', client: CLIENT, attributes: [{ field: 'fullname' }],
          claims: [{ name: 'level', value: 'user' }, { name: 'name', field: 'fullname' }],
        },
      },
    ];
    for (const r of routes) {
      const res = await root.put(`/api/routes/${r.id}`, { data: r });
      expect(res.ok(), `route ${r.id}: ${await res.text()}`).toBeTruthy();
    }
    // Where the provider now accepts the application's callback, as an agent
    // reads it: under the routes' path, on the public address.
    const listed = (await (await root.get('/api/oidc/clients')).json()) as Array<{ id: string; acceptedUnder: string[] }>;
    expect(listed.find((c) => c.id === CLIENT)?.acceptedUnder).toEqual([`${DATA_URL}/e2e-oidc/`]);

    // Discovery on the data plane, every address public and under the issuer.
    const anon = await request.newContext();
    await expect
      .poll(async () => (await anon.get(`${DATA_URL}/.well-known/openid-configuration`)).status())
      .toBe(200);
    const discovery = await (await anon.get(`${DATA_URL}/.well-known/openid-configuration`)).json();
    expect(discovery.issuer).toBe(DATA_URL);
    expect(discovery.authorization_endpoint).toBe(`${DATA_URL}/meerkat/oidc/authorize`);
    expect(discovery.token_endpoint).toBe(`${DATA_URL}/meerkat/oidc/token`);
    expect(discovery.code_challenge_methods_supported).toContain('S256');
    expect(discovery.claims_supported).toEqual(expect.arrayContaining(['level', 'preferred_username', 'name']));
    await anon.dispose();

    // alice: past the first route (not named), granted by the second.
    const a = await signIn(browser, alice.username, alice.password);
    const tokA = await exchange(DATA_URL, secret, a.code, a.verifier);
    expect(tokA.status, JSON.stringify(tokA.body)).toBe(200);
    await verifySignature(tokA.body.id_token);
    const idA = decode(tokA.body.id_token).claims;
    expect(idA).toMatchObject({ iss: DATA_URL, aud: CLIENT, nonce: a.nonce, sub: alice.id, level: 'user', name: 'Alice Martin' });
    expect(idA.preferred_username).toBeUndefined();
    // A code is good once.
    const replay = await exchange(DATA_URL, secret, a.code, a.verifier);
    expect(replay.status).toBe(400);
    expect(replay.body.error).toBe('invalid_grant');
    // userinfo answers what the access token says about the person.
    const app = await request.newContext();
    const info = await app.get(`${DATA_URL}/meerkat/oidc/userinfo`, {
      headers: { Authorization: `Bearer ${tokA.body.access_token}` },
    });
    expect(info.ok(), await info.text()).toBeTruthy();
    expect(await info.json()).toMatchObject({ sub: alice.id, level: 'user', name: 'Alice Martin' });
    await app.dispose();

    // tadmin: named by the first route, which decides. Exchanged on the
    // internal plane this time, the way a server inside the cluster does.
    const t = await signIn(browser, tadmin.username, tadmin.password);
    const tokT = await exchange(INTERNAL_URL, secret, t.code, t.verifier);
    expect(tokT.status, JSON.stringify(tokT.body)).toBe(200);
    await verifySignature(tokT.body.id_token);
    const idT = decode(tokT.body.id_token).claims;
    expect(idT).toMatchObject({ iss: DATA_URL, aud: CLIENT, sub: tadmin.id, level: 'admin', preferred_username: tadmin.username });
    expect(idT.name).toBeUndefined();

    // The console's drawer chooses the issuer among the public addresses, shows
    // the public discovery at the top beside the internal one, and lists the
    // client with the routes bound to it - no address on its row; and a client
    // declared elsewhere shows up while it is open, with no reload.
    const context = await browser.newContext({ storageState: authFile('root') });
    const page = await context.newPage();
    await page.goto(ADMIN_URL + '/infra/routes');
    await page.getByRole('button', { name: 'OIDC / JWT' }).click();
    const drawer = page.locator('app-signing-keys-panel');
    const clients = drawer.locator('section.clients');
    await expect(clients.locator('mat-row', { hasText: CLIENT })).toBeVisible();
    for (const r of ROUTES) await expect(clients.locator('mat-row', { hasText: CLIENT })).toContainText(r);
    await expect(clients.locator('mat-row', { hasText: CLIENT })).not.toContainText(`${DATA_URL}/e2e-oidc/`);
    await expect(clients).not.toContainText('/.well-known/openid-configuration');
    await expect(drawer.locator('.jwks', { hasText: 'OIDC discovery (public)' })).toContainText(
      `${DATA_URL}/.well-known/openid-configuration`,
    );
    await expect(drawer.locator('mat-select')).toContainText(DATA_URL);
    await expect(drawer).not.toContainText('/meerkat/oidc/authorize');
    // The renew button sits with the key pairs, ahead of the clients.
    const renew = drawer.getByRole('button', { name: 'Renew keys' });
    await expect(renew).toBeVisible();
    expect((await renew.boundingBox())!.y).toBeLessThan((await clients.boundingBox())!.y);

    const other = await root.post('/api/oidc/clients', {
      data: { id: SECOND, confidential: false },
    });
    expect(other.status(), await other.text()).toBe(201);
    await expect(clients.locator('mat-row', { hasText: SECOND })).toBeVisible();
    await context.close();
  } finally {
    await cleanUp(root, before, certId);
    await root.dispose();
  }
});

test('oidc provider: the community image refuses a client by name, and still shows the section', async ({ browser }) => {
  test.skip(ENTERPRISE, 'the Enterprise image serves the provider: the previous test covers it');
  const root = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const res = await root.post('/api/oidc/clients', {
    data: { id: CLIENT, confidential: true },
  });
  expect(res.status()).toBeGreaterThanOrEqual(400);
  expect(await res.text()).toContain('The OIDC provider is part of the Enterprise edition');
  await root.dispose();

  const context = await browser.newContext({ storageState: authFile('root') });
  const page = await context.newPage();
  await page.goto(ADMIN_URL + '/infra/routes');
  await page.getByRole('button', { name: 'OIDC / JWT' }).click();
  // The keys are not Enterprise, and stay; the clients section is there,
  // dimmed, and so is the public discovery line, locked and with no address.
  const drawer = page.locator('app-signing-keys-panel');
  await expect(drawer).toContainText('RS256');
  await expect(drawer.locator('section.clients')).toContainText('OIDC clients');
  const pub = drawer.locator('.jwks', { hasText: 'OIDC discovery (public)' });
  await expect(pub).toHaveAttribute('ee-feature', 'SAUTH-05');
  await expect(pub).not.toContainText('/.well-known/openid-configuration');
  // No public name declared: the drawer says where to declare one.
  await expect(drawer).toContainText('Declare a public name in Domains & TLS, Application tab');
  await context.close();
});
