---
title: Roles and groups
section: Access control
order: 132
summary: One role catalogue for the gateway, groups per organisation, and how a person's effective roles are worked out.
---

# Roles and groups

Roles are the names your rules are written in - `orders-reader`, `support`,
`billing-admin`. Groups are how people get them.

The split is deliberate: **the catalogue is gateway-wide, the groups belong to an
organisation**. One vocabulary for the whole installation, assembled differently
for each organisation.

## The catalogue

**Application > Roles.** A role has a name, unique across the installation, a
description, optional tags, and at most one parent.

Role names are also written into route rules, so they are restricted to letters,
digits, `-` and `_`. A role whose name breaks that is refused when a route tries
to use it.

**A parent implies its children.** Holding a parent role grants it and everything
below it, all the way down:

```
staff
  support
    support-lead
  billing
```

Someone holding `staff` satisfies a rule asking for `support-lead`. Someone
holding `support` satisfies `support-lead` but not `billing`. Grant the top of a
branch to grant the branch.

A role cannot become its own ancestor - a cycle is refused. Deleting a role lifts
its children to the top level instead of deleting them with it, and removes the
role from every group that held it. Some roles are marked as **system** roles and
cannot be deleted at all.

Tags are free classification - one per microservice, say - and they travel with
the role where expressions can read them. They have no effect on access
decisions.

## Groups

**A group belongs to one organisation** and holds a set of roles from the
catalogue. Its name is unique within that organisation. A member of the
organisation is then assigned groups, and their roles are the roles of those
groups.

Nothing else grants a role. There is no role on an account, and no role outside
an organisation:

> [!WARNING]
> A session with **no active organisation holds no roles at all**. Not the roles
> of another organisation, not a subset - none. Any rule asking for a role
> therefore also requires an active organisation, even when the rule does not
> mention one.

## One group at a time, or all of them

Each organisation chooses how its groups combine:

| Mode | What it does |
|---|---|
| Cumulative (the default) | every group the person is assigned counts at once |
| Exclusive | one group applies, chosen at sign-in |

Exclusive mode exists for the case where somebody wears two hats in the same
organisation and the two must not be worn together. When it is on and the person
has more than one group, they are sent to `/select-group`, and they may switch
later from the user button. An exclusive session that has not chosen carries **no
roles** - the same rule as no organisation.

There is no installation-wide default for this: it is a property of the
organisation, and an organisation that says nothing is cumulative.

## How effective roles are computed

On every request, for the account, the active organisation and the active group:

1. take the groups assigned in **this** organisation - or only the active one, in exclusive mode;
2. take the roles of those groups;
3. expand each role down the hierarchy, adding every descendant;
4. sort, drop duplicates.

The result is what route rules and endpoint rules are answered against, and what
is forwarded upstream with the identity - so your service receives the expanded
list and never has to know the hierarchy.

The computation is remembered for a few seconds per account, organisation and
group. A role granted or withdrawn therefore takes effect within seconds, without
signing anyone out.

## Letting an upstream authority decide

Group assignments can be placed by hand, or projected from what an external
authority reports: a directory group, a GitHub team, a claim from an identity
provider. A rule maps one reported group to one group of one organisation, and
runs on every external sign-in - so membership follows the directory rather than a
spreadsheet.

A rule only ever manages what a rule placed. A membership or a group an
administrator assigned by hand is never removed by a rule, and a rule that would
match everything is refused.

> [!NOTE] **Enterprise edition.**
> Creating and changing group rules is part of the Enterprise edition. Reading
> them, and deleting one, is not gated - so a community image can still see and
> clear what an Enterprise image left behind.
