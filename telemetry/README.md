# The browser half of the tracing (OBS-04)

What this builds is ONE file, `dist/telemetry.js`, which Meerkat embeds and
serves itself.

## Why it is not a CDN link

A page served by the gateway must not go and fetch code somewhere else. Three
reasons, and each one is enough on its own:

- **An air-gapped installation has no internet.** A gateway whose injected
  script 404s breaks every page it was supposed to help.
- **A Content-Security-Policy that allows a CDN allows everything on it.**
  The point of the gateway is to narrow what a page may load, not widen it.
- **A third party would see every one of your users.** The request alone
  carries the referrer, the address and the user agent.

So it is built here, embedded in the binary, and served from `/meerkat` like
the user-button. One origin, one policy, nothing to reach.

## Why the versions are pinned rather than "always the latest"

The instrumentation packages are `0.x`, and in OpenTelemetry JS that means a
MINOR version is allowed to break. "Always build the latest" would make two
builds of the same commit produce two different binaries, and it would let a
breaking change into an Enterprise image without anybody deciding it.

So: `package-lock.json` pins, an automated bump proposes the new version, and
`npm run check` is what says whether it may be taken. Up to date because
something proposes it, safe because something proves it.

## What `npm run check` proves

It serves the real bundle to a real headless browser and asserts the four
properties this feature stands on:

1. the bundle loads with no error;
2. a call to one of OUR urls carries a `traceparent`, in the W3C shape;
3. a call to a THIRD PARTY carries none - never leak where your users go;
4. spans leave in BATCHES, not one request per span.

Break any of those and the injection is worse than useless, which is why they
are checked on every version bump rather than read from a changelog.
