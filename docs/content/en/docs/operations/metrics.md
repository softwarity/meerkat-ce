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
your alerting and your dashboards.

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
| `meerkat_endpoint_duration_seconds_total` | seconds spent answering it |

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

`/metrics` has a **port of its own**, like PostgreSQL's exporter (9187) or
RabbitMQ's (15692). It is chosen in the console, under **Infra, Metrics
endpoint**, when the exposition is switched on: **9091** by default, another one if the
platform already uses it. The gateway opens that port on every node while the
switch is on, moves it when it changes, and closes it when the switch goes off. It
serves `/metrics` and nothing else - not the console, not the API.

A port this node cannot open, taken or reserved, is refused with the reason, and
nothing is saved. The ports this gateway's two planes listen on (8080 and 9090 by default) are
refused outright.

Three things gate it, and the refusal says which one is missing:

1. the Enterprise image;
2. a switch that ships **off**;
3. the network, and a token if you want one.

By default the port **asks for no token**. The network is the lock: it is never
published, and no route or ingress goes in front of it. A scrape with no credential
is a monitoring configuration with no secret to rotate.

A second switch, **Require a token**, is for a port that other workloads of the
cluster can reach and should not read. The counters name every route and every
endpoint template, which is an operational map of the installation rather than a
public page. The token is then checked by the control plane's own funnel, and it is
minted with the `metrics` scope, which opens that one path and nothing else.

![Access tokens, where a scraper's credential is minted](img/console/access-tokens.webp)

`/metrics` also answers on the control plane, **always** with a token. That is the
door for an installation whose Prometheus reaches the gateway only through the
console's address.

In Kubernetes the Service has to declare the port for a `ServiceMonitor` to find
it. The chart does it with the `metrics.port` value, on a `-metrics` Service that is
always ClusterIP, and that value has to repeat the port chosen in the console.

The format is Prometheus text, `version=0.0.4`, announced in the content type.
Counters only ever go up and Prometheus does its own differencing; the console's
window is derived from the same counters rather than the reverse.

## Pushed over OTLP

The endpoint is one way out, where a scraper comes to fetch. The other is to
**send** the same counters to an OpenTelemetry collector, which is what a stack
that receives rather than scrapes wants. It is switched on under **Infra,
OpenTelemetry**, beside the traces, and goes to the same collector with the
same credential: every 30 seconds, running totals since the gateway started,
one resource per node (`service.instance.id`).

The names are OpenTelemetry's (`meerkat.requests`, `meerkat.request.duration`
in seconds...), chosen so that a collector translating them back to Prometheus
lands on **the same series** the endpoint gives. A dashboard written on one
door reads the other.

It needs a collector that receives metrics. Jaeger takes traces and nothing
else: put an OpenTelemetry Collector in front of it. The Test button on that
page says so.

## The files to write

The Metrics endpoint page carries real resources, served by the gateway under
`/monitoring/` on the control plane: one `prometheus.yml`, a Swarm compose file, a
Kubernetes `ServiceMonitor`, and Grafana's datasource, its dashboard provisioning
and a ready dashboard. They are copyable and downloadable, and they carry **this**
installation's metrics port. The token block appears in them only when the port
asks for one.

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
 draws it, and the query is on the Metrics endpoint page.
- Nothing about one request in particular: that is the other half, and it has its own page
 ([traces](/docs/operations/tracing)). A counter detects and scopes; a trace explains one case.
- `grpc-status` is not read, so every gRPC call counts as `2xx` and a gRPC route's
 failure rate reads zero.
- Nothing about WHO called: no label is ever a user, which is what bounds the cardinality. That
 question is answered in the [access log](/docs/operations/logs).
