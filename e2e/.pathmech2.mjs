import http from 'node:http';
const app = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end('<html><head></head><body>path=' + req.url + '</body></html>');
});
await new Promise((r) => app.listen(7789, r));
const login = await fetch('http://localhost:9099/login', {
  method: 'POST', redirect: 'manual',
  headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  body: 'username=admin&password=test1234',
});
const jar = login.headers.getSetCookie().map(c => c.split(';')[0]).join('; ');
const save = async (id, filters) => {
  const r = await fetch('http://localhost:9099/api/routes/' + id, {
    method: 'PUT', headers: { 'Content-Type': 'application/json', cookie: jar },
    body: JSON.stringify({
      id, name: id, order: 4, enabled: true, access: {}, isUi: true,
      upstream: 'http://127.0.0.1:7789',
      predicates: [{ type: 'path', args: { patterns: ['/pm/**'] } }],
      filters, ui: { userButton: { enabled: true } },
      locales: { mechanisms: ['path', 'query'], param: 'lg', disabled: [] },
    }),
  });
  return r.status;
};
console.log('plain route:', await save('pathmech', []));
const ask = async (path, lang) => {
  const r = await fetch('http://localhost:8099' + path, { headers: { cookie: 'MEERKAT_LANG=' + lang } });
  return (await r.text()).match(/path=[^<]*/)[0];
};
console.log('  /pm         ->', await ask('/pm', 'fr'));
console.log('  /pm/        ->', await ask('/pm/', 'fr'));
console.log('  /pm/route   ->', await ask('/pm/route', 'fr'));
console.log('  /pm/en/route->', await ask('/pm/en/route', 'fr'));
console.log('with strip-prefix:', await save('pathmech', [{ type: 'strip-prefix', args: { parts: 1 } }]));
console.log('  /pm/route   ->', await ask('/pm/route', 'fr'));
console.log('  /pm/en/route->', await ask('/pm/en/route', 'fr'));
app.close();
