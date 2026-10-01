// The rights matrix is not written, it is READ: internal/admin/rights.json is
// what a Go test extracts from the guards the control plane's endpoints are
// registered behind, and this turns it into the two Markdown pages. Change a
// guard without regenerating that file and the test fails, so this page cannot
// say something the code does not.
//
// The output is generated, so it is gitignored; `npm run content` remakes it.
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = join(here, '..', '..', 'internal', 'admin', 'rights.json');
const CONTENT = join(here, '..', 'content');

// The columns, in the order a reader climbs them. Root last: it has everything,
// and a column that is always ticked says so by being last and full.
const COLUMNS = ['any', 'dev', 'tenant-admin', 'tenant-creator', 'app-admin', 'infra-admin', 'root'];

// What each guard admits. This is the ONE place the reading lives: the Go test
// records which guard stands in front of which endpoint, and refuses a guard
// that is not listed here.
const GUARDS = {
  rootOnly: { admits: ['root'] },
  infraAdmin: { admits: ['infra-admin', 'root'] },
  gw: { admits: ['infra-admin', 'root'] },
  appAdmin: { admits: ['app-admin', 'root'] },
  domainAdmin: { admits: ['infra-admin', 'app-admin', 'root'] },
  tenantScoped: {
    admits: ['tenant-admin', 'root'],
    note: { en: 'for the organisation in the path', fr: "pour l'organisation du chemin" },
  },
  schedules: {
    admits: ['app-admin', 'root'],
    note: { en: 'or a token scoped to schedules', fr: 'ou un jeton au périmètre schedules' },
  },
  tenantCreator: { admits: ['tenant-creator', 'root'] },
  devOrInfra: { admits: ['dev', 'infra-admin', 'root'] },
  authed: {
    admits: ['any', 'dev', 'tenant-admin', 'tenant-creator', 'app-admin', 'infra-admin', 'root'],
    note: { en: 'the handler scopes what it answers', fr: 'le handler cadre lui-même sa réponse' },
  },
  agentDoor: {
    admits: ['any', 'dev', 'tenant-admin', 'tenant-creator', 'app-admin', 'infra-admin', 'root'],
    note: {
      en: 'when the agent is switched on; each tool checks again',
      fr: "quand l'agent est ouvert ; chaque outil revérifie",
    },
  },
};

const HEAD = {
  en: {
    title: 'Rights matrix',
    summary: 'Who may call what on the control plane, read from the guards in the code rather than written by hand.',
    intro: [
      'This page is **generated from the code**. Every endpoint of the control plane is registered behind one guard,',
      'and a Go test walks those registrations and writes what it finds to `internal/admin/rights.json`. Change a guard',
      'without regenerating that file and the build fails - so what is written here is what the gateway does.',
      '',
      'The capabilities stack: **root has all of them**. An infra admin holds the routing plane, an app admin the',
      "application's identity, a tenant admin the organisations they administer. A control-plane token acts with its",
      "owner's capabilities, read again on every request, and its perimeter only ever takes away.",
    ].join('\n'),
    cols: { any: 'Any account', dev: 'Dev', 'tenant-admin': 'Tenant admin', 'tenant-creator': 'Tenant creator', 'app-admin': 'App admin', 'infra-admin': 'Infra admin', root: 'Root' },
    endpoint: 'Endpoint',
    note: 'Note',
    count: (n) => `${n} endpoints.`,
  },
  fr: {
    title: 'Matrice des droits',
    summary: "Qui peut appeler quoi sur le plan de contrôle, lu dans les gardes du code plutôt qu'écrit à la main.",
    intro: [
      'Cette page est **générée depuis le code**. Chaque endpoint du plan de contrôle est enregistré derrière une garde,',
      "et un test Go parcourt ces enregistrements et écrit ce qu'il trouve dans `internal/admin/rights.json`. Changer une",
      'garde sans régénérer ce fichier fait échouer le build : ce qui est écrit ici est ce que fait la passerelle.',
      '',
      "Les capacités se cumulent : **root les a toutes**. Un admin d'infrastructure tient le plan de routage, un admin",
      "applicatif l'identité de l'application, un admin d'organisation les organisations qu'il administre. Un jeton du",
      'plan de contrôle agit avec les capacités de son propriétaire, relues à chaque requête, et son périmètre ne fait',
      'que retirer.',
    ].join('\n'),
    cols: { any: 'Tout compte', dev: 'Dev', 'tenant-admin': 'Admin organisation', 'tenant-creator': 'Créateur organisation', 'app-admin': 'Admin applicatif', 'infra-admin': 'Admin infra', root: 'Root' },
    endpoint: 'Endpoint',
    note: 'Note',
    count: (n) => `${n} endpoints.`,
  },
};

// The section an endpoint belongs to: its first path segment, and for the
// settings the second too - "settings" alone would put TLS beside tenancy.
function section(path) {
  if (path === '/mcp') return 'mcp';
  const parts = path.replace(/^\/api\//, '').split('/');
  if (parts[0] === 'settings' && parts[1]) return `settings/${parts[1]}`;
  return parts[0];
}

const rights = JSON.parse(await readFile(SRC, 'utf8'));

for (const lang of ['en', 'fr']) {
  const w = HEAD[lang];
  const out = [
    '---',
    `title: ${w.title}`,
    `section: ${lang === 'fr' ? 'Qualité' : 'Quality'}`,
    'order: 31',
    `summary: ${w.summary}`,
    '---',
    '',
    `# ${w.title}`,
    '',
    w.intro,
    '',
    w.count(rights.length),
    '',
  ];
  const sections = [...new Set(rights.map((r) => section(r.path)))].sort();
  for (const s of sections) {
    out.push(`## \`${s}\``, '');
    out.push(`| ${w.endpoint} | ${COLUMNS.map((c) => w.cols[c]).join(' | ')} | ${w.note} |`);
    out.push(`| --- | ${COLUMNS.map(() => ':---:').join(' | ')} | --- |`);
    for (const r of rights.filter((x) => section(x.path) === s)) {
      const g = GUARDS[r.guard];
      if (!g) throw new Error(`gen-rights: guard ${r.guard} has no reading - add it to GUARDS`);
      const cells = COLUMNS.map((c) => (g.admits.includes(c) ? 'x' : ''));
      const method = r.method === '*' ? '' : `${r.method} `;
      out.push(`| \`${method}${r.path}\` | ${cells.join(' | ')} | ${g.note?.[lang] ?? ''} |`);
    }
    out.push('');
  }
  const dir = join(CONTENT, lang, 'project');
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, 'rights.md'), out.join('\n'));
}

console.log(`[gen-rights] content/{en,fr}/project/rights.md <- internal/admin/rights.json (${rights.length} endpoints)`);
