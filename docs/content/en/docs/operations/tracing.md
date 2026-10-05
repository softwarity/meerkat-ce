---
title: Traces
section: Operations
order: 210
summary: Where a request's seconds go, and the time only the gateway can measure.
---

# Traces

You already trace your services, and one hole is left: the time spent
**before** them.

```
[ browser ................................ ] 4.2 s
     [ orders-api ...................... ]   3.9 s
         [ payment-api ................ ]    3.8 s
     ^^^^
     300 ms nobody can explain
```

Those 300 ms are the gateway's: picking the route, checking access, applying
the filters, reaching the upstream. Nobody else in the cluster can measure
them, and a gap in a trace is where a discussion between two teams begins.
Meerkat declares itself in it:

```
[ browser ................................ ] 4.2 s
  [ meerkat .............................. ] 4.1 s
      route=orders   access=session, role orders_write
      [ orders-api ...................... ]   3.9 s
          [ payment-api ................ ]    3.8 s
```

## Traces and metrics do not replace each other

| | answers | complete? | kept |
|---|---|---|---|
| [Metrics](/docs/operations/metrics) | how many, right now | 100% | months |
| Traces | where it breaks, and on whose side | sampled | a few days |

You never alert on a trace, and you never diagnose one case from a counter.

## With nothing turned on

`traceparent`, `tracestate` and `baggage` cross the gateway intact: an
installation that already traces its services keeps its whole trace.

And **every request gets an identifier**, whether anything brought one or not.
It comes back to you three ways:

```
response header      Meerkat-Trace-Id: 4bf92f3577b34da6a3ce929d0e0e4736
in the log           {"type":"access","trace_id":"4bf92f35...", ...}
in the page footer   discreet, under the branding, selectable
```

That is what lets your support start from an identifier read off a screen, and
it is the key that joins a line of the [access log](/docs/operations/logs) to
your service's business audit. It exists on 100% of requests and lives as long
as your logs - a trace, by contrast, is sampled and kept a few days.

An identifier opened here travels with the sampling bit at `00`, meaning "here
is the name of this journey, nobody is expected to report it": with no export
turned on you will see the identifier everywhere and no trace in your backend,
which is the expected behaviour.

## Exporting to your collector

> [!NOTE] Enterprise edition
> The context travels in both editions. The export is Enterprise.

The export speaks **OTLP**, not a product: the address can be any OTLP
endpoint. We recommend an **OpenTelemetry Collector**. It takes every signal
at one address and sends each where it belongs: traces to Tempo, metrics to
Prometheus, logs and audit to Loki. It also samples complete traces (tail
sampling), so the gateway can record **100%** and let it choose.

The setting is under **Infra, OpenTelemetry**. It applies live, on every node.

![The OpenTelemetry screen: the export switch, the collector address, and the Traces tab with its sampling and its ceiling](img/console/opentelemetry.webp)

A master switch, **Export to an OpenTelemetry collector**, turns the export on.
Four tabs then say what is sent:

| tab | what |
|---|---|
| **Traces** | one switch: the traces this page is about |
| **Metrics** | one switch: the gateway's counters, pushed every 30 seconds. It is the only way they leave the gateway (see [metrics](/docs/operations/metrics)) |
| **Audit** | two switches: **Send the audit logs** (sign-ins, refusals, credential changes, [Endpoint audit](/docs/operations/audit#auditing-a-routes-operations) operations) and **Send Meerkat's console audit too** (see [audit](/docs/operations/audit)) |
| **Logs** | one of three modes, below |

The **Logs** tab has three modes (see [logs](/docs/operations/logs)):

- **Not sent**: the logs stay in their usual format.
- **Written for an agent**: the logs are written as OpenTelemetry JSON on
  stdout. A Collector on each node (a DaemonSet) reads them. Works with the
  export off, in both editions.
- **Pushed to the collector**: the logs are sent over OTLP to the collector
  address, for nodes with no agent. Needs the export (Enterprise).

| field | | default |
|---|---|---|
| Collector address | the **base** address: `/v1/traces` and `/v1/metrics` are appended, and a pasted path is trimmed | - |
| Auth header | the header name, such as `Authorization`. Usually needed only by a hosted collector | - |
| Its value | stored in the vault: the configuration keeps only the reference (`Bearer $otlp-token`) | - |
| Journeys recorded (%) | the share of traces **started here**. A caller's own sampling decision is kept | **10%** |
| Ceiling (per second) | at most this many traces recorded per second, whoever decided. 0 for no limit | **200/s** |

The **Test** button posts an **empty** batch to `/v1/traces` before you save.
It exercises the whole path: name resolution, network, TLS, credential. No
trace is written. It refuses a web page: the address of a collector's UI
answers 200 to anything, but it is not the OTLP port. With metrics on, it also
asks `/v1/metrics`, and says when a backend takes only traces.

**A trace is not a counter.** At 100%, a day at 400 req/s is thirty-five
million traces to bill and index. At 10% the latency profile is the same, and
you find one request by its `trace_id` when it was drawn. What you want
complete is the [metrics](/docs/operations/metrics) and the
[access log](/docs/operations/logs): they stay at 100%.

Meerkat emits two spans: a `SERVER` span for the whole crossing, up to the
response being **written**, and inside it a `CLIENT` span around the call to
your service. **The gap between the two is the gateway's own time.** The route
and the access verdict are attributes, not spans.

A collector that is down never slows the traffic: spans leave from a bounded
queue, and what does not fit is dropped.

### No collector yet?

**No collector yet?** on the OpenTelemetry page opens the files to deploy an
OpenTelemetry Collector, for Docker Swarm or Kubernetes:

- the Collector's configuration;
- its logs agent on each node (a DaemonSet on Kubernetes), which reads the
  containers' outputs;
- on Kubernetes, an `opentelemetry` Service: every producer sends to
  `opentelemetry:4318`;
- two Grafana companion files: the datasources (Prometheus, Tempo, Loki) and a
  dashboard for the [metrics](/docs/operations/metrics).

![No collector yet? opens a drawer with the files to deploy one, for Docker Swarm or for Kubernetes](img/console/opentelemetry-collector.webp)

## Starting the trace at the click

A UI route's second switch, **Start the trace from the UI**, injects the
OpenTelemetry bundle into that application's pages.

What it shows you and the gateway never will: the network in front of it, the
browser's own queue, the render - and above all **the calls that never reach
you**. A DNS lookup that fails, a CORS refusal, a device offline: your
dashboards are green and support is taking calls.

Three things to know:

- **The script is served by Meerkat, never by a CDN**: an installation cut off
  from the Internet works, and no third party sees your users.
- **A third party never receives a `traceparent`**: the page carries it only to
  the addresses the gateway serves.
- **Your collector stays private**: the page's spans come back through the
  gateway, which relays them with a credential no page ever sees.

## Choosing which routes are traced

Tracing is decided **route by route**, in the route editor, section
**OpenTelemetry**, with two switches. A request that **no route answers** - a
404 - is never traced: no route said yes, and such a request is nobody's
journey. It stays counted in the metrics and written in the access log.

**Include this route in OpenTelemetry** (on by default): the gateway records its
own spans for what that route answers, and the route has its own series among
the metrics pushed to the collector. Off, the route produces **nothing at all** -
no crossing, no call out, no injected bundle, no series of its own - though it
still counts in the gateway's totals (`meerkat.gateway.requests`), and the
context the caller sent travels on untouched, without the gateway naming itself
as its parent.

**Start the trace from the UI** (off by default, and available only when the
first is on and the route is a UI one): the OpenTelemetry bundle is injected
into that application's pages, and the journey begins at the **click** rather
than at the gateway.

- The first alone: span `00` is Meerkat's, and your service is its child.
- Both: span `00` is the page's, and Meerkat's crossing becomes the child of
  the click.

That is what keeps the backend readable. A gateway sees everything cross it,
and most of what it sees is nobody's journey: the RabbitMQ admin an operator
keeps open in a tab, the Jaeger a colleague is reading, a health probe every ten
seconds. Tick the applications worth following, and the operational interfaces
served beside them cost nothing.

The other reason to turn a route off is an application that **already** carries
its own agent: two agents on one page produce two traces for one click, and
neither is the whole story.

What it does not change: the `traceparent` is still generated and still in the
access log - that is the key joining a line of ours to your service's own audit,
and it is not a trace.

## Seeing the gateway's own work

By default a crossing is two spans: entering Meerkat, and calling your service. The gap between them is the gateway's
own time, and it is one number.

The **Detail the gateway's own work** switch (Infra, OpenTelemetry) divides it: every recorded crossing then carries its
steps - the identity handed to your service, with who the caller is and the token's signature, and every query to the
store, named `SELECT users` with its text (the placeholders, never the values).

It is **off by default**, on purpose. It adds no trace: it deepens the ones already sampled, so the rate and the ceiling
keep their meaning - but each weighs more in your collector. Turn it on while you look at where the gateway's time
goes, then off again.

## Who made the call

A trace answers "what was slow"; a support ticket asks "for whom". The **Name the caller on the spans** switch
(Infra, OpenTelemetry) puts the signed-in caller on the trace:

| Attribute | What |
|---|---|
| `user.id`, `user.name` | the account (OpenTelemetry's own names, which a backend that knows them shows as a user) |
| `meerkat.tenant.id`, `meerkat.tenant.name` | the active organisation |
| `meerkat.group` | the session's chosen group, in an organisation that works in exclusive mode |
| `meerkat.roles` | the roles the access rules were judged on - a list, and what explains a refusal |

An anonymous call carries nothing.

**Once per journey, never twice.** A journey that starts in a page carries the person on the browser's spans; the
gateway's own span on that journey stays silent. Any other journey - a backend, a script, a page without the bundle -
carries it on the gateway's span. The bundle marks the journeys it opens with `meerkat=b` in `tracestate`, which is how
the gateway tells them apart. A trace backend's search still finds the whole trace from any of these attributes: a trace
matches as soon as one of its spans does.

It is **off by default**: a person in a trace is personal data going to a collector somebody else may run.

The browser's spans are stamped **by the gateway**, at the relay that forwards them to your collector: the page is
never told who is reading it, and an attribute a page sets under one of these names is replaced by the gateway's
answer rather than believed.

## What is missing

- `tracestate` and `baggage` cross through, but the gateway does not declare
  itself in them.
- No clickable link between a line of the [audit trail](/docs/operations/audit)
  and its trace.
- A response that never ends - SSE, WebSocket - closes its span at
  establishment: the rest is counted, not traced.
