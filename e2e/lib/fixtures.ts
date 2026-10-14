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
  // anonymousStatus: what a caller with no session gets, when it is not the
  // 401 an API answers. A page meant for a browser bounces to the sign-in
  // form instead, which is right for a person and wrong to assert as 401.
  probe?: {
    method: string;
    path: string;
    body?: unknown;
    allowedStatus?: number[];
    anonymousStatus?: number[];
  };
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
// E2E_EDITION=ce asks for the community run from a tree that has ee/ (see
// scripts/start-gateway.mjs, which builds accordingly).
export const ENTERPRISE = existsSync(join(__dirname, '..', '..', 'ee')) && process.env.E2E_EDITION !== 'ce';

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
  // acme's id: the organisation tadmin administers and alice belongs to.
  tenantId: string;
  // Every organisation the seed furnished, by name: acme always, globex on the
  // Enterprise build only (a community one serves a single organisation).
  tenants: Record<string, {
    id: string;
    admin: string;
    members: string[];
    groups: Array<{ id: string; name: string }>;
    // Names of its secrets, in the vault scope tenant:<id>.
    vault: string[];
  }>;
  // root, infra-admin, app-admin, tenant-admin, user; and on Enterprise
  // globex-admin and globex-user, the second organisation's people.
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
