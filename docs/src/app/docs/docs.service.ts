import { Injectable, signal } from '@angular/core';

// What the site reads at runtime, written by scripts/build-docs.mjs from the
// Markdown under docs/content. The shapes are that script's output: change one
// and the other has to move with it.

export interface TocEntry {
  slug: string;
  title: string;
  section: string;
  summary: string;
  order: number;
}

export interface DocPage extends TocEntry {
  headings: { id: string; text: string }[];
  html: string;
}

export interface Hit {
  slug: string;
  title: string;
  section: string;
  summary: string;
  score: number;
}

export type Lang = 'en' | 'fr';

// The reader's language, kept here because three components need it and none
// of them owns it. Remembered, so the site does not ask twice.
const LANG_KEY = 'meerkat-docs-lang';

@Injectable({ providedIn: 'root' })
export class DocsService {
  readonly lang = signal<Lang>(this.initialLang());

  private readonly tocs = new Map<Lang, Promise<TocEntry[]>>();
  private readonly pages = new Map<string, Promise<DocPage>>();
  private readonly indexes = new Map<Lang, Promise<SearchIndex>>();

  setLang(lang: Lang): void {
    this.lang.set(lang);
    try {
      localStorage.setItem(LANG_KEY, lang);
    } catch {
      // Private browsing: the choice holds for this page and no further.
    }
  }

  toc(lang: Lang): Promise<TocEntry[]> {
    return this.cached(this.tocs, lang, () => this.get<TocEntry[]>(`assets/docs/${lang}/toc.json`));
  }

  page(lang: Lang, slug: string): Promise<DocPage> {
    // One file per page: the reader downloads the page they asked for, not the
    // book. The slug's slashes are flattened by the build, see build-docs.mjs.
    const file = slug.split('/').join('__');
    return this.cached(this.pages, `${lang}/${slug}`, () => this.get<DocPage>(`assets/docs/${lang}/${file}.json`));
  }

  // The index is fetched on the FIRST search and never again: it is the one
  // file that grows with the documentation, and a reader who never searches
  // should not pay for it.
  async search(lang: Lang, query: string, limit = 12): Promise<Hit[]> {
    const q = query.trim();
    if (q.length < 2) return [];
    const { index, pages } = await this.cached(this.indexes, lang, () =>
      this.get<SearchIndex>(`assets/docs/${lang}/search.json`),
    );

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
      if (!postings.length) continue;
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

  private initialLang(): Lang {
    try {
      const stored = localStorage.getItem(LANG_KEY);
      if (stored === 'en' || stored === 'fr') return stored;
    } catch {
      // Fall through to the browser's own preference.
    }
    return navigator.language?.toLowerCase().startsWith('fr') ? 'fr' : 'en';
  }

  private cached<K, V>(store: Map<K, Promise<V>>, key: K, make: () => Promise<V>): Promise<V> {
    const held = store.get(key);
    if (held) return held;
    const fresh = make();
    store.set(key, fresh);
    return fresh;
  }

  private async get<T>(url: string): Promise<T> {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${url}: ${res.status}`);
    return (await res.json()) as T;
  }
}

interface SearchIndex {
  // Pages are numbered in the index; this is the numbering.
  pages: { slug: string; title: string; section: string; summary: string }[];
  index: Record<string, Record<string, number>>;
}
