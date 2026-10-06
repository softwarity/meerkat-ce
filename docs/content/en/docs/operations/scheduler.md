---
title: Scheduled calls
section: Operations
order: 212
summary: The gateway calls your services at the hour you name, carrying the roles the schedule asks for, and with no broker to install.
---

# Scheduled calls

A service asks to be called later: "POST `/jobs/close` on me, every day". At
the hour named, the gateway makes that call **through its own front door**,
with an identity, and records what answered.

Your service exposes an endpoint - it already exposes some. No library to
embed, no protocol of ours, no broker to install.

## It runs as `meerkat`, carrying the roles the schedule asks for

That is the whole security model, and it fits in a sentence: **a schedule
reaches exactly what its roles reach**.

There is **no account** behind a scheduled call. The schedule says which roles
its call must carry (`roles`, or `role` when there is only one), the gateway
makes the call under the name `meerkat` holding them, and everything standing
in front of the service reads them as it reads anybody's: the predicates, the
filters, the route's rule, the per-endpoint rules, the organisation's hours. A
schedule asking for the wrong role is not a hole: it is a schedule that fails
every night, and the console keeps the sentence that says so.

```json
{ "role": "station_fetch", "routeId": "stations-api", "path": "/fetch/station" }
```

The roles asked for go through the **catalogue's hierarchy**, exactly as a
session's do: asking for a parent role grants what it implies.

No service account to create, so no rights to keep up to date for ever. What
bounds a schedule is **the token that wrote it**: a token whose perimeter is
`schedules`, minted by root, traced in the audit trail, revocable in a click.
Two services that must not reach the same things get two tokens.

If you do want to know whose a schedule is, file it in the **metadata**:
`"metadata": { "owner": "stations-svc" }` is a key like any other, and one you
search on.

## When the endpoint being called is protected

**Closed by the route's rule, or by an endpoint rule**: nothing to put in the
schedule but `roles`. The service receives the identity forwarded the way it is
for a human - as headers, as `REMOTE_USER` or as a signed JWT, according to
what the route declares. A wrong role gives a 403 the console shows in as many
words, not a hole.

**Closed by a secret the service expects itself** - a key of its own, a third
party: that is what `headers` is for, with a **vault reference** for a value.
The secret is read only at the moment of the call: neither the database, nor
the API, nor the console sees it.

## At least once

A call can succeed at your end and fail on the way back, and a gateway can
stop in the middle of one. Every call therefore carries a run identifier, the
same on every attempt:

```http
POST /jobs/close
Meerkat-Job: sch_83b4f3a9...
Meerkat-Job-Run: GvbDqbNG__YTOY00
Meerkat-Job-Attempt: 1
```

**Your service must ignore an identifier it has already seen.** Exactly-once
would need a transaction shared between the gateway and your service, so a
shared database: precisely the dependency this product does without.

When is the same run sent twice? Only when nobody can know whether you got it:

| What happened | What the gateway does |
|---|---|
| You answered, whatever the status | The run closes on that answer. That same call is **never** sent again. A few failures get another ATTEMPT, as a new run with a new identifier - see "Which failures are tried again" |
| You answered `202` | The work is yours. Never sent again, whatever happens to the gateway |
| The gateway making the call stopped before your answer | Another gateway sends it again, **same run identifier**, `Meerkat-Job-Attempt` one higher |

The last line has three bounds. **Three attempts** at most, so that a call
that brings a gateway down does not bring every gateway down in turn. **Never
later than `catchUp`**, the same rule as a turn missed while the gateway was
down. And **how soon**: at once when the gateway stopped cleanly - a rolling
deployment hands its calls in flight over as it leaves - and when the lease
runs out, five minutes, when it died.

## Long work answers 202

The call **starts** the work; it does not wait for it. A service answering
`202 Accepted` keeps the run open and reports for itself:

```http
PATCH /api/schedules/{id}/run
{"run": "GvbDqbNG__YTOY00", "state": "running", "progress": 60}
```

and at the end `{"state": "done", "detail": "12000 rows"}`. That is what makes
a three-hour job expressible without a three-hour HTTP request.

**With which token?** Yours, the one that created the schedule, on the same API
as everything else. The scheduled call brought no token with it: its identity
was posed inside the gateway, and never travelled on a wire.

Every report renews the **lease**. Say nothing for five minutes and the run is
closed as lost rather than left running for ever - and it is **not** sent
again: the 202 said the work was yours.

Any other answer closes the run there and then: 2xx succeeded, the rest
failed, with the status and what the answer said - an error page is cut down
to the sentence a human reads on it, never to its doctype.

## When the service is down

Nothing special happens, and that is the point: the call comes in by the front
door, so it gets what a browser would have got. The gateway answers `502` for a
service that is not responding, the run closes as failed, and the console keeps
the sentence: "502 Bad Gateway: Unavailable This application is not responding".

- **Three attempts, then the next turn.** A failure worth trying again - see
  below - is tried again 30 seconds later, then 2 minutes later, and that is
  all. A service down all night costs three calls and then one per cadence,
  never one per second.
- **No stuck run.** The failure closes the turn there and then; the lease has
  nothing to reclaim, and the schedule is not held by a turn that will never
  come back.
- **No contagion.** A schedule that fails suspends no other, and does not
  suspend itself: pausing is an operator's decision. Each call is made on its
  own, too: a service that takes its time holds its own call, and nobody
  else's.
- **A call that does not answer** within the schedule's `timeout` is closed by
  us, and says so - "no answer within the 3s this schedule allows" - rather
  than letting it read as a service breaking when it is us who stopped waiting.

So a service that is down reads as a service that is down, and is fixed by
turning it back on: nothing to replay, nothing to unblock.

### Which failures are tried again

Behind a scheduled call there is an internal service, so a handful of answers
are usually a **moment** rather than a verdict - a gateway that has just
restarted, a version still rolling out:

| The answer | What happens |
|---|---|
| `401`, `403`, `404` | tried again: the account is not known yet, the roles are not loaded, the route of a version still coming up is not there |
| `502`, `503`, `504`, or no answer within the `timeout` | tried again: nobody answered |
| anything else, `500` first of all | **not** tried again. That is the service SAYING something, and a bug at the far end is handled at the far end |
| `lost` - it took the work with a `202` and went quiet | not tried again: it has the work |

An attempt that is tried again is a **new run**, with its own identifier: a
service that deduplicates would have thrown away a call it never acted on.
It is the same **turn**, so the attempts count together - three at most - and
the history links them.

Two bounds beyond the three attempts: the schedule's `catchUp`, because a call
only worth making on time is not worth making twenty minutes later, and the
growing wait between attempts, because what is being waited for is a service
coming back up.

### When the service cannot run it yet

Nothing is broken, the work simply cannot be done now: the extract is not
published, a dependency has not answered. The service knows why, and it knows
when to come back - so it says so, rather than leaving the gateway to guess:

```http
HTTP/1.1 424 Failed Dependency
Retry-After: 600

the daily extract is not published yet
```

The turn comes back then. `424` and not `503`: a `503` says "I am down",
which is what trying again already covers, while `424` says "I am fine, what
I needed is not" - which only the service can know.

For a job accepted with a `202`, the same answer rides on the report:

```json
{"run": "GvbDqbNG__YTOY00", "state": "failed",
 "detail": "the daily extract is not published yet", "retryIn": "PT10M"}
```

Either way, the **reason stays on the run that gave it**, and the turn that
follows points back at it: the history reads "not published yet, put off to
10:15, done" as one chain.

Three bounds, and they are clamped rather than refused - a reasonable answer
should never cost a turn: **a minute** at the soonest, **a day** at the
furthest, and **five in a row** at most. A service that has asked five times
is not waiting for a moment, it is broken: the turn is let go, the schedule's
own cadence takes over, and the screen says why.

## The API, and where it lives

On the **control plane**, not on the one your applications answer. A scheduled
call is a service the gateway provides, like the agent endpoint: no browser
calls it. A **backend** does, from inside the cluster, by the gateway's internal
name.

```
http://meerkat:9090/api/schedules
```

| | |
|---|---|
| `POST /api/schedules` | create - the id comes back in the answer |
| `GET /api/schedules` | the list, filtered |
| `GET /api/schedules/{id}` | one of them |
| `PUT /api/schedules/{id}` | change it |
| `DELETE /api/schedules/{id}` | remove |
| `POST /api/schedules/{id}/pause` and `/resume` | stop it firing, let it fire again |
| `POST /api/schedules/{id}/run` | bring the next turn forward |
| `GET /api/schedules/{id}/runs` | what each turn did, newest first |
| `PATCH /api/schedules/{id}/run` | report on the run in flight |

### The token

A control-plane token whose **perimeter is `schedules`**: it opens this API and
nothing else on this port - not the configuration, not the accounts, not the
routes. Narrow on purpose: it lives in a deployment manifest, often in another
team's repository, and it is the one nobody remembers to rotate.

**What the calls may reach** is not decided here: it is the schedule's own
`roles` field. So one backend keeps **one token** and schedules for as many
different endpoints as it serves - a token per endpoint would be one more
secret to rotate, for a question the schedule already answers.

### The ids are ours

`POST` creates and hands back the id; everything else takes it. Would rather
not keep it? File the schedule under your own **metadata** and ask for it back
that way - which is what a reconciliation loop does.

A schedule names a **route**, never a URL: a service that moves keeps its
schedules.

```json
{
  "name": "station 42, hourly poll",
  "roles": ["station_fetch"],
  "method": "POST",
  "routeId": "stations-api",
  "path": "/fetch/station",
  "every": "PT1H",
  "headers": { "X-Action": "fetch", "X-Api-Key": "${stations-key}" },
  "body": { "kind": "poll", "region": "west", "station": "42" },
  "metadata": { "svc": "stations", "station": "42", "env": "prod" }
}
```

### The verb, the headers, the body

**`method`** is the verb of the **outgoing** call, the one the gateway will
send to the service: `POST` by default, with `POST`, `PUT`, `PATCH` and
`DELETE` accepted. `GET` is refused, and the message says why: a scheduled call
is an action, and a schedule that only reads is a schedule whose answer nobody
looks at.

**`headers`** carries what the service asked to be told, beyond what the
gateway writes by itself. Ten at most. A value may be a **vault reference** -
`${stations-key}` or `$stations-key` - resolved at the moment of the call,
against the application's scope or the organisation's when the schedule has
one: the row keeps the reference, so the secret appears neither in this API's
answers nor in the console. A name the vault does not hold goes out
**verbatim**, `$typo`, rather than becoming an empty header, which would fail
in a far more confusing way.

Five names are refused as the schedule is written, each with its reason:
`Authorization`, because the gateway writes it and it is the identity of the
account the schedule runs as; `Host`, because the route decides it;
`Meerkat-Job`, `Meerkat-Job-Run` and `Meerkat-Job-Attempt`, because the gateway
writes them and they are how your service recognises a turn it has already
seen.

**`body`** is JSON. An object or an array goes out as it stands, with
`Content-Type: application/json`. A JSON **string** goes out as its content,
`text/plain; charset=utf-8` by default: that is how a form, an XML document or
a line of text is sent from a field that is otherwise JSON. An explicit
`contentType` always wins, since it is the one the service asked for.

### Metadata: the service's own filing system

`metadata` is an open object, yours: any keys you like, text values, which
Meerkat stores and matches on without ever trying to know what they mean. It is
what makes **one token enough** - it is the metadata that tells one schedule
from a thousand.

You ask for them back the same way:

```bash
# one station
curl "http://meerkat:9090/api/schedules?meta.station=42" -H "$AUTH"

# two conditions, both required
curl "http://meerkat:9090/api/schedules?meta.kind=poll&meta.region=west" -H "$AUTH"

# an expression, for a fleet
curl "http://meerkat:9090/api/schedules?meta.station=~^[0-9]{1,2}$" -H "$AUTH"
```

`meta.<key>=<value>` matches exactly, `meta.<key>=~<expression>` matches by
regular expression. Two more filters, on what the gateway knows by itself:
`tenant` and `route`. One that does not compile is a refusal saying so, never an
empty list that would read as "there is nothing".

Twenty keys at most per schedule: it is a filing system, not a place to keep
your data.

> [!NOTE]
> Your users never talk to Meerkat. The station service asks for what belongs
> to the station in front of it, and hands its client whatever it decides to
> show.

## A cadence, a calendar, or one date

A schedule says **when** one of three ways, never two.

**`every`, a cadence**: an ISO 8601 duration - `PT30M`, `PT6H`, `P1D` - counted
from the **end** of the last run. The shortest is one minute. It is what "every
six hours" wants, and it is what keeps a slow job from firing again the instant
it returns.

**`cron`, a calendar**: the five standard fields, with lists, ranges, steps,
the names `MON`-`SUN` and `JAN`-`DEC`, and the shortcuts `@hourly`, `@daily`,
`@weekly`, `@monthly`, `@yearly`.

```json
{
  "name": "monday report",
  "routeId": "orders-api",
  "method": "POST",
  "path": "/jobs/report",
  "cron": "0 3 * * MON",
  "timezone": "Europe/Paris"
}
```

**`at`, a single date**: one RFC 3339 moment, with its offset -
`2026-10-03T04:00:00Z`, `2026-10-03T06:00:00+02:00`. The call goes out then,
**once**, and the schedule is **finished**: nothing is armed after it.

```json
{
  "name": "cart 4471, reminder",
  "routeId": "orders-api",
  "path": "/jobs/remind",
  "at": "2026-10-03T04:00:00Z",
  "metadata": { "cart": "4471" }
}
```

That is the **delayed action**: the trigger is something that happened - a
basket left behind, a booking made, a document deposited - rather than
anything on a calendar. Your service handles what happened, posts the date it
wants to be called back at, and forgets about it.

A few things follow from "once":

- **The date carries its own offset**, so `timezone` means nothing beside it
  and is refused. `startAt` is refused too: it places the first turn of
  something that repeats, and a single date is its own first turn.
- **A date already past is not an error**: the call goes out at once if
  `catchUp` still allows it, and is dropped otherwise - exactly like a turn
  missed while the gateway was down.
- **The row stays** once it has gone, with its result, so somebody can see
  that it went out. It is swept after a retention root chooses on the
  Scheduler screen, a month by default. A schedule that repeats is never
  swept.
- **Moving its date** with a `PUT` moves the turn: that is how a delayed
  action is pushed back, and how a finished one is given another date.

Take the calendar for "every Monday at three" or "the first of the month": a
cadence cannot say it, and it **drifts** a little every turn.

Take the cadence for a period. A cron expresses none: its fields are positions
in a calendar, not steps in a sequence.

```
0 */7 * * *    00:00, 07:00, 14:00, 21:00, then midnight  -> a 3-hour gap
*/45 * * * *   0, 45, then 60, then 105                   -> a 15-minute gap
*/20 * * * *   0, 20, 40, 60                              -> this one lands right
```

`*/n` is a period only when `n` divides 60 or 24. "Every 90 minutes" is written
`PT90M`, and no other way. A cadence also has no zone to choose and no
clock-change night to cross, and fifty schedules on `PT1H` spread themselves
out where fifty on `0 * * * *` all go out on the same second.

As in every crontab, when **both** day fields are restricted a day matching
one **or** the other is a match: `0 0 1 * MON` is the first of the month and
every Monday.

### A calendar is read somewhere

`timezone` is an IANA name - `Europe/Paris` - and **UTC** when you say nothing:
"three in the morning" means three UTC until you write down where. A zone this
gateway does not know is refused as it is written, rather than quietly falling
back to UTC.

The two nights that matter are handled: the hour that **does not exist** in
spring moves the turn, the one that **happens twice** in autumn fires once. A
cadence ignores the field.

## Three settings that avoid three surprises

| Setting | What it prevents |
|---|---|
| `overlap` (`skip` by default) | A slow job that has not come back: the next turn does not go out. Nothing more is owed while a run is in flight, and its close **arms the next turn**, counted from then - so a slow job never finds one waiting in the past the instant it returns |
| `catchUp`, in seconds | A gateway down all night firing twelve turns on waking. Zero means "run it whenever you can". It also bounds how late a call cut short by a stopping gateway is sent again, and whether a delayed action nobody was up for still goes out |
| `timeout` | A call that never answers. It bounds the **answer**, not the work: that is what the 202 is for. Two minutes at most (`PT2M`) |

## What each turn leaves behind

The schedule itself keeps the **last** result, which is what a list shows.
Every turn that ended is also kept on its own, and that is what answers "since
when is this failing", "did last night's close run" and "what did it answer":

```bash
curl "http://meerkat:9090/api/schedules/sch_9f2c/runs" -H "$AUTH"
```

```json
[
  {
    "id": "run-17906f3c9d2a4e10-9f2c11",
    "runId": "GvbDqbNG__YTOY00",
    "attempt": 1,
    "node": "meerkat-7d9c",
    "startedAt": 1790770190,
    "endedAt": 1790770191,
    "state": "done",
    "status": 200,
    "detail": "200 OK"
  }
]
```

Four ways a turn ends, and the difference matters:

| | |
|---|---|
| `done` | the service answered 2xx |
| `failed` | it answered something else, or nobody answered within the timeout |
| `lost` | it took the work with a `202` and then went quiet past the lease |
| `dropped` | the turn **never went out**: it came round later than `catchUp` allows |

`attempt` says which attempt ended the turn: `2` means a gateway stopped
before the answer and another sent the same call again. `cause` and `ofRun`
are the chain - an ordinary turn says nothing, a replay names the run it
continues.

### Running a turn again

A turn that was dropped, or that failed for a reason since fixed, is asked for
again by name:

```bash
curl -X POST "http://meerkat:9090/api/schedules/sch_9f2c/run" -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"replayOf":"run-17906f3c9d2a4e10-9f2c11"}'
```

It goes out as a **new run**, with a new identifier and **today's payload** -
the schedule as it stands now, not as it stood that night. What it replays is
written in the history, so the two read as one chain. Without a body, the same
call is the plain "run now".

The history is swept on the same retention as a finished delayed action - a
month by default, root's to change - and goes with the schedule when it is
deleted.

## When it looks, and in a cluster

**There is no tick.** The gateway sleeps until the next thing owed and wakes on
the second. Writing a schedule, pausing it, bringing it forward or closing a
run wakes it at once; a backstop covers the rest, once a minute.

Several gateways share one table and **nothing takes a lock**: taking a turn is
a conditional `UPDATE`, so two gateways at the same instant produce one winner.
The calls **spread over the gateways**, several at a time on each - a slow
service holds its own call and nobody else's.

What a cluster really adds is the **lease**: a run nobody says anything about
for five minutes is settled by whichever gateway sees it first, sent again or
closed as lost.

While a run is in flight nothing more is owed: "run now" answers `409`, and the
run's close arms the next turn.

## What it is not

**A message queue.** No fan-out to several consumers, no acknowledgement, no
dead letters: one schedule, one call, at least once. If you need the other
thing, you need a broker.

> [!TIP]
> Already running Kubernetes CronJobs and want them to stay the clock? Have one
> call `POST /api/schedules/{id}/run` with an API token - it answers `409`
> while the previous run is still in flight. The gateway keeps the identity,
> the containment and the record; your cluster keeps the calendar. Meerkat
> needs no rights on your cluster for that.

## What the operator sees

**Data plane, Scheduler** lists everything scheduled, filtered by
organisation and service, updated live: a run starting, advancing and finishing
appears without refreshing anything. Three actions live there, and they are the
ones wanted at two in the morning: pause, bring the next turn forward, remove.
There is no button to create one, by decision: a schedule belongs to the
service that needs it, which creates it through the API from its own screens.

![The Scheduler screen: a cadence, a cron line and one date, each with its next run and its last](img/console/scheduler.webp)

A run in flight says where it stands - **calling**, waiting for the answer, or
**accepted**, the service took the work - and which attempt it is when a
gateway stopped before the answer. Its next turn reads "after this run": the
close arms it, and "run now" waits for that.

A schedule's drawer keeps **every turn it ran**: when it ended, how, what
answered, which node made the call, which attempt it was - and a button to
run a dropped or failed one again.

A **delayed action** that has gone reads "finished" and keeps its result on
screen until the sweep. A switch hides them when a service posts enough of
them to bury the rest, and root chooses beside it how long they are kept.

The hours are written in the **operator's own zone** - the one on their profile,
set once and read by both planes - or in **UTC**, on a button that says which is
showing: "2:24 AM" means nothing until it says whose 2:24 it is.

An **API** button opens, in the same drawer, the mechanism and the API: the
commands and the payload already filled in with **this** installation's address,
because the screen creates nothing and the next step is a call the service makes
itself.

A schedule's drawer keeps, folded away, the **payload that recreates it**: the
fields a row cannot show - the body sent, the three settings - and a worked
example for whoever has to write the next one.

The **Metadata** filter there takes what the API takes - `station=42`, or
`station=~^st-` - because an operator looking for one schedule in a fleet looks
for it by what the service wrote on it.

Creating a schedule is not among them, deliberately: a schedule runs as an
account, and the administrators reading that screen are not the accounts it
should run as. The API refuses it too, in as many words: creation belongs to
the service, with its own token.
