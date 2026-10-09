---
title: What Meerkat is
section: The product
order: 1
summary: One entry point in front of your internal applications, which takes care of everything that is not your team's core business.
---

# What Meerkat is

Meerkat is an **app-gateway**: a single door in front of the applications your
organisation runs. Requests arrive at Meerkat, it decides what to do with them,
and passes them on.

What it takes care of, so your services do not have to:

- **Who is calling** - sign-in pages, SSO, multi-factor, API tokens, sessions.
- **Who may pass** - roles, groups, organisations, per-route and per-endpoint rules.
- **How the request travels** - routing, rewriting, headers, rate limits, TLS.
- **What is happening** - traffic, audit trail, metrics, the health of your services.

## Why a gateway

An application is no longer one program: it is a dozen services, written by
different teams. A gateway puts a single door in front of them. The browser
talks to that door only; the door decides who comes in and where each request
goes, and the services behind it receive requests already checked.

::: figure gateway
A request goes in through the gateway, which signs the user in, checks the
rights, routes and records, then hands it to the right service with a signed
JWT. A request without the rights stops at the door.
:::

An internal application usually starts without any of this. Then it needs a
login page, so someone writes one. Then it needs a way to reset a forgotten
password, which means sending e-mails. Then a second factor, because security
asked. Then the customer wants to sign in with their own directory, Active
Directory for one, Azure for the next. Then sessions have to expire, and be
revoked when someone leaves.

Meanwhile a second application needs the same, and the two disagree about what
a session is. A third one arrives, written by another team, in another
language.

Then come the questions nobody planned for. Who may see this screen, and who
decides? Who changed that setting last Tuesday - the audit trail. Why is this
page slow - the traces, from the browser to the database. What happened at
3 a.m. - the logs, in one place rather than one per service. How many calls
may a customer make - the rate limits. The certificates, which expire. The
maintenance page, for the evening of the upgrade.

**None of this is your business.** It is what every application needs before it
can do what it is for, and it ends up written in each of them, maintained in
each of them, reviewed by security in each of them - slightly differently every
time. Meerkat is the place where those questions are answered once, in front of
all your applications, so that your teams write the part only they can write.

## Why an APP gateway rather than an API gateway

An **API gateway** - Kong, APISIX, Traefik - answers one of those questions: how
a request is routed. Everything else is a plugin to configure, or a product to
install beside it: an identity provider for the sign-in, an authentication
proxy, a secrets manager, a metrics stack, a log collector, a certificate
manager, a tunnel for developers to test against the cluster (mirrord,
Telepresence), screens you build yourself. Some eight products to choose, deploy,
secure and upgrade, and to keep in step with each other - the
[case for Meerkat](/product/the-case) counts them.

An **APP gateway** is all of it in one, off the shelf and ready to use: the
sign-in pages, the accounts and roles, the vault, the audit trail, the traces,
the certificates, the developer tunnel and the console that drives them come
in the same image, and already work together. You start it, you add your
applications.

All in one does not mean closed. What your company already runs stays in
charge, and Meerkat plugs into it:

- **Your identity provider** - Entra ID, Okta, Google, Keycloak or any OpenID
  Connect provider, and with Enterprise SAML 2.0, LDAP or Active Directory -
  signs people in; Meerkat does not ask you to move your accounts.
- **Your observability stack** gets what Meerkat sees. Its logs are written as
  OpenTelemetry JSON for the agent already on your nodes; with Enterprise, the
  traces from the browser to your services, the metrics, the logs and the audit
  trail are sent to your collector, and on to Grafana, Datadog, Elastic or
  whatever you use.
- **Your services** keep their own checks if they want them: the JWT they
  receive verifies against the gateway's published keys.
- **Your PKI** stays the authority: import your certificates, or, with
  Enterprise, let your own ACME server issue and renew them.
- **Your git repository** holds the configuration with Enterprise, one
  directory per platform, reviewed like the rest of your code.
- **Your automation** drives it all through the admin API, or an AI agent
  through MCP.

The built-in screens are there so you need nothing else to start - not so you
cannot use what you have. What needs Enterprise is listed on
[editions](/product/editions).

> [!NOTE]
> Meerkat proxies your applications as they are. It does not ask them to embed
> a library or to speak a protocol of its own.

## One image

The gateway is written in pure Go and ships as a single image. It deploys on
Docker, Swarm or Kubernetes, and there is nothing else to deploy to use it: no
database to run beside it, no cache, no message broker. It serves your
applications on one port and its administration console on another.

In figures: the Community image weighs **70 MB**, the Enterprise image
**200 MB**.

## Why the name

::: figure meerkat
Meerkat, standing guard.
:::

The meerkat is nature's sentinel: it stands guard at the burrow entrance and
raises the alert, so the rest of the colony can work without worrying about
anything. That is exactly what this gateway does for your services. Even the
[plug](https://github.com/softwarity/plug) tunnel fits the picture - it is how
a developer's machine digs its way into the burrow. And since a group of
meerkats is called a *mob*, you already know what to call a cluster of Meerkat
nodes.
