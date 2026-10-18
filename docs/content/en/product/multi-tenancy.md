---
title: Multi-tenancy
section: The product
order: 6.5
summary: Several organisations behind one gateway - customers, subsidiaries or departments - each with its members, groups, roles and administrators, and the organisation passed to your services with every request.
---

# Multi-tenancy

Several customers, subsidiaries or departments often use the same internal
applications, and must not see each other. Meerkat keeps them apart at the
door: each one is an **organisation**, and every request is made in exactly one
of them.

> [!NOTE] **Enterprise edition.**
> More than one organisation is part of the Enterprise edition. The Community
> image serves a single organisation, which no screen ever names.

## What an organisation holds

- **Members**, each an administrator or a user of that organisation, enabled or
  not.
- **Groups**, which grant roles from the gateway's role catalogue. A person's
  roles are those of their groups in the organisation they are working in.
- **An owner**, who need not be a member.
- **Settings of its own**: session lifetime, group mode, and in Enterprise its
  working hours.

An administrator of an organisation manages its members, groups and group rules
from the console, and nothing outside it.

## At sign-in

A person with one membership is placed in that organisation without a question.
A person with several chooses one, and can change later from the user button.
In Enterprise, a group rule can make a directory group, a GitHub team or an
OpenID Connect claim a membership at each sign-in.

## What your services receive

The organisation travels with the request, beside the identity and the roles,
in the headers or the signed JWT the route sends. Route rules and per-endpoint
rules are answered in that organisation. A service learns which organisation
the caller works in as a fact, not as a parameter the caller chose.

## Read more

- [Organisations](/docs/access/tenants), in the documentation: memberships, the
  choice at sign-in, what an organisation can override, isolation.
- [Roles and groups](/docs/access/roles).
- [Editions](/product/editions), and how to try Enterprise for free.
