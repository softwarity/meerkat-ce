---
title: In front of your microservices
section: The product
order: 2
navTitle: Microservices
summary: Where the gateway sits in your cluster: one published point in front of services that do not change, for interfaces as much as for APIs.
---

# In front of your microservices

::: lead
An internal application is no longer a program: it is a dozen services, written
in several languages, shipped by several teams. The question is then not only
what a gateway can do, but where it goes.
:::

## One door, not a library per service

Every one of those services needs to know who is calling and what that person
is allowed to do. Answered service by service, that means as many login pages
as services, as many user tables, and sessions that do not mean the same thing
from one service to the next. Answered once, in front, it means a door.

::: figure mesh
The path of one request: it reaches the gateway, which decides, and leaves for
the service concerned - an interface or an API.
:::

A **UI route** is a page a person looks at, so the gateway dresses it on the
way through: the navigation portal, the account button, the theme, light and
dark mode, injected into the HTML. The application carries nothing for that and
does not even know it is being dressed.

An **API route** is called by a program, so there is nothing to dress. It
receives a signed token saying who is calling, with their roles and their
organisation, and it authenticates nobody. A service is often both: its pages
behind a UI route, its own API behind an API route.

## In the cluster

::: figure cluster
One published point, several interchangeable replicas behind it, and your
services staying inside.
:::

Your ingress publishes one thing: the gateway's data plane. Your services stay
reachable from inside the cluster only, which means none of them has a door to
guard. The admin console is not on that port at all: it is the second plane, on
an internal network.

Behind the ingress, several replicas serve the same routes. They never talk to
each other: what they have in common is in the database, and a route that
changes reaches the others in a second. Sessions live there too, so there is no
affinity to ask of the load balancer: a request lands on whichever replica, and
a rolling update takes nobody's session with it.

> [!NOTE]
> That shared state needs a PostgreSQL, and it is an Enterprise capability. One
> gateway on its embedded storage shares nothing with anybody and does not need
> it: that is the community edition, and it carries a whole application.

## What your services stop carrying

::: cards
### The login page

It is served by the gateway, in your colours, with the password, the second
factor, passkeys and your corporate directory behind it. None of your services
has one any more.

### The user table

Accounts, roles, groups and organisations are in the gateway. Your services no
longer store identities and no longer have to keep them agreeing with each
other.

### The access rules

Who gets through is decided at the door, per route and down to one operation of
your OpenAPI spec. The rule changes without redeploying the service it protects.

### The dressing

The navigation portal, the account button and the theme arrive in the HTML on
the way through. One more interface gets them without being touched.
:::

## Where the gateway does not go

Traffic between your services stays between your services. Meerkat holds the
door, which is what comes in and who is allowed, not the cluster's internal
circulation. If you already run a service mesh for east-west traffic, the two
do not overlap: one takes care of your services among themselves, the other of
the world knocking.

## Adding a service

::: steps
### Declare the route

A path, a target, and the gateway routes. From the console, from the admin API,
or by letting an agent do it.

### Say whether it is an interface or an API

That is what decides whether the response is dressed or the request leaves with
a signed token.

### Pose the access rule

A level, roles, named accounts, and if needed a rule per operation taken from
the service's spec.
:::

The service itself does not move: it carries no library of ours, speaks no
protocol of ours, and does not know it is behind a gateway.

::: cta
### The detail

The full Kubernetes shape, with the Deployment, the two Services, the Ingress
and the probes, is on [Kubernetes cluster](/docs/deploy/kubernetes). The two
planes and what separates them are on
[Architecture](/docs/concepts/architecture). What the same assembly costs
otherwise is on [the Meerkat case](/product/the-case).
:::
