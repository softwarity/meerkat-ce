---
title: Metrics
section: The console
order: 182
summary: What the gateway has actually served - five figures, two curves, and a ranking of routes down to their endpoints.
---

# Metrics

The **Metrics** entry in the rail is what the gateway has really served. Every
other screen shows what is **configured**, and in a configuration a route that is
failing and a route nobody calls look identical.

It is at `/traffic`: outside `/api`, the paths of this port belong to the product.
Root and infra admins only: the figures name every route and every service, which
is a map of the installation.

![The Metrics screen: five figures over the last minute, the traffic and latency curves, and the ranking of routes](img/console/traffic.webp)

Requests per second, the refused-or-failed share, the mean answer, the p95 and
what is in flight; under them the two curves, and the ranking with its three
tabs - the failing one carrying its count.

## What is on the page

- **Over the last minute**: requests per second, the share refused or failed, the
  mean answer, the **p95** - the time 95% of the answers came under, which the
  mean hides: ninety fast answers and ten of three seconds average out to
  something nobody waited for - and how many requests are in flight.
- **Traffic** and **How long an answer takes**: two curves, the second drawing
  the mean and the p95 side by side, fed by the gateway pushing an interval every
  five seconds. Nothing is polled.
- **Routes**, ranked on one of three axes - **Slowest**, **Failing**,
  **Costliest** - over the window the samples cover. The Failing tab carries its
  count, so it catches the eye without anyone switching tabs to find out whether
  there is anything to switch for.

![The ranking of routes, scrolled: five routes with their requests, failures, mean answer and time spent](img/console/metrics.webp)

Further down the same screen: the five routes over the minutes the samples
cover. *Inventory (maintenance)* answers every request in under a millisecond
and counts them all as refused - which is exactly what a maintenance route does.

A route **opens on its endpoints**, in the same table and over the same period, so
the lines add up. The unit is the template (`/orders/{id}`), never the raw path: a
raw path would be one series per order.

An endpoint row marked **deduced** was inferred from the shape of the paths this
gateway saw, not from a spec - the segments that looked like identifiers were
folded away. It can be wrong, since a year reads like an id. Declare an OpenAPI
spec on the [route](/docs/console/routes) for exact names.

The curves are drawn even when empty: *nothing measured yet* and *not connected*
are two answers a reader has to be able to tell apart.

> [!NOTE]
> The window is held in memory and starts again when the gateway restarts. On a
> cluster, samples are summed over every node, and an endpoint is counted on the
> node that answered - the screen says so where it matters.

## Keeping a history

The counters leave the gateway one way: pushed over OTLP to a collector, from
**Infra, OpenTelemetry**, tab **Metrics**, and written by the collector into
Prometheus. See [metrics](/docs/operations/metrics).

> [!NOTE]
> Enterprise edition: getting the counters out. The counters and this screen are
> in both editions - curves with nothing to install is what the community image
> promises; what is sold is externalising them into a stack you already run.

## Traps

- **The counters are per node.** Each node pushes its own, under its own
  `service.instance.id`, and a dashboard sums them.
- **A restart resets the curves.** The history is in Prometheus, when the
  counters are pushed there.
- **A route nobody has called has nothing to say**, and neither do its endpoints.
  That is not a failure.
- **Deduced templates can be wrong.** Treat them as a hint until a spec is
  declared.
