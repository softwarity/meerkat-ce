// The service behind the documentation instance's routes.
//
// A screenshot of a gateway in front of nothing shows 502s: the traffic screen
// fills with failures, the portal bar sits on an error page. This answers on
// 127.0.0.1:8099 the way a small order service would - an HTML page at the
// root (what the Docs portal route shows, in light and in dark), JSON under
// /orders and /shipments (what the Orders API route proxies, and what the
// OpenAPI spec the seed deposits describes), and a little latency so the
// curves are curves.
//
//   node docs/scripts/screenshots-upstream.mjs        (keeps running)
//
// The routes name their services as a stack would (http://docs:8080,
// http://orders-api:8080), and the documentation instance runs with
// HTTP_PROXY pointing here: a proxied request arrives with its whole address,
// and only its path matters to what is answered.
//
// No dependency: node:http only.
import { createServer } from 'node:http';

const PORT = Number(process.env.UPSTREAM_PORT || 8099);

const ORDERS = [
  { id: 'ORD-8814', status: 'Shipped', tracking: 'DHL 4411 9920 31', lines: 3, total: '1 240.00 EUR' },
  { id: 'ORD-8815', status: 'Awaiting stock', tracking: '-', lines: 1, total: '89.00 EUR' },
  { id: 'ORD-8816', status: 'Picking', tracking: '-', lines: 7, total: '3 104.50 EUR' },
  { id: 'ORD-8817', status: 'Shipped', tracking: 'DHL 4411 9920 44', lines: 2, total: '412.90 EUR' },
  { id: 'ORD-8818', status: 'Invoiced', tracking: 'DHL 4411 9919 02', lines: 5, total: '2 018.00 EUR' },
  { id: 'ORD-8819', status: 'On hold, credit check', tracking: '-', lines: 1, total: '15 900.00 EUR' },
];

const page = () => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Orders - Acme Corp</title>
<style>
  :root { color-scheme: light dark; --bg: #f4f6fa; --card: #ffffff; --line: #e3e7ef; --ink: #1d2433; --mute: #5d6678; --chip: #dceff3; --chip-ink: #17606f; }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #161a24; --card: #1e2330; --line: #2b3242; --ink: #e8ecf4; --mute: #a3acbe; --chip: #17343b; --chip-ink: #8fd6e3; }
  }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--ink); font: 16px/1.4 system-ui, -apple-system, "Segoe UI", sans-serif; }
  main { padding: 40px 44px; }
  h1 { margin: 0 0 4px; font-size: 28px; }
  p.sub { margin: 0 0 24px; color: var(--mute); }
  .kpis { display: flex; gap: 16px; margin-bottom: 26px; flex-wrap: wrap; }
  .kpi { background: var(--card); border: 1px solid var(--line); border-radius: 12px; padding: 18px 20px; min-width: 208px; }
  .kpi span { display: block; font-size: 12px; letter-spacing: .12em; text-transform: uppercase; color: var(--mute); }
  .kpi b { font-size: 26px; }
  table { width: 100%; border-collapse: collapse; background: var(--card); border: 1px solid var(--line); border-radius: 12px; overflow: hidden; }
  th { text-align: left; font-size: 12px; letter-spacing: .12em; text-transform: uppercase; color: var(--mute); padding: 14px 18px; border-bottom: 1px solid var(--line); }
  td { padding: 12px 18px; border-bottom: 1px solid var(--line); }
  tr:last-child td { border-bottom: 0; }
  td:first-child { font-weight: 700; }
  .chip { display: inline-block; padding: 2px 10px; border-radius: 8px; background: var(--chip); color: var(--chip-ink); font-size: 13px; }
</style>
</head>
<body>
<main>
  <h1>Orders</h1>
  <p class="sub">Open orders for Acme Corp, warehouse Lyon.</p>
  <div class="kpis">
    <div class="kpi"><span>Open</span><b>184</b></div>
    <div class="kpi"><span>Shipped today</span><b>41</b></div>
    <div class="kpi"><span>On hold</span><b>6</b></div>
    <div class="kpi"><span>Backorder value</span><b>28 740 EUR</b></div>
  </div>
  <table>
    <thead><tr><th>Order</th><th>Status</th><th>Tracking</th><th>Lines</th><th>Total</th></tr></thead>
    <tbody>
${ORDERS.map((o) => `      <tr><td>${o.id}</td><td><span class="chip">${o.status}</span></td><td>${o.tracking}</td><td>${o.lines} line${o.lines > 1 ? 's' : ''}</td><td>${o.total}</td></tr>`).join('\n')}
    </tbody>
  </table>
</main>
</body>
</html>
`;

const json = (res, status, body) => {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
};

// What an order service answers, roughly: enough for the status classes to
// differ (a missing order is a 404, a refused export a 429 from the gateway
// before it ever gets here) and for latency to have a shape.
function api(req, res, path) {
  const parts = path.split('/').filter(Boolean);
  const [kind, id, sub] = parts;
  if (kind === 'orders') {
    if (!id) return json(res, req.method === 'POST' ? 201 : 200, req.method === 'POST' ? { id: 'ORD-8820' } : ORDERS);
    if (id === 'export') return json(res, 200, { rows: ORDERS.length });
    if (id === 'reindex') return json(res, 202, { queued: true });
    const order = ORDERS.find((o) => o.id === id);
    if (!order) return json(res, 404, { error: `no order ${id}` });
    if (sub === 'lines') return json(res, 200, [{ sku: 'ACM-104', qty: 2 }]);
    if (sub === 'refund') return json(res, 202, { refunded: order.total });
    return json(res, req.method === 'DELETE' ? 204 : 200, order);
  }
  if (kind === 'shipments') {
    if (!id) return json(res, 200, [{ id: 'SHP-310', order: 'ORD-8814' }]);
    if (sub === 'tracking') return json(res, 200, { carrier: 'DHL', state: 'in transit' });
    return json(res, 200, { id, order: 'ORD-8814' });
  }
  if (kind === 'health') return json(res, 200, { status: 'up' });
  return json(res, 404, { error: `nothing at ${path}` });
}

createServer((req, res) => {
  const path = new URL(req.url, 'http://upstream').pathname;
  const delay = 8 + Math.floor(Math.random() * 70) + (path.includes('export') ? 180 : 0);
  setTimeout(() => {
    if (path === '/' || path === '/index.html') {
      res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' });
      return res.end(page());
    }
    if (path === '/favicon.ico') {
      res.writeHead(204);
      return res.end();
    }
    api(req, res, path);
  }, delay);
}).listen(PORT, '127.0.0.1', () => console.log(`upstream on http://127.0.0.1:${PORT}`));
