---
title: What Meerkat is
section: Getting started
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
