import { Component, computed, effect, inject, signal } from '@angular/core';
import { RouterLink, RouterOutlet } from '@angular/router';
import { DocsService, Hit, Lang, TocEntry } from './docs.service';

// The documentation's frame: the sections down the left, the language, and the
// search. The page itself is the router outlet - see docs-page.component.
//
// The sections come from the table of contents the build writes, not from a
// list kept here: a page added under docs/content appears in the navigation
// because it exists, and nobody has to remember a second place.
@Component({
  selector: 'app-docs-shell',
  imports: [RouterLink, RouterOutlet],
  styles: [
    `
      :host {
        display: block;
      }
      .layout {
        display: grid;
        grid-template-columns: 250px minmax(0, 1fr);
        gap: 28px;
        align-items: start;
      }
      @media (max-width: 900px) {
        .layout {
          grid-template-columns: 1fr;
        }
        .side {
          position: static;
          max-height: none;
        }
      }
      .side {
        position: sticky;
        top: 12px;
        max-height: calc(100vh - 24px);
        overflow-y: auto;
        padding-right: 6px;
      }
      .tools {
        display: flex;
        gap: 6px;
        margin-bottom: 12px;
      }
      .search {
        flex: 1 1 auto;
        min-width: 0;
        width: 100%;
        padding: 6px 10px;
        border: 1px solid var(--border-color);
        border-radius: 8px;
        background: var(--bg-secondary);
        color: var(--text-primary);
        font: inherit;
      }
      .lang {
        display: flex;
        flex: 0 0 auto;
        gap: 4px;
      }
      .lang button {
        border: 1px solid var(--border-color);
        background: var(--bg-secondary);
        color: var(--text-primary);
        border-radius: 8px;
        padding: 6px 9px;
        cursor: pointer;
        font-size: 0.8em;
      }
      .lang button.on {
        border-color: var(--accent, #25c2e0);
        color: var(--accent, #25c2e0);
      }
      .section-name {
        font-size: 0.72em;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: var(--text-muted);
        margin: 16px 0 6px;
      }
      .side a {
        display: block;
        padding: 4px 8px;
        border-radius: 6px;
        color: var(--text-primary);
        text-decoration: none;
        font-size: 0.93em;
      }
      .side a:hover {
        background: var(--bg-secondary);
      }
      .results {
        border: 1px solid var(--border-color);
        border-radius: 8px;
        background: var(--bg-secondary);
        padding: 6px;
        margin-bottom: 10px;
      }
      .results a {
        display: block;
        padding: 6px 8px;
        border-radius: 6px;
        text-decoration: none;
        color: var(--text-primary);
      }
      .results a:hover {
        background: var(--bg-primary);
      }
      .results .where {
        display: block;
        font-size: 0.72em;
        color: var(--text-muted);
      }
      .empty {
        padding: 6px 8px;
        color: var(--text-muted);
        font-size: 0.85em;
      }
    `,
  ],
  template: `
    <div class="layout">
      <nav class="side">
        <div class="tools">
          <input
            class="search"
            type="search"
            [value]="query()"
            (input)="onQuery($any($event.target).value)"
            [placeholder]="lang() === 'fr' ? 'Rechercher' : 'Search'"
            [attr.aria-label]="lang() === 'fr' ? 'Rechercher dans la documentation' : 'Search the documentation'"
          />
          <span class="lang">
            <button [class.on]="lang() === 'en'" (click)="setLang('en')">EN</button>
            <button [class.on]="lang() === 'fr'" (click)="setLang('fr')">FR</button>
          </span>
        </div>

        @if (query().length > 1) {
          <div class="results">
            @for (hit of hits(); track hit.slug) {
              <a [routerLink]="['/docs', hit.slug]" (click)="clearQuery()">
                {{ hit.title }}
                <span class="where">{{ hit.section }}</span>
              </a>
            } @empty {
              <p class="empty">{{ lang() === 'fr' ? 'Aucun resultat' : 'No result' }}</p>
            }
          </div>
        }

        @for (group of sections(); track group.name) {
          <div class="section-name">{{ group.name }}</div>
          @for (page of group.pages; track page.slug) {
            <a [routerLink]="['/docs', page.slug]">{{ page.title }}</a>
          }
        }
      </nav>

      <div><router-outlet /></div>
    </div>
  `,
})
export class DocsShellComponent {
  private readonly docs = inject(DocsService);
  protected readonly lang = this.docs.lang;
  protected readonly query = signal('');
  protected readonly hits = signal<Hit[]>([]);
  private readonly toc = signal<TocEntry[]>([]);

  // Grouped by section, each in the order the front matter asked for.
  protected readonly sections = computed(() => {
    const groups: { name: string; pages: TocEntry[] }[] = [];
    for (const page of this.toc()) {
      const group = groups.find((g) => g.name === page.section);
      if (group) group.pages.push(page);
      else groups.push({ name: page.section, pages: [page] });
    }
    return groups;
  });

  constructor() {
    effect(() => {
      const lang = this.lang();
      this.docs.toc(lang).then((toc) => this.toc.set(toc));
      // A language change re-runs the search, so the results are in the
      // language being read rather than the one they were typed in.
      if (this.query().length > 1) void this.runSearch(this.query());
    });
  }

  protected setLang(lang: Lang): void {
    this.docs.setLang(lang);
  }

  protected onQuery(value: string): void {
    this.query.set(value);
    void this.runSearch(value);
  }

  protected clearQuery(): void {
    this.query.set('');
    this.hits.set([]);
  }

  private async runSearch(value: string): Promise<void> {
    const hits = await this.docs.search(this.lang(), value);
    // The reader may have typed on while this was in flight.
    if (this.query() === value) this.hits.set(hits);
  }
}
