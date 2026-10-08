---
title: Upstream health
section: Operations
order: 218
summary: How long a route waits for its service, when it stops calling it, and how the console knows.
---

# Upstream health

A slow service is not that service's problem: the gateway is on the path of every
request, so a connection held open for three minutes is a connection not serving
anyone else, and the failure surfaces far from where it started.

Three mechanisms answer that, and they are separate on purpose: a **bound** on the
wait, a **breaker** that stops calling, and a **verdict** the console reads.

## How long a route waits

Two bounds, each inherited separately, at three levels:

1. what the route says;
2. otherwise the installation, in **Routes > Global**;
3. otherwise the product: `5s` to connect, `15s` for the first answer.

| Bound | What it covers | Range |
|---|---|---|
| Connect | accepting the connection, TLS handshake included | `1s` to `30s` |
| First answer | the wait for the answer's first line - the service is thinking | `1s` to `10m` |

Past either, the caller gets a `502` instead of waiting.

What is **never** bounded is the body. A download or a websocket already under way
runs as long as it needs: what is bounded is how long a service may take to accept a
connection and to *start* answering.

A maximum exists because these are the guard rails themselves: a response bound of
an hour is a route with no bound at all, written in a way that looks like one.

Routes naming the same pair of bounds share a connection pool, so writing them per
route does not multiply transports.

## The circuit breaker

Bounding the wait stops one slow service from holding a connection forever. It does
not stop the gateway from sending it a thousand more requests that will each wait
their own bound - which is how a service that is merely down takes the gateway's
capacity with it, and how it is met by a stampede the moment it recovers.

So, per route and **off by default**:

- after N **consecutive** failures the route stops calling and serves the
 unavailable page immediately;
- after a cool-down, **one** request is let through. If it works the circuit closes;
 if it fails, the cool-down starts again.

| Setting | Default | Range |
|---|---|---|
| Consecutive failures that trip it | five | two to a hundred |
| Cool-down | `15s` | `1s` to `5m` |

Consecutive on purpose: what this looks for is a service that stopped answering, not
one that fails now and then under load. Any success resets the count.

What counts as a failure is deliberately narrow: a transport error, or a `502`,
`503` or `504` - the answers the gateway itself produces when it could not reach or
could not wait. **Not a `500`**: a service answering `500` is up and has a bug, and
taking it out of rotation would turn one broken endpoint into a whole route nobody
can reach, including the endpoints that work.

It ships off because a breaker turns one kind of failure into another: a service
merely slow to recover gets refused for the whole cool-down. An installation should
meet that because somebody chose it.

## What the console shows

Each route in the Routes list carries a **heart**, and the reason in its tooltip:

| Heart | Means |
|---|---|
| Green | every replica of the service is ready, or an external host accepts a connection |
| Orange | some replicas are ready, not all: the service answers, on less than it was given |
| Red, broken | none is ready, none is wanted (scaled to zero), the host refuses, or the circuit is open |

For a service the runtime runs, the row also says **how many replicas are ready**
(`2/3`, beside the name) and **which image they run** (after the upstream: the tag,
or the start of the digest when the tag is `latest`). Two images on one service is
a deployment under way, or one that stopped half way; the tooltip lists each image
with its digest and how many replicas run it.

### Without a timer

The gateway does not ask the runtime on a schedule: it **listens** to it.

- **Kubernetes**: it lists the Services and pods of its own namespace once, then
  watches them. A pod that starts, becomes ready, crashes or is replaced arrives as an
  event from the API server, and the screen changes with it. A Service's replicas are
  the pods its selector picks. A Service without a selector has no pods to count, and
  is checked like an external host.
- **Docker and Swarm**: one inventory, then Docker's event stream. In a Swarm, a
  container's events come only from the node it runs on, so the gateway listens to
  every node, through the read-only socket proxy the stack deploys on each of them.
- A stream that breaks is reopened, starting with a fresh inventory: nothing that
  happened meanwhile is missed.

What no event can tell is an **external host** going away. Those are checked with a
plain TCP connect every 30 seconds, with a 2 second timeout: a connect and nothing
more, no HTTP request, so nothing lands in the service's logs or counts against its
limits. Behind an `HTTP_PROXY`, the proxy is what gets dialled. A gateway with no
external target runs no timer at all. An external service never shows replicas or
images: they are not the gateway's business.

An open circuit is a target down, whatever the runtime or the connect said: real
traffic wins.

### What it needs

| Runtime | Grant |
|---|---|
| Kubernetes | `list` and `watch` on `pods` and `services` in the gateway's namespace. The chart grants it with `rbac.watch`, on by default - see [Kubernetes](/docs/deploy/kubernetes) |
| Docker | the Docker API, read-only: the socket proxy of the compose file, or the socket mounted for the tunnel |
| Swarm | the socket proxy on every node (`mode: global`), which the stack deploys - see [Which shape to deploy](/docs/deploy/shapes) |

Without the grant, the targets are checked by a connection, and the gateway says once
which right it lacks.

## In a cluster

The breaker's state and the target check are **per node**, and that is the right choice.
Two gateways may genuinely disagree about an upstream - a network path, a DNS answer, a
sidecar - and a shared verdict would let one node's bad minute open a circuit for
everybody.

The cost is stated: a service coming back is discovered once per node rather than
once, which is one probe each. And the health screen answers for **the node that was
asked**.
