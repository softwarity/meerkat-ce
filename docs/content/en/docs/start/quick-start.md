---
title: Quick start
section: Getting started
order: 2
summary: Run the gateway, sign in to the console, and see traffic go through - in about five minutes.
---

# Quick start

Two ports, one container, no database to install. At the end of this page you
have a gateway running, an administrator account, and a request that went
through it.

## Start the gateway

Pick the edition you run and where you run it: the commands and the files below
follow.

::: widget install
:::

The two published ports are two different jobs:

| Port | Plane | Who reaches it |
|---|---|---|
| 8080 | data plane | your users - this is the one that faces the network |
| 9090 | control plane | the administration console - keep it internal |

`/data` holds everything the gateway knows: routes, accounts, the vault, the
certificates. Back up that volume and you have backed up the gateway.

> [!WARNING]
> Never publish the control plane on the internet. It is the console and the
> admin API; nothing about it is meant to be public.

## The first administrator

On its very first start - and only when the account table is empty - the
gateway creates one account, `admin`, with global rights.

- With `MEERKAT_ADMIN_PASSWORD` set, that is its password.
- Without it, a password is generated and **written once** in the log, as a warning line. That account is then flagged to change its password at first sign-in.

```bash
docker logs meerkat | grep 'admin account created'
```

> [!NOTE]
> The variable is read only while there is no account at all. Setting it later
> changes nothing, and it is not a way to reset a forgotten password.

## Sign in to the console

Open **http://localhost:9090** and sign in as `admin`. If the password was
generated, the console asks you for a new one before letting you anywhere else.

## Send a first request through

A fresh gateway is **empty**: it has no route, so every address on port 8080
answers 404. Give it one, pointing at a public test service:

1. In the console, open **Infra > Routes** and click **New route**. Name it
   `demo`.
2. In **Target**, keep **Proxy** and type the upstream: `https://httpbin.org`.
3. In **Predicates**, add a **path** predicate with the pattern `/demo/**`.
4. In **Incoming**, add a **strip-prefix** filter with `1` part: the test
   service knows `/get`, not `/demo/get`.
5. Click **Save**. The route serves at once, with nothing to restart.

Then open **[http://localhost:8080/demo/get](http://localhost:8080/demo/get)**
in your browser.

What you see is httpbin's answer: the request as the service received it, with
the headers the gateway added on the way. The call went in through the gateway
and out to the service. Open **Metrics** in the console to see it counted, on
the `demo` route.

## Next

That route is open to anyone and points at somebody else's service. [Your first
route](/docs/start/first-route) puts one of your own behind the gateway, says
who may reach it, and checks it before your users do.
