---
title: Tenants
section: The console
order: 178
summary: One organisation at a time - its name, hours, groups, members, rules, and the destructive acts.
---

# Tenants

The **Tenants** entry exists only when the gateway serves
[several organisations](/docs/console/application). Clicking it lands on the
first organisation you administer; the drawer lists the others and carries the
**New tenant** button. There is no list page: an organisation is a thing you work
inside.

The same screens serve root, the organisation's owner and its administrators - the
API scopes every call.

## The sections

Each is a route of its own, so deep links work.

| Section | What it holds |
|---|---|
| **General** | The name, the description, the enabled switch, and the working hours |
| **Groups** | The roles-to-groups matrix, and the group mode |
| **Members** | Who is in this organisation and in which groups |
| **Group rules** | What an authority says, turned into membership here |
| **Danger zone** | Transfer of ownership, and deletion |

**Groups**, **Members** and **Group rules** are the same screens documented under
[Groups, Members and Group rules](/docs/console/organisation) - in
single-organisation mode they sit under Application instead, because there is one
organisation and nobody names it.

## General

- **Name** and **Description** - the name is what the drawer, the access rules and
  the sign-in organisation picker show.
- **Enabled** - part of this section's Save, not a switch in the header.
- Under the name, the line says when it was created and by whom, and who owns it.
- **Working hours** - this organisation's own access window, or the application's
  if it defines none. The form says what it is inheriting.

> [!NOTE]
> Enterprise edition: working hours.

## Danger zone

- **Transfer ownership** - a tenant has a single owner, and ownership is
  independent of membership: the previous owner keeps their membership unchanged,
  and the new owner need not be an administrator.
- **Delete this tenant** - removes the organisation, its memberships and its
  groups, with a type-to-confirm. There is no undo.

## Traps

- **Disabling an organisation cuts access for everyone in it** who needs one. It
  deletes nothing.
- **Switching the gateway back to a single organisation serves the first one
  only.** The others are not deleted, and the
  **License** screen says how many are being held back.
- **A member is not an account.** Create the account on
  [Users](/docs/console/users) first; here you place it.
