import { Injectable, signal } from '@angular/core';
import { Lang, LANGS } from './i18n';

// What the site reads at runtime, written by scripts/build-site.mjs. The
// shapes are that script's output: change one and the other has to move with
// it.

export interface NavPage {
  slug: string;
  title: string;
  summary: string;
}
export interface NavSection {
  name: string;
  pages: NavPage[];
}
export interface NavArea {
  id: string;
  label: string;
  short: string;
  icon: string;
  home: string;
  versioned: boolean;
  sections: NavSection[];
}
export interface DocVersion {
  id: string;
  prefix: string;
}
export interface Nav {
  areas: NavArea[];
  docs: { current: string; versions: DocVersion[] };
}
export interface Page {
  slug: string;
  title: string;
  area: string;
  section: string;
  summary: string;
  layout: string;
  widget: string;
  headings: { id: string; text: string }[];
  html: string;
}
export interface Hit extends NavPage {
  area: string;
  section: string;
  score: number;
}

interface SearchFile {
  pages: { slug: string; title: string; area: string; section: string; summary: string }[];
  index: Record<string, Record<string, number>>;
}

const LANG_KEY = 'meerkat-site-lang';
const SCHEME_KEY = 'meerkat-site-scheme';
export type Scheme = 'system' | 'light' | 'dark';

@Injectable({ providedIn: 'root' })
export class SiteService {
  // The language is READ FROM THE URL, not kept here as the truth: an address
  // sent to someone else has to open in the language it was written in. This
  // signal only follows the router, and the remembered choice decides where a
  // reader who typed no language lands.
  readonly lang = signal<Lang>('en');
  readonly scheme = signal<Scheme>(this.storedScheme());

  private readonly navs = new Map<Lang, Promise<Nav>>();
  private readonly pages = new Map<string, Promise<Page>>();
  private readonly indexes = new Map<string, Promise<SearchFile>>();
  private readonly versionNavs = new Map<string, Promise<NavSection[]>>();

  // ---- language ------------------------------------------------------------

  preferredLang(): Lang {
    try {
      const stored = localStorage.getItem(LANG_KEY);
      if (LANGS.includes(stored as Lang)) return stored as Lang;
    } catch {
      // Private browsing: fall through to what the browser asks for.
    }
    return navigator.language?.toLowerCase().startsWith('fr') ? 'fr' : 'en';
  }

  rememberLang(lang: Lang): void {
    this.lang.set(lang);
    try {
      localStorage.setItem(LANG_KEY, lang);
    } catch {
      // The choice holds for this page and no further.
    }
  }

  // ---- appearance ----------------------------------------------------------

  setScheme(scheme: Scheme): void {
    this.scheme.set(scheme);
    this.applyScheme();
    try {
      localStorage.setItem(SCHEME_KEY, scheme);
    } catch {
      /* same as above */
    }
  }

  applyScheme(): void {
    const s = this.scheme();
    // `light dark` is what a page says when it accepts the system's answer;
    // naming one of the two is what overrides it.
    document.documentElement.style.colorScheme = s === 'system' ? 'light dark' : s;
  }

  private storedScheme(): Scheme {
    try {
      const v = localStorage.getItem(SCHEME_KEY);
      if (v === 'light' || v === 'dark' || v === 'system') return v;
    } catch {
      /* ignore */
    }
    return 'system';
  }

  // ---- content -------------------------------------------------------------

  nav(lang: Lang): Promise<Nav> {
    return this.cached(this.navs, lang, () => this.get<Nav>(`${lang}/nav.json`));
  }

  page(lang: Lang, slug: string): Promise<Page> {
    // One file per page: the reader downloads the page they asked for, not the
    // book. The slug's slashes are flattened by the build.
    const file = slug.split('/').join('__');
    return this.cached(this.pages, `${lang}/${slug}`, () => this.get<Page>(`${lang}/p/${file}.json`));
  }

  // The sections of one documentation version. The current one is already in
  // nav.json; an older one is fetched only when someone switches to it.
  async sectionsFor(lang: Lang, nav: Nav, version: string): Promise<NavSection[]> {
    if (version === nav.docs.current) return nav.areas.find((a) => a.id === 'docs')?.sections ?? [];
    return this.cached(this.versionNavs, `${lang}/${version}`, () =>
      this.get<NavSection[]>(`${lang}/nav-docs-${version}.json`),
    );
  }

  // ---- addresses -----------------------------------------------------------
  //
  // A page is named by its slug; the address is built here, once, so nothing
  // else in the site has to know that the home page has no slug segment and
  // that the current documentation version has no version segment.

  pathFor(lang: Lang, slug: string): string {
    return slug === 'index' ? `/${lang}` : `/${lang}/${slug}`;
  }

  // Which documentation version a slug belongs to. Everything outside the
  // documentation belongs to none, and the current version wears no segment.
  versionOf(slug: string, nav: Nav): string {
    if (!slug.startsWith('docs/')) return '';
    const second = slug.split('/')[1];
    const known = nav.docs.versions.find((v) => v.prefix && v.prefix === second);
    return known ? known.id : nav.docs.current;
  }

  // The same page in another version, when it exists there; its documentation
  // home when it does not.
  async sameIn(lang: Lang, slug: string, nav: Nav, version: string): Promise<string> {
    const from = this.versionOf(slug, nav);
    const fromPrefix = nav.docs.versions.find((v) => v.id === from)?.prefix ?? '';
    const toPrefix = nav.docs.versions.find((v) => v.id === version)?.prefix ?? '';
    const bare = fromPrefix ? slug.replace(`docs/${fromPrefix}/`, 'docs/') : slug;
    const target = toPrefix ? bare.replace('docs/', `docs/${toPrefix}/`) : bare;
    try {
      await this.page(lang, target);
      return target;
    } catch {
      return toPrefix ? `docs/${toPrefix}/index` : 'docs/index';
    }
  }

  // ---- search --------------------------------------------------------------
  //
  // The index is fetched on the FIRST search and never again: it is the one
  // file that grows with the site, and a reader who never searches should not
  // pay for it.

  async search(lang: Lang, version: string, nav: Nav, query: string, limit = 12): Promise<Hit[]> {
    const q = query.trim();
    if (q.length < 2) return [];
    const isCurrent = !version || version === nav.docs.current;
    const file = isCurrent ? `${lang}/search.json` : `${lang}/search-docs-${version}.json`;
    const { index, pages } = await this.cached(this.indexes, file, () => this.get<SearchFile>(file));

    const words = q
      .toLowerCase()
      .split(/[^\p{L}\p{N}_-]+/u)
      .filter((w) => w.length > 1);
    if (!words.length) return [];

    const scores = new Map<number, number>();
    for (const word of words) {
      // Exact term first, then anything starting with it: typing "strip" has
      // to find strip-prefix before the reader has finished the word.
      const postings: Record<string, number>[] = [];
      if (index[word]) postings.push(index[word]);
      for (const term of Object.keys(index)) {
        if (term !== word && term.startsWith(word)) postings.push(index[term]);
      }
      for (const posting of postings) {
        for (const [id, weight] of Object.entries(posting)) {
          const n = Number(id);
          scores.set(n, (scores.get(n) || 0) + weight);
        }
      }
    }

    return [...scores.entries()]
      .map(([id, score]) => ({ ...pages[id], score }))
      .filter((hit) => !!hit?.title)
      .sort((a, b) => b.score - a.score)
      .slice(0, limit);
  }

  // ---- plumbing ------------------------------------------------------------

  private cached<K, V>(store: Map<K, Promise<V>>, key: K, make: () => Promise<V>): Promise<V> {
    const held = store.get(key);
    if (held) return held;
    const fresh = make();
    store.set(key, fresh);
    return fresh;
  }

  private async get<T>(path: string): Promise<T> {
    // Against document.baseURI, so the same bundle serves from / and from
    // /meerkat/ without knowing which.
    const url = new URL(`assets/site/${path}`, document.baseURI);
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${path}: ${res.status}`);
    return (await res.json()) as T;
  }
}
