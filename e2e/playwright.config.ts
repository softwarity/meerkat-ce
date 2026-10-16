import { defineConfig } from '@playwright/test';

// The suite runs against a REAL gateway: a fresh binary built from the tree,
// a disposable database, and the console EMBEDDED in that binary, which is
// what ships (scripts/start-gateway.mjs stages the production build the way
// `make ui` does). E2E_CONSOLE=served serves it instead as static files
// behind the admin plane's dev proxy (--console-url), for a developer
// iterating on the console. Ports are offset from the dev ones so a running
// `make dev` never collides.
export const CONSOLE_PORT = 14200;
const SERVED = process.env.E2E_CONSOLE === 'served';
export const DATA_URL = 'http://localhost:18082';
export const ADMIN_URL = 'http://localhost:19092';
// The internal plane: what the services behind the gateway call (the JWKS,
// the scheduler's API).
export const INTERNAL_URL = 'http://localhost:17072';
export const ROOT_PASSWORD = 'e2e-Root-Password-1';
// The local go-httpbin every seeded route answers from (see webServer below).
export const HTTPBIN_URL = 'http://localhost:18099';

export default defineConfig({
  testDir: '.',
  testMatch: ['setup/**/*.setup.ts', 'tests/**/*.spec.ts'],
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: ADMIN_URL,
  },
  projects: [
    // Seeds the profiles (root, infra-admin, app-admin, tenant-admin, user)
    // through the real HTTP flows and saves one storage state per profile.
    { name: 'setup', testMatch: /setup\/.*\.setup\.ts/ },
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
      dependencies: ['setup'],
      testMatch: /tests\/.*\.spec\.ts/,
      testIgnore: [/tests\/tenancy\.spec\.ts/, /tests\/portal\.spec\.ts/, /tests\/oidc-provider\.spec\.ts/],
    },
    // The tenancy switch flips the WHOLE installation to single mode for a
    // moment. Run beside the rest, it made a landing test read the wrong mode
    // (tenant-admin sent to /license): it has to run after everything else,
    // which a file's position does not guarantee and a project dependency does.
    {
      name: 'tenancy',
      use: { browserName: 'chromium' },
      dependencies: ['chromium'],
      testMatch: /tests\/tenancy\.spec\.ts/,
    },
    // The portal tests switch the WHOLE installation's bar on for a moment,
    // and with it every proxied page swaps its user button for the bar: a page
    // test running beside them would find neither where it expects. Last,
    // then, and in one file, so they run one after the other.
    {
      name: 'portal',
      use: { browserName: 'chromium' },
      dependencies: ['tenancy'],
      testMatch: /tests\/portal\.spec\.ts/,
    },
    // The OIDC provider sets the issuer the WHOLE gateway signs with, and
    // while a client exists the data plane answers the discovery path in
    // front of the routes: a spec reading either beside it would read its
    // values. Last, after the portal.
    {
      name: 'oidc-provider',
      use: { browserName: 'chromium' },
      dependencies: ['portal'],
      testMatch: /tests\/oidc-provider\.spec\.ts/,
    },
  ],
  webServer: [
    ...(SERVED
      ? [
          {
            command: 'node scripts/serve-console.mjs',
            url: `http://localhost:${CONSOLE_PORT}/`,
            reuseExistingServer: true,
            timeout: 30_000,
          },
        ]
      : []),
    {
      // SMTP sink: the self-registration flow reads its confirmation mail
      // back from .tmp/mail. Port-only readiness (no HTTP endpoint).
      command: 'node scripts/smtp-sink.mjs',
      port: 12525,
      reuseExistingServer: true,
      timeout: 30_000,
    },
    {
      // The upstream the demo routes proxy to, run HERE rather than reached on
      // the internet. Depending on httpbin.org meant three tests failing on
      // i/o timeouts from a CI runner - a verdict about somebody else's
      // website, delivered as if it were about this product.
      // go-httpbin: same API, multi-arch, and it starts in milliseconds.
      command: 'docker run --rm -p 18099:8080 mccutchen/go-httpbin',
      url: 'http://localhost:18099/status/200',
      reuseExistingServer: true,
      timeout: 120_000,
    },
    {
      command: 'node scripts/start-gateway.mjs',
      url: `${ADMIN_URL}/login`,
      reuseExistingServer: false,
      timeout: 180_000,
    },
  ],
});
