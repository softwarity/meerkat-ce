import { Component, computed, effect, inject, signal } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { DocsService, DocPage } from './docs.service';

// One page of the documentation: the HTML the build produced, plus the
// on-page table of contents its h2s gave.
//
// bypassSecurityTrustHtml is safe HERE and would not be anywhere else: this
// HTML is not user input, it is the output of scripts/build-docs.mjs over
// Markdown files kept in this repository - the same trust one gives a
// component's own template.
@Component({
  selector: 'app-docs-page',
  imports: [RouterLink],
  styles: [
    `
      :host {
        display: block;
      }
      .page {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 190px;
        gap: 28px;
        align-items: start;
      }
      @media (max-width: 1100px) {
        .page {
          grid-template-columns: 1fr;
        }
        .onpage {
          display: none;
        }
      }
      .onpage {
        position: sticky;
        top: 12px;
        font-size: 0.85em;
      }
      .onpage .label {
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.08em;
        font-size: 0.8em;
        margin-bottom: 6px;
      }
      .onpage a {
        display: block;
        padding: 3px 0;
        color: var(--text-muted);
        text-decoration: none;
      }
      .onpage a:hover {
        color: var(--accent, #25c2e0);
      }
      .body ::ng-deep img {
        max-width: 100%;
        border: 1px solid var(--border-color);
        border-radius: 8px;
      }
      .body ::ng-deep .table-wrap {
        overflow-x: auto;
      }
      .body ::ng-deep table {
        border-collapse: collapse;
        width: 100%;
      }
      .body ::ng-deep th,
      .body ::ng-deep td {
        border: 1px solid var(--border-color);
        padding: 6px 10px;
        text-align: left;
      }
      .body ::ng-deep pre {
        overflow-x: auto;
        padding: 12px;
        border-radius: 8px;
        background: var(--bg-secondary);
      }
      .body ::ng-deep .callout {
        border-left: 3px solid var(--accent, #25c2e0);
        background: var(--bg-secondary);
        padding: 10px 14px;
        border-radius: 0 8px 8px 0;
        margin: 14px 0;
      }
      .body ::ng-deep .callout.warning {
        border-left-color: var(--accent-red, #cf222e);
      }
      .miss {
        color: var(--text-muted);
      }
    `,
  ],
  template: `
    @if (page(); as p) {
      <div class="page">
        <article class="body" [innerHTML]="html()"></article>
        @if (p.headings.length > 1) {
          <nav class="onpage">
            <div class="label">{{ lang() === 'fr' ? 'Sur cette page' : 'On this page' }}</div>
            @for (h of p.headings; track h.id) {
              <a href="javascript:void(0)" (click)="jump(h.id)">{{ h.text }}</a>
            }
          </nav>
        }
      </div>
    } @else if (missing()) {
      <p class="miss">
        {{ lang() === 'fr' ? 'Cette page n a pas encore ete ecrite.' : 'This page has not been written yet.' }}
        <a routerLink="/docs">{{ lang() === 'fr' ? 'Revenir au sommaire' : 'Back to the contents' }}</a>
      </p>
    }
  `,
})
export class DocsPageComponent {
  private readonly docs = inject(DocsService);
  private readonly route = inject(ActivatedRoute);
  private readonly sanitizer = inject(DomSanitizer);

  protected readonly lang = this.docs.lang;
  protected readonly page = signal<DocPage | null>(null);
  protected readonly missing = signal(false);
  protected readonly html = computed<SafeHtml>(() =>
    this.sanitizer.bypassSecurityTrustHtml(this.page()?.html ?? ''),
  );

  private readonly slug = signal('');

  // Scrolling is done by hand rather than by an href: with hash routing the
  // whole route lives in location.hash, so "#a-heading" would not move within
  // the page - it would navigate away from it.
  protected jump(id: string): void {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  constructor() {
    this.route.url.subscribe((segments) => this.slug.set(segments.map((s) => s.path).join('/')));
    effect(() => {
      // /docs with nothing after it is the contents page.
      const slug = this.slug() || 'index';
      const lang = this.lang();
      this.missing.set(false);
      this.docs
        .page(lang, slug)
        .then((p) => {
          this.page.set(p);
          // A deep link may name a heading (?at=the-heading). The hash itself
          // is not available for anchors here: it carries the whole route.
          const at = this.route.snapshot.queryParamMap.get('at');
          if (at) setTimeout(() => this.jump(at), 0);
        })
        .catch(() => {
          this.page.set(null);
          this.missing.set(true);
        });
    });
  }
}
