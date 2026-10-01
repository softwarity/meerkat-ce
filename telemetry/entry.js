// What Meerkat injects into a proxied UI page, and the whole browser half of
// OBS-04.
//
// It reads ONE global, which the gateway writes inline just above the script
// tag. Inline rather than fetched: a round trip for the configuration would
// happen while the page is already firing its first calls, and those are
// exactly the ones worth measuring. The bundle stays immutable and cacheable;
// the configuration is per page and weighs nothing.
//
//   <script>window.__MEERKAT_OTEL__ = { ... }</script>
//   <script defer src="/meerkat/telemetry.js"></script>
//
// The gateway builds `propagate` from its OWN route table, so a page carries
// trace context to the hosts Meerkat serves and to nobody else. That is a
// privacy property, not an optimisation: a `traceparent` sent to a third party
// tells them a request happened, and their analytics keeps it.

// Static imports, deliberately: a dynamic import() defeats esbuild's
// tree-shaking here, and the bundle goes from 25 to 45 KB gzipped for nothing.
import {
  WebTracerProvider,
  BatchSpanProcessor,
  TraceIdRatioBasedSampler,
  ParentBasedSampler,
} from "@opentelemetry/sdk-trace-web";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { XMLHttpRequestInstrumentation } from "@opentelemetry/instrumentation-xml-http-request";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";

const cfg = window.__MEERKAT_OTEL__ || {};

// Nothing configured means nothing instrumented. A page that got the script
// without the configuration - a cached HTML, a switch turned off between two
// loads - stays an ordinary page rather than throwing on every fetch.
if (cfg.endpoint) {
  start(cfg);
}

function start(cfg) {
  // The urls that may carry trace context.
  //
  // SAME ORIGIN IS COMPUTED HERE, not baked in by the gateway: the page's own
  // address depends on the host it was asked for, and one gateway serves
  // several. `^/` catches a relative call, and the escaped origin catches the
  // same address written in full - which is what an application does when it
  // builds its urls from a configured base.
  //
  // Everything else is a THIRD PARTY and gets nothing. That is a privacy
  // property rather than an optimisation: a traceparent sent to an analytics
  // host tells them a request happened, and their pipeline keeps it.
  const urls = [];
  if (cfg.sameOrigin !== false) {
    urls.push(/^\//);
    try {
      urls.push(new RegExp("^" + escapeRe(location.origin) + "/"));
    } catch {
      /* an origin we cannot express; the relative rule still stands */
    }
  }
  // Extra hosts the gateway knows are its own - a route pinned to another
  // name. Written by us, still read defensively: one bad entry must not cost
  // the whole page its telemetry.
  for (const pattern of cfg.propagate || []) {
    try {
      urls.push(new RegExp(pattern));
    } catch {
      /* the gateway wrote something we cannot read; ignore that one */
    }
  }

  // ParentBased around a ratio: when a parent already decided - which happens
  // as soon as anything upstream of this page traces - we FOLLOW it rather
  // than rolling the dice again. Sampling is decided once, at the head of the
  // chain; a second decision here is how traces end up with missing parents.
  const sampler = new ParentBasedSampler({
    root: new TraceIdRatioBasedSampler(typeof cfg.sample === "number" ? cfg.sample : 0.1),
  });

  const provider = new WebTracerProvider({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: cfg.service || "browser",
    }),
    sampler,
    // Batched, and that is what makes this affordable: forty calls on a page
    // leave as a handful of exports, not forty. The queue is bounded, and a
    // collector that stops answering costs dropped spans rather than a page
    // that waits.
    spanProcessors: [
      namer(),
      new BatchSpanProcessor(new OTLPTraceExporter({ url: cfg.endpoint }), {
        scheduledDelayMillis: 5000,
        maxExportBatchSize: 512,
        maxQueueSize: 2048,
      }),
    ],
  });
  provider.register();

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: urls,
        clearTimingResources: true,
      }),
      new XMLHttpRequestInstrumentation({
        propagateTraceHeaderCorsUrls: urls,
      }),
    ],
  });
}

// namer names each call after what it asked for.
//
// Left alone, the fetch and XHR instrumentations call every span by its verb
// alone - "GET" - so in a backend every journey that starts in a page has the
// same name, and the list of traces is a column of GETs. A span processor runs
// on EVERY span at its start, where both instrumentations have already set the
// method and the url, so one hook names them both.
//
// The PATH, never the full url: a query string carries whatever the page put in
// it, and a span's name is indexed by every backend as an operation.
//
// A FUNCTION rather than a constant: start() runs as the script loads, before a
// constant declared below it has a value - which is how the first build handed
// the provider an undefined processor.
function namer() {
  return {
    onStart(span) {
      const a = span.attributes || {};
      const url = a["url.full"] || a["http.url"];
      const method = a["http.request.method"] || a["http.method"];
      if (!url || !method) return;
      span.updateName(method + " " + templated(url));
    },
    onEnd() {},
    shutdown() {
      return Promise.resolve();
    },
    forceFlush() {
      return Promise.resolve();
    },
  };
}

// templated turns a path into the shape it has in common with its siblings.
//
// The gateway names its own span after the route's OpenAPI template for the
// reason that matters here too: a name carrying identifiers becomes one
// operation per identifier in the backend's list, which is the list becoming
// useless. The page has no template, so what LOOKS like an identifier - a
// number, a uuid, a long hex or token - is written {id}.
const ID_LIKE =
  /^(\d+|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9a-f]{16,}|[A-Za-z0-9_-]{32,})$/i;

function templated(url) {
  let path;
  try {
    path = new URL(url, location.href).pathname;
  } catch {
    return url;
  }
  return path
    .split("/")
    .map((seg) => (ID_LIKE.test(seg) ? "{id}" : seg))
    .join("/");
}

// escapeRe makes a literal string safe inside a regular expression. An origin
// carries dots and a colon, and an unescaped dot matches anything - which
// would quietly widen "our own address" to a great many others.
function escapeRe(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
