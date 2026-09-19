// The release build, in one process because the order matters and because the
// deploy workflow calls it with arguments:
//
//   npm run build -- --base-href /meerkat/
//
// npm hands those arguments to THIS script, and this script hands them to
// `ng build` - the one step they are meant for. A shell chain could not: npm
// appends them to the end of the line, where the last command would get them,
// and every asset would be served from the wrong root.
//
// Steps: the release tag, the generated pages, the content, the bundle, and
// one HTML file per page so GitHub Pages answers real paths with real titles.
import { spawnSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const passthrough = process.argv.slice(2);

function run(command, args) {
  const res = spawnSync(command, args, { cwd: root, stdio: 'inherit', shell: process.platform === 'win32' });
  if (res.status !== 0) process.exit(res.status ?? 1);
}

run('node', ['scripts/gen-version.mjs']);
run('node', ['scripts/gen-tests.mjs']);
run('node', ['scripts/build-site.mjs', '--strict']);
run('npx', ['ng', 'build', ...passthrough]);
run('node', ['scripts/static-pages.mjs']);
