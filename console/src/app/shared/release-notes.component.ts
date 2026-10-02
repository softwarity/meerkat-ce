import { Component, computed, inject } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';

export interface ReleaseNotes {
  version: string;
  dev?: boolean;
  parts?: { title: string; markdown: string }[];
}

// The running version and what it brought (CONSOLE-15), opened from the
// account menu. A development build is shown as the last release it carries,
// and the window opens on what is in development. The gateway picks the
// sections: the minor or major release, then its patches that say something.
@Component({
  selector: 'app-release-notes-dialog',
  imports: [MatButtonModule, MatDialogModule],
  styles: [
    `
      .missing {
        margin: 8px 0;
        color: var(--mat-sys-on-surface-variant);
        font-style: italic;
      }
      .notes {
        line-height: 1.5;
      }
      h3.part {
        margin: 24px 0 4px;
        padding-bottom: 4px;
        border-bottom: 1px solid var(--mat-sys-outline-variant);
        color: var(--mat-sys-primary);
        font-size: 1.05rem;
        font-weight: 600;
      }
      h3.part:first-child {
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
    <h2 mat-dialog-title>
      <ng-container i18n="@@Release_notes_title">Meerkat {{ data.version }}</ng-container>
    </h2>
    <mat-dialog-content>
      @for (p of parts(); track p.title) {
        <h3 class="part">
          @if (p.title === 'NEXT RELEASE') {
            <ng-container i18n="@@Next_release">Next release</ng-container>
          } @else {
            {{ p.title }}
          }
        </h3>
        @if (p.html) {
          <div class="notes" [innerHTML]="p.html"></div>
        } @else {
          <p class="missing" i18n="@@Release_notes_missing">Missing information.</p>
        }
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Close">Close</button>
    </mat-dialog-actions>
  `,
})
export class ReleaseNotesDialogComponent {
  protected readonly data = inject<ReleaseNotes>(MAT_DIALOG_DATA);
  protected readonly parts = computed(() =>
    (this.data.parts ?? []).map((p) => ({ title: p.title, html: renderNotes(p.markdown) })),
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
      // One level under the sections the dialog titles (h3).
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
