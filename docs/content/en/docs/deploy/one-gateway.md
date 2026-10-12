---
title: One gateway
section: Deploying
order: 234
summary: Docker Compose and Helm for a single gateway on its own volume, and what each environment variable decides.
---

# One gateway

One binary, three ports. **8080 is the application plane** - what your users
reach. **9090 is the control plane** - the admin console. **7070 is the
internal plane** - what your services call: the JWKS, the scheduler's API.
Only the first one belongs on the internet, and they are three Services in the
chart for exactly that reason: an Ingress in front of your applications must
not be able to reach the console by accident, and publishing the console must
not publish what the backends call. The internal plane is published nowhere:
the services on the same network reach it by name.

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
      # The runtime, through the read-only proxy below.
      DOCKER_HOST: tcp://docker-proxy:2375
    networks: [default, runtime]
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

  # The Docker socket behind a filter that refuses every write.
  docker-proxy:
    image: tecnativa/docker-socket-proxy:v0.4.1
    environment:
      { SERVICES: 1, TASKS: 1, CONTAINERS: 1, NETWORKS: 1, IMAGES: 1, NODES: 1, EVENTS: 1, INFO: 1 }
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks: [runtime]
    restart: unless-stopped

networks:
  runtime:
    internal: true

volumes:
  meerkat-data:
```

The proxy is what lets the gateway read the runtime without being able to act on
it: the route editor offers the containers it finds, and the Routes list shows
whether each target runs and on which image, as Docker's events say (see
[Upstream health](/docs/operations/upstream-health)). Mounting the socket in the
gateway itself would hand whoever controls the gateway the whole host. Leave the
proxy out and the gateway runs as before, without either.

## Opening the developer tunnel

The tunnel plugs a developer's machine into your stack: a service they run on
their laptop answers, for them, under its cluster name, and everything else
goes on talking to it as if it were still deployed. It is
[plug](https://github.com/softwarity/plug) compiled into the gateway - the
community image runs plug beside the gateway instead, which is plug's own
default and needs nothing from Meerkat. What the whole thing is for is on the
[Dev mode](/product/dev-mode) page.

Deploying it opens nothing yet. The tunnel also has a switch of its own in the
console, **Infra, Plug**, which ships off, beside developer mode; that page
records the address developers use and hands them the commands to install plug
on macOS, Linux and Windows. See [Plug](/docs/operations/plug).

The Enterprise file is the one above plus three things: the Enterprise image, a
port, and the socket that lets the agent give a name to a machine. The image
lives in a private registry, and a commercial agreement is what opens it - see
[the editions](/product/editions).

```yaml
services:
  meerkat:
    image: ghcr.io/softwarity/meerkat:latest
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

### The evaluation image

To try Enterprise before talking to anyone, the **evaluation image** is the
Enterprise one with nothing taken out, public on Docker Hub, with an evaluation
notice on its pages and in its console - not licensed for production. Same
file, one line changed, no registry to log into:

```yaml
    image: docker.io/softwarity/meerkat:eval   # or X.Y.Z-eval, to pin a release
```

The published [docker-compose.ee.yml](/deploy/docker-compose.ee.yml) takes its
image from `MEERKAT_IMAGE`, so it runs the evaluation untouched:

```bash
MEERKAT_IMAGE=docker.io/softwarity/meerkat:eval docker compose -f docker-compose.ee.yml up -d
```

## Kubernetes, with Helm

One gateway on its own volume - the shape below. Several gateways serving one
installation is a different set of objects (no volume, a shared database, three
replicas), and it has its own page:
[Kubernetes cluster](/docs/deploy/kubernetes), with the manifests and with the
part nobody writes down - how not to turn the entry point into a single point
of failure.

```bash
helm repo add meerkat https://www.softwarity.io/deploy

helm install meerkat meerkat/meerkat \
  --set admin.password='your-first-password'

# Enterprise, with the tunnel open (`plug.enabled` is already true):
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='your-first-password'

# Evaluation: everything Enterprise does, public, not for production.
helm install meerkat meerkat/meerkat \
  -f https://www.softwarity.io/deploy/values-eval-one-node.yaml \
  --set admin.password='your-first-password'

# Enterprise in production: the developer surface is closed, and the chart
# then grants nothing at all on the namespace.
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='your-first-password' \
  --set production=true
```

What the chart grants this gateway's ServiceAccount, **in its own namespace
only**, is what the agent needs and nothing more:

| on | for |
|---|---|
| `services`, `endpoints` | point a cluster name at a developer's machine, and put it back |
| `endpointslices` (list, deletecollection) | drop the slice Kubernetes keeps for the deployed pod behind a Service with a named port - without it one request in two still reaches that pod |
| `deployments` (get, patch) | its own restart, and the sweep that restores a parked Service |
| `pods` (get, create, delete) | inherit the replaced service's environment, and run - for the length of a session - the pod that serves its volumes to the developer's machine |
| `pods/exec` | read that environment inside the parked pod, the way mirrord does |
| `persistentvolumeclaims` (get) | refuse a `ReadWriteOncePod` volume with a reason, rather than with a pod that never starts |

`pods/exec` is the widest of the seven: in this namespace it amounts to running
code in the pods, and reading an environment a pod has **already resolved** is
reading the secrets it resolved. It is also why the agent never asks for
`get secrets`. It sees no other namespace, and the list is plug's own
(`deploy/plug-k8s.yaml` in that repository): a right the agent grew since would
be discovered at the moment somebody uses the feature, which is the worst place
to find out.

The grant **follows the tunnel** rather than a switch of its own.
`production: true` closes the developer surface: the gateway opens no tunnel,
and the chart then grants strictly nothing - a right nobody exercises is
surface for nothing. Otherwise the tunnel can open - it does once it is switched
on under **Infra, Plug** - so the deployment grants what it may be asked to use,
which is why `plug.enabled` defaults to **true**: the other way round - the
tunnel open, the rights missing - is the one combination that cannot work, with
an agent saying every minute that it cannot do its job. `plug.enabled: false` still refuses the grant outright, for an
installation that wants the developer surface without the tunnel.

On OpenShift or OKD the chart installs as it is, under the default SCC - see [OpenShift and OKD](/docs/deploy/kubernetes#openshift-and-okd) before overriding its security context.

## Behind a proxy or a load balancer

Most gateways are not reached directly: an Ingress controller, nginx, Traefik, a
cloud load balancer or a CDN stands in front. Then the connection the gateway
accepts comes from that proxy, and who the caller really is travels in two
headers the proxy writes: `X-Forwarded-For` (the caller's address) and
`X-Forwarded-Proto` (whether the browser used https).

Those are headers, and anybody can write a header. So the gateway believes them
**only from a proxy it trusts**, and that decides a lot: every per-address
counter (failed sign-ins, password resets, a route's rate limit per IP), the
address the sign-in history and the audit show, whether a session cookie is
`Secure`, and whether the plain port redirects to https.

**What it deduces by default.** With nothing configured, the gateway looks at
the address of the connection's other end - the TCP peer:

- a **loopback or private** peer (`127.0.0.0/8`, `::1`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`) is a proxy: its headers are believed. That is the Ingress in a Kubernetes cluster, the Docker bridge, a compose network, nginx on the same host;
- any **other** peer is a stranger: its `X-Forwarded-*` headers are ignored, and its own address is the caller's.

Behind several proxies, `X-Forwarded-For` is read from the right: each trusted
hop is skipped, and the first address that is not one of them is the caller.
Whatever the caller wrote further left is never read. The carrier-grade NAT
range (`100.64.0.0/10`) is not trusted - it is where subscribers' devices live,
a Tailscale network for one, not proxies - nor is link-local.

**When to set `MEERKAT_TRUSTED_PROXIES`.** When whatever stands in front reaches
the gateway from **public** addresses: a CDN, or a cloud load balancer that does
not come through a private network (Google Cloud's reach the backends from
`130.211.0.0/22` and `35.191.0.0/16`, for example). Name its ranges, as published
by the provider:

```yaml
environment:
  MEERKAT_TRUSTED_PROXIES: "130.211.0.0/22, 35.191.0.0/16"
```

The value is a comma-separated list of CIDR ranges or single addresses, and it
**replaces** the default rather than adding to it: list the private proxy too if
there is one between the load balancer and the gateway. In the chart it is
`trustedProxies: ["130.211.0.0/22", "35.191.0.0/16"]`.

`none` trusts nobody: every address is the TCP peer, and no forwarded header is
ever read. That is the value for a gateway reached **directly** from the
internet, and for a LAN where people reach the gateway from private addresses
without a proxy - the default would trust each of them as one.

A value that does not parse stops the gateway at startup with the reason,
rather than falling back to a default nobody chose. The startup log says which
set is in force (`forwarded headers ... trusted_proxies=`).

**What goes wrong when it is wrong.**

- **Too narrow** (the load balancer is not trusted): every caller is counted as the load balancer. One person's failed sign-ins throttle everybody, the history and the audit show the balancer's address for everyone, and behind a TLS-terminating balancer the gateway believes the browser is on plain http - cookies lose `Secure` and the redirect to https loops.
- **Too wide** (a caller's address is trusted): that caller can name any address they like in `X-Forwarded-For`, mint a fresh budget per request and walk through every counter, and write the address the audit records.

The audit never shows only what a header claimed: when the address came from a
proxy, the line keeps both - `203.0.113.5 via 10.42.0.15`.

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
| `MEERKAT_ADDR` / `MEERKAT_ADMIN_ADDR` | `:8080` / `:9090` | The application and control planes. |
| `MEERKAT_INTERNAL_ADDR` / `MEERKAT_INTERNAL_TLS_ADDR` | `:7070` / `:7443` | The internal plane: the JWKS and the scheduler's API, for the services behind the gateway. Never published; its HTTPS door opens when a certificate is placed on it. |
| `MEERKAT_TLS_ADDR` / `MEERKAT_ADMIN_TLS_ADDR` | `:8443` / `:9443` | The HTTPS doors, opened when a certificate exists for a name. Having one IS the activation - there is no switch that could say "on" while nothing is served. |
| `MEERKAT_DATABASE_URL` | - | An external PostgreSQL instead of the embedded store. Enterprise, and the prerequisite of a [cluster](/docs/deploy/kubernetes). |
| `MEERKAT_VAULT_KEY` | a file under the data directory | The vault master key. It has to be supplied, and identical, on every node of a cluster. |
| `MEERKAT_PLUG_ADDR` | `:22222` | Moves the developer tunnel's port. It does not turn it off: the **Infra, Plug** switch does, and so does closing the developer surface. |
| `MEERKAT_PRODUCTION` | unset | Declares this gateway production and closes the developer surface for good. |
| `MEERKAT_TRUSTED_PROXIES` | loopback and private ranges | Whose `X-Forwarded-For` and `X-Forwarded-Proto` are believed: CIDR ranges or addresses, comma-separated, replacing the default; `none` trusts nobody. See [Behind a proxy or a load balancer](#behind-a-proxy-or-a-load-balancer). |
| `MEERKAT_TENANCY` | `single` | One implicit organisation, or several (Enterprise). Chosen once, at the first start; the console owns it afterwards. |
