---
title: Groups, Members and Group rules
section: The console
order: 172
summary: Binding people to roles - the groups of an organisation, who is in them, and the rules that fill them from a directory.
---

# Groups, Members and Group rules

Three screens that administer **the organisation being served**. In
single-organisation mode they live under **Application**, because there is one
organisation and nobody names it. With several, the same screens are reached per
organisation from [Tenants](/#/docs/console/tenants), where the question *which
one* is asked.

The chain is short: a **role** is global, a **group** is a bag of roles inside one
organisation, and a **membership** puts a person in groups.

## Groups

![The Groups screen: the role catalogue down the side, three group columns, a tick where a role belongs to a group](img/console/groups.webp)

Three groups - Finance, Front desk, Warehouse - against the catalogue. Each row
shows the role's description over its technical name, and the *Whole column* row
sits just under the headers.

A matrix: the **role catalogue** down the side, this organisation's **groups**
across the top, a tick where a role belongs to a group.

- **Ticking a parent role ticks what hangs under it.** The catalogue's hierarchy
  is doing the work.
- **A group column's menu** renames or deletes it; the round + button adds one.
- **The Whole column row** under the headers ticks or clears a whole group over
  the rows currently on screen: what you see is what you tick.
- **The search and the tag picker** narrow the rows. On a large catalogue, tags
  are what make this screen usable.
- Everything saves on the click.

Above the matrix, **Group mode** decides how a person's groups combine:

| Mode | Effect |
|---|---|
| **Cumulative** | The roles of every group the person is in are merged |
| **Exclusive** | One group is picked at sign-in |

## Members

![The Members screen: the accounts down the side, the same three group columns, and the last connection](img/console/members.webp)

The same installation in single-organisation mode: no membership column and no
admin badge, just who is in which group, and when each account last signed in.

A matrix again: the accounts down the side, this organisation's groups across the
top.

- In several-organisations mode, the **first column** is membership itself: the
  tick joins or leaves, and the **admin** badge beside it promotes a member to
  administrator of the organisation. The owner shows a read-only **owner** badge
  instead, and ownership is transferred in the organisation's Danger zone.
- In single-organisation mode those two columns are gone: an enabled account **is**
  a member here, and the `app admin` capability already says who administers.
- Group columns are disabled for a non-member: join first.
- The last column shows the last connection and carries the password reset, scoped
  to this organisation.

Members and [Users](/#/docs/console/users) answer two different questions and stay
apart: Users is the account, this is the assignment.

## Group rules

What an **authority** says, turned into membership and groups here. A GitHub
organisation or team, an LDAP group, a claim from an identity provider: a rule
says *anyone from this authority*, or *anyone whose group is X*, and grants the
groups on the right.

The screen refuses to show an empty table with a button that leads nowhere: with
no enabled authority it says to go and add one, and with no group it says to create
one first.

> [!NOTE]
> Enterprise edition: group rules come with the directories feature.

Two things about rules that surprise people:

- **Nothing changes until people sign in again.** A rule is applied on arrival, so
  deleting one takes back what it granted at each person's next sign-in.
- **What an administrator placed by hand stays.** A rule fills groups; it does not
  own them.

## Traps

- **In several-organisations mode these entries disappear from Application.** They
  are per organisation, under Tenants.
- **Exclusive mode changes what people see at sign-in**: they are asked which
  group. Do not switch it on a live installation without telling anyone.
- **A group with no roles grants nothing**, and a role in no group reaches nobody.
- **Deleting a group loses its assignments.** The people stay, their roles from
  that group do not.
