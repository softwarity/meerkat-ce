---
title: Performance
section: The product
order: 8
widget: benchmark
summary: What the gateway costs a request, measured next to Kong, APISIX and Traefik on the same machine, in the same run, read live from the latest benchmark the CI made.
---

# Performance

The figures below come from the latest commit the CI benchmarked, read live.
Meerkat is measured next to three open-source gateways, each given the same
single CPU, in front of the same service, under the same load, in the same run:
they compare products, not machines.

Nothing on this page is written by hand. The table is fetched from the
benchmark branch every time somebody opens it, so it follows the code rather
than the last time anyone remembered to update a slide.
