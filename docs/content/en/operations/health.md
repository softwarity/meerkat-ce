---
title: Health probes
section: Operations
order: 212
summary: Which of /healthz and /readyz answers what, and why the liveness probe never touches the database.
---

# Health probes

Two probes, served on **both** planes, answering two different questions. Pointing
an orchestrator at the wrong one is the most expensive mistake available here, so
they are kept apart on purpose (OBS-02).

## /healthz is liveness

It answers `UP` unconditionally, with the version.

That is the correct answer rather than a lazy one. Liveness decides whether to
**kill** the process. A liveness probe that failed because the database was
unreachable would turn a database blip into a restart of every node at once - each
one killed for a fault none of them has, and none of them repairs by dying. What
belongs to a dependency belongs to readiness.

```json
{"status":"UP","version":"dev"}
```

## /readyz is readiness

It decides whether to **send traffic**, so it asks the two questions that make a
node useless:

- **does the store answer?** A ping, bounded to two seconds. A readiness check that
  hangs is a readiness check that says nothing, and the orchestrator's own timeout
  then decides on no information at all.
- **has the router compiled its table at least once?** A node that has accepted
  connections but never compiled answers `404` to everything, which a rolling
  update happily reads as a healthy instance and feeds live traffic.

On failure it is a `503` **with the reason**, because an operator reading a probe
failure needs to know which of the two it was.

```json
{"status":"DOWN","reason":"the store is not answering","version":"dev"}
```

```json
{"status":"DOWN","reason":"the routing table has not been compiled yet","version":"dev"}
```

A node whose database has gone still holds its compiled table and still answers
requests, while it can no longer resolve a session, read a setting or reload a
route. That is exactly the state that used to declare itself ready.

## Both escape the HTTPS redirect

Where the plain port redirects to HTTPS, these two paths are exempt: a `308` reads
as "not ready" to most probes, and a gateway would be taken out of rotation for
being correctly configured.

## In a cluster

The load balancer probes `/readyz` and **never** `/healthz`. There is no session
affinity to ask for - sessions live in the database, and the five-second cache is
invalidated by the change bus - and no primary node to aim at, since the advisory
lock guards whatever must happen only once.
