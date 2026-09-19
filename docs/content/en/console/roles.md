---
title: Roles
section: The console
order: 168
summary: The global role catalogue, its hierarchy, and the tags that make it usable.
---

# Roles

**Application > Roles.** One catalogue for the whole gateway: the names your
access rules and your services speak. Roles are global; what binds a person to
one is a [group](/#/docs/console/organisation) inside an organisation.

![The Roles screen: the catalogue as a tree under an All roles row, with descriptions and tags](img/console/roles.webp)

Three top-level roles with their children, each with the sentence a person
granting roles will read, and the tags the group matrix filters on.

## The tree

The table is the catalogue as a tree, with the root row at the top: whatever hangs
on the root is a top-level role.

- **The + on a row** creates a child of that role. The + on the root row creates a
  top-level one.
- **The drag handle** moves a role: drop it on another to make it a child, on the
  root row to make it top-level. It is disabled while a filter is on, because the
  list you see is not the tree.
- **Clicking a row** opens the role in the drawer.
- A **system** mark means a role Meerkat itself relies on.

The hierarchy is not decoration: **a parent implies its children**. Ticking a
parent in the groups matrix ticks what hangs under it, so a catalogue shaped like
your organisation is a catalogue you assign in one click.

## The editor

- **Name** - the technical name, the one a rule or a service reads.
- **Description** - the human sentence the organisation screens put forward. Write
  it: a person granting roles reads this, not the name.
- **Tags** - free labels, proposed from what the catalogue already uses so a
  second spelling does not creep in. The groups matrix and this table filter on
  them, which is what makes a catalogue of two hundred roles workable.
- **Danger zone** - deletion.

The **parent is not a field**: moving a role is what drag and drop is for, and two
ways to move one would only invite them to disagree. A role created from a row's +
is born under that row.

## Traps

- **Renaming a role renames a contract.** Rules, endpoint overrides and services
  that read the name follow the name, not the row. Rename deliberately.
- **A role grants nothing on its own.** It has to be in a group, and someone has
  to be in that group.
- **Tags only help if they are spelt alike.** Take the suggestion the field
  offers.
- **Filter, then drag** does not work: clear the filter to reorder.
