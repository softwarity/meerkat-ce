---
title: Developer experience
section: Softwarity
order: 3
summary: DX is not a slogan here: it is measured in what a developer does not have to do, and in how long the first useful minute takes.
---

# Developer experience

Developer experience is the part of a product that never appears in a feature
list and decides whether the product is used. We treat it as a requirement, and
it has a test: **how long until something useful happens, and how much of it
did you have to read first?**

## One command, then something works

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

No database to create, no message broker, no configuration file, no chart to
render. The gateway starts with its storage inside it, serves its own sign-in
pages and its own console. The moment you need several gateways it takes a
PostgreSQL - and not before.

## An error names what is allowed

A rejected value that says `invalid argument` has told you nothing. Everywhere
we can, a refusal says what was given, what is accepted, and where to change
it. That rule is written down in the project, it is reviewed, and it is the
single cheapest thing a product can do for the people using it.

## The workstation joins the cluster

The cluster has all the services and all the data, and reproducing it on a
laptop is somewhere between painful and forbidden.
[plug](https://github.com/softwarity/plug) turns that around: the workstation
joins the mesh, a service running locally answers under its cluster name, and
everyone else looking at the application is told which service is substituted
and by whom. See [Dev mode](/product/dev-mode).

## Configuration is something you do, not something you write

An operator configures in the console, exports, and replays the export
somewhere else. There is no YAML dialect to learn for the common path, and the
things you would want to version - routes, roles, settings - come out as one
document that can be read in a review.

## The documentation matches the version you run

Documentation rots silently, and the person who finds out is the one following
it at two in the morning. So the pages you are reading are versioned with the
product: the documentation for 1.3 is the documentation as it was when 1.3
shipped, not today's with the new screens in it. A version selector sits in
every documentation page.

What you read is also checkable rather than promised: the
[test coverage](/project/tests) page is the exact file the integration suite
executes, and what is built and what is not is
[one table read from the code](/project/roadmap).
