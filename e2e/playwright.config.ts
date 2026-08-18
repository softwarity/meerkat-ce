import { defineConfig } from '@playwright/test';

// The suite runs against a REAL gateway: a fresh binary built from the tree,
// a disposable database, and the console served as static files behind the
// admin plane's dev proxy (--console-url). Ports are offset from the dev ones
// so a running `make dev` never collides.
export const CONSOLE_PORT = 14200;
export const DATA_URL = 'http://localhost:18082';
export const ADMIN_URL = 'http://localhost:19092';
export const ROOT_PASSWORD = 'e2e-Root-Password-1';

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
    },
  ],
  webServer: [
    {
      command: 'node scripts/serve-console.mjs',
      url: `http://localhost:${CONSOLE_PORT}/`,
      reuseExistingServer: true,
      timeout: 30_000,
    },
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
