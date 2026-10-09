---
title: Dev mode
section: The product
order: 7
summary: A developer's workstation joins the cluster and stands in for a deployed service, and everyone looking at the application is told which one, and by whom.
---

# Dev mode

Every team knows the problem: the cluster holds all the services **and all the
data**, and rebuilding that on a laptop is hard, sometimes forbidden. Meerkat
does not copy the environment. It lets a developer run **one service on their
own machine, in place of the deployed one**, for as long as they work on it -
everything else stays as it is. The tunnel behind it is
[plug](https://github.com/softwarity/plug).

> [!NOTE] Enterprise edition
> plug itself is free, and runs beside any stack. What this page describes -
> plug integrated into Meerkat, with a key per developer, the substitution
> named and announced to everyone - is Enterprise. The developers' Swagger,
> further down, is in every edition.

## How it works

An administrator marks a user as a developer (`dev`). The developer adds their
**public SSH key** to their Meerkat account (Profile, Developer), then starts
their service locally through plug:

```bash
plug -p <gateway-host> -s user-mng-service:8080:3000 npm run start
```

- **The workstation reaches the cluster**: the local service calls the other
  services by their names, as if it ran inside the cluster.
- **The cluster reaches the workstation**: `-s` declares a **substitution**.
  Traffic meant for `user-mng-service` now goes to the developer's machine,
  and every route that uses that service follows on its own.
- **It lasts as long as the session**: stop the process and the deployed
  service is back. A gateway restart ends it too.

Installing it, per system, and opening the tunnel: [Plug](/docs/operations/plug).

## Taking an access back

The key lives on the developer's account, and the gateway checks it at every
connection. Remove the key, or the developer role, and the tunnel closes within
seconds: no certificate or token was issued that could outlive it, so there is
nothing to expire and nothing to revoke elsewhere. The profile page shows the
key's fingerprint exactly as OpenSSH prints it.

## Everybody sees who is serving what

A substitution changes what the application in front of you *is*. So everyone
using it is told, by name: "checkout, by Alice" rather than "checkout, by
somebody".

- **At sign-in**, a page names what is substituted and by whom, before handing
  the application over.
- **While you work**, a small strip says it again on every page, and updates
  live as substitutions start and stop.

> [!NOTE]
> That naming is what the integration brings: each developer has their own key,
> so each connection says who. With Community, plug runs on its own beside the
> gateway, with one shared key: a connection then proves the caller *has* plug,
> not who they are, and Meerkat knows nothing of the substitution. See
> [One gateway](/docs/deploy/one-gateway) for the compose file and the Helm
> values that open the integrated tunnel.

## The developers' Swagger, served by the gateway

The gateway sees every API of the installation, and knows the OpenAPI
description each route declares. So Meerkat turns them into one page, at
`/meerkat/apidocs`, open to developers and to nobody else - in every edition.

It is not just one more Swagger UI next to a service:

- **Every API in one place**, disabled routes included: a developer sees what
  exists and what is being built.
- **Nothing to install, nothing from a CDN.** Swagger UI is inside the gateway,
  so an installation without internet has the same page, and no request leaves
  for a third party.
- **The calls go through the gateway**, with its authentication, access rules
  and rewrites. What the page shows is what the service answers in production,
  not what it would answer if called directly.
- **And above all, you choose who is calling.** *Try it out* can act as a given
  **user**, as **groups** or as **roles**, and shows the identity that results.
  That is the question a developer really asks of a protected API: "what does
  it answer someone who only has this role?". Without it, you need one test
  account per case, and people end up testing with their own.

The console has its own page, at `/apidocs`, for Meerkat's administration API:
the one the console and the [AI agent](/docs/agent/overview) use.

## Still to come

- **Limiting a substitution to some users**: today it applies to all traffic.
- **A console screen listing the live sessions**, and an audit entry for every
  substitution. Adding a key is already in the
  [audit trail](/docs/operations/audit), with its fingerprint.
