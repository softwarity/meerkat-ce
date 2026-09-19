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

```bash
docker run -d --name meerkat \
  -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD='choose-one-now' \
  -v meerkat-data:/data \
  docker.io/softwarity/meerkat:latest
```

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

Open **http://localhost:9090**. Sign in as `admin`. If the password was
generated, the console asks you for a new one before letting you anywhere else.

The console has two planes of its own in the left navigation - **Infra** for
the installation (routes, TLS, authentication authorities, configuration) and
**Application** for what your users meet (accounts, roles, built-in pages,
portal).

## See traffic go through

A gateway with no routes answers 404 to everything, so a fresh install seeds
three demonstration routes pointing at `httpbin.org`:

| Route | Matches | Access |
|---|---|---|
| `demo` | `/demo/**` | open to everyone |
| `demo-secure` | `/secure/**` | any signed-in account |
| `trap` | `/**` | open, ordered last - catches what nothing else matched |

```bash
curl -i http://localhost:8080/demo/get
```

The `/demo` prefix is stripped before the call, so what answers is
`https://httpbin.org/get`. Ask for `/secure/get` in a browser instead, and you
land on the sign-in page the gateway serves itself.

> [!TIP]
> These three routes are ordinary routes, stored like any other. Delete them
> once you have your own - starting with the catch-all, which is what makes a
> fresh gateway answer something on every path.

## Next

Point a route at one of your own services: [Your first
route](/docs/start/first-route).
