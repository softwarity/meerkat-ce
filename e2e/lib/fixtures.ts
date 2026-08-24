import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import scenariosFile from '../scenarios.json';

// scenarios.json is THE shared source of truth: the suites execute it, the
// doc site renders it. Keep this module dumb - read, type, resolve.

export interface Scenario {
  id: string;
  domain: string;
  kind: 'api' | 'ui' | 'flow';
  // allowedStatus: acceptable statuses for ALLOWED profiles when plain 2xx is
  // not guaranteed (e.g. 422 while a dependency is not configured) - the
  // point stays that it is NOT 401/403.
  probe?: { method: string; path: string; body?: unknown; allowedStatus?: number[] };
  check?: { railItem: string };
  spec?: string;
  // "ee" when the scenario needs the Enterprise product (several
  // organisations). Absent means it runs on both.
  edition?: 'ee';
  allowed: string[];
  title: Record<string, string>;
  description: Record<string, string>;
}

export interface Profile {
  id: string;
  username: string;
}

// Which product this tree can build, asked of the TREE rather than of a
// repository name: the community mirror is this commit with ee/ removed, and
// it has to run the suite too - a pull request there proves nothing if the
// integration matrix does not run at all.
export const ENTERPRISE = existsSync(join(__dirname, '..', '..', 'ee'));

export const profiles: Profile[] = scenariosFile.profiles;

// Scenarios this tree can run. An `edition: "ee"` row needs several
// organisations, which a community binary refuses BY DESIGN - running it there
// would report a correct refusal as a broken suite.
export const scenarios: Scenario[] = (scenariosFile.scenarios as Scenario[]).filter(
  (s) => ENTERPRISE || s.edition !== 'ee',
);

export const AUTH_DIR = join(__dirname, '..', '.auth');
// The two planes ISOLATE their sessions (MEERKAT_ADMIN_SESSION vs
// MEERKAT_SESSION): each profile carries one storage state per plane.
export const authFile = (profileId: string) => join(AUTH_DIR, `${profileId}.json`);
export const authDataFile = (profileId: string) => join(AUTH_DIR, `${profileId}.data.json`);

// Written by the setup project: ids and passwords of the seeded fixtures.
export interface Seeded {
  tenantId: string;
  users: Record<string, { id: string; username: string; password: string }>;
}

export function seeded(): Seeded {
  const p = join(AUTH_DIR, 'seeded.json');
  if (!existsSync(p)) throw new Error('run the setup project first (it seeds profiles and writes .auth/seeded.json)');
  return JSON.parse(readFileSync(p, 'utf8'));
}

// Resolves {tenantId} / {aliceId} placeholders against the seeded fixtures.
export function resolvePath(path: string): string {
  const s = seeded();
  return path.replaceAll('{tenantId}', s.tenantId).replaceAll('{aliceId}', s.users['user'].id);
}
