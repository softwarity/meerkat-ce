---
title: What Meerkat is
section: The product
order: 1
summary: One entry point in front of your internal applications, which takes care of everything that is not your team's core business.
---

# What Meerkat is

Meerkat is an **app gateway**: a single door in front of the applications your
organisation runs. Requests arrive at Meerkat, it decides what to do with them,
and passes them on.

What it takes care of, so your services do not have to:

- **Who is calling** - sign-in pages, SSO, multi-factor, API tokens, sessions.
- **Who may pass** - roles, groups, organisations, per-route and per-endpoint rules.
- **How the request travels** - routing, rewriting, headers, rate limits, TLS.
- **What is happening** - traffic, audit trail, metrics, upstream health.

## Why a gateway at all

An internal application usually starts without any of this. Then it needs a
login page, so someone writes one. Then a second application needs the same one,
and the two disagree about what a session is. Meerkat is the place where those
questions are answered once.

> [!NOTE]
> Meerkat proxies your applications as they are. It does not ask them to embed
> a library or to speak a protocol of its own.

## One binary

The gateway is a single Go binary with no dependency: it serves the data plane
on one port and its administration console on another. Nothing else has to be
installed for it to run.

In figures: the Community image weighs **70 MB** - under 20 MB to download,
once the layers are compressed - and the binary depends on **twelve direct
libraries**, in pure Go: no CGO, so nothing to install on the host and nothing
that compiles differently from one machine to the next. That is what makes the
"1 pod" line of [the case](/product/the-case) something other than theory.

The Enterprise image weighs **190 MB**, and the difference is not the gateway:
it is the **[plug](/product/dev-mode) clients** it hands out itself, one signed
binary per operating system and architecture - 80 MB a developer fetches from
the gateway rather than from a download site.

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
