// The Test coverage page is not written, it is READ: e2e/scenarios.json is the
// file the Playwright suite executes, and this turns it into the two Markdown
// pages that say so. If a scenario shows on the site, a test enforces it - and
// nobody can let the page drift, because nobody writes it.
//
// The output is generated, so it is gitignored; `npm run content` remakes it.
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = join(here, '..', '..', 'e2e', 'scenarios.json');
const CONTENT = join(here, '..', 'content');

const DOMAINS = {
  gateway: { en: 'Gateway scope', fr: 'Périmètre gateway' },
  application: { en: 'Application scope', fr: 'Périmètre application' },
  tenant: { en: 'Tenant scope', fr: 'Périmètre tenant' },
  console: { en: 'Console navigation', fr: 'Navigation console' },
  auth: { en: 'Sign-in and profile flows', fr: 'Flux de connexion et de profil' },
};

const HEAD = {
  en: {
    title: 'Test coverage',
    summary: 'The exact matrix the integration suite runs: every scenario, played as every profile, allowed and refused.',
    intro:
      'This page renders the file the Playwright integration suite executes, ' +
      '`e2e/scenarios.json`, as it is. Every scenario runs as each of the profiles below, ' +
      'and the suite checks BOTH that the allowed profiles get through AND that every ' +
      'other one is refused - an access test that only proves the yes half proves nothing.',
    profiles: 'The profiles',
    allowed: 'Allowed',
    refused: 'Refused',
    probe: 'Probe',
    scenario: 'Scenario',
  },
  fr: {
    title: 'Couverture de tests',
    summary: "La matrice exacte que joue la suite d'intégration : chaque scénario, joué par chaque profil, autorisé et refusé.",
    intro:
      "Cette page rend tel quel le fichier que la suite d'intégration Playwright exécute, " +
      '`e2e/scenarios.json`. Chaque scénario est joué avec chacun des profils ci-dessous, ' +
      'et la suite vérifie A LA FOIS que les profils autorisés passent ET que tous les ' +
      "autres sont refusés - un test d'accès qui ne prouve que la moitié oui ne prouve rien.",
    profiles: 'Les profils',
    allowed: 'Autorisés',
    refused: 'Refusés',
    probe: 'Sonde',
    scenario: 'Scenario',
  },
};

const data = JSON.parse(await readFile(SRC, 'utf8'));

for (const lang of ['en', 'fr']) {
  const w = HEAD[lang];
  const out = [
    '---',
    `title: ${w.title}`,
    `section: ${lang === 'fr' ? 'Qualité' : 'Quality'}`,
    'order: 30',
    `summary: ${w.summary}`,
    '---',
    '',
    `# ${w.title}`,
    '',
    w.intro,
    '',
    `## ${w.profiles}`,
    '',
    `| ${lang === 'fr' ? 'Profil' : 'Profile'} | ${lang === 'fr' ? 'Ce que ce compte est' : 'What this account is'} |`,
    '| --- | --- |',
    ...data.profiles.map((p) => `| \`${p.id}\` | ${p[lang]} |`),
    '',
  ];

  const domains = [...new Set(data.scenarios.map((s) => s.domain))];
  for (const domain of domains) {
    out.push(`## ${DOMAINS[domain]?.[lang] || domain}`, '');
    out.push(`| ${w.scenario} | ${w.probe} | ${w.allowed} |`, '| --- | --- | --- |');
    for (const sc of data.scenarios.filter((s) => s.domain === domain)) {
      const probe = sc.probe ? `\`${sc.probe.method} ${sc.probe.path}\`` : `_${sc.kind}_`;
      const allowed = sc.allowed.length ? sc.allowed.map((a) => `\`${a}\``).join(' ') : '-';
      out.push(`| **${sc.title[lang]}** ${sc.description[lang]} | ${probe} | ${allowed} |`);
    }
    out.push('');
  }

  const dir = join(CONTENT, lang, 'project');
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, 'tests.md'), out.join('\n'));
}

console.log(`[gen-tests] content/{en,fr}/project/tests.md <- e2e/scenarios.json (${data.scenarios.length} scenarios)`);
