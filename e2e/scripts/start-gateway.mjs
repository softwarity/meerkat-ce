// Builds the gateway from the working tree and runs it on the e2e ports with
// a DISPOSABLE database: every run starts from a clean slate, the setup
// project re-seeds the profiles through the real HTTP flows.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const repo = fileURLToPath(new URL('../..', import.meta.url));
const tmp = fileURLToPath(new URL('../.tmp', import.meta.url));
const bin = `${tmp}/meerkat`;

// Which product this tree carries, asked of the tree: the community mirror is
// this commit with ee/ removed, and the suite has to run there too. With ee/
// the Enterprise binary is built and the gateway comes up multi-organisation,
// which is what the whole matrix expects; without it the community binary
// comes up single, and the scenarios that need several organisations are the
// ones marked `edition: "ee"` and left out (see lib/fixtures).
const enterprise = existsSync(`${repo}/ee`);
const tags = enterprise ? ['-tags', 'ee'] : [];
console.log(enterprise ? 'ee/ is here: Enterprise binary, multi-organisation' : 'no ee/: community binary, single organisation');
const build = spawnSync('go', ['build', ...tags, '-o', bin, './cmd/meerkat'], { cwd: repo, stdio: 'inherit' });
if (build.status !== 0) process.exit(build.status ?? 1);

rmSync(`${tmp}/data`, { recursive: true, force: true });
mkdirSync(`${tmp}/data`, { recursive: true });

const child = spawn(
  bin,
  [
    '-addr', ':18082',
    '-admin-addr', ':19092',
    '-console-url', 'http://localhost:14200',
    '-data', `${tmp}/data`,
    // The shape the binary can serve. The community one starts single
    // whatever this says - it warns and carries on - but asking for a mode a
    // build cannot serve puts a warning in every run's log, and a log nobody
    // can read is a log nobody reads.
    '-tenancy', enterprise ? 'multi' : 'single',
  ],
  {
    stdio: 'inherit',
    env: {
      ...process.env,
      MEERKAT_ADMIN_PASSWORD: 'e2e-Root-Password-1',
      // The demo routes proxy to the httpbin started beside us, never to the
      // internet: see playwright.config.ts.
      MEERKAT_DEMO_UPSTREAM: 'http://localhost:18099',
    },
  },
);
child.on('exit', (code) => process.exit(code ?? 1));
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => child.kill());
