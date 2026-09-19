// Put the deployment files ON the site, so a reader downloads what this
// repository actually ships rather than copying a block out of a page.
//
// They are COPIES, made at build time from deploy/ - the one place they are
// written. A file kept twice drifts, and the copy that drifts is always the
// one a stranger downloads.
//
// The chart goes as a .tgz because that is what `helm install` takes. A chart
// archive is a gzipped tar holding one directory named after the chart, which
// is what tar makes here: no helm on the runner, and nothing to install to
// publish a page.
import { readFile, mkdir, copyFile, rm, readdir } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const site = join(here, '..');
const repo = join(site, '..');
const deploy = join(repo, 'deploy');
const out = join(site, 'public', 'deploy');

await rm(out, { recursive: true, force: true });
await mkdir(out, { recursive: true });

// The compose files and the values, as they are.
const plain = [
  ['docker-compose.yml', 'docker-compose.yml'],
  ['docker-compose.ee.yml', 'docker-compose.ee.yml'],
  ['stack.swarm.yml', 'stack.swarm.yml'],
  ['helm/values-ce-one-node.yaml', 'values-ce-one-node.yaml'],
  ['helm/values-ee-one-node.yaml', 'values-ee-one-node.yaml'],
  ['helm/values-ee-cluster.yaml', 'values-ee-cluster.yaml'],
];
for (const [from, to] of plain) {
  await copyFile(join(deploy, from), join(out, to));
}

// The chart, packaged under the name helm expects.
const chart = await readFile(join(deploy, 'helm', 'meerkat', 'Chart.yaml'), 'utf8');
const version = /^version:\s*(.+)$/m.exec(chart)?.[1]?.trim();
if (!version) throw new Error('deploy/helm/meerkat/Chart.yaml: no version to name the archive with');
const archive = `meerkat-${version}.tgz`;

// FILES only, named one by one. `tar -C helm meerkat` also writes the two
// directory entries, and helm then refuses the archive it just downloaded
// with "unable to load chart archive" - its own packager writes no directory
// entry either. Checked against one: the two listings differ by those lines
// alone.
const files = [];
const walk = async (dir, prefix) => {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    if (entry.name.startsWith('.')) continue;
    const rel = `${prefix}/${entry.name}`;
    if (entry.isDirectory()) await walk(join(dir, entry.name), rel);
    else files.push(rel);
  }
};
await walk(join(deploy, 'helm', 'meerkat'), 'meerkat');
files.sort();
const tar = spawnSync('tar', ['-czf', join(out, archive), '-C', join(deploy, 'helm'), ...files], {
  encoding: 'utf8',
  // macOS tar smuggles an AppleDouble twin of every file into the archive
  // (._name, carrying extended attributes). Its own listing hides them; the
  // Go reader inside helm does not, and reads ._helpers.tpl as a template:
  // "control characters are not allowed". Ignored on Linux, where the runner
  // packages the real thing.
  env: { ...process.env, COPYFILE_DISABLE: '1' },
});
if (tar.status !== 0) throw new Error(`packaging the chart failed: ${tar.stderr || tar.status}`);

// The page names the archive, and the archive carries the chart's version:
// one of the two has to learn it from the other, and a page is written by
// hand. So the build writes the name it produced, and the page reads it.
await copyFile(join(out, archive), join(out, 'meerkat-chart.tgz'));

console.log(`[gen-deploy] ${plain.length} file(s) + ${archive} (also as meerkat-chart.tgz)`);
