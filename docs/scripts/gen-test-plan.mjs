// The Test plan page is not written, it is ASSEMBLED: e2e/platform/plan.json
// says what each feature has to prove and where, FEATURES.md says how far each
// feature is built, and the test sources say what already names it. The page
// is the three read together, so a feature added to FEATURES.md and forgotten
// in the plan shows up as such instead of silently missing.
//
// The output is generated, so it is gitignored; `npm run content` remakes it.
import { readdir, mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const ROOT = join(here, '..', '..');
const CONTENT = join(here, '..', 'content');

const plan = JSON.parse(await readFile(join(ROOT, 'e2e', 'platform', 'plan.json'), 'utf8'));

// FEATURES.md: one row per feature, state in the first cell.
const features = new Map();
for (const line of (await readFile(join(ROOT, 'FEATURES.md'), 'utf8')).split('\n')) {
  const m = line.match(/^\| \[(.)\] \| ([A-Z0-9]+-\d+) \| \*\*(.+?)\*\* \| .* \| .* \| ([^|]*) \|\s*$/);
  if (m) features.set(m[2], { state: m[1], title: m[3], edition: m[4].trim() });
}

// What the tests already name. A mention is not a proof, but a feature no test
// names at all is one nobody has checked - which is what this column is for.
async function walk(dir, keep, out = []) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return out; // ee/ is absent from the community mirror
  }
  for (const e of entries) {
    if (e.name === 'node_modules' || e.name.startsWith('.')) continue;
    const p = join(dir, e.name);
    if (e.isDirectory()) await walk(p, keep, out);
    else if (keep(e.name)) out.push(await readFile(p, 'utf8'));
  }
  return out;
}
const goTests = [
  ...(await walk(join(ROOT, 'internal'), (n) => n.endsWith('_test.go'))),
  ...(await walk(join(ROOT, 'ee'), (n) => n.endsWith('_test.go'))),
  ...(await walk(join(ROOT, 'cmd'), (n) => n.endsWith('_test.go'))),
];
const e2e = [
  await readFile(join(ROOT, 'e2e', 'scenarios.json'), 'utf8'),
  ...(await walk(join(ROOT, 'e2e', 'tests'), (n) => n.endsWith('.ts'))),
];
const mentions = (id, texts) => {
  const re = new RegExp(`\\b${id}\\b`);
  return texts.filter((t) => re.test(t)).length;
};

const STATE = {
  en: { x: 'built', '~': 'partly built', ' ': 'not built', '-': 'retired' },
  fr: { x: 'construite', '~': 'en partie construite', ' ': 'non construite', '-': 'retirée' },
};
const FEAS = {
  en: { easy: 'easy', medium: 'medium', hard: 'hard', blocked: 'not built', out: 'nothing to test' },
  fr: { easy: 'facile', medium: 'moyenne', hard: 'difficile', blocked: 'non construite', out: 'rien à tester' },
};
const FAMILY = {
  AUTH: ['Sign-in', 'Connexion'], MFA: ['Second factor', 'Second facteur'], RBAC: ['Roles and access', 'Rôles et accès'],
  MODEL: ['Account model', 'Modèle de compte'], TENANT: ['Organisations', 'Organisations'], ROUTE: ['Routing', 'Routage'],
  SVC: ['Services', 'Services'], UIF: ['Injected UI', 'Interface injectée'], PORTAL: ['Portal', 'Portail'],
  SAUTH: ['Identity to the upstream', "Identité transmise à l'upstream"], VAULT: ['Vault', 'Coffre'], SSL: ['TLS', 'TLS'],
  NOTIF: ['Notifications', 'Notifications'], SCHED: ['Scheduled calls', 'Appels planifiés'], I18N: ['Languages', 'Langues'],
  THEME: ['Themes', 'Thèmes'], CONSOLE: ['Console', 'Console'], LIFE: ['Lifecycle', 'Cycle de vie'],
  DEV: ['Developer tunnel', 'Tunnel développeur'], CFG: ['Configurations', 'Configurations'], PAGE: ['Pages', 'Pages'],
  QUOTA: ['Quotas', 'Quotas'], AUD: ['Audit', 'Audit'], ISSUE: ['Issues', 'Signalements'], MCP: ['Agents (MCP)', 'Agents (MCP)'],
  SEC: ['Security', 'Sécurité'], PERF: ['Performance and cluster', 'Performances et cluster'], OBS: ['Observability', 'Observabilité'],
  DEPLOY: ['Deployment', 'Déploiement'], STORE: ['Storage', 'Stockage'], QUAL: ['Quality', 'Qualité'],
};

const AREA = {
  Deployment: 'Déploiement', Upgrade: 'Montée de version', Cluster: 'Cluster', Edition: 'Édition',
  Security: 'Sécurité', Soak: 'Endurance', Performance: 'Performances', Lifecycle: 'Cycle de vie',
};

const W = {
  en: {
    title: 'Test plan',
    summary: 'Every feature, what the test platform has to prove about it, on which targets and editions, and in which order.',
    intro:
      'This page is assembled from three sources: `e2e/platform/plan.json` (what each feature has to prove, where, how urgent and how hard), ' +
      '`FEATURES.md` (how far the feature is built) and the test sources (which already name it). The platform itself - targets, fixtures, ' +
      'CI chain - is described on [Test platform](/project/test-platform). Feature titles come from FEATURES.md, which is written in French.',
    targets: 'Targets',
    legend: 'Priorities',
    prios: [
      ['P0', 'blocks a release: the image, the targets, the cluster and the upgrade path'],
      ['P1', 'the features people deploy Meerkat for, proved through the image'],
      ['P2', 'the rest of what is built, proved where the platform adds something the unit tests cannot'],
      ['P3', 'covered well enough by the unit and integration suites; on the platform when convenient'],
      ['-', 'not built, retired, or nothing to test'],
    ],
    totals: 'In numbers',
    platform: 'Platform-wide checks',
    platformIntro: 'What belongs to no single feature: deploying, upgrading, losing a node, running for hours.',
    cols: ['ID', 'Feature', 'What has to be proved', 'Targets', 'Editions', 'Priority', 'Feasibility', 'Tests naming it today'],
    pcols: ['ID', 'Area', 'What has to be proved', 'Targets', 'Editions', 'Priority', 'Feasibility'],
    unit: 'Go',
    it: 'e2e',
    none: 'none',
    unplanned: 'In FEATURES.md and not in the plan yet',
    count: (n, w) => `${n} ${w}`,
  },
  fr: {
    title: 'Plan de test',
    summary: "Pour chaque fonctionnalité, ce que la plateforme de test doit prouver, sur quelles cibles, dans quelles éditions et dans quel ordre.",
    intro:
      "Cette page est assemblée à partir de trois sources : `e2e/platform/plan.json` (ce que chaque fonctionnalité doit prouver, où, avec quelle urgence et quelle difficulté), " +
      "`FEATURES.md` (l'état d'avancement de la fonctionnalité) et les sources des tests (ceux qui la citent déjà). La plateforme elle-même - cibles, services de test, " +
      'chaîne de CI - est décrite sur la page [Plateforme de test](/project/test-platform). Les titres des fonctionnalités viennent de FEATURES.md.',
    targets: 'Les cibles',
    legend: 'Les priorités',
    prios: [
      ['P0', "bloque la publication d'une version : l'image, les cibles, le cluster et la montée de version"],
      ['P1', "les fonctionnalités pour lesquelles on déploie Meerkat, prouvées à travers l'image"],
      ['P2', 'le reste de ce qui est construit, prouvé là où la plateforme apporte ce que les tests unitaires ne peuvent pas apporter'],
      ['P3', "suffisamment couvert par les suites unitaires et d'intégration ; sur la plateforme quand l'occasion se présente"],
      ['-', 'non construite, retirée, ou rien à tester'],
    ],
    totals: 'En chiffres',
    platform: "Contrôles à l'échelle de la plateforme",
    platformIntro: "Ce qui ne relève d'aucune fonctionnalité en particulier : déployer, monter de version, perdre un nœud, tourner pendant des heures.",
    cols: ['ID', 'Fonctionnalité', 'Ce qui doit être prouvé', 'Cibles', 'Éditions', 'Priorité', 'Faisabilité', "Tests qui la citent aujourd'hui"],
    pcols: ['ID', 'Domaine', 'Ce qui doit être prouvé', 'Cibles', 'Éditions', 'Priorité', 'Faisabilité'],
    unit: 'Go',
    it: 'e2e',
    none: 'aucun',
    unplanned: 'Dans FEATURES.md et pas encore dans le plan',
    count: (n, w) => `${n} ${w}`,
  },
};

const cell = (s) => String(s).replace(/\|/g, '\\|');
const ordered = ['P0', 'P1', 'P2', 'P3', '-'];

for (const lang of ['en', 'fr']) {
  const w = W[lang];
  const out = [
    '---',
    `title: ${w.title}`,
    `section: ${lang === 'fr' ? 'Qualité' : 'Quality'}`,
    'order: 32',
    `summary: ${w.summary}`,
    '---',
    '',
    `# ${w.title}`,
    '',
    w.intro,
    '',
    `## ${w.targets}`,
    '',
    `| | ${lang === 'fr' ? 'Cible' : 'Target'} |`,
    '| --- | --- |',
    ...Object.entries(plan.targets).map(([k, v]) => `| \`${k}\` | ${v[lang]} |`),
    '',
    `## ${w.legend}`,
    '',
    ...w.prios.map(([p, t]) => `- **${p}** - ${t}`),
    '',
  ];

  // Totals: priority by feasibility, so the cheap urgent work is one look away.
  const all = [...plan.features, ...plan.platform];
  const feas = ['easy', 'medium', 'hard', 'blocked', 'out'];
  out.push(`## ${w.totals}`, '');
  out.push(`| | ${feas.map((f) => FEAS[lang][f]).join(' | ')} | Total |`, `| --- |${feas.map(() => ' --- |').join('')} --- |`);
  for (const p of ordered) {
    const row = all.filter((e) => e.priority === p);
    if (!row.length) continue;
    out.push(`| **${p}** | ${feas.map((f) => row.filter((e) => e.feasibility === f).length || '').join(' | ')} | ${row.length} |`);
  }
  out.push('');

  // Platform-wide checks first: they are what a release is blocked on.
  out.push(`## ${w.platform}`, '', w.platformIntro, '');
  out.push(`| ${w.pcols.join(' | ')} |`, `|${w.pcols.map(() => ' --- |').join('')}`);
  for (const e of [...plan.platform].sort((a, b) => ordered.indexOf(a.priority) - ordered.indexOf(b.priority))) {
    out.push(
      `| ${e.id} | ${(lang === 'fr' ? AREA[e.category] : e.category) ?? ''} | ${cell(e[lang])} | ${e.targets.map((t) => `\`${t}\``).join(' ')} | ${e.editions.join(', ')} | ${e.priority} | ${FEAS[lang][e.feasibility]} |`,
    );
  }
  out.push('');

  // Then every feature, by family, in FEATURES.md's order.
  const byFamily = new Map();
  for (const e of plan.features) {
    const fam = e.id.split('-')[0];
    if (!byFamily.has(fam)) byFamily.set(fam, []);
    byFamily.get(fam).push(e);
  }
  for (const [fam, list] of byFamily) {
    out.push(`## ${FAMILY[fam]?.[lang === 'fr' ? 1 : 0] ?? fam}`, '');
    out.push(`| ${w.cols.join(' | ')} |`, `|${w.cols.map(() => ' --- |').join('')}`);
    for (const e of list) {
      const f = features.get(e.id);
      const go = mentions(e.id, goTests);
      const it = mentions(e.id, e2e);
      const today = go || it ? [go ? `${w.unit} ${go}` : '', it ? `${w.it} ${it}` : ''].filter(Boolean).join(', ') : w.none;
      const title = f ? `${cell(f.title)} _(${STATE[lang][f.state] ?? f.state})_` : '';
      out.push(
        `| ${e.id} | ${title} | ${cell(e[lang])} | ${e.targets.map((t) => `\`${t}\``).join(' ') || '-'} | ${e.editions.join(', ')} | ${e.priority} | ${FEAS[lang][e.feasibility]} | ${today} |`,
      );
    }
    out.push('');
  }

  // The drift check, on the page itself: a feature the plan forgot.
  const planned = new Set(plan.features.map((e) => e.id));
  const missing = [...features.keys()].filter((id) => !planned.has(id));
  if (missing.length) {
    out.push(`## ${w.unplanned}`, '', missing.map((id) => `- ${id} - ${features.get(id).title}`).join('\n'), '');
  }

  const dir = join(CONTENT, lang, 'project');
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, 'test-plan.md'), out.join('\n'));
}

console.log(
  `[gen-test-plan] content/{en,fr}/project/test-plan.md <- e2e/platform/plan.json (${plan.features.length} features, ${plan.platform.length} platform checks)`,
);
