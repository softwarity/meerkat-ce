---
title: A route's access rule
section: Access control
order: 134
summary: The one rule that says who may reach a route - level, roles, named users - and what happens when a route says nothing.
---

# A route's access rule

Every route carries one rule, in the route editor's **Security** section. It has
three parts, and they are not alternatives:

- **a level**: what kind of caller, in terms of session and organisation;
- **roles**: which roles, held in the active organisation;
- **named users**: people who pass anyway.

The level and the roles are **ANDed**. The named users are **ORed over the top**.

## The levels

| Level | Who passes |
|---|---|
| **Delegated** (nothing chosen) | everyone, signed in or not. No session is even looked up; the service decides |
| **Signed in** | anybody with an account - including one that belongs to no organisation yet |
| **In an organisation** | a session with an active organisation. Turns away an account still awaiting access |
| **In one of these organisations** | the active organisation must be one of those you name |
| **Nobody** | refused before the service is called. Only the named users get through |

There is no *public* level, because delegating already is one, and a level named
public would promise something the gateway cannot grant - your service may still
refuse.

*Signed in* letting a member-less account through is deliberate: it is what makes
a waiting-room page, or a self-service profile, reachable by somebody who has
just been recognised and granted nothing.

## Roles

Pick any number; **any one of them grants access**. They are read in the *active*
organisation, and they include everything the person's roles imply through the
hierarchy.

Because the catalogue is global while groups belong to an organisation, the two
axes say genuinely different things:

| Rule | Means |
|---|---|
| roles `billing-admin`, level *In an organisation* | a billing administrator of whichever organisation is active - a cross-organisation console |
| roles `billing-admin`, level *In one of these*, Acme | a billing administrator **of Acme** |

Leaving the roles empty means any role passes - the level alone decides.

> [!NOTE]
> A rule asking for a role always needs an organisation too, whether or not you
> said so: roles only exist inside one. In practice, choose *In an organisation*
> whenever you name a role.

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
| Changing organisation would lift the refusal | `/select-tenant`, which says why |
| They belong to no organisation and the rule needs one | `/account-pending`, the waiting room |
| Anything else | `/refused`, naming the rule that turned them away and listing what this session can open |
| No session at all | the sign-in page, with the destination kept |

The offer to switch organisation is filtered against reality: it is only made
when another organisation the person belongs to would actually satisfy the rule.

On a **service route**, the answer is a `403` with a sentence naming what was
missing - the organisation, one of these roles, this endpoint is closed - or a
`401` with `WWW-Authenticate: Session` when there is no session at all. Nobody
reads an HTML page in a `curl`.
