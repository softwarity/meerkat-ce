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

Each route in the Routes list carries a **heart**: green when its target is up,
broken and red when it is down, and the reason in the tooltip.

The gateway checks every enabled route's target in the background, every 30
seconds, never on the request path:

- a service that discovery knows (Docker, Swarm) is up when at least one replica is
  ready, and down at zero;
- anything else - an external host, a Kubernetes service - gets a plain TCP connect
  with a 2 second timeout. A connect and nothing more: no HTTP request, so nothing
  lands in the service's logs or counts against its limits. Behind an `HTTP_PROXY`,
  the proxy is what gets dialled;
- an open circuit is a target down, whatever the connect said: real traffic wins.

A change is pushed to the open screen at once, no reload needed. A route nobody calls
now has something to say. Not there yet: a service's own `/health`.

## In a cluster

The breaker's state and the target check are **per node**, and that is the right choice.
Two gateways may genuinely disagree about an upstream - a network path, a DNS answer, a
sidecar - and a shared verdict would let one node's bad minute open a circuit for
everybody.

The cost is stated: a service coming back is discovered once per node rather than
once, which is one probe each. And the health screen answers for **the node that was
asked**.
