---
title: GitHub
section: Authentication
order: 108
summary: Sign in with a GitHub account, restricted to the organisations you name.
---

# GitHub

GitHub is plain OAuth2, not OpenID Connect. It proves the account, but it signs
nothing: there is no identity token to verify against published keys, so the
trust rests on the code exchange itself and on TLS. That is a step below an OIDC
provider, and worth knowing before you put it in front of an internal
application.

It is in both images, community included. It suits a gateway whose users are
developers you already know by their GitHub organisation.

## Declare it in Meerkat

**Infra > Authentication > New authority > GitHub.**

| Field | What to put in it |
|---|---|
| Client ID | from the OAuth app, `Iv1.`... |
| Client secret | from the same place. **Required**, unlike OIDC |
| Allowed organisations | comma separated, for instance `acme-io, acme-labs` |

> [!WARNING]
> Leaving **Allowed organisations** empty lets in **any GitHub account in the
> world**. Combined with self-registration it means anyone with a GitHub account
> gets an account here - one that reaches nothing until an administrator places
> it somewhere, but an account nonetheless.

## Declare Meerkat at GitHub

The drawer hands you the two values GitHub's form asks for, with copy buttons
and a link straight to the creation page:

| GitHub's label | What to paste |
|---|---|
| Homepage URL | your data plane origin, `https://apps.acme.io` |
| Authorization callback URL | `https://apps.acme.io/login/github/callback` |

Then paste the Client ID and the secret back into Meerkat.

## The exchange

- Requested scopes: `user:email`, plus `read:org` **only when you named organisations**. Adding that second scope makes GitHub ask the person for consent again, once.
- `state` is minted and checked. There is **no PKCE and no nonce** here: GitHub OAuth apps support neither, so `state` is the only guard against a replayed callback.
- GitHub's habit of answering an error inside a `200` response is handled.

## What Meerkat reads

| Identity | Where from |
|---|---|
| Subject, the stable link | the **numeric** account id, never the login - a login can be renamed and taken by somebody else |
| Username | `login` |
| Full name | `name` |
| E-mail | the account's address, then the verified-address list, preferring the primary verified one |
| Groups | `<org>` for each organisation, `<org>/<team>` for each team |

An address is only marked **verified** when it was found in that verified list.
If the list cannot be read, the address stays unverified - which means it will
not be matched against an existing local account. That is deliberate: matching
on an unverified address is an account takeover.

Groups are fetched only when you named organisations. They become memberships
and roles through a group rule on an organisation.

## How the organisation restriction behaves

The check runs once the identity is assembled and before anything local is
touched. An entry matches if it is the organisation itself, or one of its teams:
both `acme-io` and `acme-io/platform` satisfy `acme-io`. Case is ignored.
Otherwise the person is refused, and told which organisations are allowed.

If Meerkat cannot read the account's organisations at all, the sign-in **fails**
rather than being treated as "belongs to nothing". The usual cause is named in
the error: an organisation that restricts third-party applications has to
approve this OAuth app.

## Test the connection

For GitHub the button only checks that the authorize URL answers. It cannot
validate the client secret - only a real sign-in does that.
