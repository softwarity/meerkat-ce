---
title: A route's access rule
section: Access control
order: 134
summary: The one rule that says who may reach a route - level, roles or capabilities, named users - and what happens when a route says nothing.
---

# A route's access rule

Every route carries one rule, in the route editor's **Security** section. It has
three parts, and they are not alternatives:

- **a level**: what kind of caller, in terms of session and tenant;
- **roles**: which roles, held in the active tenant;
- **named users**: people who pass anyway.

The level and the roles are **ANDed**. The named users are **ORed over the top**.

## The levels

| Level | Who passes |
|---|---|
| **Delegated** (nothing chosen) | everyone, signed in or not. No session is even looked up; the service decides |
| **Signed in** | anybody with an account - including one that belongs to no tenant yet |
| **In a tenant** | a session with an active tenant. Turns away an account still awaiting access |
| **In one of these tenants** | the active tenant must be one of those you name |
| **With capabilities** | an account holding one of the capabilities you pick. Roles do not count here (see [below](#with-capabilities)) |
| **Nobody** | refused before the service is called. Only the named users get through |

There is no *public* level, because delegating already is one, and a level named
public would promise something the gateway cannot grant - your service may still
refuse.

*Signed in* letting a member-less account through is deliberate: it is what makes
a waiting-room page, or a self-service profile, reachable by somebody who has
just been recognised and granted nothing.

![A route's own rule: a level, the roles it accepts, and the users it lets through anyway](img/console/route-editor-security.webp)

## Roles

Pick any number; **any one of them grants access**. They are read in the *active*
tenant, and they include everything the person's roles imply through the
hierarchy.

Because the catalogue is global while groups belong to a tenant, the two
axes say genuinely different things:

| Rule | Means |
|---|---|
| roles `billing-admin`, level *In a tenant* | a billing administrator of whichever tenant is active - a cross-tenant console |
| roles `billing-admin`, level *In one of these*, Acme | a billing administrator **of Acme** |

Leaving the roles empty means any role passes - the level alone decides.

> [!NOTE]
> A rule asking for a role always needs a tenant too, whether or not you
> said so: roles only exist inside one. In practice, choose *In a tenant*
> whenever you name a role.

## With capabilities

Some pages are not for whoever holds a role, but for whoever administers: a
members screen, an operations page. A tenant administrator can put any role of
the catalogue in a group, `ROLE_ADMIN` included, so a role cannot guard such a
page. The level **With capabilities** asks what the account **is** instead:

![A members screen's route kept for the administrator of Acme Logistics and for application administrators: the level With capabilities, its capabilities, and the tenant it is narrowed to](img/console/route-editor-capabilities.webp)

| Capability | Who holds it |
|---|---|
| **Root** | root |
| **Infra admin** | an account with the infra admin capability, or root |
| **App admin** | an account with the app admin capability, or root |
| **Developer** | an account with the dev capability |
| **Tenant admin** | the owner of the active tenant, or an enabled **ADMIN** member of it. Offered when the installation runs several tenants |

- **Any one of those you pick grants access**, and root always passes.
- **The level asks no role**: in the editor, the roles give way to the
  capabilities.
- **Tenant admin of** narrows the tenant administrator. Left empty, it is the
  administrator of whichever tenant is active; with tenants named, only the
  administrator of one of them, and that tenant must be the active one. An
  administrator with no active tenant is refused.
- **The whole route only**: an operation's override and a rate limit do not
  offer this level, and the API refuses it there.
- Named users still pass over the top.

The route can also tell its service what the caller is: **Auth forward** offers
the `capabilities` attribute, a list in a JWT and a comma-separated value in a
header, so the page knows which actions to show.

## Named users

A list of usernames that passes **whatever the level requires** - including
*Nobody*. It is the exception mechanism: a service account, a support login, an
application dedicated to one person.

That is also how "only these people" is written: level **Nobody**, plus the
usernames. A rule that names users but poses no level and no role is **not a
rule**, and the names are dropped when it is saved - under a delegated route
everyone is already through, so naming somebody says nothing.

## When a route says nothing

The request is proxied. No session is resolved, no cookie is read, nothing is
refused.

> [!WARNING]
> An empty rule is **not** "authenticated" and **not** "public": it is *not
> gated*. Meerkat adds conditions, it never removes the ones your service applies.
> If a route must require a session, choose *Signed in* at the very least.

## Several routes, one request

A route's rule is part of **choosing** the route, not something applied after.
That has one surprising and useful consequence: a route whose rule turns this
caller away is **passed over**, and the next matching route is tried.

So two routes on the same path can serve two audiences - a rich page for members
of Acme, a landing page for everybody else - simply by ordering them. The refusal
of the first route is remembered and delivered only if no other route answers.

The one exception is **Nobody**: a `deny` refuses on the spot and never falls
through. It is how you close a path rather than reopen it further down.

## What a refusal looks like

On a **UI route**, the person lands on a page in their own language:

| Situation | Where they land |
|---|---|
| Changing tenant would lift the refusal | `/select-tenant`, which says why |
| They belong to no tenant and the rule needs one | `/account-pending`, the waiting room |
| Anything else | `/refused`, naming the rule that turned them away and listing what this session can open |
| No session at all | the sign-in page, with the destination kept |

The offer to switch tenant is filtered against reality: it is only made
when another tenant the person belongs to would actually satisfy the rule.

On a **service route**, the answer is a `403` with a sentence naming what was
missing - the tenant, one of these roles, one of these capabilities, this
endpoint is closed - or a
`401` with `WWW-Authenticate: Session` when there is no session at all. Nobody
reads an HTML page in a `curl`.
