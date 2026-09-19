---
title: Metrics
section: The console
order: 182
summary: What the gateway has actually served - four figures, two curves, and a ranking of routes down to their endpoints.
---

# Metrics

The **Metrics** entry in the rail is what the gateway has really served. Every
other screen shows what is **configured**, and in a configuration a route that is
failing and a route nobody calls look identical.

It is at `/traffic`, because `/metrics` on this port is the Prometheus exposition.
Root and infra admins only: the figures name every route and every service, which
is a map of the installation.

![The Metrics screen: four figures over the last minute, the traffic and latency curves, and the ranking of routes](img/console/traffic.webp)

Requests per second, the refused-or-failed share, the mean answer and what is in
flight; under them the two curves, and the ranking with its three tabs - the
failing one carrying its count.

## What is on the page

- **Over the last minute**: requests per second, the share refused or failed, the
  mean answer, and how many requests are in flight.
- **Traffic** and **How long an answer takes**: two curves, fed by the gateway
  pushing an interval every five seconds. Nothing is polled.
- **Routes**, ranked on one of three axes - **slowest**, **failing**, **costliest**
  - over the window the samples cover. The failing tab carries its count, so an eye
  is caught without switching to find out whether there is anything to switch for.

![The ranking of routes, scrolled: five routes with their requests, failures, mean answer and time spent](img/console/metrics.webp)

Further down the same screen: the five routes over two minutes. *Inventory
(maintenance)* answers every request in 0.4 ms and counts them all as refused -
which is exactly what a maintenance route does.

A route **opens on its endpoints**, in the same table and over the same period, so
the lines add up. The unit is the template (`/orders/{id}`), never the raw path: a
raw path would be one series per order.

An endpoint row marked **deduced** was inferred from the shape of the paths this
gateway saw, not from a spec - the segments that looked like identifiers were
folded away. It can be wrong, since a year reads like an id. Declare an OpenAPI
spec on the [route](/#/docs/console/routes) for exact names.

The curves are drawn even when empty: *nothing measured yet* and *not connected*
are two answers a reader has to be able to tell apart.

> [!NOTE]
> The window is held in memory and starts again when the gateway restarts. On a
> cluster, samples are summed over every node, and an endpoint is counted on the
> node that answered - the screen says so where it matters.

## Prometheus

The button in the header opens the other half of the feature, and its state is on
its face: nobody has to open the drawer to find out whether anything is scraping.

Inside: the switch that exposes `/metrics`, and the files to write - a single
`prometheus.yml` carrying both discovery styles (one per platform, the one you
download comes back with its block uncommented), the Grafana datasource and a
provisioned dashboard, a Docker Swarm compose file, and a Kubernetes
ServiceMonitor. They carry this installation's real listening port and network
name, not the address you reached the console by.

`/metrics` answers on the console's port and needs a token minted with the
**metrics** perimeter, which opens that one path and nothing else. See
[Access tokens](/#/docs/console/access-and-agents).

> [!NOTE]
> Enterprise edition: exposing `/metrics`. The counters and this screen are in
> both editions - curves with nothing to install is what the community image
> promises; what is sold is externalising them into a stack you already run.

## Traps

- **Prometheus scrapes every node, not the service in front of them.** The
  counters are per node, so a load balancer would hand a different one to each
  scrape and draw a curve that is nobody's.
- **A restart resets the curves**, not the counters Prometheus reads.
- **A route nobody has called has nothing to say**, and neither do its endpoints.
  That is not a failure.
- **Deduced templates can be wrong.** Treat them as a hint until a spec is
  declared.
