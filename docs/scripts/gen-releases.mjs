// The list of releases a gateway reads when its update check is on (CONSOLE-16),
// served at the root of the site as /releases.json. Read from RELEASE_NOTES.md,
// the file the release wrote: every numbered section, with the date and the
// migration .github/scripts/release-facts.sh put in its comment. A release older
// than that comment has neither, and no gateway running this check is behind it.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const source = join(here, '..', '..', 'RELEASE_NOTES.md');
const target = join(here, '..', 'public', 'releases.json');

const FACTS = /<!--\s*release:\s*built=(\S+)\s+migrates=(true|false)\s*-->/;
const NUMBERED = /^\d+\.\d+\.\d+$/;

// release-facts.sh's line about the database: the console says it on its own,
// beside what it means for the license, and would say it twice.
const MIGRATION_LINE = /^- \*\*This version migrates the database\*\*.*\n(?:[ \t]+.*\n)*/m;

// A version's notes as the console reads them: no comments, no separator.
function clean(lines) {
  return lines
    .join('\n')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(MIGRATION_LINE, '')
    .trim()
    .replace(/\n*---$/, '')
    .trim();
}

function releasesOf(notes) {
  const releases = [];
  let name = null;
  let body = [];
  const flush = () => {
    if (name === null || !NUMBERED.test(name)) return;
    const facts = FACTS.exec(body.join('\n'));
    releases.push({
      version: name,
      ...(facts ? { built: facts[1] } : {}),
      ...(facts?.[2] === 'true' ? { migrates: true } : {}),
      notes: clean(body),
    });
  };
  for (const line of notes.split('\n')) {
    if (line.startsWith('## ')) {
      flush();
      name = line.slice(3).trim();
      body = [];
    } else {
      body.push(line);
    }
  }
  flush();
  return releases;
}

const releases = releasesOf(readFileSync(source, 'utf8'));
mkdirSync(dirname(target), { recursive: true });
writeFileSync(target, JSON.stringify({ releases }) + '\n');
console.log(`[gen-releases] ${target} <- ${releases.length} releases`);
