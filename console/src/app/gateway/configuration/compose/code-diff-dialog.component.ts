import { Component, ElementRef, OnDestroy, afterNextRender, inject, viewChildren } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatButtonModule } from '@angular/material/button';
import { css } from '@codemirror/lang-css';
import { javascript } from '@codemirror/lang-javascript';
import { MergeView } from '@codemirror/merge';
import { EditorState, Extension } from '@codemirror/state';
import { basicSetup, EditorView } from 'codemirror';
import { catppuccin, editorSurface } from '../../../shared/catppuccin';
import { CodePair } from './compose.model';

export interface CodeDiffData {
  title: string;
  leftName: string;
  rightName: string;
  pairs: CodePair[];
}

// The code a field holds on the two sides of a comparison, side by side and
// line by line - the left on the left, as on the loom. A chip says a block
// is there; only the code says how it differs. Read-only: the choice is
// still made on the loom.
@Component({
  selector: 'app-code-diff-dialog',
  imports: [MatDialogModule, MatButtonModule],
  styles: [
    `
      .heads {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 8px;
        font-size: 0.78rem;
        color: var(--mat-sys-on-surface-variant);
        margin: 0 0 6px;
      }
      h3 {
        margin: 16px 0 6px;
        font-size: 0.85rem;
        font-weight: 500;
        font-family: var(--mk-mono, monospace);
      }
      h3:first-of-type {
        margin-top: 0;
      }
      .absent {
        font-style: italic;
        font-weight: 400;
        color: var(--mat-sys-on-surface-variant);
      }
      .merge {
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 8px;
        overflow: hidden;
        max-height: 60vh;
        overflow-y: auto;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>{{ data.title }}</h2>
    <mat-dialog-content>
      <div class="heads">
        <span>{{ data.leftName }}</span><span>{{ data.rightName }}</span>
      </div>
      @for (p of data.pairs; track $index) {
        <h3>
          {{ p.label }}
          @if (p.left === null) {
            <span class="absent">- only in {{ data.rightName }}</span>
          } @else if (p.right === null) {
            <span class="absent">- only in {{ data.leftName }}</span>
          }
        </h3>
        <div class="merge" #merge></div>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Close">Close</button>
    </mat-dialog-actions>
  `,
})
export class CodeDiffDialogComponent implements OnDestroy {
  protected readonly data = inject<CodeDiffData>(MAT_DIALOG_DATA);
  private readonly hosts = viewChildren<ElementRef<HTMLDivElement>>('merge');
  private views: MergeView[] = [];

  constructor() {
    afterNextRender(() => {
      this.hosts().forEach((h, i) => {
        const p = this.data.pairs[i];
        const ext = (): Extension[] => [
          basicSetup,
          p.lang === 'css' ? css() : p.lang === 'js' ? javascript() : [],
          catppuccin,
          editorSurface,
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
        ];
        this.views.push(
          new MergeView({
            a: { doc: p.left ?? '', extensions: ext() },
            b: { doc: p.right ?? '', extensions: ext() },
            parent: h.nativeElement,
            highlightChanges: true,
            gutter: true,
          }),
        );
      });
    });
  }

  ngOnDestroy(): void {
    this.views.forEach((v) => v.destroy());
  }
}
