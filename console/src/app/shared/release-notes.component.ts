import { Component, computed, inject } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { MeService } from '../me.service';

export interface ReleaseNotes {
  version: string;
  dev?: boolean;
  parts?: { title: string; markdown: string }[];
}

// The running version and what it brought (CONSOLE-15): a section of Meerkat,
// a page rather than a window - notes are read, scrolled and come back to, and
// a dialog over a screen one was not looking at is no place for that. A
// development build is shown as the last release it carries, and the page
// opens on what is in development. The gateway picks the sections: the minor
// or major release, then its patches that say something.
@Component({
  selector: 'app-release-notes-page',
  imports: [LoadingIndicatorComponent],
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        min-height: 0;
      }
      .banner {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 12px 24px;
        flex: none;
      }
      .banner h1 {
        font-size: 1.15rem;
        font-weight: 500;
        margin: 0;
        flex: 1;
      }
      .content {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        padding: 0 24px 24px;
      }
      .notes-column {
        max-width: 860px;
      }
      .missing {
        margin: 8px 0;
        color: var(--mat-sys-on-surface-variant);
        font-style: italic;
      }
      .notes {
        line-height: 1.5;
      }
      h2.part {
        margin: 24px 0 4px;
        padding-bottom: 4px;
        border-bottom: 1px solid var(--mat-sys-outline-variant);
        color: var(--mat-sys-primary);
        font-size: 1.05rem;
        font-weight: 600;
      }
      h2.part:first-child {
        margin-top: 0;
      }
      .notes :is(h4, h5) {
        margin: 20px 0 6px;
        font-size: 1rem;
        font-weight: 600;
      }
      .notes ul {
        margin: 0;
        padding-left: 20px;
      }
      .notes li {
        margin: 4px 0;
      }
      .notes p {
        margin: 8px 0;
      }
      .notes code {
        font-family: var(--mk-mono, monospace);
        font-size: 0.9em;
      }
    `,
  ],
  template: `
    <div class="banner">
      <h1>
        @if (notes.value(); as n) {
          <ng-container i18n="@@Meerkat_VERSION_EDITION">Meerkat {{ n.version }} {{ enterprise() ? 'EE' : 'CE' }}</ng-container>
        } @else {
          <ng-container i18n="@@Release_notes">Release notes</ng-container>
        }
      </h1>
    </div>
    @if (notes.isLoading()) {
      <loading-indicator withContainer />
    } @else {
      <div class="content">
        <div class="notes-column">
          @for (p of parts(); track p.title) {
            <h2 class="part">
              @if (p.title === 'NEXT RELEASE') {
                <ng-container i18n="@@Next_release">Next release</ng-container>
              } @else {
                {{ p.title }}
              }
            </h2>
            @if (p.html) {
              <div class="notes" [innerHTML]="p.html"></div>
            } @else {
              <p class="missing" i18n="@@Release_notes_missing">Missing information.</p>
            }
          } @empty {
            <p class="missing" i18n="@@Release_notes_missing">Missing information.</p>
          }
        </div>
      </div>
    }
  `,
})
export class ReleaseNotesPageComponent {
  protected readonly notes = httpResource<ReleaseNotes>(() => '/api/release-notes');
  protected readonly enterprise = inject(MeService).enterprise;
  protected readonly parts = computed(() =>
    (this.notes.value()?.parts ?? []).map((p) => ({ title: p.title, html: renderNotes(p.markdown) })),
  );
}

// The notes are written in a small, steady subset of Markdown: headings,
// paragraphs, bullets wrapped over indented lines, bold, code and web links. That much is
// rendered here rather than with a library; everything is escaped first.
export function renderNotes(md: string): string {
  const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const inline = (s: string) =>
    esc(s)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  const out: string[] = [];
  let para: string[] = [];
  let items: string[] = [];
  const flushPara = () => {
    if (para.length) out.push(`<p>${inline(para.join(' '))}</p>`);
    para = [];
  };
  const flushList = () => {
    if (items.length) out.push(`<ul>${items.map((i) => `<li>${inline(i)}</li>`).join('')}</ul>`);
    items = [];
  };
  for (const raw of md.split('\n')) {
    const line = raw.trimEnd();
    const heading = /^(#{3,4})\s+(.*)$/.exec(line);
    if (heading) {
      flushPara();
      flushList();
      // Under the sections the page titles (h2): ### is h4, #### is h5.
      const level = heading[1].length + 1;
      out.push(`<h${level}>${inline(heading[2])}</h${level}>`);
    } else if (/^- /.test(line)) {
      flushPara();
      items.push(line.slice(2));
    } else if (items.length && /^\s+\S/.test(line)) {
      items[items.length - 1] += ' ' + line.trim();
    } else if (!line.trim() || line.trim() === '---') {
      flushPara();
      flushList();
    } else {
      flushList();
      para.push(line.trim());
    }
  }
  flushPara();
  flushList();
  return out.join('');
}
