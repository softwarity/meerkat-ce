import { cpSync, existsSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { buildAll, licenseFile, repo } from './lib';

// Before anything starts: the licenses are there (a missing one fails the run,
// it does not skip it), the console is staged in the binary - its watermark is
// one of the surfaces the notice is checked on - and the binaries are built
// once for every file.
export default function globalSetup(): void {
  if (!existsSync(`${repo}/ee`)) {
    throw new Error('the editions are those of the Enterprise build: this tree has no ee/');
  }
  licenseFile('team');
  licenseFile('enterprise');
  stageConsole();
  buildAll();
}

// What `make ui` does, from the build already there (the CI builds it first),
// as scripts/start-gateway.mjs does it.
function stageConsole(): void {
  const build = `${repo}/console/dist/console/browser`;
  const dist = `${repo}/internal/admin/ui/dist`;
  if (existsSync(`${build}/index.html`)) {
    for (const entry of readdirSync(dist)) {
      if (entry !== '.gitkeep') rmSync(`${dist}/${entry}`, { recursive: true, force: true });
    }
    cpSync(build, dist, { recursive: true });
    writeFileSync(`${dist}/.gitkeep`, '');
  }
  if (!existsSync(`${dist}/index.html`)) {
    throw new Error('no console to put in the binary: run cd console && npm run build');
  }
}
