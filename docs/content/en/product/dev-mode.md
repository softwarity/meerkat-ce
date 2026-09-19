---
title: Dev mode
section: The product
order: 5
summary: A developer's workstation joins the cluster and stands in for a deployed service, and everyone looking at the application is told which one, and by whom.
---

# Dev mode

The problem every internal-application team knows: the cluster holds all the
services **and all the data**, and that environment is hard, sometimes
forbidden, to reproduce on a developer's machine. Meerkat's answer is not to
duplicate the environment but to bring the workstation **into the routing
mesh**, like a tenant restricted to routing, powered by
[plug](https://github.com/softwarity/plug).

## How it works

An admin flags a user as `dev`. The developer deposits their **public SSH key**
on their Meerkat account (Profile, Developer, key), then runs their service
locally through plug:

```bash
plug -p cluster --service user-mng-service npm run start
```

- **Workstation to cluster**: the local process resolves and reaches the cluster
  services as if it ran inside the cluster - plug's original behaviour.
- **Cluster to workstation**: `--service` declares a **substitution**. Traffic to
  `user-mng-service` goes through the reverse tunnel to the local process, which
  is seen, inbound and outbound, as a full member of the cluster. Every route
  referencing the service follows automatically.
- **It lives as long as the session**: the substitution vanishes when the process
  stops, and a gateway restart takes both ends with it.

## A key, not a certificate

A certificate authority was examined and dropped. A certificate carries its own
validity: once issued it is accepted until it expires, so taking it back means
waiting or standing up revocation machinery. A gateway is up by definition - it
can answer *is this key still allowed* at every single connection. So
**revoking is deleting the line**, and someone who leaves stops being able to
tunnel within seconds. The profile page shows the SHA256 fingerprint OpenSSH
itself prints, and no expiry date: the absence is the design.

## Nobody picks a variant, everybody is told

There is no menu of developers to choose from, and no tester role. A
substitution changes what the application in front of you *is*, so it is news
for everyone looking at it: Meerkat's job is to make the current state
**visible and attributed** - "checkout, by Alice" rather than "checkout, by
somebody" - not to offer a choice nobody can make correctly.

That attribution is what the Enterprise edition adds. Standalone plug holds one
key baked into the published binary, so a connection proves the caller *has*
plug, not who they are - which is honest, and enough for the trusted clusters
it targets.

> [!NOTE]
> The tunnel embedded in the gateway is Enterprise. plug itself stays a product
> of its own: the community image runs it as a standalone agent beside the
> gateway, which is plug's own default and needs nothing from Meerkat. See
> [One gateway](/docs/deploy/one-gateway) for the compose file and the Helm
> values that open it.

## Still to come

The state is served but not yet shown: a page after login naming what is
substituted, a strip that stays true while you work, a console screen listing
the live sessions, and the audit trail of every key deposited and every
substitution posed.
