---
title: API tokens
section: Authentication
order: 116
summary: Personal tokens for machine calls through the gateway, and why the control plane's tokens are a different thing entirely.
---

# API tokens

There are two kinds of token, they look identical, and they are not the same
thing at all:

| | Personal API token | Control-plane token |
|---|---|---|
| For | calling your applications **through** the gateway | calling the gateway's **admin API**, or connecting an agent |
| Works on | the data plane port | the admin port only |
| Minted by | anybody signed in, from their profile | **root only**, from the console |
| Managed in | `/profile/tokens` on the data plane | **Infra > Access tokens** |
| Carries | the organisation and group of the session that minted it | a perimeter: how far, over what, from where |

Both are presented the same way, and only that way:

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.acme.io/orders
```

No query parameter, no alternative header. The string starts with `mk_` so it is
greppable in a log and recognisable to a secret scanner. A request that also
carries a session cookie uses the **cookie**: the token is read only when there
is no cookie.

Both kinds live in the same table and look the same to the eye. Only the port
they open tells them apart.

## Personal API tokens

Meant for a script, a cron job or a service that has to reach an application
behind the gateway as a real person.

They are allowed by default; the switch is *Allow personal API tokens* in
**Application > Security**. With it off, the page does not exist - and existing
tokens stop working.

**Minting one**, from `/profile/tokens`: a name, and a lifetime chosen from
thirty, sixty, ninety or three hundred and sixty-five days, or never. Ninety
days is pre-selected. The secret is shown **once**, in a dialog; afterwards the
list shows only its first twelve characters and the date it was last used
(stamped at most once a minute).

**What it carries is the context of the session that created it**: the active
organisation and, in exclusive-group mode, the active group. There is no picker.
If you need a token for another organisation, switch to it first and then mint
one - the page tells you which context it is about to capture, and says so when
there is none.

**What it does not carry is roles.** Roles are recomputed on every request from
the account, the organisation and the group, so a role granted tomorrow applies
to a token minted today, and a role withdrawn stops applying at once.

**Revoking** is on the same page: disable to park it, revoke to destroy it.
Either takes effect across every gateway within seconds. A token also stops the
moment its owner is disabled, falls outside their access window, or is deleted.

> [!WARNING]
> Signing out does **not** revoke your tokens, and neither does changing your
> password. A token is an independent credential with its own lifetime - revoke
> it explicitly.

## Control-plane tokens

These open the administration API and the agent endpoint. Only root can mint
them, because a token acts with its owner's capabilities, and handing out
control-plane access is a root decision.

The screen is **Infra > Access tokens**. Tokens that belong to a connected agent
do not appear there: an agent's connection is managed in the **MCP** section,
which mints the same kind of token through a consent flow instead of a copied
secret.

### The perimeter, on three axes

A perimeter only ever **takes away**. It never grants what the owner does not
already have.

**How far** - the scope:

| Scope | What it opens |
|---|---|
| `metrics` | the `/metrics` exposition and nothing else: a scraper's credential |
| `readonly` | reads, and the testers. What counts as a read is decided per endpoint, not by the HTTP verb |
| `full` | everything its owner may do |

An empty scope reads as `readonly`: the safe value when nothing was said.

**Over what** - the domain: the routing plane (`gateway`), the application's
identity (`app`), or everything (empty). The domain masks the bearer's own
capabilities, and **root is dropped** rather than kept - a domain that left root
standing would confine nothing. So a gateway-domain token minted by root drives
routes and nothing else, administers no organisation, and cannot mint further
tokens.

**From where** - a list of CIDR ranges, empty meaning anywhere. It is judged on
the **TCP peer address**, never on a forwarded header.

> [!WARNING]
> Behind a reverse proxy the control plane sees the proxy's address, not the
> caller's. An address restriction is only meaningful when whatever uses the
> token reaches the port directly.

### Living with them

A token's name, scope, domain, ranges and expiry can all be edited **without
changing the secret**. *Renew* does the opposite: it rotates the secret and keeps
everything else, and the old secret dies the moment it is next used.

Every mutation made with a token is audited with the token's name beside the
account - `admin, via claude-desktop`, not `admin` - so a change an agent made is
attributable after the fact.

Neither the *Allow personal API tokens* switch nor anything else on the
application's Security screen affects them: a control-plane token is a root
capability, not part of that policy.
