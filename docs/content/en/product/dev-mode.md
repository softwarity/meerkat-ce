---
title: Dev mode
section: The product
order: 7
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

## Taking an access back

The key lives on the developer's account, and the gateway checks it at every
connection. Removing it, or removing dev mode, closes the tunnel within
seconds: nothing was issued that would outlive the removal, so there is no
expiry to wait for and no revocation to propagate anywhere else. The profile
page shows the SHA256 fingerprint OpenSSH itself prints.

## Everybody sees who is serving what

A substitution changes what the application in front of you *is*. So it is not
a developer's preference, it is news for everyone looking at it, and it is
named: "checkout, by Alice" rather than "checkout, by somebody".

- **At sign-in**, a page names what is substituted and by whom, before handing
  the application over.
- **While you work**, a collapsible strip says it again on every page, and
  follows substitutions live as they appear and disappear.

> [!NOTE]
> The tunnel embedded in the gateway is Enterprise, and it is what brings that
> attribution: every developer deposits their own key, so a connection says
> who. The community image runs plug as a standalone agent beside the gateway,
> with a shared key: a connection then proves the caller *has* plug, not who
> they are. See [One gateway](/docs/deploy/one-gateway) for the compose file
> and the Helm values that open it.

## Still to come

The **scope** of a substitution: today it holds for all traffic, and nothing
yet limits it to the people who asked for it. On the operations side, what is
missing is the console screen listing the live sessions, and the audit entry
for every key deposited and every substitution posed.
