---
title: Authenticating and authorising
section: Access control
order: 130
summary: The difference between proving who someone is and deciding whether they may pass, and where each rule is written.
---

# Authenticating and authorising

Two different questions, answered in two different places.

**Authenticating** is proving who somebody is: a password, a provider, a
passkey, a token. It happens once, at the start, and it produces a session or a
recognised token. That is the [Authentication](/docs/auth/overview) section.

**Authorising** is deciding whether *this* caller may have *this* request. It
happens on every request, and it is what this section is about.

> [!NOTE]
> Meerkat gates **in addition** to your service, never instead of it. It never
> tells your application that a request is legitimate; it refuses the ones that
> are not allowed to arrive.

## Where a rule can be written

| Rule | Where | Answers |
|---|---|---|
| A route's access rule | the route's *Security* section | may this caller reach this route at all |
| An endpoint rule | **Infra > Endpoint security**, from the route's OpenAPI spec | may this caller call *this operation* |
| Membership and groups | an organisation's members and groups | which roles this person holds, here |
| The role catalogue | **Application > Roles** | which role names exist, and which imply which |
| Capabilities | **Application > Users** | who may administer the gateway itself |

Roles come from groups, groups belong to an organisation, and **roles only exist
inside an organisation**: a session with no active organisation holds no roles at
all, whatever the person is a member of elsewhere.

## The order on one request

1. The route's **predicates** decide which routes could answer.
2. The route's **access rule** is evaluated - as part of choosing the route, not after it. If the rule poses no condition, the request is proxied immediately and no session is even looked up.
3. A session or a `Bearer` token is resolved, and the rule is answered against the caller.
4. The route's gates and rate limits run.
5. If the route carries endpoint rules, the operation is matched and its own rule is answered.
6. The request is proxied, carrying the identity as headers or a signed JWT.

Two consequences of step two being part of the selection:

- **The first matching route whose rule accepts the caller wins.** A route whose rule turns this caller away is passed over, and the next matching route is tried. The refusal of the first one is remembered, and is what the caller gets if nothing else answers.
- **A `deny` rule never falls through.** It refuses there and then.

## The default nobody should get wrong

A route that declares **no** access rule is *delegated*: the gateway poses no
condition, does not resolve a session, and hands the request to the upstream,
which applies whatever rules it has of its own.

> [!WARNING]
> No rule means **not gated**. It does not mean "authenticated" and it does not
> mean "public" - there is no *public* level to choose, because delegating already
> is one. If a route must require a session, say so: `auth` at the very least.

## What a refusal looks like

A refusal on a **UI route** lands on a page, in the visitor's language: the
organisation chooser when changing organisation would actually help, the waiting
room when the account belongs nowhere, and otherwise `/refused`, which names the
rule that turned them away and offers what this session *can* open.

A refusal on a **service route** is a `403` with a sentence: nobody reads a page
in a `curl`. A caller with no session at all gets `401` and, on a navigation, a
redirect to the sign-in page.

## What is not there

- **No global switch to turn access control off.** The equivalent is route-by-route: leave a rule empty and that route is delegated.
- **No impersonation.** There is no "sign in as this person" for an administrator, in any edition.
