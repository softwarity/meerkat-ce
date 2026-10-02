// The gate on every version bump (OBS-04).
//
// The instrumentation packages are 0.x, and in OpenTelemetry JS that means a
// MINOR release is allowed to break. So the question "may we take the new
// version" is not answered by reading a changelog: it is answered by serving
// the real bundle to a real browser and checking that the four properties this
// feature stands on still hold.
//
//   1. it loads at all
//   2. our own calls carry a traceparent, in the W3C shape
//   3. a THIRD PARTY carries none - never leak where your users go
//   4. spans leave in BATCHES - one export per span would double every page
//
// Chromium comes from e2e/node_modules rather than a second install here: the
// browser weighs more than everything else in this repository put together.

import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const bundlePath = path.join(here, "dist", "telemetry.js");
if (!fs.existsSync(bundlePath)) {
  console.error("no dist/telemetry.js: run `npm run build` first");
  process.exit(1);
}

let chromium;
try {
  // Playwright is CommonJS, so the namespace hands it over under `default`.
  const pw = await import(path.join(here, "..", "e2e", "node_modules", "playwright", "index.js"));
  chromium = (pw.default ?? pw).chromium;
} catch {
  console.error("playwright not found: run `npm install` in e2e/ once, it brings the browser");
  process.exit(1);
}

const bundle = fs.readFileSync(bundlePath);
const exported = [];
const ours = [];
const third = [];

const cors = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "*",
};

// The third party, on its OWN origin - which is the whole point. Pointing this
// at a name that does not resolve would prove nothing: the request would never
// leave, and the test would pass without ever checking anything.
const other = http.createServer((req, res) => {
  if (req.method === "OPTIONS") return res.writeHead(204, cors).end();
  third.push({ url: req.url, tp: req.headers["traceparent"] ?? null });
  res.writeHead(200, cors).end("{}");
});
const thirdPort = await listen(other);

const site = http.createServer((req, res) => {
  if (req.method === "OPTIONS") return res.writeHead(204, cors).end();
  if (req.url === "/meerkat/telemetry.js")
    return res.writeHead(200, { "Content-Type": "text/javascript" }).end(bundle);
  if (req.url === "/meerkat/telemetry") {
    let body = "";
    req.on("data", (c) => (body += c));
    return req.on("end", () => {
      exported.push(body);
      res.writeHead(200, { ...cors, "Content-Type": "application/json" }).end("{}");
    });
  }
  if (req.url.startsWith("/api/")) {
    ours.push({ url: req.url, tp: req.headers["traceparent"] ?? null, ts: req.headers["tracestate"] ?? null });
    return res.writeHead(200, cors).end("{}");
  }
  // The page, written the way the gateway writes it: the configuration inline,
  // then the bundle.
  // The shape the gateway writes: no propagate list at all, because the page
  // works its own origin out. Which is the thing this now has to prove.
  const config = {
    endpoint: "/meerkat/telemetry",
    sample: 1,
    service: "compat-probe",
    sameOrigin: true,
  };
  res.writeHead(200, { "Content-Type": "text/html" }).end(
    `<!doctype html><meta charset="utf-8">
     <script>window.__MEERKAT_OTEL__=${JSON.stringify(config)}</script>
     <script src="/meerkat/telemetry.js"></script>`,
  );
});
const sitePort = await listen(site);

const browser = await chromium.launch();
const page = await browser.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(`http://127.0.0.1:${sitePort}/`);
await page.waitForTimeout(400);
await page.evaluate(async ({ site, third }) => {
  await fetch("/api/relative");
  // An identifier in the path, which must not become part of the name.
  await fetch("/api/orders/12345?token=secret");
  await fetch(site + "/api/absolute");
  await fetch(third + "/analytics").catch(() => {});
  await new Promise((done) => {
    const x = new XMLHttpRequest();
    x.open("GET", "/api/xhr");
    x.onloadend = done;
    x.send();
  });
}, { site: `http://127.0.0.1:${sitePort}`, third: `http://127.0.0.1:${thirdPort}` });
// One flush of the batch processor, whose scheduled delay is 5 s.
await page.waitForTimeout(6500);
await browser.close();
site.close();
other.close();

const w3c = /^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$/;
const spans = exported.length
  ? (JSON.parse(exported[0]).resourceSpans?.[0]?.scopeSpans ?? []).flatMap((s) => s.spans ?? [])
  : [];

let failed = 0;
const check = (label, ok, detail = "") => {
  if (!ok) failed++;
  console.log(`${ok ? "ok  " : "FAIL"}  ${label}${detail ? "   " + detail : ""}`);
};

check("the bundle loads with no error", errors.length === 0, errors[0] ?? "");
check(`our calls carry a traceparent (${ours.length} seen)`, ours.length === 4 && ours.every((h) => h.tp));
check("the traceparent is W3C-shaped", ours.every((h) => w3c.test(h.tp ?? "")), ours[0]?.tp ?? "");
// The journeys the page opens are signed, so the gateway leaves the person to
// the spans it stamps at the relay rather than saying it twice.
check("our journeys are marked meerkat=b", ours.every((h) => (h.ts ?? "").split(",").includes("meerkat=b")), String(ours[0]?.ts));
check(`the third party was reached (${third.length})`, third.length === 1);
check("the third party got NO traceparent", third.every((h) => h.tp === null), String(third[0]?.tp));
check(`spans left in batches (${exported.length} export)`, exported.length === 1);
check(`several spans in ONE export (${spans.length})`, spans.length > 1);
check("OTLP carries trace and span ids", spans.length > 0 && spans.every((s) => s.traceId && s.spanId));
// Each call named after what it asked for, not after its verb alone - and
// with the identifier and the query string kept out of the name.
const names = spans.map((s) => s.name);
check("a call is named after its path", names.includes("GET /api/relative"), names.join(", "));
check("an identifier in the path becomes {id}", names.includes("GET /api/orders/{id}"), names.join(", "));
check("the query string stays out of the name", names.every((n) => !n.includes("?") && !n.includes("secret")));
check("XMLHttpRequest is named too", names.includes("GET /api/xhr"), names.join(", "));

if (process.env.DUMP_OTLP && exported.length) {
  console.log("\n--- ce que l'exportateur officiel envoie ---");
  console.log(JSON.stringify(JSON.parse(exported[0]), null, 1).slice(0, 2600));
}

process.exit(failed === 0 ? 0 : 1);

function listen(server) {
  return new Promise((r) => server.listen(0, "127.0.0.1", () => r(server.address().port)));
}
