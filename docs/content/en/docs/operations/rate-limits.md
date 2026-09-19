---
title: Rate limits
section: Operations
order: 221
summary: How a bound is written, what a refusal carries, and why the number on the screen means something else on a cluster.
---

# Rate limits

The word is **per**, not **for**. Nobody writes a limit *for* alice: a list of names
is a list nobody maintains, and it answers the wrong question. A rule says what the
counter is **keyed on**, and the budgets make themselves - one per caller (ROUTE-08).

## The two halves of a rule

| Half | What it says |
|---|---|
| **per** | what the counter is keyed on: the whole route, a user, an API token, an organisation, or a client address |
| **applies to** | who the rule is about, written as an `Access` - the same vocabulary the access rules already use. Empty means everybody. |

Which is what turns a role into a pricing tier without inventing a concept: *a
thousand a minute per user, for anyone holding `partner`* and *sixty a minute per
user, for anyone holding `trial`* are two rows, and neither names a person.

A window is an ISO-8601 duration (`PT1M`, `PT1H`), between one second and one day.
Below a second a sliding window is measuring jitter; past a day it is a quota that
has to survive a restart, and these counters do not.

## Several apply at once

This is where a bound differs from an access rule, where the first match wins. Limits
are bounds, and you want all of them:

- five thousand a minute for the whole route protects the service;
- a hundred a minute per user stops one caller taking it all;
- sixty a minute per address covers whoever has no account.

The **first bound exceeded** refuses.

> [!WARNING]
> A rule narrowed with *only for* bounds the callers it describes and **nobody else**.
> Three narrow rules make a route that looks bounded and is wide open to everyone the
> three do not describe - exactly the case one believes is covered. The console says so
> when no bound applies to everybody.

## Where a bound is checked

Bounds that need no identity - one for the whole route, one per address - are checked
**outermost**: before a session is looked up, before a body is read, before an upstream
is dialled. Refusing early is the whole point of a bound.

Bounds keyed on a user, a token or an organisation cannot be, so they cost a session
resolve - paid only by the routes that ask for one. A rule *narrowed* to some callers
needs an identity too, whatever it counts by.

## What a refusal carries

A `429`, with `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` and
`Retry-After`. A refusal without them is a door with no sign on it: a client that
cannot read when to come back either gives up or hammers, and both are worse for the
service the bound was installed to protect. `Retry-After` is never zero - that would
be an invitation.

A refused request is **not counted**. A refusal that still increments turns a burst
into a lockout that outlives it: the caller backs off, and the counter they are backing
off from is still being fed by their own refusals.

## The window slides

A bucket that resets answers "a hundred in the last minute" wrongly at every boundary:
two hundred requests land in two seconds, one on each side of the reset. That is not an
approximation, it is a fault. So the shape is the standard two-window estimate - the
current window's count plus the previous one's, weighted by how far into the current one
we are - which costs two integers per key and is off by less than a percent.

## The keys are capped

A limit keyed per address makes one counter per address, and the set of addresses is
chosen by whoever sends the requests: a flood would grow the gateway's memory through the
very mechanism installed to survive a flood.

So each rule tracks at most ten thousand distinct callers, expired keys are swept first,
and the long tail **shares one counter**. Refusing the untracked outright would let
anybody deny service by rotating addresses; letting them through unbounded would let
anybody bypass the bound the same way.

## What a reload does

Counters are built when a route is compiled, so **saving a route resets them** and a
caller mid-window starts again. That is the honest trade for a design with no
persistence: carrying counters across a reload would mean keying them by rule identity,
and a rule has no identity - editing *a hundred a minute* into *two hundred a minute*
would inherit the count of a bound that no longer exists.

## Per endpoint

An operation of a route's OpenAPI inventory can carry its own bounds, on top of whatever
the route carries (QUOTA-05). Chosen operation by operation and never as a default over
the whole inventory: a bound is a counter per operation **and** per caller, so a spec with
two hundred operations and ten thousand keys would be two million counters - the
cardinality lesson, one level down.

The screen is **Infra > Endpoint rate limits**.

## In a cluster: read the number twice

> [!WARNING]
> These counters live **in memory, on each node**. A bound of a hundred a minute on four
> nodes lets up to four hundred a minute through the installation. The console says so
> where the number is typed, because a figure that means something else on a cluster than
> it does alone has to say it at the place where it is written.

That is a decision rather than an omission: an exact shared counter costs a round trip to
the database on every request, which is not a price a gateway can pay on the path of all
traffic. This is the **protective** half - approximate, free, and enough against abuse.

One counter *is* in the database and is exact: the **brute-force counter** on sign-ins
(AUTH-11). Five attempts means five for the installation, not five per node, and a restart
no longer lets a throttled attacker back in.

## What is missing

- **Throttling.** Exceeding a bound blocks; it does not slow down (QUOTA-02).
- **Billable counters.** The half that gets invoiced or shown to a customer is a
  different mechanism, written in batches, and it does not exist yet (QUOTA-03) - with it,
  the consumption screen and its alert thresholds.
- **Shared counters.** Correct counting across a cluster waits on the billable counters
  (QUOTA-04). The path is traced: the sign-in counter is exactly that shape.
