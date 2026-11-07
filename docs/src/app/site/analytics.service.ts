import { Injectable, signal } from '@angular/core';

// Audience measurement, with Google Analytics 4 - and only with the reader's
// consent, asked once, as the CNIL requires of a cookie that is not needed to
// read the site.
//
// Nothing of Google is loaded before "Accept": no script, no cookie, no
// request. That is Consent Mode's basic implementation, the strictest one -
// the advanced one loads the tag first and sends cookieless pings, which is
// still a request to Google about a reader who said nothing yet. "Refuse" is
// as easy as "Accept", and the footer's "Cookies" link asks again at any time.
//
// MEASUREMENT_ID is the GA4 property's "G-..." id. Empty, there is no banner
// and nothing is measured. It is public by nature (every page that measures
// shows it), so it lives here rather than in a secret.
const MEASUREMENT_ID = 'G-D11Q13E556';

// Measured only where the site is published: a local `ng serve`, or the site
// served behind a gateway for a test, is not an audience.
const HOST = 'meerkat.softwarity.dev';

const CONSENT_KEY = 'meerkat-site-consent';

type Consent = 'granted' | 'denied';

declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
  }
}

@Injectable({ providedIn: 'root' })
export class AnalyticsService {
  // Whether this site measures at all: an id, on the published host.
  readonly enabled = !!MEASUREMENT_ID && location.hostname === HOST;

  // The reader's answer; null while they have given none.
  readonly consent = signal<Consent | null>(this.stored());
  // The banner is up: no answer yet, or the reader asked to change it.
  readonly asking = signal(this.enabled && this.consent() === null);

  private loaded = false;
  // Google's tag counts the page it loads on by itself: it ignores
  // send_page_view:false (measured 2026-11-07, two page views per first page).
  // Loaded at start, for a reader who accepted on an earlier visit, it counts
  // the page the application is about to report: that report is skipped.
  private tagCounts = false;

  constructor() {
    if (this.consent() === 'granted') {
      this.load();
      this.tagCounts = true;
    }
    // One listener each for the whole site, the Markdown pages included: their
    // links and code blocks are not components that could report themselves.
    document.addEventListener('click', (e) => this.clicked(e), { capture: true });
    document.addEventListener('copy', () => this.copiedSelection());
  }

  decide(consent: Consent): void {
    this.consent.set(consent);
    this.asking.set(false);
    try {
      localStorage.setItem(CONSENT_KEY, consent);
    } catch {
      /* private window, blocked storage: asked again next visit */
    }
    if (consent === 'granted') {
      window.gtag?.('consent', 'update', { analytics_storage: 'granted' });
      // The tag, loading now, counts the page being read.
      this.load();
      this.interest(location.pathname);
      return;
    }
    // A withdrawn consent: Google stops writing, and its cookies go.
    window.gtag?.('consent', 'update', { analytics_storage: 'denied' });
    this.dropCookies();
  }

  ask(): void {
    if (this.enabled) this.asking.set(true);
  }

  // One page read: sent by the page once its title is set, since the
  // application changes pages without reloading. The property's "page changes
  // based on browser history events" stays off: it fires before the new title
  // is set, and would count each page a second time.
  pageView(title: string): void {
    this.interest(location.pathname);
    if (this.tagCounts) {
      this.tagCounts = false;
      return;
    }
    this.event('page_view', { page_title: title, page_location: location.href });
  }

  // The gestures that say a reader is going further than reading - copying a
  // command, fetching a deployment file, going to the code or the image,
  // writing to sales, looking at the prices. Each has its own name, so that
  // the property can mark it as a key event; the names are snake_case, as GA4
  // wants them, and stable: a report is built on them.
  event(name: string, params: Record<string, string> = {}): void {
    if (!this.loaded || this.consent() !== 'granted') return;
    window.gtag?.('event', name, params);
  }

  private interest(path: string): void {
    if (/^\/pricing(\/|$)/.test(path)) this.event('view_pricing');
  }

  private clicked(e: MouseEvent): void {
    const a = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null;
    if (!a) return;
    let url: URL;
    try {
      url = new URL(a.href);
    } catch {
      return;
    }
    const placement = a.closest('.topbar') ? 'topbar' : a.closest('.site-footer') ? 'footer' : 'page';
    if (url.protocol === 'mailto:') {
      if (!url.pathname.startsWith('sales@')) return;
      const plan = (url.searchParams.get('subject') ?? '').replace(/^Meerkat\s*/, '') || 'general';
      this.event('contact_sales', { plan, placement });
    } else if (a.hasAttribute('download') && url.pathname.includes('/deploy/')) {
      this.event('download_deploy_file', { file_name: url.pathname.split('/').pop() ?? '' });
    } else if (url.hostname === 'github.com' && /^\/softwarity\/meerkat-ce(\/|$)/.test(url.pathname)) {
      this.event('github_click', { link_url: url.href, placement });
    } else if (url.hostname === 'hub.docker.com' || url.hostname === 'ghcr.io') {
      this.event('image_click', { link_url: url.href, placement });
    }
  }

  // A command selected by hand in a code block and copied. The install
  // widget's Copy button writes to the clipboard without this event, and
  // reports itself.
  private copiedSelection(): void {
    const sel = document.getSelection();
    const node = sel?.anchorNode;
    const el = node instanceof Element ? node : node?.parentElement;
    if (!sel || !el?.closest('pre')) return;
    const command = sel.toString().trim().split(/\s+/)[0];
    if (command) this.event('copy_command', { command, source: 'selection' });
  }

  private load(): void {
    if (!this.enabled || this.loaded) return;
    this.loaded = true;
    window.dataLayer = window.dataLayer ?? [];
    // gtag queues its arguments object, not an array: Google's loader reads
    // the queue that way.
    window.gtag = function gtag() {
      // eslint-disable-next-line prefer-rest-params
      window.dataLayer!.push(arguments);
    };
    window.gtag('consent', 'default', {
      analytics_storage: 'granted',
      ad_storage: 'denied',
      ad_user_data: 'denied',
      ad_personalization: 'denied',
    });
    window.gtag('js', new Date());
    // The tag counts this page; the next ones are sent by pageView (above).
    window.gtag('config', MEASUREMENT_ID);
    const script = document.createElement('script');
    script.async = true;
    script.src = `https://www.googletagmanager.com/gtag/js?id=${MEASUREMENT_ID}`;
    document.head.appendChild(script);
  }

  private dropCookies(): void {
    const domains = ['', location.hostname, `.${location.hostname.split('.').slice(-2).join('.')}`];
    for (const c of document.cookie.split(';')) {
      const name = c.split('=')[0].trim();
      if (!name.startsWith('_ga')) continue;
      for (const d of domains) {
        document.cookie = `${name}=; Max-Age=0; path=/${d ? `; domain=${d}` : ''}`;
      }
    }
  }

  private stored(): Consent | null {
    try {
      const v = localStorage.getItem(CONSENT_KEY);
      return v === 'granted' || v === 'denied' ? v : null;
    } catch {
      return null;
    }
  }
}
