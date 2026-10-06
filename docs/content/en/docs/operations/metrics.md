---
title: Metrics
section: Operations
order: 209
summary: What the gateway counts, where the console shows it, and how to keep a history by pushing it over OTLP.
---

# Metrics

The gateway counts what it serves, and the console draws it: the **Metrics**
screen (`/data-plane/metrics`) shows the last hour, with nothing to install - that is the
zero-dependency promise, in both editions. See
[the traffic screen](/docs/operations/traffic) and
[the Metrics screen](/docs/console/traffic).

That window is held in memory and starts again when the gateway restarts. To
keep a history, alert on it or draw it beside the rest of your platform, the
counters are **pushed over OTLP** to an OpenTelemetry collector, which writes
them into Prometheus. That is the one way they leave the gateway: nothing
scrapes it.

## What is counted

Per route, under the names they take in Prometheus once a collector has
translated them:

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
| `meerkat_gateway_requests_total` | requests answered by the whole gateway, every route included, by status class |
| `meerkat_gateway_request_duration_seconds` | how long an answer took, every route included |
| `meerkat_requests_in_flight` | a gauge - the one number that says *saturated* rather than *busy* |
| `meerkat_logins_total` | sign-in attempts, by `outcome` |
| `meerkat_unmatched_total` | requests that matched no route at all |

A route with **Include this route in OpenTelemetry** off has no series of its
own. Its requests still count in the two `meerkat_gateway_` totals.

A gRPC call always answers `200` and puts its verdict in `grpc-status`. It is
counted by that verdict, translated the way gRPC maps its codes to HTTP:
`UNAVAILABLE` is a `5xx`, `NOT_FOUND` or `PERMISSION_DENIED` a `4xx`. The caller
still receives its `200` and its trailer.

Labels are bounded by construction: a route id and name, a status class, a bucket,
an operation template, and a `source` saying whether that template was `declared`
or `deduced`. Never a user, never an address, never a raw path.

## Keeping a history: push over OTLP

Switch it on under **Infra, OpenTelemetry**, tab **Metrics**. The counters go to
the collector the traces go to, with the same credential: every 30 seconds,
running totals since the gateway started, one resource per node
(`service.instance.id`).

The names are OpenTelemetry's (`meerkat.requests`, `meerkat.request.duration`
in seconds...), chosen so that a collector writing them into Prometheus lands on
the series above: a monotonic sum gets `_total`, a unit in seconds gets
`_seconds`. The collector files the console hands out (Infra, OpenTelemetry, *No
collector yet?*) already route metrics to Prometheus.

It needs a collector that receives metrics. A backend that takes only traces
will not do: put an OpenTelemetry Collector in front of it. The Test button
says so.

> [!NOTE] Enterprise edition
> Pushing the counters is Enterprise. The counters and the Metrics screen are in
> both editions; what is sold is externalising them into a stack you already run.

## The Grafana dashboard

Among the collector's files are two Grafana companions.
`grafana-dashboard.json` is a ready dashboard on these series: traffic by route,
failure rate, p95, the slowest and costliest endpoints, silent services, refused
sign-ins. `grafana-datasources.yaml` declares the Prometheus, Tempo and Loki
datasources (the dashboard reads Prometheus, uid `meerkat-prometheus`).

## What is missing

- Nothing about one request in particular: that is the other half, and it has its own page
 ([traces](/docs/operations/tracing)). A counter detects and scopes; a trace explains one case.
- Nothing about WHO called: no label is ever a user, which is what bounds the cardinality. That
 question is answered in the [access log](/docs/operations/logs).
