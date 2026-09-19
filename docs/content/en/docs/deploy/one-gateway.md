---
title: One gateway
section: Deploying
order: 234
summary: Docker Compose and Helm for a single gateway on its own volume, and what each environment variable decides.
---

# One gateway

One binary, two ports. **8080 is the application plane** - what your users
reach. **9090 is the control plane** - the admin console. Only the first one
belongs on the internet, and they are two Services in the chart for exactly
that reason: an Ingress in front of your applications must not be able to
reach the console by accident.

Everything the gateway knows - routes, accounts, the sealed vault, the
certificates - lives in one directory. Give it a volume and you have made the
gateway restartable; back that volume up and you have backed up the gateway.

## Docker Compose

The community image, in full:

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080"   # application plane
      - "9090:9090"   # control plane - keep this one internal
    environment:
      # Read ONCE, on the first start, to create the admin account.
      MEERKAT_ADMIN_PASSWORD: your-first-password
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

## Opening the developer tunnel

The tunnel plugs a developer's machine into your stack: a service they run on
their laptop answers, for them, under its cluster name, and everything else
goes on talking to it as if it were still deployed. It is
[plug](https://github.com/softwarity/plug) compiled into the gateway - the
community image runs plug beside the gateway instead, which is plug's own
default and needs nothing from Meerkat. What the whole thing is for is on the
[Dev mode](/product/dev-mode) page.

The Enterprise file is the one above plus three things: the Enterprise image, a
port, and the socket that lets the agent give a name to a machine. The image
lives in a private registry, and a commercial agreement is what opens it - see
[the editions](/product/editions).

```yaml
services:
  meerkat:
    image: ghcr.io/softwarity/meerkat-ee:latest
    ports:
      - "8080:8080"
      - "9090:9090"
      - "22222:22222"   # the developer tunnel
    environment:
      MEERKAT_ADMIN_PASSWORD: your-first-password
      # MEERKAT_PRODUCTION: "1"   # see below
    volumes:
      - meerkat-data:/data
      - /var/run/docker.sock:/var/run/docker.sock
```

> [!WARNING]
> The socket is a real grant. Whoever can act as this container can act on this
> Docker daemon. Mount it on the clusters your developers already trust, and
> nowhere else. Without it the gateway runs exactly as before: the tunnel
> writes one line saying which resource it lacks, and serves as usual.

## Kubernetes, with Helm

One gateway on its own volume - the shape below. Several gateways serving one
installation is a different set of objects (no volume, a shared database, three
replicas), and it has its own page:
[Kubernetes cluster](/docs/deploy/kubernetes), with the manifests and with the
part nobody writes down - how not to turn the entry point into a single point
of failure.

```bash
helm install meerkat ./deploy/helm/meerkat \
  --set admin.password='your-first-password'

# Enterprise, with the tunnel open:
helm install meerkat ./deploy/helm/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat-ee \
  --set admin.password='your-first-password' \
  --set plug.enabled=true
```

`plug.enabled` is what grants this gateway's ServiceAccount the right to
**manage Services in its own namespace** - and nothing else. That is what lets
a session point a cluster name at a developer's machine and put it back
afterwards. The agent cannot read a secret, touch a pod, or see another
namespace. Off, the chart grants nothing at all.

## Declaring a gateway production

`MEERKAT_PRODUCTION` (`production: true` in the chart) closes the whole
developer surface here - the tooling, the API docs, the UI test mode, the
tunnel - whatever the database says.

It is worth setting even where nobody expects a tunnel, and the reason is the
interesting one: **the stored switch travels**. A configuration export, a
restored backup, a database copied from staging can each carry developer mode
ON into production, and nobody notices until there is a tunnel port in front of
customers. The environment does not travel - it is a property of where the
process runs.

It only goes one way: it closes a surface, it never opens one an operator
turned off. The console shows the switch disabled *with the reason* rather than
hiding it, because someone looking for tooling that is not there has to learn
why.

## What each variable does

| Variable | Default | What it decides |
| --- | --- | --- |
| `MEERKAT_ADMIN_PASSWORD` | - | The first admin account, on the FIRST start only. Never read again. |
| `MEERKAT_DATA` | `data` | Where the gateway keeps everything it knows. |
| `MEERKAT_ADDR` / `MEERKAT_ADMIN_ADDR` | `:8080` / `:9090` | The two planes. |
| `MEERKAT_TLS_ADDR` / `MEERKAT_ADMIN_TLS_ADDR` | `:8443` / `:9443` | The HTTPS doors, opened when a certificate exists for a name. Having one IS the activation - there is no switch that could say "on" while nothing is served. |
| `MEERKAT_DATABASE_URL` | - | An external PostgreSQL instead of the embedded store. Enterprise, and the prerequisite of a [cluster](/docs/deploy/kubernetes). |
| `MEERKAT_VAULT_KEY` | a file under the data directory | The vault master key. It has to be supplied, and identical, on every node of a cluster. |
| `MEERKAT_PLUG_ADDR` | `:22222` | Moves the developer tunnel's port. It does not turn it off: closing the developer surface does. |
| `MEERKAT_PRODUCTION` | unset | Declares this gateway production and closes the developer surface for good. |
| `MEERKAT_TENANCY` | `single` | One implicit organisation, or several (Enterprise). Chosen once, at the first start; the console owns it afterwards. |
