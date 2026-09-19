---
title: The traffic screen
section: Operations
order: 203
summary: What the built-in traffic view shows, what it deliberately does not, and how long it remembers.
---

# The traffic screen

The console has always shown what is **configured**. This screen shows what was
actually **served**, which is the half somebody needs when they are on call: a
route that is failing and a route nobody calls look identical in a configuration.

It lives at `/traffic` on the control plane, under the **Metrics** entry of the
rail, and it needs root or the gateway-admin capability.

![The traffic screen](img/console/traffic.webp)

## What it shows

- Two curves over the last hour, one point every `5s`: what was answered and what
  failed.
- Four numbers over the **last minute**. Averaging an hour would keep a
  five-minute incident in the headline long after it ended, so the period is
  written on the screen rather than assumed.
- A ranking of routes on three axes: the slowest, the ones failing, and the ones
  spending the most time overall.
- Opening a route lists **its endpoints**, over the same period the table draws.
  Two clocks on one screen is a screen whose lines do not add up to the row above
  them.

![The same screen, scrolled to the per-route ranking](img/console/metrics.webp)

It is fed by the live channel rather than polled: the gateway pushes each
interval as it is measured.

## The unit of an endpoint is a template

An endpoint is `GET /orders/{id}`, never `GET /orders/1042`. Counting raw paths
would be one series per order, kept for the life of the process, and **chosen by
whoever sends the requests** - a loop over `curl` would take the gateway down
through its own instrument.

A template comes from one of two places, and the screen says which:

- **declared** - an OpenAPI spec deposited on the route, the per-endpoint rules
  somebody wrote, or a spec the control plane resolved from the service itself.
  Somebody wrote it down, so it is exact.
- **deduced** - the shape of a path this gateway saw, with segments that look like
  identifiers folded into `{id}`. It is sometimes wrong: the year in
  `/files/2024/report` is not an id. Those lines are marked **deduced** everywhere
  they appear.

Deduction is bounded twice: by the fold, and by a budget of two hundred templates
per route, everything past it sharing a single bucket.

## What it does not show

- **Individual requests.** The counters are aggregates; nothing records one call
  (OBS-03).
- **Percentiles.** The latency histogram is collected and exposed, but no p95
  curve is drawn here.
- **Traces.** `traceparent` is not propagated to upstreams (OBS-04).
- **Who called.** No label is ever a user, an address or a raw path. That is what
  keeps the cardinality bounded.
- **gRPC outcomes.** A gRPC call always answers `200` and puts its verdict in
  `grpc-status`, which the counters do not read - so a failing gRPC route reads as
  healthy here (ROUTE-20).

## Retention

One hour, in memory, as a ring. A restart starts a new hour and nothing is
written down. Endpoints keep their own coarser history: one point a minute,
seventy minutes of it, written only when something actually happened - so an
endpoint nobody calls costs nothing at all.

## In a cluster

The curves are summed over every node, and the screen says how many nodes the
interval covers.

The difference is taken **per node** and the differences are then added, never the
other way round. Summing totals first and differencing the sum looks equivalent
and is not: the moment a node leaves - a rolling update, a crash - the sum falls
by everything that node had ever counted, and the drop reads as a spike of the
survivors' whole history landing in five seconds.

An endpoint is counted on the node that answered it.
