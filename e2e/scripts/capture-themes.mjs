// Theme examples for the documentation: invented brands, each in a register a
// reader recognises (streaming, social, bank, developer tool, groceries,
// fashion), shown on the sign-in page in the arrangement that suits it.
//
// No real brand, on purpose: a sign-in page wearing a known company's colours
// and mark, served by somebody else, is what a phishing kit looks like, and a
// commercial page that borrows a trademark implies a partnership there is not.
// The point - with the right theme the page becomes yours - is made as well by
// six identities that do not resemble each other.
//
// Runs against a gateway already started (ADMIN_URL, DATA_URL, ADMIN_PASSWORD),
// gives it the Enterprise license this machine holds (the arrangements past
// Centered are Enterprise, and the doc shows the product, not the evaluation
// notice), then for each brand: theme, branding, arrangement, capture.
//
//   ADMIN_URL=http://localhost:19392 DATA_URL=http://localhost:18382 \
//   ADMIN_PASSWORD=... OUT=/some/dir node scripts/capture-themes.mjs [brand...]
//
// The site takes them as WebP, in docs/public/img/login-examples:
//   for f in $OUT/*.png; do cwebp -q 80 -resize 1280 0 "$f" -o "${f%.png}.webp"; done

import { chromium, request } from '@playwright/test';
import { existsSync, mkdirSync, readFileSync, readdirSync } from 'node:fs';
import { homedir } from 'node:os';

const ADMIN = process.env.ADMIN_URL ?? 'http://localhost:19392';
const DATA = process.env.DATA_URL ?? 'http://localhost:18382';
const PASSWORD = process.env.ADMIN_PASSWORD;
const OUT = process.env.OUT ?? 'theme-shots';
if (!PASSWORD) throw new Error('ADMIN_PASSWORD: the password of the gateway admin');
mkdirSync(OUT, { recursive: true });

const svg = (body) => 'data:image/svg+xml;base64,' + Buffer.from(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">${body}</svg>`).toString('base64');

// ── the brands ──────────────────────────────────────────────────────────────
// `shots` lists [arrangement, side, scheme] to capture; the first is the
// brand's own.
const BRANDS = [
  {
    key: 'nightreel',
    appName: 'Nightreel',
    tagline: 'Every story, one sign-in',
    colors: { primary: '#D32F45', secondary: '#6D4C7D', neutral: '#1C1A22' },
    fonts: { display: 'Montserrat', body: 'Inter' },
    logo: svg('<rect width="64" height="64" rx="15" fill="#D32F45"/><path d="M25 19v26l21-13z" fill="#fff"/>'),
    background: `
      background: radial-gradient(ellipse at 30% 40%, #3a0d1a 0%, #120a12 55%, #07060a 100%);
      --cards: 1;`,
    scene: 'posters',
    shots: [['drawer', 'right', 'dark']],
  },
  {
    key: 'glimmo',
    appName: 'Glimmo',
    tagline: 'Share the moment',
    colors: { primary: '#C2185B', secondary: '#7B2FF7', tertiary: '#FF7A00' },
    fonts: { display: 'Nunito', body: 'Nunito' },
    logo: svg('<defs><linearGradient id="g" x1="0" y1="1" x2="1" y2="0"><stop offset="0" stop-color="#FF7A00"/><stop offset=".5" stop-color="#D6246E"/><stop offset="1" stop-color="#7B2FF7"/></linearGradient></defs><circle cx="32" cy="32" r="30" fill="url(#g)"/><path d="M32 14c2 10 8 16 18 18-10 2-16 8-18 18-2-10-8-16-18-18 10-2 16-8 18-18z" fill="#fff"/>'),
    background: `background: #fff6f0;`,
    backgroundDark: `background: #1a1020;`,
    scene: 'blobs',
    shots: [['centered', '', 'light'], ['centered', '', 'dark']],
  },
  {
    key: 'halvern',
    appName: 'Halvern Bank',
    tagline: 'Private banking, online',
    colors: { primary: '#1F3C88', secondary: '#2A6F7F', neutral: '#5D6470' },
    fonts: { display: 'Source Serif 4', body: 'Inter' },
    logo: svg('<rect width="64" height="64" rx="12" fill="#1F3C88"/><path d="M12 25 32 13l20 12z" fill="#fff"/><rect x="16" y="28" width="5" height="17" fill="#fff"/><rect x="29.5" y="28" width="5" height="17" fill="#fff"/><rect x="43" y="28" width="5" height="17" fill="#fff"/><rect x="12" y="47" width="40" height="5" rx="1" fill="#fff"/>'),
    background: `background: linear-gradient(160deg, #f4f7fc 0%, #dfe7f3 100%);`,
    backgroundDark: `background: linear-gradient(160deg, #0f1f45 0%, #1f3c88 60%, #2a4f9e 100%);`,
    scene: 'columns',
    shots: [['split', 'right', 'light'], ['split', 'right', 'dark']],
  },
  {
    key: 'monoforge',
    appName: 'Monoforge',
    tagline: 'Build. Ship. Repeat.',
    colors: { primary: '#3FB950', neutral: '#30363D', neutralVariant: '#3A4048' },
    fonts: { display: 'Roboto', body: 'Inter', code: 'JetBrains Mono' },
    flat: true,
    logo: svg('<rect x="2" y="2" width="60" height="60" rx="12" fill="#0d1117" stroke="#3FB950" stroke-width="3"/><path d="M17 22l11 10-11 10" fill="none" stroke="#3FB950" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/><path d="M32 44h15" stroke="#e6edf3" stroke-width="5" stroke-linecap="round"/>'),
    background: `background: #0d1117;`,
    scene: 'code',
    shots: [['bare', '', 'dark']],
  },
  {
    key: 'basketa',
    appName: 'Basketa',
    tagline: 'Fresh groceries, delivered',
    colors: { primary: '#E8590C', secondary: '#2B8A3E', tertiary: '#F2B705' },
    fonts: { display: 'Raleway', body: 'Open Sans' },
    logo: svg('<rect width="64" height="64" rx="18" fill="#E8590C"/><path d="M20 27a12 12 0 0 1 24 0" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round"/><path d="M12 27h40l-5 22H17z" fill="#fff"/><path d="M24 33v10M32 33v10M40 33v10" stroke="#E8590C" stroke-width="3" stroke-linecap="round"/>'),
    background: `background: #fff8ee;`,
    scene: 'produce',
    shots: [['banner', '', 'light']],
  },
  {
    key: 'velune',
    appName: 'Maison Velune',
    tagline: 'Atelier privé',
    colors: { primary: '#A67C2E', secondary: '#6B5B45', neutral: '#2A2622' },
    fonts: { display: 'Playfair Display', body: 'Lora' },
    flat: true,
    logo: svg('<circle cx="32" cy="32" r="30" fill="#14110e"/><path d="M38 13a20 20 0 1 0 0 38 16 16 0 1 1 0-38z" fill="#C9A24D"/>'),
    background: `background: radial-gradient(ellipse at 70% 30%, #fffaf2 0%, #f1e7d6 75%);`,
    backgroundDark: `background: radial-gradient(ellipse at 70% 30%, #3b2f22 0%, #17130f 60%, #0b0907 100%);`,
    scene: 'silk',
    shots: [['split', 'left', 'dark'], ['drawer', 'left', 'light']],
  },
];

// ── the pictures behind ─────────────────────────────────────────────────────
// Drawn here, in a page, and photographed: abstract on purpose, so nothing in
// them belongs to anybody.
const SCENES = {
  posters: () => {
    const hues = ['#7a1022', '#3d1f5c', '#0f3b4a', '#5a2a0f', '#2b123d', '#6b0f1a', '#14324f', '#4a1530'];
    let s = '';
    for (let r = 0; r < 4; r++) for (let c = 0; c < 9; c++) {
      const h = hues[(r * 3 + c) % hues.length], k = hues[(r + c * 5) % hues.length];
      s += `<div style="position:absolute;left:${c * 190 - 60 + (r % 2) * 95}px;top:${r * 270 - 40}px;width:160px;height:240px;border-radius:12px;background:linear-gradient(160deg,${h},${k} 70%,#000);opacity:.85;box-shadow:0 20px 40px #000a"></div>`;
    }
    return `<div style="position:absolute;inset:-80px;transform:rotate(-8deg) scale(1.1);filter:blur(1.5px)">${s}</div><div style="position:absolute;inset:0;background:radial-gradient(ellipse at 35% 45%,transparent 20%,#07060aee 75%)"></div>`;
  },
  blobs: (dark) =>
    [['#ff9a5c', 8, 10, 520], ['#f35a9b', 55, 5, 600], ['#9b6bff', 70, 55, 560], ['#ffd27a', 15, 60, 480]]
      .map(([c, x, y, d]) => `<div style="position:absolute;left:${x}%;top:${y}%;width:${d}px;height:${d}px;border-radius:50%;background:${c};filter:blur(90px);opacity:${dark ? 0.4 : 0.75}"></div>`).join(''),
  columns: (dark) => {
    const [col, edge] = dark ? ['#ffffff1c,#ffffff06 70%,#ffffff1c', '#c9a24d66'] : ['#1f3c881a,#1f3c8806 70%,#1f3c881a', '#c9a24d88'];
    let s = '';
    for (let i = 0; i < 12; i++) s += `<div style="position:absolute;top:0;bottom:0;left:${2 + i * 8.5}%;width:4%;background:linear-gradient(90deg,${col});border-left:1px solid ${edge};border-right:1px solid ${edge}"></div>`;
    return s;
  },
  code: () => {
    const lines = ['func main() {', '  gate := meerkat.New()', '  gate.Route("/api", svc)', '  log.Fatal(gate.Serve())', '}', 'git push origin main', '$ make test', 'ok   ./...   0.42s', 'deploy: done in 38s'];
    let s = '';
    for (let i = 0; i < 26; i++) s += `<div style="white-space:pre;opacity:${0.05 + (i % 5) * 0.03}">${lines[i % lines.length].padEnd(40)}   ${lines[(i * 7) % lines.length]}</div>`;
    return `<div style="position:absolute;inset:0;background-image:radial-gradient(#30363d 1px,transparent 1px);background-size:22px 22px"></div><div style="position:absolute;left:60px;top:40px;font:20px/38px 'JetBrains Mono',monospace;color:#3FB950">${s}</div><div style="position:absolute;inset:0;background:radial-gradient(ellipse at 50% 50%,#0d1117cc 25%,transparent 70%)"></div>`;
  },
  produce: () =>
    [['#ffb347', 6, 12, 180], ['#ff7043', 80, 8, 140], ['#8bc34a', 70, 62, 200], ['#ffd54f', 12, 70, 150], ['#e57373', 40, 80, 120], ['#aed581', 90, 40, 110], ['#ffcc80', 30, 20, 90]]
      .map(([c, x, y, d]) => `<div style="position:absolute;left:${x}%;top:${y}%;width:${d}px;height:${d}px;border-radius:50%;background:radial-gradient(circle at 35% 30%,#fff8,${c} 45%);opacity:.55"></div>`).join(''),
  silk: (dark) =>
    [0, 1, 2, 3, 4, 5, 6, 7].map((i) => `<div style="position:absolute;left:${-30 + i * 9}%;top:${-40 + i * 6}%;width:140%;height:140%;border-radius:50%;border:${dark ? 1.5 : 2.5}px solid ${dark ? '#c9a24d' : '#a67c2e'}${(dark ? ['55', '48', '40', '33', '28', '22', '18', '12'] : ['aa', '99', '88', '77', '66', '55', '44', '33'])[i]}"></div>`).join('')
      + (dark ? '' : '<div style="position:absolute;right:12%;top:18%;width:340px;height:340px;border-radius:50%;background:radial-gradient(circle at 35% 35%,#fffdf8,#ead9b8 70%);box-shadow:0 30px 80px #a67c2e44"></div>'),
};

async function picture(browser, brand, dark = false) {
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 } });
  const ground = dark ? brand.backgroundDark : brand.background;
  await page.setContent(`<html><body style="margin:0;width:1600px;height:1000px;position:relative;overflow:hidden;${ground}">${SCENES[brand.scene](dark)}</body></html>`);
  await page.waitForTimeout(300);
  const jpg = await page.screenshot({ type: 'jpeg', quality: 82 });
  await page.close();
  return 'data:image/jpeg;base64,' + jpg.toString('base64');
}

// ── the gateway ─────────────────────────────────────────────────────────────

const api = await request.newContext({ baseURL: ADMIN });
const ok = async (r, what) => {
  if (r.status() >= 300) throw new Error(`${what} -> ${r.status()} ${await r.text()}`);
  return r;
};
await ok(await api.post('/login', { form: { username: 'admin', password: PASSWORD }, maxRedirects: 0 }).then((r) => (r.status() === 303 ? { status: () => 200 } : r)), 'sign in');

// The license this machine holds, never one from the repository (mirrored in
// public): the arrangements past Centered are Enterprise, and an evaluation
// prints its notice on every page.
{
  const dir = `${homedir()}/.config/softwarity/licenses`;
  const file = process.env.MEERKAT_DOC_LICENSE
    || (existsSync(dir) ? readdirSync(dir).filter((f) => f.endsWith('.lic') && f.includes('enterprise')).map((f) => `${dir}/${f}`)[0] : '');
  if (!file) throw new Error('no Enterprise license: set MEERKAT_DOC_LICENSE, or the captures carry the evaluation notice');
  await ok(await api.put('/api/license', { data: { license: readFileSync(file, 'utf8') } }), 'license');
}

const browser = await chromium.launch();
const wanted = process.argv.slice(2);
for (const brand of BRANDS.filter((b) => !wanted.length || wanted.includes(b.key))) {
  // A theme of its own, made active.
  const themes = await (await api.get('/api/themes')).json();
  let theme = themes.find((t) => t.name === brand.appName);
  const body = { name: brand.appName, colors: brand.colors, fonts: brand.fonts, contrast: 'standard', flat: !!brand.flat };
  theme = theme
    ? await (await ok(await api.put(`/api/themes/${theme.id}`, { data: { ...theme, ...body } }), 'theme')).json()
    : await (await ok(await api.post('/api/themes', { data: body }), 'theme')).json();
  await ok(await api.post(`/api/themes/${theme.id}/activate`), 'activate');

  // Its name, mark and picture.
  const branding = await (await api.get('/api/branding')).json();
  await ok(await api.put('/api/branding', {
    data: {
      ...branding,
      appName: brand.appName,
      tagline: brand.tagline,
      logo: brand.logo,
      logoSize: 'large',
      favicon: '',
      // A picture per scheme when the brand has two: one picture under both
      // schemes leaves light text on a light picture, or dark on dark.
      background: brand.backgroundDark
        ? { image: await picture(browser, brand), imageDark: await picture(browser, brand, true), both: false, fit: 'cover', fitDark: 'cover', dim: 15, dimDark: 15 }
        : { image: await picture(browser, brand), fit: 'cover', dim: 15, both: true },
    },
  }), 'branding');

  for (const [layout, side, scheme] of brand.shots) {
    const settings = await (await api.get('/api/settings')).json();
    await ok(await api.put('/api/settings', { data: { ...settings, pageLayout: side ? { name: layout, side } : { name: layout } } }), 'layout');
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: scheme, locale: 'en-US' });
    const page = await context.newPage();
    // The pages read the branding and the settings through a short cache:
    // ask again until the name is this brand's and the body wears the
    // arrangement (and its side) just saved.
    const want = ['mk-' + layout, ...(side ? ['side-' + side] : [])];
    for (let i = 0; i < 30; i++) {
      await page.goto(DATA + '/login', { waitUntil: 'networkidle' });
      const cls = await page.evaluate(() => document.body.className.split(/\s+/));
      if ((await page.content()).includes(brand.appName) && want.every((c) => cls.includes(c))) break;
      if (i === 29) throw new Error(`${brand.key}: the page never showed ${want.join(' ')}`);
      await page.waitForTimeout(500);
    }
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(1200);
    const file = `${OUT}/${brand.key}-${layout}-${scheme}.png`;
    await page.screenshot({ path: file });
    console.log('captured', file);
    await context.close();
  }
}
await browser.close();
await api.dispose();
