// @ts-nocheck - the measuring function runs in the page, in plain JavaScript.
import { expect, request, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile } from '../lib/fixtures';

// Every word of every built-in page stays readable, in both schemes, under the
// default palette AND under one whose container text leans the other way from
// the page's text (a dark word on a mid fill in the dark scheme). A rule that
// mixed the two once drew grey on blue on every alternative button; this
// measures each visible text against what is really behind it (WCAG AA: 4.5:1,
// 3:1 for large text). The wordmark is a gradient clipped to its letters, and
// a disabled control is exempt, as WCAG has it.
test('flow-pages-contrast: every built-in page is readable in both schemes and with an inverted palette', async ({ browser }) => {
  test.setTimeout(300_000);
  const api = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile('root') });
  const made = await api.post('/api/themes', {
    data: {
      id: 'e2e-inverted', name: `e2e inverted ${Date.now()}`, contrast: 'standard', colorMatch: true,
      colors: { primary: '#665a48', secondary: '#638da5', tertiary: '#8e9b8c', error: '#8c3a2b', neutral: '#665a48', neutralVariant: '#b7aa95' },
    },
  });
  expect(made.ok(), await made.text()).toBeTruthy();
  const inverted = (await made.json()).id as string;
  const listed = await (await api.get('/api/themes/templates')).json();
  const pages = (Array.isArray(listed) ? listed : listed.templates || listed.pages || [])
    .map((t: { key?: string; id?: string } | string) => (typeof t === 'string' ? t : t.key || t.id))
    .filter((k: unknown): k is string => typeof k === 'string' && !k.startsWith('mail:'));
  expect(pages.length, 'the preview lists the pages').toBeGreaterThan(30);

  const context = await browser.newContext({ storageState: authFile('root'), viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  const problems: string[] = [];
  try {
    for (const theme of ['sentinels-watch', inverted]) {
      for (const scheme of ['light', 'dark']) {
        for (const key of pages) {
          await page.goto(`${ADMIN_URL}/api/themes/${theme}/preview?template=${encodeURIComponent(key)}&scheme=${scheme}&locale=en`);
          // the cards fade in: their entrance is switched off rather than
          // waited for, so what is measured is what stays on screen
          await page.addStyleTag({ content: '*, *::before { animation: none !important; transition: none !important; }' });
          const found: string[] = await page.evaluate(() => {
    const parse = (c) => { const m = c.match(/[\d.]+/g); if (!m) return null; let v = m.map(Number); if (c.startsWith('color(srgb')) v = [v[0] * 255, v[1] * 255, v[2] * 255, v[3] ?? 1]; return { r: v[0], g: v[1], b: v[2], a: v[3] ?? 1 }; };
    const lum = ({ r, g, b }) => { const f = (x) => { x /= 255; return x <= .03928 ? x / 12.92 : ((x + .055) / 1.055) ** 2.4; }; return .2126 * f(r) + .7152 * f(g) + .0722 * f(b); };
    const over = (top, under) => ({ r: top.r * top.a + under.r * (1 - top.a), g: top.g * top.a + under.g * (1 - top.a), b: top.b * top.a + under.b * (1 - top.a), a: 1 });
    const bgOf = (el) => { const chain = []; for (let e = el; e; e = e.parentElement) chain.push(parse(getComputedStyle(e).backgroundColor)); let acc = parse(getComputedStyle(document.body).backgroundColor) || { r: 255, g: 255, b: 255, a: 1 }; if (acc.a < 1) acc = { r: 128, g: 128, b: 128, a: 1 };
      for (const c of chain.reverse()) if (c && c.a > 0) acc = over(c, acc); return acc; };
    const out = [];
    for (const el of document.querySelectorAll('body *')) {
      if (!el.offsetParent && getComputedStyle(el).position !== 'fixed') continue;
      const own = [...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim()); if (!own) continue;
      const cs = getComputedStyle(el); if (cs.visibility === 'hidden' || +cs.opacity === 0) continue;
      if (el.closest('[disabled], .disabled, .eval-band, .mk-eval, .wordmark')) continue;
      let fg = parse(cs.color); const bg = bgOf(el); if (!fg) continue; fg = over(fg, bg);
      const [x, y] = [lum(fg), lum(bg)].sort((a, b) => b - a); const ratio = (x + .05) / (y + .05);
      const big = parseFloat(cs.fontSize) >= 24 || (parseFloat(cs.fontSize) >= 18.6 && +cs.fontWeight >= 700);
      if (ratio < (big ? 3 : 4.5)) out.push(`${ratio.toFixed(2)} ${el.tagName.toLowerCase()}.${[...el.classList].join('.')} "${el.textContent.trim().slice(0, 40)}"`);
    }
    return out;
  });
          for (const f of found) problems.push(`${theme} ${scheme} ${key}: ${f}`);
        }
      }
    }
  } finally {
    await context.close();
    await api.delete(`/api/themes/${inverted}`);
  }
  expect(problems, problems.join('\n')).toEqual([]);
});
