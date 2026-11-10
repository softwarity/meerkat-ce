---
title: Topology
section: The console
order: 151
summary: What serves the installation and what it serves - the instances in sync, the database they share, the services behind the routes and whether each answers.
---

# Topology

**Infra, Topology** draws the installation: the Meerkat instances serving it, the
database they share, and the services the routes point at. It is the first
entry of the Infra section, before the routes: what serves, before what is served.

It is at `/infra/topology`. Root and infra admins only.

![The Topology screen: three instances in sync on PostgreSQL on the left, the services behind the routes on the right](img/console/topology.webp)

## The instances

On the left, in the **Meerkat** frame, every instance running, oldest first:

- its **host name** - the pod or the container, what you find it by - with a
  button that copies it;
- its **version** in the corner, and when it **started** in its tooltip, in the
  zone of your profile;
- **Not ready**, and why in its tooltip (what its `/readyz` answers), only when
  it is not; **Past the license** for an instance beyond what the license covers.

The instance that answered the screen wears the frame's colour.

Between two instances, **in sync** says the change bus keeps them in step: a
route, a certificate, a session or the license saved on one reaches the others
within a second. Each instance is wired straight down to the **database** below,
the one they share. On the embedded database there is one instance and nothing to keep in sync.

Every instance says who it is every five seconds, whether or not it carries
traffic. One that stops says so and leaves the drawing at once; one that goes
silent leaves it after twenty seconds.

## The services

On the right, in the **Services** frame: one card per address the enabled routes
point at, named by the routes that lead there, the address beneath with a button
that copies it. The icon is `web`
when one of those routes serves pages to people, `api` otherwise. Only what a
route declares is shown: nothing here is discovered.

A badge in its corner says what the gateway sees of it - how many of its
replicas answer, out of how many - and its tooltip says it in words:

- **3/3**, in the accent: answering;
- **1/2**, in orange: some replicas are missing;
- **0/2**, in red: not answering, with the reason and since when;
- **?** for a service no traffic and no check has reached.

A service the runtime does not count - an external host, checked by connecting -
is one address: **1/1** or **0/1**.

The ones that want a look come first, and the list scrolls on its own when there
are many. This is a view: a service is changed through its routes, on the
[Routes](/docs/console/routes) screen.

## What it warns about

Said in words at the top of the screen, and as a badge on **Topology** and on
**Infra** in the rail:

- **more instances than the license covers**, and the time the evaluation notice
  comes back if they are still that many - a rolling update that ends before then
  is not an overrun;
- **two versions running at once**: an update in progress, or one that stopped
  half way;
- **an instance not ready**, and why;
- **a service not answering**.

The screen follows the gateway live: nothing to refresh.
