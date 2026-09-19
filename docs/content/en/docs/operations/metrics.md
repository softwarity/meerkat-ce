---
title: Metrics
section: Operations
order: 209
summary: What the gateway counts, and how to scrape it into the monitoring stack you already run.
---

# Metrics

The counters are in both editions, and so are the console's own curves: that is
the zero-dependency promise, a gateway you can see with nothing installed. What is
sold is **externalising** them, into the stack that already holds your retention,
your alerting and your dashboards (OBS-05).

> [!NOTE] Enterprise edition
> The `/metrics` exposition is Enterprise. The community image does not merely
> refuse it: the code that knows the format is not linked in, so it has no way to
> answer. It says so rather than returning an empty body.

## What is counted

Per route:

| Series | What it holds |
|---|---|
| `meerkat_requests_total` | requests answered, by status class, from `1xx` to `5xx` |
| `meerkat_request_duration_seconds` | a histogram on twelve boundaries, with its sum and count |
| `meerkat_upstream_failures_total` | failures between the gateway and the service, by `kind`: `connect`, `timeout`, `refused`, `upstream` |

Per endpoint, where the unit is the template and never a raw path:

| Series | What it holds |
|---|---|
| `meerkat_endpoint_requests_total` | requests answered by that operation |
| `meerkat_endpoint_errors_total` | the `4xx` and `5xx` among them |
| `meerkat_endpoint_duration_seconds_sum` | seconds spent answering it |

There is no per-endpoint histogram, deliberately: a spec declaring two hundred
operations would turn twelve buckets into two thousand four hundred series for one
route. The sum and the count give a mean, which is what a per-endpoint view is read
on; the distribution stays at the route.

And the gateway itself:

| Series | What it holds |
|---|---|
| `meerkat_requests_in_flight` | a gauge - the one number that says *saturated* rather than *busy* |
| `meerkat_logins_total` | sign-in attempts, by `outcome` |
| `meerkat_unmatched_total` | requests that matched no route at all |

Labels are bounded by construction: a route id and name, a status class, a bucket,
an operation template, and a `source` saying whether that template was `declared`
or `deduced`. Never a user, never an address, never a raw path.

## The endpoint

`/metrics` sits on the **control plane** - no port to open, it is where the console
already is. Three things gate it, and the refusal says which one is missing:

1. the Enterprise image;
2. a switch that ships **off**, in the Prometheus drawer of the Metrics screen;
3. the gateway-admin capability, or a token whose scope is `metrics`.

![Access tokens, where a scraper's credential is minted](img/console/access-tokens.webp)

The guard is not a formality. The counters name every route and every endpoint
template, which is an operational map of the installation rather than a public
page. A `metrics` token opens that one path and nothing else - a scraper's
credential lives in the configuration of a monitoring stack, often another team's
repository, which is where a token is most likely to leak and least likely to be
rotated.

The format is Prometheus text, `version=0.0.4`, announced in the content type.
Counters only ever go up and Prometheus does its own differencing; the console's
window is derived from the same counters rather than the reverse.

## The files to write

The Prometheus drawer carries real resources, served by the gateway under
`/monitoring/` on the control plane: one `prometheus.yml`, a Swarm compose file, a
Kubernetes `ServiceMonitor`, and Grafana's datasource, its dashboard provisioning
and a ready dashboard. They are copyable and downloadable, and they carry **this**
installation's listening port - not the port the console was reached at, since a
published port or an ingress sits in between and a scrape addresses the container.

There is one `prometheus.yml` and not one per platform: the scrape does not change
from Swarm to Kubernetes, only the discovery of the target does. The file carries
both, and a button per platform hands back the version you want.

The Grafana compose also turns Grafana's own sign-in form off and puts it behind
the gateway, so the route decides who enters and the forwarded identity says who
they are. That makes not publishing Grafana's port a rule rather than a comfort.

## In a cluster

The counters are **per node**. Both platform files therefore target every node -
`tasks.meerkat` in Swarm, `role: pod` in Kubernetes - and never the service in
front of them: scraping a virtual address would pull a different node on each pass
and draw a curve that belongs to nobody.

Prometheus **pulls**, so the console's built-in view is more live than it is, not a
degraded version of it.

## What is missing

- No p95 curve in the console. The histogram is collected and exposed; Grafana
  draws it, and the query is in the drawer.
- No `traceparent` propagated to upstreams (OBS-04).
- `grpc-status` is not read, so every gRPC call counts as `2xx` and a gRPC route's
  failure rate reads zero (ROUTE-20).
- Logs have no configurable level and there is no request log (OBS-03).
