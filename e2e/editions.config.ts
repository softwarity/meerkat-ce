import { defineConfig } from '@playwright/test';

// The editions: what a license does to the build that ships as
// softwarity/meerkat - none, Team, Enterprise, pasted or mounted, broken,
// past its term, over its ceilings, across a cluster - on real binaries with
// the CI's real licenses (editions/lib.ts says where they come from).
//
//   npx playwright test -c editions.config.ts
//
// Separate from the integration suite (playwright.config.ts) because every
// test here starts and stops its own gateways, with their own binaries: the
// suite's single gateway, started once for everything, is the opposite shape.
//
// Nothing is recorded: no trace, no screenshot, no HTML report. A license is
// pasted through these requests, the CI that runs them is public, and a file
// that leaks cannot be revoked.
export default defineConfig({
  testDir: 'editions',
  testMatch: /.*\.spec\.ts/,
  globalSetup: './editions/global-setup.ts',
  // Each file is one installation, run in order; the files side by side, on
  // their own ports. The cluster is the long one, and it waits on clocks, not
  // on a processor.
  fullyParallel: false,
  workers: 3,
  timeout: 10 * 60_000,
  reporter: [['list']],
  outputDir: '.editions/results',
  use: { trace: 'off', screenshot: 'off', video: 'off' },
});
