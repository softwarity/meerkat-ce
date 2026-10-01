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
> The context travels in both editions; the export is Enterprise, as it is for
> Prometheus.

What is spoken is **OTLP**, not a product: the address points at whatever you
already run - an OpenTelemetry Collector, Tempo, Jaeger, or a vendor's
endpoint.

The setting is in the console, under **Infra, OpenTelemetry**, with the other
systems this installation is wired to. It is applied live, on every node.

One switch sends to the collector, and two say **what** is sent: the
**traces**, which this page is mostly about, and the **metrics** - the same
counters the [metrics endpoint](/docs/operations/metrics) exposes, pushed over
OTLP every 30 seconds. Either, or both.

| field | | default |
|---|---|---|
| Collector address | the **base** address: `/v1/traces` (and `/v1/metrics` when the metrics are sent) is appended for you, and a pasted path is trimmed | - |
| Authentication header | a **vault reference** (`$otlp-token`), never the key itself | - |
| Journeys recorded | the share of journeys **opened here**: at the front door for a call that arrives undecided, in the page when the browser half is on. A decision already taken by the caller is honoured rather than rolled again | **10%** |
| Ceiling | the ceiling on journeys recorded per second, whoever decided - your budget | **200/s** |

The **Test** button, beside Save, asks the collector before anything is saved:
the gateway posts an **empty** batch to `/v1/traces` and hands back what came
of it, with how long it took. The whole path is exercised - name resolution,
network, TLS, the credential - and no trace is written anywhere. The mistakes that actually happen are named, and the first is the one a status
code alone cannot see: the address of a collector's **UI** rather than its OTLP
port. A web interface answers 200 to anything - it serves its page for any path
- so the probe looks at WHAT answered as well, and refuses a page (Jaeger
listens for traces on 4318 and serves its UI on 16686). The other is a
credential that was refused. With metrics on, it asks `/v1/metrics` too, and
says so when a backend takes traces and nothing else - which Jaeger does.

**A trace is not a counter, and is not meant to be exhaustive.** At 100%, a day
at 400 req/s is thirty-five million traces for your backend to bill and index.
At 10% the latency profile is the same - it is a distribution, not an inventory
- and the one request you are chasing you find by its `trace_id` when it was
drawn. What you want complete is [metrics](/docs/operations/metrics) and the
[access log](/docs/operations/logs): they are at 100% and stay there because
they do not cost per request.

Meerkat emits two spans: a `SERVER` span covering the whole crossing, from the
request arriving to the response being **written** - outgoing filters and
injections included - and, inside it, a `CLIENT` span around the call to your
service. **The gap between the two is its own time.** The route picked and
the access verdict are attributes there, not spans - one span per internal step
would run to thousands per request.

A collector that is down never slows the traffic: spans leave from a bounded
queue, and what does not fit is dropped.

::: details A collector, if you have none
The OpenTelemetry page hands you the file, for Docker Swarm and for
Kubernetes, with the address to copy into the field above.
:::

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
**OpenTelemetry**, with two switches.

**Push traces of this route to OpenTelemetry** (on by default): the gateway
records its own spans for what that route answers. Off, the route produces
**nothing at all** - no crossing, no call out, no injected bundle - and the
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

## What is missing

- `tracestate` and `baggage` cross through, but the gateway does not declare
  itself in them.
- No clickable link between a line of the [audit trail](/docs/operations/audit)
  and its trace.
- A response that never ends - SSE, WebSocket - closes its span at
  establishment: the rest is counted, not traced.
