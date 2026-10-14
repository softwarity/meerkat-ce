// Builds the gateway from the working tree and runs it on the e2e ports with
// a DISPOSABLE database: every run starts from a clean slate, the setup
// project re-seeds the profiles through the real HTTP flows.
//
// The console is EMBEDDED in the binary by default - what `make ui` does and
// what ships: the production build is staged into internal/admin/ui/dist
// before `go build`, and the gateway serves it itself, stamp and headers
// included. E2E_CONSOLE=served keeps the old way for a developer iterating on
// the console: a static server on 14200 behind -console-url.
import { spawn, spawnSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const repo = fileURLToPath(new URL('../..', import.meta.url));
const tmp = fileURLToPath(new URL('../.tmp', import.meta.url));
const bin = `${tmp}/meerkat`;
// The same switch as playwright.config.ts, which starts the static server.
const CONSOLE_MODE = process.env.E2E_CONSOLE === 'served' ? 'served' : 'embedded';

if (CONSOLE_MODE === 'embedded') {
  const build = `${repo}/console/dist/console/browser`;
  if (!existsSync(`${build}/index.html`)) {
    console.error(`console build missing at ${build}: run cd console && npm run build`);
    process.exit(1);
  }
  // What `make ui` does, from the build already there (CI downloads it).
  const dist = `${repo}/internal/admin/ui/dist`;
  for (const entry of readdirSync(dist)) {
    if (entry !== '.gitkeep') rmSync(`${dist}/${entry}`, { recursive: true, force: true });
  }
  cpSync(build, dist, { recursive: true });
  writeFileSync(`${dist}/.gitkeep`, '');
  console.log('console embedded in the binary (E2E_CONSOLE=served to use the static server)');
}

// Which product this tree carries, asked of the tree: the community mirror is
// this commit with ee/ removed, and the suite has to run there too. With ee/
// the Enterprise binary is built and the gateway comes up multi-organisation,
// which is what the whole matrix expects; without it the community binary
// comes up single, and the scenarios that need several organisations are the
// ones marked `edition: "ee"` and left out (see lib/fixtures).
// E2E_EDITION=ce builds the community binary from a tree that has ee/, so the
// community run can be had without the mirror (lib/fixtures reads the same).
const enterprise = existsSync(`${repo}/ee`) && process.env.E2E_EDITION !== 'ce';
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
    '-internal-addr', ':17072',
    ...(CONSOLE_MODE === 'served' ? ['-console-url', 'http://localhost:14200'] : []),
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
    },
  },
);
child.on('exit', (code) => process.exit(code ?? 1));
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => child.kill());
