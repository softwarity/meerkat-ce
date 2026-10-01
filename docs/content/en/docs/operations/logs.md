---
title: Logs
section: Operations
order: 211
summary: One line per request through the front door, with the account as the gateway authenticated it.
---

# Logs

Two logs, and they are not read at the same pace.

```
operational log    startup, reloads, an upstream going down
                   on standard error, a few thousand lines/day

access log         one line per request, refusals included
                   on standard output, 35 million/day at 400 req/s
```

A single stream carries both, and every line is a typed object: a collector
routes them on the `type` field, to two destinations and two retentions.

## The operational log

```
MEERKAT_LOG_LEVEL    debug | info | warn | error      info by default
MEERKAT_LOG_FORMAT   json | text                      json in production, text elsewhere
```

A typo in the level falls back to `info` rather than stopping a start.

## The access log

Ships off: at four hundred requests a second, that is thirty-five million lines
a day.

```
MEERKAT_ACCESS_LOG=1
```

One line, as it comes out:

```json
{"time":"2026-09-22T09:14:07Z","type":"access","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",
 "method":"GET","path":"/patients/42","endpoint":"/patients/{id}","route":"dmp-api",
 "status":200,"ms":45,"outcome":"ok","user":"dr.martin","ip":"10.0.0.7"}
```

| | |
|---|---|
| `type` | `access` or `app`: the discriminator a collector splits the two logs on |
| `trace_id` | the **join key** with your service's business audit |
| `path` | the path as it was asked for, identifiers included |
| `endpoint` | the template, when the route declares a spec - which is what makes these lines countable |
| `user` | the account **as this gateway authenticated it**, never as a header claimed it |
| `ip` | the address **resolved by the gateway**, never an `X-Forwarded-For` written by the caller |
| `outcome` | `ok`, `refused`, `rejected`, `failed`, `upstream-down` |

`outcome` separates `refused` from `failed`: a 403 is the gateway doing its
job, a 502 is a service that went down. Reading both under the same word buries
the first in the second on the morning an upstream lets go.

Turning the operational level down does not silence this log: cutting the noise
must not carry the audit trail away.

> [!NOTE] The line no service can write in your place
> A service that was refused **never saw the call**, and of the caller it knows
> only what the gateway told it. The gateway is what authenticated, so the
> register that holds up is the one it keeps - including for an application
> nobody can modify any more.

What is not in it: the body. `POST /orders/12/validate` is recorded;
"exceptional 40% discount" is for your service to write, under the same
`trace_id`, in its own audit.

```
MEERKAT    who, when, from where, with which identity, on which endpoint,
           with what result
  |
  |  trace_id: 4bf92f35...
  v
SERVICE    what the operation meant, on which data, with which values
```

Nothing is written to a file: the runtime already rotates what a container
writes on its standard output.

> [!WARNING] Remember to bound Docker's logs
> The `json-file` driver has no limit by default, and thirty-five million lines
> a day fill a disk.
>
> ```
> docker --log-opt max-size=50m --log-opt max-file=5
> ```
>
> Kubernetes does it on its own: 10 Mi, 5 files.

## What is missing

- Sending the logs to an OpenTelemetry collector (the **Logs** signal, beside
  traces and metrics): the operational log, the access log of the routes that
  ask for it, and the [audit trail](/docs/operations/audit). A copy: each stays
  where it is written today.
- The level set from the console rather than at startup only.
- The token's name next to the account when a machine calls.
