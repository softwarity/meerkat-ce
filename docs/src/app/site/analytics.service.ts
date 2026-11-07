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
const MEASUREMENT_ID = '';

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
  // The page being read, kept for the page view sent at the moment of consent.
  private last: { title: string; location: string } | null = null;

  constructor() {
    if (this.consent() === 'granted') this.load();
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
      this.load();
      if (this.last) this.send(this.last);
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
  // application changes pages without reloading.
  pageView(title: string): void {
    this.last = { title, location: location.href };
    if (this.loaded && this.consent() === 'granted') this.send(this.last);
  }

  private send(view: { title: string; location: string }): void {
    window.gtag?.('event', 'page_view', { page_title: view.title, page_location: view.location });
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
    // Page views are sent by the application, one per page read (above).
    window.gtag('config', MEASUREMENT_ID, { send_page_view: false });
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
