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
import { readFile, writeFile, mkdir, copyFile, rm, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
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
  ['helm/values-eval-one-node.yaml', 'values-eval-one-node.yaml'],
  ['helm/values-eval-cluster.yaml', 'values-eval-cluster.yaml'],
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
// Packs the chart directory under root into dest. FILES only, named one by
// one, for the reason above.
const pack = async (root, dest) => {
  const files = [];
  const walk = async (dir, prefix) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      if (entry.name.startsWith('.')) continue;
      const rel = `${prefix}/${entry.name}`;
      if (entry.isDirectory()) await walk(join(dir, entry.name), rel);
      else files.push(rel);
    }
  };
  await walk(join(root, 'meerkat'), 'meerkat');
  files.sort();
  const tar = spawnSync('tar', ['-czf', dest, '-C', root, ...files], {
    encoding: 'utf8',
    // macOS tar smuggles an AppleDouble twin of every file into the archive
    // (._name, carrying extended attributes). Its own listing hides them; the
    // Go reader inside helm does not, and reads ._helpers.tpl as a template:
    // "control characters are not allowed". Ignored on Linux, where the runner
    // packages the real thing.
    env: { ...process.env, COPYFILE_DISABLE: '1' },
  });
  if (tar.status !== 0) throw new Error(`packaging the chart failed: ${tar.stderr || tar.status}`);
};
await pack(join(deploy, 'helm'), join(out, archive));

// The page names the archive, and the archive carries the chart's version:
// one of the two has to learn it from the other, and a page is written by
// hand. So the build writes the name it produced, and the page reads it.
await copyFile(join(out, archive), join(out, 'meerkat-chart.tgz'));

// And the index that makes /deploy a Helm REPOSITORY (DEPLOY-07):
//
//   helm repo add meerkat https://www.softwarity.io/deploy
//   helm install meerkat meerkat/meerkat -f https://www.softwarity.io/deploy/values-ce-one-node.yaml
//
// Written here rather than by `helm repo index`, so building the site needs no
// helm on the machine. The format is small and fixed: one entry per chart
// version, its digest, and a URL RELATIVE to the index - helm resolves it
// against the repository's address, so the same index works on the preview
// server and on the published site.
const fieldOf = (text) => (name) =>
  /^(?:"?)(.*?)(?:"?)$/.exec((new RegExp(`^${name}:\\s*(.+)$`, 'm').exec(text)?.[1] ?? '').trim())?.[1] ?? '';
const now = new Date().toISOString();
const q = (s) => JSON.stringify(s);
const entry = async (text, file, created) => {
  const field = fieldOf(text);
  const digest = createHash('sha256').update(await readFile(join(out, file))).digest('hex');
  return [
    `  - apiVersion: ${field('apiVersion')}`,
    `    appVersion: ${q(field('appVersion'))}`,
    `    created: ${q(created)}`,
    `    description: ${q(field('description'))}`,
    `    digest: ${digest}`,
    `    home: ${q(field('home'))}`,
    '    name: meerkat',
    `    type: ${field('type') || 'application'}`,
    '    urls:',
    `    - ${file}`,
    `    version: ${q(field('version'))}`,
  ];
};
const entries = await entry(chart, archive, now);

// EVERY version released, not only this one. An installation that pins one -
// Argo CD, Flux, Terraform, a skill's CHART_VERSION, a version validated in
// staging and taken to production a week later - asks for it again at each
// sync, and an index that forgets it answers "chart not found" the day the
// next release ships, with nobody having changed anything on their side.
// helm install without --version still takes the newest.
//
// Each one is rebuilt from its release tag (vX.Y.Z, whose Chart.yaml the
// release bot stamped with that version): the archives are not kept anywhere
// between two builds, and a tag is what that version WAS. A tag whose chart
// says another version, or has none, is not a chart release and is skipped.
// Without the history (a shallow clone) only this version is published - the
// CI fetches it whole (fetch-depth: 0).
const git = (...args) => spawnSync('git', args, { cwd: repo, encoding: 'utf8', maxBuffer: 64 << 20 });
const tags = (git('tag', '--list', 'v*').stdout || '').split('\n').filter((t) => /^v\d+\.\d+\.\d+$/.test(t));
let older = 0;
for (const tag of tags) {
  const text = git('show', `${tag}:deploy/helm/meerkat/Chart.yaml`);
  if (text.status !== 0) continue;
  const v = fieldOf(text.stdout)('version');
  if (v !== tag.slice(1) || v === version) continue;
  const tmp = join(out, `.chart-${v}`);
  await mkdir(tmp, { recursive: true });
  // Bytes, not text: a tar read as UTF-8 is a corrupted tar.
  const files = spawnSync('git', ['archive', '--format=tar', tag, 'deploy/helm/meerkat'], { cwd: repo, maxBuffer: 64 << 20 });
  if (files.status !== 0) continue;
  const untar = spawnSync('tar', ['-x', '-C', tmp, '--strip-components=2'], { input: files.stdout });
  if (untar.status !== 0) {
    await rm(tmp, { recursive: true, force: true });
    continue;
  }
  const file = `meerkat-${v}.tgz`;
  await pack(tmp, join(out, file));
  await rm(tmp, { recursive: true, force: true });
  const created = (git('log', '-1', '--format=%cI', tag).stdout || '').trim() || now;
  entries.push(...(await entry(text.stdout, file, created)));
  older++;
}

const index = ['apiVersion: v1', 'entries:', '  meerkat:', ...entries, `generated: ${q(now)}`, ''].join('\n');
await writeFile(join(out, 'index.yaml'), index);

console.log(`[gen-deploy] ${plain.length} file(s) + ${archive} (also as meerkat-chart.tgz) + ${older} older release(s) + index.yaml`);
