import { Role } from '../api.service';
import { TreeGuide } from './tree-prefix.component';

// One row of a flattened role hierarchy: the role and the guide glyphs that
// draw its branch (see TreePrefixComponent).
export interface RoleTreeRow {
  role: Role;
  guides: TreeGuide[];
}

// Depth-first flatten of the parentId hierarchy, siblings in name order, with
// the guides materializing each branch.
//
// Shared by the roles page and the groups matrix, because two screens drawing
// the same catalogue with two different pictures of it is how a reader learns
// to distrust both. Sorting inside a level rather than globally is what makes
// two loads give the same order.
//
// An orphaned parentId (a parent deleted mid-flight) degrades to top level: an
// orphan has to stay reachable, or nobody can fix it from the screen.
//
// The list is taken AS GIVEN: a caller that filters passes the surviving set,
// and the guides are computed over that set. Computing them over the whole
// catalogue would draw a branch to a sibling nobody can see.
// A filtered row: the same row, plus whether it matched on its own or was
// pulled in to hold the branch up.
export interface FilteredRoleRow extends RoleTreeRow {
  context: boolean;
}

// Search by name/description plus a single tag, over the TREE.
//
// Shared by the roles page and the groups matrix: they draw one catalogue, so
// they must also narrow it the same way - a search that keeps a role on one
// screen and drops it on the other teaches a reader to distrust both.
//
// Ancestors of a match come along, dimmed: a row drawn under nothing reads as
// a top-level role, at the very moment someone filtered in order to understand
// where a role sits.
export function filterRoleTree(roles: Role[], query: string, tag: string): FilteredRoleRow[] {
  const q = query.trim().toLowerCase();
  if (!q && !tag) return flattenRoles(roles).map((row) => ({ ...row, context: false }));

  const matches = new Set<string>();
  for (const r of roles) {
    const matchQ =
      !q || r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q);
    const matchTag = !tag || (r.tags ?? []).includes(tag);
    if (matchQ && matchTag) matches.add(r.id);
  }

  const parentOf = new Map(roles.filter((r) => r.parentId).map((r) => [r.id, r.parentId!]));
  const visible = new Set(matches);
  for (const id of matches) {
    const seen = new Set<string>();
    let cur = parentOf.get(id);
    while (cur && !seen.has(cur)) {
      seen.add(cur); // a cycle would otherwise walk forever
      visible.add(cur);
      cur = parentOf.get(cur);
    }
  }
  return flattenRoles(roles.filter((r) => visible.has(r.id))).map((row) => ({
    ...row,
    context: !matches.has(row.role.id),
  }));
}

export function flattenRoles(roles: Role[]): RoleTreeRow[] {
  const ids = new Set(roles.map((r) => r.id));
  const byParent = new Map<string, Role[]>();
  for (const r of roles) {
    const key = r.parentId && ids.has(r.parentId) ? r.parentId : '';
    byParent.set(key, [...(byParent.get(key) ?? []), r]);
  }
  for (const children of byParent.values()) children.sort((a, b) => a.name.localeCompare(b.name));

  const out: RoleTreeRow[] = [];
  const walk = (parentId: string, prefix: TreeGuide[]): void => {
    const children = byParent.get(parentId) ?? [];
    children.forEach((r, i) => {
      const last = i === children.length - 1;
      out.push({ role: r, guides: [...prefix, last ? 'attach-end' : 'attach-continue'] });
      walk(r.id, [...prefix, last ? 'empty' : 'continue']);
    });
  };
  walk('', []);
  return out;
}
