---
title: Our expertise
section: Softwarity
order: 2
summary: Angular and Go, gateways and identity, and the part of an internal application nobody wants to write twice.
---

# Our expertise

We work at the two ends of an internal application: the **front door** a
request comes through, and the **screen** an operator actually uses. They are
the two places where an IT team loses the most time and gets the least credit.

## The front door

A gateway is not a router. In front of an internal application it is where
identity, access rules, organisations, quotas and audit either exist or are
scattered across every service behind it. We know that ground:

- **Reverse proxying done properly**: predicates and filters, hot reload,
  header hygiene, the long-lived responses (WebSocket, live channels) that
  every default configuration breaks.
- **Identity**: sessions, signed tokens to upstreams, OpenID Connect, LDAP and
  Active Directory, two-factor, passkeys. And the line we do not cross - an
  external directory authenticates, it never decides roles.
- **Authorisation that survives an audit**: hierarchical roles, groups per
  organisation, a rule per route and per endpoint, and a trail that says who
  changed what, with the before and the after.

## The screen

An administration console is a product, not a form over a database. We build
them with Angular, on the current major, signal-first, with Material as the
design system rather than as a widget shop - and we publish the components we
had to write instead of carrying them from project to project.

The same care goes into the pages an end user meets: the sign-in flow, the
second factor, the account menu, the navigation between applications. Those are
the pages that decide what people think of the whole system, and they are
usually the last ones anybody looks at.

## The languages we choose, and why

::: grid
### Go for what runs

One static binary, no runtime to install, no dependency to patch at three in
the morning. A gateway that starts in milliseconds and holds a connection for
hours is a Go program, and the standard library does most of the work.

### Angular for what is looked at

Large internal applications live for ten years and change hands four times.
Angular's opinions - structure, typing, an upgrade path that is documented
rather than improvised - are worth more over that span than the freedom to
arrange a project however the last developer felt.
:::

## Working with us

> [!NOTE]
> How we engage - support, integration, bespoke work around the products - is
> being written here. In the meantime, the way to reach us is
> [on the contact page](/softwarity/team).
