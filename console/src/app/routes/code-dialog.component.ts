import { afterNextRender, Component, ElementRef, inject, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { css } from '@codemirror/lang-css';
import { javascript } from '@codemirror/lang-javascript';
import { oneDark } from '@codemirror/theme-one-dark';
import { basicSetup, EditorView } from 'codemirror';

export interface CodeDialogData {
  code: string;
  language: 'css' | 'js';
  // A block with its own job says so, and says what it is handed: a script
  // whose argument nobody names is a script written by guesswork.
  title?: string;
  hint?: string;
}

// Paste-a-snippet editor for a UI route's injected style/script block -
// CodeMirror 6 (modular, tree-shaken) with CSS or JavaScript highlighting.
// This component is LAZY-imported by the route editor so CodeMirror never
// weighs on the initial bundle. Resolves to the edited text on apply,
// undefined on cancel.
@Component({
  selector: 'app-code-dialog',
  imports: [MatButtonModule, MatDialogModule],
  styles: [
    `
      mat-dialog-content {
        padding-top: 8px;
      }
      .hint {
        margin: 0 0 10px;
        max-width: min(720px, 78vw);
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
      }
      .hint code {
        font-family: var(--mat-sys-body-medium-font, monospace);
        background: var(--mat-sys-surface-container-high);
        border-radius: 4px;
        padding: 0 4px;
      }
      .editor {
        width: min(720px, 78vw);
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: var(--mat-sys-corner-medium, 8px);
        overflow: hidden;
      }
      .editor :global(.cm-editor) {
        height: 420px;
      }
    `,
  ],
  template: `
    @if (data.title) {
      <h2 mat-dialog-title>{{ data.title }}</h2>
    } @else if (data.language === 'css') {
      <h2 mat-dialog-title i18n="@@Custom_CSS">Custom CSS</h2>
    } @else {
      <h2 mat-dialog-title i18n="@@Custom_JavaScript">Custom JavaScript</h2>
    }
    <mat-dialog-content>
      @if (data.hint) {
        <p class="hint">{{ data.hint }}</p>
      }
      <div class="editor" #editor></div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <!-- close() explicitly, NOT mat-dialog-close: written bare, that
           directive closes with the empty STRING, which the caller cannot tell
           from "the admin emptied the editor and saved" - so cancelling wiped
           the block that was there. Escape and the backdrop already close with
           undefined; this makes the button agree with them. -->
      <button matButton (click)="ref.close()" i18n="@@Cancel">Cancel</button>
      <button matButton="filled" (click)="apply()" i18n="@@Save">Save</button>
    </mat-dialog-actions>
  `,
})
export class CodeDialogComponent {
  protected readonly data = inject<CodeDialogData>(MAT_DIALOG_DATA);
  protected readonly ref = inject(MatDialogRef<CodeDialogComponent, string | undefined>);
  private readonly editorHost = viewChild.required<ElementRef<HTMLDivElement>>('editor');
  private view?: EditorView;

  constructor() {
    afterNextRender(() => {
      this.view = new EditorView({
        doc: this.data.code ?? '',
        extensions: [
          basicSetup,
          this.data.language === 'css' ? css() : javascript(),
          oneDark,
          EditorView.theme({ '&': { height: '420px' } }),
        ],
        parent: this.editorHost().nativeElement,
      });
      this.view.focus();
    });
  }

  protected apply(): void {
    this.ref.close(this.view?.state.doc.toString() ?? '');
  }
}
